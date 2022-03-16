// copyright (c) f-secure corporation
// https://foundry.f-secure.com
//
// use of this source code is governed by the license
// that can be found in the license file.

package ums

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/usbarmory/tamago/dma"
	"github.com/usbarmory/tamago/soc/imx6"
	"github.com/usbarmory/tamago/soc/imx6/usb"
)

const maxPacketSize = 512

func (d *Drive) ConfigureUSB() (device *usb.Device) {
	device = &usb.Device{
		Setup: d.setup,
	}

	// Supported Language Code Zero: English
	device.SetLanguageCodes([]uint16{0x0409})

	// device descriptor
	device.Descriptor = &usb.DeviceDescriptor{}
	device.Descriptor.SetDefaults()

	// http://pid.codes/1209/2702/
	device.Descriptor.VendorId = 0x1209
	device.Descriptor.ProductId = 0x2702

	device.Descriptor.Device = 0x0001

	iManufacturer, _ := device.AddString(VendorID)
	device.Descriptor.Manufacturer = iManufacturer

	iProduct, _ := device.AddString(ProductID)
	device.Descriptor.Product = iProduct

	// p9, 4.1.1 Serial Number, USB Mass Storage Class 1.0
	//
	// The serial number format is [0-9A-F]{12,}, the NXP Unique
	// ID is converted accordingly.
	uid := imx6.UniqueID()
	serial := strings.ToUpper(hex.EncodeToString(uid[:]))

	iSerial, _ := device.AddString(serial)
	device.Descriptor.SerialNumber = iSerial

	conf := &usb.ConfigurationDescriptor{}
	conf.SetDefaults()

	device.AddConfiguration(conf)

	// device qualifier
	device.Qualifier = &usb.DeviceQualifierDescriptor{}
	device.Qualifier.SetDefaults()
	device.Qualifier.NumConfigurations = uint8(len(device.Configurations))

	// interface
	iface := &usb.InterfaceDescriptor{}
	iface.SetDefaults()
	iface.NumEndpoints = 2
	// Mass Storage
	iface.InterfaceClass = 0x8
	// SCSI
	iface.InterfaceSubClass = 0x6
	// Bulk-Only
	iface.InterfaceProtocol = 0x50
	iface.Interface = 0

	// EP1 IN endpoint (bulk)
	ep1IN := &usb.EndpointDescriptor{}
	ep1IN.SetDefaults()
	ep1IN.EndpointAddress = 0x81
	ep1IN.Attributes = 2
	ep1IN.MaxPacketSize = maxPacketSize
	ep1IN.Zero = false
	ep1IN.Function = d.tx

	iface.Endpoints = append(iface.Endpoints, ep1IN)

	// EP2 OUT endpoint (bulk)
	ep1OUT := &usb.EndpointDescriptor{}
	ep1OUT.SetDefaults()
	ep1OUT.EndpointAddress = 0x01
	ep1OUT.Attributes = 2
	ep1OUT.MaxPacketSize = maxPacketSize
	ep1OUT.Zero = false
	ep1OUT.Function = d.rx

	iface.Endpoints = append(iface.Endpoints, ep1OUT)

	device.Configurations[0].AddInterface(iface)

	return
}

// setup handles the class specific control requests specified at
// p7, 3.1 - 3.2, USB Mass Storage Class 1.0
func (d *Drive) setup(setup *usb.SetupData) (in []byte, ack bool, done bool, err error) {
	switch setup.Request {
	case usb.BULK_ONLY_MASS_STORAGE_RESET:
		// For we ack this request without resetting.
	case usb.GET_MAX_LUN:
		in = []byte{0x00}
	}

	return
}

func parseCBW(buf []byte) (cbw *usb.CBW, err error) {
	if len(buf) == 0 {
		return
	}

	if len(buf) != usb.CBW_LENGTH {
		return nil, fmt.Errorf("invalid CBW size %d != %d", len(buf), usb.CBW_LENGTH)
	}

	cbw = &usb.CBW{}
	err = binary.Read(bytes.NewReader(buf), binary.LittleEndian, cbw)

	if err != nil {
		return
	}

	if cbw.Length < 6 || cbw.Length > usb.CBW_CB_MAX_LENGTH {
		return nil, fmt.Errorf("invalid Command Block Length %d", cbw.Length)
	}

	if cbw.Signature != usb.CBW_SIGNATURE {
		return nil, fmt.Errorf("invalid CBW signature %x", cbw.Signature)
	}

	return
}

func (d *Drive) rx(buf []byte, lastErr error) (res []byte, err error) {
	var cbw *usb.CBW

	if d.dataPending != nil {
		defer dma.Release(d.dataPending.addr)
		err = d.handleWrite()

		if err != nil {
			return
		}

		csw := d.dataPending.csw
		csw.DataResidue = 0

		d.send <- d.dataPending.csw.Bytes()

		d.dataPending = nil

		return
	}

	cbw, err = parseCBW(buf)

	if err != nil {
		return
	}

	csw, data, err := d.handleCDB(cbw.CommandBlock, cbw)

	defer func() {
		if csw != nil {
			d.send <- csw.Bytes()
		}
	}()

	if err != nil {
		csw.DataResidue = cbw.DataTransferLength
		csw.Status = usb.CSW_STATUS_COMMAND_FAILED
		return
	}

	if len(data) > 0 {
		d.send <- data
	}

	if d.dataPending != nil {
		d.dataPending.addr, d.dataPending.buf = dma.Reserve(d.dataPending.size, usb.DTD_PAGE_SIZE)
		res = d.dataPending.buf
	}

	return
}

func (d *Drive) tx(_ []byte, lastErr error) (in []byte, err error) {
	select {
	case buf := <-d.free:
		dma.Release(buf)
	default:
	}

	in = <-d.send

	if reserved, addr := dma.Reserved(in); reserved {
		d.free <- addr
	}

	return
}
