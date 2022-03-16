// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"archive/zip"
	"bytes"
	"flag"
	"fmt"
	"log"
	"os"
	"path"

	"github.com/usbarmory/hid"

	"github.com/usbarmory/armory-drive/assets"
)

type Mode int

type Config struct {
	branch   string
	release  string
	install  bool
	upgrade  int
	recovery bool

	logPublicKey string
	frPublicKey  string
	logOrigin    string

	table     string
	tableHash string
	srkKey    string
	srkCrt    string
	index     int

	dev hid.Device
}

var conf *Config

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stdout)

	conf = &Config{}

	flag.Usage = func() {
		fmt.Println(usage)
	}

	flag.StringVar(&conf.branch, "b", "master", "release branch")
	flag.StringVar(&conf.release, "r", "latest", "release version")
	flag.BoolVar(&conf.install, "I", false, "first time install")
	flag.IntVar(&conf.upgrade, "U", -1, "upgrade (unsigned: 0, F-Secure keys: 1, user keys: 2)")
	flag.BoolVar(&conf.recovery, "R", false, "recovery install")

	flag.StringVar(&conf.logPublicKey, "p", "", "transparency log authentication key")
	flag.StringVar(&conf.frPublicKey, "f", "", "manifest authentication key")
	flag.StringVar(&conf.logOrigin, "l", assets.DefaultLogOrigin, "firmware transparency log origin")

	flag.StringVar(&conf.srkKey, "C", "", "SRK private key in PEM format")
	flag.StringVar(&conf.srkCrt, "c", "", "SRK public key in PEM format")
	flag.StringVar(&conf.table, "t", "", "SRK table")
	flag.StringVar(&conf.tableHash, "T", "", "SRK table hash")
	flag.IntVar(&conf.index, "x", -1, "index for SRK key")
}

func confirm(msg string) bool {
	var res string

	fmt.Printf("\n%s (y/N): ", msg)
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
	case conf.recovery:
		if confirm("Are you recovering an Armory Drive installation on a Secure Booted USB armory?") {
			recovery()
		}
	case conf.install ||
		conf.upgrade < 0 && confirm("Are you installing Armory Drive for the first time on the target USB armory?"):
		install()
	case conf.upgrade >= 0 || confirm("Are you upgrading Armory Drive on a USB armory already running Armory Drive firmware?"):
		upgrade()
	}

	log.Printf("\nGoodbye")
}

func recovery() {
	switch {
	case confirm("Is Secure Boot enabled on your USB armory using F-Secure signing keys?"):
		installFirmware(signedByFSecure)
	case confirm("Is Secure Boot enabled on your USB armory using your own signing keys?"):
		checkHABArguments()
		installFirmware(signedByUser)
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
	switch {
	case conf.upgrade == unsigned ||
		conf.upgrade < 0 && !confirm("Is Secure Boot enabled on your USB armory?"):
		upgradeFirmware(unsigned)
		return
	case conf.upgrade == signedByFSecure ||
		conf.upgrade < 0 && confirm("Is Secure Boot enabled on your USB armory using F-Secure signing keys?"):
		upgradeFirmware(signedByFSecure)
	case conf.upgrade == signedByUser ||
		confirm("Is Secure Boot enabled on your USB armory using your own signing keys?"):
		checkHABArguments()
		upgradeFirmware(signedByUser)
	default:
		log.Fatal("Goodbye")
	}
}

func ota(assets *releaseAssets) {
	log.Printf("\nWait for the USB armory blue LED to blink to indicate pairing mode.")
	log.Printf("\nAn F-Secure drive should appear on your system.")

	if conf.install {
		log.Printf("\nDo *not* pair with mobile application at this time.")
	}

	mountPoint := prompt("Please specify the path of the mounted F-Secure drive")

	log.Printf("\nCreating firmware update archive.")

	otaFile := new(bytes.Buffer)
	w := zip.NewWriter(otaFile)

	var files = []struct {
		Name string
		Body []byte
	}{
		{imxPath, assets.imx},
		{csfPath, assets.csf},
		{logPath, assets.log},
	}

	for _, file := range files {
		f, err := w.Create(file.Name)

		if err != nil {
			log.Fatal(err)
		}

		if _, err = f.Write(file.Body); err != nil {
			log.Fatal(err)
		}
	}

	if err := w.Close(); err != nil {
		log.Fatal(err)
	}

	log.Printf("Copying firmware to USB armory in pairing mode at %s", mountPoint)

	if err := os.WriteFile(path.Join(mountPoint, otaPath), otaFile.Bytes(), 0600); err != nil {
		log.Fatal(err)
	}
	log.Printf("\nCopied %d bytes to %s", otaFile.Len(), path.Join(mountPoint, otaPath))

	log.Printf("\n1. Please eject the drive mounted at %s to flash the firmware.", mountPoint)
	log.Printf("2. Wait for the white LED to turn on and then off for the update to complete.")
	log.Printf("3. Once the update is complete unplug the USB armory and set eMMC boot mode as explained at:")
	log.Printf("     https://github.com/usbarmory/usbarmory/wiki/Boot-Modes-(Mk-II)")

	log.Printf("\nAfter doing so you can use your new Armory Drive installation, following this tutorial:")
	log.Printf("  https://github.com/usbarmory/armory-drive/wiki/Tutorial")
}

func installFirmware(mode Mode) {
	var imx []byte

	assets, err := downloadRelease(conf.release)

	if err != nil {
		log.Fatalf("Download error, %v", err)
	}

	switch mode {
	case unsigned:
		log.Println(unsignedFirmwareWarning)

		if !confirm("Proceed?") {
			log.Fatal("Goodbye")
		}

		imx = assets.imx
	case signedByFSecure:
		log.Println(fscSignedFirmwareWarning)

		if !confirm("Proceed?") {
			log.Fatal("Goodbye")
		}

		imx = fixupSRKHash(assets.imx, assets.srk)
	case signedByUser:
		log.Println(userSignedFirmwareWarning)

		if !confirm("Proceed?") {
			log.Fatal("Goodbye")
		}

		if assets.srk, err = os.ReadFile(conf.tableHash); err != nil {
			log.Fatal(err)
		}

		if err = sign(assets); err != nil {
			log.Fatal(err)
		}

		imx = fixupSRKHash(assets.imx, assets.srk)
	default:
		log.Fatal("invalid installation mode")
	}

	if conf.recovery {
		if mode == signedByUser {
			// In case of recovery and user signature the SDP
			// signature is performed without fixup (which we don't
			// need anyway on recovery).
			imx = assets.imx
		}

		imx = append(imx, assets.sdp...)
	}

	log.Printf("\nFollow instructions at https://github.com/usbarmory/usbarmory/wiki/Boot-Modes-(Mk-II)")
	log.Printf("to set the target USB armory in SDP mode.")

	log.Printf("\nWaiting for target USB armory to be plugged to this computer in SDP mode.")

	if err = imxLoad(imx); err != nil {
		log.Fatal(err)
	}

	ota(assets)
}

func upgradeFirmware(mode Mode) {
	assets, err := downloadRelease(conf.release)

	if err != nil {
		log.Fatalf("Download error, %v", err)
	}

	if mode == signedByUser {
		if err = sign(assets); err != nil {
			log.Fatal(err)
		}
	}

	log.Printf("\nFollow instructions at https://github.com/usbarmory/armory-drive/wiki/Firmware-Updates")
	log.Printf("to set the Armory Drive firmware in pairing mode.")

	if !confirm("Confirm that target USB armory is plugged to this computer in pairing mode.") {
		log.Fatal("Goodbye")
	}

	ota(assets)
}
