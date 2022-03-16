// copyright (c) f-secure corporation
// https://foundry.f-secure.com
//
// use of this source code is governed by the license
// that can be found in the license file.

package ums

import (
	"github.com/usbarmory/armory-drive/api"
	"github.com/usbarmory/armory-drive/internal/crypto"

	"github.com/usbarmory/tamago/board/f-secure/usbarmory/mark-two"
	"github.com/usbarmory/tamago/soc/imx6/usdhc"
)

const (
	// exactly 8 bytes required
	VendorID = "F-Secure"
	// exactly 16 bytes required
	ProductID = "USB armory Mk II"
	// exactly 4 bytes required
	ProductRevision = "1.00"
)

type Card interface {
	Detect() error
	Info() usdhc.CardInfo
	ReadBlocks(int, []byte) error
	WriteBlocks(int, []byte) error
}

// Drive represents an encrypted drive instance.
type Drive struct {
	// Cipher controls whether FDE should be applied
	Cipher bool

	// Keyring instance
	Keyring *crypto.Keyring

	// Ready represents the logical device status
	Ready bool

	// PairingComplete signals pairing completion
	PairingComplete chan bool

	// Mult is the block multiplier
	Mult int

	// Card represents the underlying storage instance
	card Card

	// send is the queue for IN device responses
	send chan []byte

	// free is the queue for IN device DMA buffers for later release
	free chan uint32

	// dataPending is the buffer for write commands which spawn across
	// multiple USB transfers
	dataPending *writeOp
}

func (d *Drive) Init(card Card) (err error) {
	if err = card.Detect(); err != nil {
		return
	}

	d.card = card
	d.PairingComplete = make(chan bool)
	d.send = make(chan []byte, 2)
	d.free = make(chan uint32, 1)

	return
}

func (d *Drive) Capacity() uint64 {
	info := d.card.Info()
	return uint64(info.Blocks) * uint64(info.BlockSize)
}

func (d *Drive) Lock() (err error) {
	// invalidate the drive
	d.Ready = false

	// clear FDE key
	if err = d.Keyring.SetCipher(api.Cipher_NONE, nil); err != nil {
		return
	}

	usbarmory.LED("white", false)

	return
}
