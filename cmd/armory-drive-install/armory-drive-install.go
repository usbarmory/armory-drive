// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"

	"github.com/flynn/hid"
)

type Mode int

type Config struct {
	releaseVersion string
	table          string
	tableHash      string
	srkKey         string
	srkCrt         string
	index          int

	dev hid.Device
}

var conf *Config

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stdout)

	conf = &Config{}

	flag.StringVar(&conf.releaseVersion, "r", "latest", "release version")

	flag.StringVar(&conf.srkKey, "C", "", "SRK private key in PEM format")
	flag.StringVar(&conf.srkCrt, "c", "", "SRK public  key in PEM format")
	flag.StringVar(&conf.table, "t", "", "SRK table")
	flag.StringVar(&conf.tableHash, "T", "", "SRK table hash")
	flag.IntVar(&conf.index, "x", -1, "Index for SRK key")
}

func confirm(msg string) bool {
	var res string

	fmt.Printf("\n%s (y/n): ", msg)
	fmt.Scanln(&res)

	return res == "y"
}

func prompt(msg string) (res string) {
	fmt.Printf("\n%s: ", msg)
	fmt.Scanln(&res)
	return
}

func main() {
	flag.Parse()

	log.Println(welcome)

	switch {
	case confirm("Are you installing Armory Drive for the first time on the target USB armory?"):
		install()
	case confirm("Are you upgrading Armory Drive on a USB armory already running Armory Drive firmware?"):
		upgrade()
	// TODO: recovery mode
	default:
		log.Fatal("Goodbye")
	}
}

func install() {
	log.Println(secureBootNotice)

	if confirm("Would you like to use unsigned releases, *without enabling* Secure Boot on the USB armory?") {
		installFirmware(unsigned)
		return
	}

	if !confirm("Would you like to *permanently enable* Secure Boot on the USB armory?") {
		log.Fatal("Goodbye")
	}

	switch {
	case confirm("Would you like to use F-Secure signed releases, enabling Secure Boot on the USB armory with permanent fusing of F-Secure public keys?"):
		installFirmware(signedByFSecure)
	case confirm("Would you like to sign releases on your own, enabling Secure Boot on the USB armory with your own public keys?"):
		checkHABArguments()
		installFirmware(signedByUser)
	default:
		log.Fatal("Goodbye")
	}
}

func upgrade() {
	if !confirm("Is Secure Boot enabled on your USB armory?") {
		upgradeFirmware(unsigned)
		return
	}

	if confirm("Is Secure Boot enabled on your USB armory using F-Secure signing keys?") {
		upgradeFirmware(signedByFSecure)
	} else {
		checkHABArguments()
		upgradeFirmware(signedByUser)
	}
}

func ota(assets *releaseAssets) {
	log.Printf("\nWait for the USB armory blue LED to blink to indicate pairing mode.\nAn F-Secure drive should appear on your system.")
	mountPoint := prompt("Please specify the path of the mounted F-Secure drive")

	// append HAB signature
	imx := append(assets.imx, assets.csf...)
	// prepend OTA signature
	imx = append(assets.sig, imx...)

	log.Printf("\nCopying firmware to USB armory in pairing mode at %s", mountPoint)
	if err := os.WriteFile(path.Join(mountPoint, OTAName), imx, 0600); err != nil {
		log.Fatal(err)
	}

	log.Printf("\nCopied %d bytes to %s", len(imx), path.Join(mountPoint, OTAName))

	log.Printf("\nPlease eject the drive mounted at %s to flash the firmware and wait for the white LED to turn on and then off for the update to complete.", mountPoint)
	log.Printf("Once the update is complete unplug the USB armory and set eMMC boot mode (see https://github.com/f-secure-foundry/armory-drive#firmware-update for instructions).")

	log.Printf("\nAfter doing so you can use your new Armory Drive installation, following this tutorial:")
	log.Printf("  https://github.com/f-secure-foundry/armory-drive/wiki/Tutorial")
}

func installFirmware(mode Mode) {
	assets, err := downloadRelease(conf.releaseVersion)

	if err != nil {
		log.Fatalf("Download error, %v", err)
	}

	switch mode {
	case unsigned:
		log.Println(unsignedFirmwareWarning)

		if !confirm("Proceed?") {
			log.Fatal("Goodbye")
		}
	case signedByFSecure:
		log.Println(fscSignedFirmwareWarning)

		if !confirm("Proceed?") {
			log.Fatal("Goodbye")
		}

		assets.imx = fixupSRKHash(assets.imx, assets.srk)
	case signedByUser:
		log.Println(userSignedFirmwareWarning)

		if !confirm("Proceed?") {
			log.Fatal("Goodbye")
		}

		if assets.srk, err = os.ReadFile(conf.tableHash); err != nil {
			log.Fatal(err)
		}

		assets.imx = fixupSRKHash(assets.imx, assets.srk)

		// On user signed releases we disable OTA authentication to
		// simplify key management. This has no security impact as the
		// executable is authenticated at boot using secure boot.
		assets.sig = nil
		assets.imx = clearOTAPublicKey(assets.imx)

		if assets.csf, err = sign(assets.imx, false); err != nil {
			log.Fatal(err)
		}
	default:
		log.Fatal("invalid installation mode")
	}

	log.Printf("\nFollow instructions at https://github.com/f-secure-foundry/usbarmory/wiki/Boot-Modes-(Mk-II)")
	log.Printf("to set the target USB armory in SDP mode.")

	if !confirm("Confirm that the target USB armory is plugged to this computer in SDP mode.") {
		log.Fatal("Goodbye")
	}

	if err = imxLoad(assets.imx); err != nil {
		log.Fatal(err)
	}

	ota(assets)
}

func upgradeFirmware(mode Mode) {
	assets, err := downloadRelease(conf.releaseVersion)

	if err != nil {
		log.Fatalf("Download error, %v", err)
	}

	log.Printf("\nFollow instructions at https://github.com/f-secure-foundry/armory-drive#firmware-update")
	log.Printf("to set the loaded Armory Drive firmware in pairing mode.")

	if !confirm("Confirm that target USB armory is plugged to this computer in pairing mode.") {
		log.Fatal("Goodbye")
	}

	if mode == signedByUser {
		// On user signed releases we disable OTA authentication to
		// simplify key management. This has no security impact as the
		// executable is authenticated at boot using secure boot.
		assets.sig = nil
		assets.imx = clearOTAPublicKey(assets.imx)
	}

	ota(assets)
}
