// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package ums

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"sync"

	"github.com/usbarmory/armory-drive/internal/ota"

	"github.com/usbarmory/tamago/dma"
	"github.com/usbarmory/tamago/soc/imx6/usb"

	"golang.org/x/sync/errgroup"
)

const (
	// p65, 3. Direct Access Block commands (SPC-5 and SBC-4), SCSI Commands Reference Manual, Rev. J
	TEST_UNIT_READY  = 0x00
	REQUEST_SENSE    = 0x03
	INQUIRY          = 0x12
	MODE_SENSE_6     = 0x1a
	START_STOP_UNIT  = 0x1b
	MODE_SENSE_10    = 0x5a
	READ_CAPACITY_10 = 0x25
	READ_10          = 0x28
	WRITE_10         = 0x2a
	REPORT_LUNS      = 0xa0

	// service actions
	SERVICE_ACTION   = 0x9e
	READ_CAPACITY_16 = 0x10

	// 04-349r1 SPC-3 MMC-5 Merge PREVENT ALLOW MEDIUM REMOVAL commands
	PREVENT_ALLOW_MEDIUM_REMOVAL = 0x1e

	// p33, 4.10, USB Mass Storage Class – UFI Command Specification Rev. 1.0
	READ_FORMAT_CAPACITIES = 0x23

	// To speed up FDE it is beneficial to report a larger block size, to
	// reduce the number of encryption/decryption iterations caused by
	// per-block IV computation.
	BLOCK_SIZE_MULTIPLIER = 8

	// These parameters control how many blocks are read/written before
	// being offloaded to DCP for decryption/encryption in a goroutine.
	//
	// Values should be tuned for optimum pipeline performance, to minimize
	// overhead while the DCP works in parallel with the next batch of
	// uSDHC read/write.
	READ_PIPELINE_SIZE  = 12
	WRITE_PIPELINE_SIZE = 20
)

type writeOp struct {
	csw    *usb.CSW
	lba    int
	blocks int
	size   int
	addr   uint32
	buf    []byte
}

// p94, 3.6.2 Standard INQUIRY data, SCSI Commands Reference Manual, Rev. J
func (d *Drive) inquiry(length int) (data []byte) {
	data = make([]byte, 5)

	// device connected, direct access block device
	data[0] = 0x00

	if !d.Ready {
		// device not connected
		data[0] |= (0b001 << 5)
	}

	// Removable Media
	data[1] = 0x80
	// SPC-3 compliant
	data[2] = 0x05
	// response data format (only 2 is allowed)
	data[3] = 0x02
	// additional length
	data[4] = byte(length - 5)

	// unused or obsolete flags
	data = append(data, make([]byte, 3)...)

	data = append(data, []byte(VendorID)...)
	data = append(data, []byte(ProductID)...)
	data = append(data, []byte(ProductRevision)...)

	if length > len(data) {
		// pad up to requested transfer length
		data = append(data, make([]byte, length-len(data))...)
	} else {
		data = data[0:length]
	}

	return
}

// p56, 2.4.1.2 Fixed format sense data, SCSI Commands Reference Manual, Rev. J
func (d *Drive) sense(length int) (data []byte, err error) {
	data = make([]byte, 18)

	if !d.Ready {
		// sense key: NOT READY
		data[2] = 0x02
		// additional sense code: MEDIUM NOT PRESENT
		data[12] = 0x3a
	}

	// error code
	data[0] = 0x70
	// additional sense length
	data[7] = byte(len(data) - 1 - 7)

	if length < len(data) {
		return nil, fmt.Errorf("unsupported REQUEST_SENSE transfer length %d > %d", length, len(data))
	}

	return
}

// p111, 3.11 MODE SENSE(6) command, SCSI Commands Reference Manual, Rev. J
func modeSense(length int) (data []byte, err error) {
	// Unsupported, an empty response is returned on all requests.
	data = make([]byte, length)

	// p378, 5.3.3 Mode parameter header formats, SCSI Commands Reference Manual, Rev. J
	data[0] = byte(length)

	return
}

