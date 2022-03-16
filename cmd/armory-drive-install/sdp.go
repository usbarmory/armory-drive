// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"errors"
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/usbarmory/armory-boot/sdp"

	"github.com/usbarmory/hid"
)

const (
	// USB vendor ID for all supported devices
	FreescaleVendorID = 0x15a2

	// On-Chip RAM (OCRAM/iRAM) address for payload staging
	iramOffset = 0x00910000

	// USB command timeout in seconds
	timeout = 10
)

// This tool should work with all SoCs from the i.MX series capable of USB HID
// based SDP, only tested devices are listed as supported, Pull Requests are
// welcome to expand this set.
var supportedDevices = map[uint16]string{
	0x007d: "Freescale SemiConductor Inc  SE Blank 6UL",
	0x0080: "Freescale SemiConductor Inc  SE Blank 6ULL",
}

// detect compatible devices in SDP mode
func detect() (err error) {
	devices, err := hid.Devices()

	if err != nil {
		return
	}

	for _, d := range devices {
		if d.VendorID != FreescaleVendorID {
			continue
		}

		if product, ok := supportedDevices[d.ProductID]; ok {
			log.Printf("Found device %04x:%04x %s", d.VendorID, d.ProductID, product)
		} else {
			continue
		}

		conf.dev, err = d.Open()

		if err != nil {
			return
		}

		break
	}

	if conf.dev == nil {
		return errors.New("no device found, target missing or invalid permissions (forgot admin shell?)")
	}

	return
}

func sendHIDReport(n int, buf []byte, wait int) (res []byte, err error) {
	err = conf.dev.Write(append([]byte{byte(n)}, buf...))

	if err != nil || wait < 0 {
		return
	}

	ok := false
	timer := time.After(time.Duration(timeout) * time.Second)

	for {
		select {
		case res, ok = <-conf.dev.ReadCh():
			if !ok {
				return nil, errors.New("error reading response")
			}

			if len(res) > 0 && res[0] == byte(wait) {
				return
			}
		case <-timer:
			return nil, errors.New("command timeout")
		}
	}
}

func dcdWrite(dcd []byte, addr uint32) (err error) {
	r1, r2 := sdp.BuildDCDWriteReport(dcd, addr)

	_, err = sendHIDReport(1, r1, -1)

	if err != nil {
		return
	}

	_, err = sendHIDReport(2, r2, 4)

	return
}

func fileWrite(imx []byte, addr uint32) (err error) {
	r1, r2 := sdp.BuildFileWriteReport(imx, addr)

	_, err = sendHIDReport(1, r1, -1)

	if err != nil {
		return
	}

	wait := -1
	timer := time.After(time.Duration(timeout) * time.Second)

	for i, r := range r2 {
		if i == len(r2)-1 {
			wait = 4
		}
	send:
		_, err = sendHIDReport(2, r, wait)

		if err != nil && runtime.GOOS == "darwin" && err.Error() == "hid: general error" {
			// On macOS access contention with the OS causes
			// errors, as a workaround we retry from the transfer
			// that got caught up.
			select {
			case <-timer:
				return
			default:
				off := uint32(i) * 1024
				r1 := &sdp.SDP{
					CommandType: sdp.WriteFile,
					Address:     addr + off,
					DataCount:   uint32(len(imx)) - off,
				}

				if _, err = sendHIDReport(1, r1.Bytes(), -1); err != nil {
					return
				}

				goto send
			}
		}

		if err != nil {
			break
		}
	}

	return
}

func jumpAddress(addr uint32) (err error) {
	r1 := sdp.BuildJumpAddressReport(addr)
	_, err = sendHIDReport(1, r1, -1)

	return
}

func imxLoad(imx []byte) (err error) {
	for {
		if err = detect(); err != nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		break
	}

	ivt, err := sdp.ParseIVT(imx)

	if err != nil {
		return fmt.Errorf("IVT parsing error: %v", err)
	}

	dcd, err := sdp.ParseDCD(imx, ivt)

	if err != nil {
		return fmt.Errorf("DCD parsing error: %v", err)
	}

	log.Printf("loading DCD at %#08x (%d bytes)", iramOffset, len(dcd))
	err = dcdWrite(dcd, iramOffset)

	if err != nil {
		return
	}

	log.Printf("loading imx to %#08x (%d bytes)", ivt.Self, len(imx))
	err = fileWrite(imx, ivt.Self)

	if err != nil {
		return
	}

	log.Printf("jumping to %#08x", ivt.Self)
	err = jumpAddress(ivt.Self)

	if err != nil {
		return
	}

	return
}
