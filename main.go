// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"runtime"
	"time"

	"github.com/f-secure-foundry/tamago/dma"
	"github.com/f-secure-foundry/tamago/soc/imx6"
	"github.com/f-secure-foundry/tamago/soc/imx6/usb"

	"github.com/f-secure-foundry/tamago/board/f-secure/usbarmory/mark-two"
)

func init() {
	if err := imx6.SetARMFreq(900); err != nil {
		panic(fmt.Sprintf("WARNING: error setting ARM frequency: %v\n", err))
	}
}

func main() {
	var pairing bool

	usbarmory.LED("blue", false)
	usbarmory.LED("white", false)

	err := usbarmory.MMC.Detect()

	if err != nil {
		panic(err)
	}

	// Secure Boot provisioning as required
	initializeHAB()

	err = keyring.Init(false)

	if err != nil {
		panic(err)
	}

	err = detect(usbarmory.SD)

	if err != nil {
		pairing = true
	}

	startBLE(true)

	if pairing {
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