// p179, 3.33 REPORT LUNS command, SCSI Commands Reference Manual, Rev. J
func reportLUNs(length int) (data []byte, err error) {
	buf := new(bytes.Buffer)
	luns := 1

	binary.Write(buf, binary.BigEndian, uint32(luns*8))
	buf.Write(make([]byte, 4))

	for lun := 0; lun < luns; lun++ {
		// The information conforms to the Logical Unit Address Method defined
		// in SCC-2, and supports only First Level addressing (for each LUN,
		// only the second byte is used and contains the assigned LUN)."
		buf.WriteByte(0x00)
		binary.Write(buf, binary.BigEndian, uint8(lun))
		buf.Write(make([]byte, 6))
	}

	data = buf.Bytes()

	if length < buf.Len() {
		data = data[0:length]
	}

	return
}

// p155, 3.22 READ CAPACITY (10) command, SCSI Commands Reference Manual, Rev. J
func (d *Drive) readCapacity10() (data []byte, err error) {
	info := d.card.Info()

	if info.Blocks <= 0 {
		return nil, fmt.Errorf("invalid block count %d", info.Blocks)
	}

	blocks := uint32(info.Blocks / d.Mult)
	blockSize := uint32(info.BlockSize * d.Mult)

	buf := new(bytes.Buffer)

	binary.Write(buf, binary.BigEndian, blocks-1)
	binary.Write(buf, binary.BigEndian, blockSize)

	return buf.Bytes(), nil
}

// p157, 3.23 READ CAPACITY (16) command, SCSI Commands Reference Manual, Rev. J
func (d *Drive) readCapacity16(length int) (data []byte, err error) {
	info := d.card.Info()
	buf := new(bytes.Buffer)

	if info.Blocks <= 0 {
		return nil, fmt.Errorf("invalid block count %d", info.Blocks)
	}

	binary.Write(buf, binary.BigEndian, uint64(info.Blocks)-1)
	binary.Write(buf, binary.BigEndian, uint64(info.BlockSize))
	buf.Grow(32 - buf.Len())

	data = buf.Bytes()

	if length < buf.Len() {
		data = data[0:length]
	}

	return
}

// p33, 4.10, USB Mass Storage Class – UFI Command Specification Rev. 1.0
func (d *Drive) readFormatCapacities() (data []byte, err error) {
	info := d.card.Info()

	blocks := uint32(info.Blocks / d.Mult)
	blockSize := uint32(info.BlockSize * d.Mult)

	buf := new(bytes.Buffer)

	// capacity list length
	binary.Write(buf, binary.BigEndian, uint32(8))
	// number of blocks
	binary.Write(buf, binary.BigEndian, blocks)
	// descriptor code: formatted media | block length
	binary.Write(buf, binary.BigEndian, uint32(0b10<<24|blockSize&0xffffff))

	return buf.Bytes(), nil
}

func (d *Drive) read(lba int, blocks int) (err error) {
	batch := READ_PIPELINE_SIZE
	info := d.card.Info()

	blockSize := info.BlockSize * d.Mult

	if !d.Ready {
		d.send <- make([]byte, blocks*blockSize)
		return
	}

	addr, buf := dma.Reserve(blocks*blockSize, usb.DTD_PAGE_SIZE)

	wg := &sync.WaitGroup{}

	for i := 0; i < blocks; i += batch {
		if i+batch > blocks {
			batch = blocks - i
		}

		start := i * blockSize
		end := start + blockSize*batch
		slice := buf[start:end]

		err = d.card.ReadBlocks((lba+i)*d.Mult, slice)

		if err != nil {
			dma.Release(addr)
			return
		}

		if d.Cipher {
			wg.Add(1)
			go d.Keyring.Cipher(slice, lba+i, batch, blockSize, false, wg)
		}
	}

	wg.Wait()
	d.send <- buf

	return
}

