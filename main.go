// Copyright (c) WithSecure Corporation
// https://foundry.withsecure.com
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

	"github.com/usbarmory/tamago/soc/nxp/imx6ul"

	usbarmory "github.com/usbarmory/tamago/board/usbarmory/mk2"
)

func init() {
	if err := imx6ul.SetARMFreq(900); err != nil {
		panic(fmt.Sprintf("WARNING: error setting ARM frequency: %v\n", err))
	}

	log.SetFlags(0)
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

	device := drive.ConfigureUSB()

	imx6ul.USB1.Init()
	imx6ul.USB1.DeviceMode()

	// To further reduce the attack surface, start the USB stack only when
	// the card is unlocked (or in pairing mode).
	if !drive.Ready {
		imx6ul.USB1.Stop()

		for !drive.Ready {
			runtime.Gosched()
			time.Sleep(10 * time.Millisecond)
		}

		imx6ul.USB1.Run()
	}

	imx6ul.USB1.Reset()
	imx6ul.USB1.Start(device)
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
