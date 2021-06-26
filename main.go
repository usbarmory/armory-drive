// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/f-secure-foundry/tamago/dma"
	"github.com/f-secure-foundry/tamago/soc/imx6"
	"github.com/f-secure-foundry/tamago/soc/imx6/usb"

	"github.com/f-secure-foundry/armory-drive/internal/ble"
	"github.com/f-secure-foundry/armory-drive/internal/hab"
	"github.com/f-secure-foundry/armory-drive/internal/remote"
	"github.com/f-secure-foundry/armory-drive/internal/pairing"

	"github.com/f-secure-foundry/tamago/board/f-secure/usbarmory/mark-two"
)

// initialized at compile time (see Makefile)
var Revision string

var session = &remote.Session{}

var pairingComplete = make(chan bool)

func init() {
	if err := imx6.SetARMFreq(900); err != nil {
		panic(fmt.Sprintf("WARNING: error setting ARM frequency: %v\n", err))
	}

	log.SetFlags(0)
}

func main() {
	var pairing bool

	usbarmory.LED("blue", false)
	usbarmory.LED("white", false)

	err := usbarmory.MMC.Detect()

	if err != nil {
		panic(err)
	}

	err = keyring.Init(false)

	if err != nil {
		panic(err)
	}

	err = detect(usbarmory.SD)

	if err != nil {
		pairing = true
	}

	b := ble.Start(handleEnvelope)
	session.PeerName = b.Name

	if pairing {
		// Secure Boot provisioning as required
		hab.Init()

		session.PairingMode = true
		session.PairingNonce = binary.BigEndian.Uint64(rng(8))

		pairingMode()
	}

	device := &usb.Device{
		Setup: setup,
	}
	configureDevice(device)

	iface := buildMassStorageInterface()
	device.Configurations[0].AddInterface(iface)

	dma.Init(dmaStart, dmaSize)

	usb.USB1.Init()
	usb.USB1.DeviceMode()

	// To further reduce the attack surface, start the USB stack only when
	// the card is unlocked (or in pairing mode).
	if !ready {
		usb.USB1.Stop()

		for !ready {
			runtime.Gosched()
			time.Sleep(10 * time.Millisecond)
		}

		usb.USB1.Run()
	}

	usb.USB1.Reset()

	// never returns
	usb.USB1.Start(device)
}

func pairingMode() {
	code, err := newPairingCode()

	if err != nil {
		panic(err)
	}

	cards = append(cards, pairing.Disk(code, Revision))
	ready = true

	go func() {
		var on bool

		for {
			select {
			case <-pairingComplete:
				usbarmory.LED("blue", false)
				return
			default:
			}

			on = !on
			usbarmory.LED("blue", on)

			runtime.Gosched()
			time.Sleep(1 * time.Second)
		}
	}()
}