func (d *Drive) write(lba int, buf []byte) (err error) {
	batch := WRITE_PIPELINE_SIZE
	info := d.card.Info()

	blockSize := info.BlockSize * d.Mult
	blocks := len(buf) / blockSize

	if !d.Ready {
		return
	}

	eg := &errgroup.Group{}

	for i := 0; i < blocks; i += batch {
		if i+batch > blocks {
			batch = blocks - i
		}

		start := i * blockSize
		end := start + blockSize*batch
		slice := buf[start:end]

		if d.Cipher {
			d.Keyring.Cipher(slice, lba+i, batch, blockSize, true, nil)
		}

		sliceBlock := (lba + i) * d.Mult

		eg.Go(func() error {
			return d.card.WriteBlocks(sliceBlock, slice)
		})
	}

	return eg.Wait()
}

func (d *Drive) handleCDB(cmd [16]byte, cbw *usb.CBW) (csw *usb.CSW, data []byte, err error) {
	op := cmd[0]
	length := int(cbw.DataTransferLength)

	// p8, 3.3 Host/Device Packet Transfer Order, USB Mass Storage Class 1.0
	csw = &usb.CSW{Tag: cbw.Tag}
	csw.SetDefaults()

	lun := int(cbw.LUN)

	if int(lun+1) > 1 {
		err = fmt.Errorf("invalid LUN")
		return
	}

	switch op {
	case TEST_UNIT_READY:
		if !d.Ready {
			csw.Status = usb.CSW_STATUS_COMMAND_FAILED
		}
	case INQUIRY:
		data = d.inquiry(length)
	case REQUEST_SENSE:
		data, err = d.sense(length)
	case START_STOP_UNIT:
		start := (cmd[4]&1 == 1)

		if !d.Ready && start {
			// locked drive cannot be started
			csw.Status = usb.CSW_STATUS_COMMAND_FAILED
			// lock drive at eject
		} else if d.Ready && !start && d.Cipher {
			d.Lock()
		} else {
			d.Ready = start
		}

		if !d.Ready && !d.Cipher {
			d.PairingComplete <- true

			go func() {
				card := d.card.(*PairingDisk)
				ota.Check(card.Data, pairingDiskPath, pairingDiskOffset, d.Keyring)
			}()
		}
	case MODE_SENSE_6, MODE_SENSE_10:
		data, err = modeSense(length)
	case REPORT_LUNS:
		data, err = reportLUNs(length)
	case READ_FORMAT_CAPACITIES:
		data, err = d.readFormatCapacities()
	case READ_CAPACITY_10:
		data, err = d.readCapacity10()
	case READ_10, WRITE_10:
		if !d.Ready {
			csw.Status = usb.CSW_STATUS_COMMAND_FAILED
		}

		lba := int(binary.BigEndian.Uint32(cmd[2:]))
		blocks := int(binary.BigEndian.Uint16(cmd[7:]))

		if op == READ_10 {
			err = d.read(lba, blocks)
		} else {
			blockSize := d.card.Info().BlockSize * d.Mult
			size := int(cbw.DataTransferLength)

			if blockSize*blocks != size {
				err = fmt.Errorf("unexpected %d blocks write transfer length (%d)", blocks, size)
			}

			d.dataPending = &writeOp{
				csw:    csw,
				lba:    lba,
				blocks: blocks,
				size:   size,
			}

			csw = nil
		}
	case SERVICE_ACTION:
		switch cmd[1] {
		case READ_CAPACITY_16:
			data, err = d.readCapacity16(length)
		default:
			err = fmt.Errorf("unsupported service action %#x %+v", op, cbw)
		}
	case PREVENT_ALLOW_MEDIUM_REMOVAL:
		// ignored events
	default:
		err = fmt.Errorf("unsupported CDB Operation Code %#x %+v", op, cbw)
	}

	return
}

func (d *Drive) handleWrite() (err error) {
	if len(d.dataPending.buf) != d.dataPending.size {
		return fmt.Errorf("len(buf) != size (%d != %d)", len(d.dataPending.buf), d.dataPending.size)
	}

	return d.write(d.dataPending.lba, d.dataPending.buf)
}
