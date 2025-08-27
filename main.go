// Copyright (c) The armory-drive authors. All Rights Reserved.
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/usbarmory/armory-drive/internal/ble"
	"github.com/usbarmory/armory-drive/internal/crypto"
	"github.com/usbarmory/armory-drive/internal/hab"
	"github.com/usbarmory/armory-drive/internal/ums"

	"github.com/usbarmory/tamago/arm"
	"github.com/usbarmory/tamago/soc/nxp/imx6ul"
	"github.com/usbarmory/tamago/soc/nxp/usb"

	usbarmory "github.com/usbarmory/tamago/board/usbarmory/mk2"
)

func init() {
	if err := imx6ul.SetARMFreq(900); err != nil {
		panic(fmt.Sprintf("WARNING: error setting ARM frequency: %v\n", err))
	}

	imx6ul.DCP.Init()
	imx6ul.DCP.EnableInterrupt()

	log.SetFlags(0)
}

func startInterruptHandler(port *usb.USB) {
	irq := imx6ul.GIC.GetInterrupt(true)

	imx6ul.GIC.EnableInterrupt(port.IRQ, true)
	imx6ul.GIC.EnableInterrupt(imx6ul.DCP.IRQ, true)

	isr := func() {
		switch irq {
		case port.IRQ:
			port.ServiceInterrupts()
		case imx6ul.DCP.IRQ:
			imx6ul.DCP.ServiceInterrupt()
		default:
			log.Printf("internal error, unexpected IRQ %d", irq)
		}
	}

	arm.ServiceInterrupts(isr)
}

func main() {
	usbarmory.LED("blue", false)
	usbarmory.LED("white", false)

	if err := usbarmory.MMC.Detect(); err != nil {
		log.Fatal(err)
	}

	keyring := &crypto.Keyring{}

	if err := keyring.Init(false); err != nil {
		log.Fatal(err)
	}

	drive := &ums.Drive{
		Cipher:  true,
		Keyring: keyring,
		Mult:    ums.BLOCK_SIZE_MULTIPLIER,
	}

	ble := &ble.BLE{
		Drive:   drive,
		Keyring: keyring,
	}
	ble.Init()

	if drive.Init(usbarmory.SD) != nil {
		var code []byte
		var err error

		// provision Secure Boot as required
		hab.Init()

		// Do not offer pairing code on first time installs (or
		// recovery) as that pairing might become invalid at reboot if
		// Secure Boot has been just activated, rather offer pairing
		// only by firmware booted internally.
		if !imx6ul.SDP {
			code, err = ble.PairingMode()

			if err != nil {
				log.Fatal(err)
			}
		}

		drive.Cipher = false
		drive.Mult = 1
		drive.Ready = true

		drive.Init(ums.Pairing(code, keyring))

		go pairingFeedback(drive.PairingComplete)
	}

	port := imx6ul.USB1

	port.Device = drive.ConfigureUSB()
	port.Init()

	// To further reduce the attack surface, start the USB stack only when
	// the card is unlocked (or in pairing mode).
	for !drive.Ready {
		runtime.Gosched()
		time.Sleep(10 * time.Millisecond)
	}

	port.DeviceMode()

	port.EnableInterrupt(usb.IRQ_URI) // reset
	port.EnableInterrupt(usb.IRQ_PCI) // port change detect
	port.EnableInterrupt(usb.IRQ_UI)  // transfer completion

	startInterruptHandler(port)
}

func pairingFeedback(done chan bool) {
	var on bool

	for {
		select {
		case <-done:
			usbarmory.LED("blue", false)
			return
		default:
		}

		on = !on
		usbarmory.LED("blue", on)

		runtime.Gosched()
		time.Sleep(1 * time.Second)
	}
}
