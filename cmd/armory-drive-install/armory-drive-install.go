// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"log"
	"os"
	"path"

	"github.com/flynn/hid"
)

const OTAName = "UA-DRIVE.ota"

const welcome =`
Welcome to the Armory Drive installer!

For more information or support on Armory Drive see:
  https://github.com/f-secure-foundry/armory-drive/wiki

This program will install or upgrade Armory Drive on your USB armory.
`

const secureBootNotice =`
████████████████████████████████████████████████████████████████████████████████

This installer supports installation of unsigned or signed Armory Drive
releases on the USB armory.

                 ***     Option #1: signed releases     ***

The installation of signed releases activates Secure Boot on the target USB
armory, fully converting the device to exclusive operation with signed
executables.

If the signed releases option is chosen you will be given the option of using
F-Secure signing keys or your own.

                 ***    Option #2: unsigned releases    ***

The installation of unsigned releases does not leverage on Secure Boot and does
not permanently modify the USB armory security state.

Unsigned releases however cannot guarantee device security as hardware bound
key material will use default test keys, lacking protection for stored armory
communication keys and leaving data encryption key freshness only to the mobile
application.

Unsigned releases are recommended only for test/evaluation purposes and are not
recommended for protection of sensitive data where device tampering is a risk.

████████████████████████████████████████████████████████████████████████████████
`

const fscSignedFirmwareWarning = `
████████████████████████████████████████████████████████████████████████████████

                 ***  Armory Drive Programming Utility  ***
                 ***           READ CAREFULLY           ***

This will provision F-Secure signed Armory Drive firmware on your USB armory. By
doing so, secure boot will be activated on the USB armory with permanent OTP
fusing of F-Secure public secure boot keys.

Fusing OTP's is an **irreversible** action that permanently fuses values on the
device. This means that your USB armory will be able to only execute F-Secure
signed Armory Drive firmware after programming is completed.

In other words your USB armory will stop acting as a generic purpose device and
will be converted to *exclusive use of F-Secure signed Armory Drive releases*.
`

const unsignedFirmwareWarning = `
████████████████████████████████████████████████████████████████████████████████

                 ***  Armory Drive Programming Utility  ***
                 ***           READ CAREFULLY           ***

This will provision unsigned Armory Drive firmware on your USB armory.

This firmware *cannot guarantee device security* as hardware bound key material
will use *default test keys*, lacking protection for stored armory communication
keys and leaving data encryption key freshness only to the mobile application.

Unsigned releases are therefore recommended exclusively for test/evaluation
purposes and are *not recommended for protection of sensitive data*.

To enable the full security model install a signed release, which enables
Secure Boot.

████████████████████████████████████████████████████████████████████████████████
`

type Config struct {
	releaseVersion string
	timeout        int
	dev            hid.Device
}

var conf *Config

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stdout)

	conf = &Config{}

	flag.IntVar(&conf.timeout, "t", 5, "timeout in seconds for command responses")
	flag.StringVar(&conf.releaseVersion, "r", "latest", "release version")
}

func confirm(msg string) bool {
	var res string

	fmt.Printf("%s (y/n): ", msg)
	fmt.Scanln(&res)

	return res == "y"
}

func prompt(msg string) (res string) {
	fmt.Printf("%s: ", msg)
	fmt.Scanln(&res)
	return
}

func main() {
	flag.Parse()

	log.Println(welcome)

	switch {
	case confirm("Are you installing Armory Drive for the first time on the target USB armory (or recovering a device)?"):
		install()
	case confirm("Are you upgrading Armory Drive on a USB armory already running Armory Drive firmware?"):
		upgrade()
	default:
		log.Fatal("Goodbye")
	}
}

func install() {
	log.Println(secureBootNotice)

	switch {
	//case confirm("Would you like to use signed releases by enabling Secure Boot on the USB armory?"):
	//	installSignedFirmware()
	case confirm("Would you like to use unsigned releases, without enabling Secure Boot on the USB armory?"):
		installUnsignedFirmware(false)
	default:
		log.Fatal("Goodbye")
	}
}

func upgrade() {
	log.Println(secureBootNotice)

	switch {
	//case confirm("Would you like to use signed releases by enabling Secure Boot on the USB armory?"):
	//	installSignedFirmware()
	case confirm("Would you like to use unsigned releases, without enabling Secure Boot on the USB armory?"):
		installUnsignedFirmware(true)
	default:
		log.Fatal("Goodbye")
	}
}

func installUnsignedFirmware(upgrade bool) {
	imx, sig, ota, _, err := downloadLatestRelease()

	if err != nil {
		log.Fatal(err)
	}

	// remove SRK hash to disable Secure Boot provisioning
	if imx, err = removeSRKHash(imx); err != nil {
		log.Fatalf("could not disable secure boot provisioning, %v", err)
	}

	log.Printf("Downloaded firmware with SHA256 %x", sha256.Sum256(imx))

	if !upgrade {
		if !confirm("Confirm that the target USB armory is plugged to this computer in SDP mode (See https://github.com/f-secure-foundry/usbarmory/wiki/Boot-Modes-(Mk-II) for help)") {
			log.Fatal("Goodbye")
		}

		if err = imxLoad(imx); err != nil {
			log.Fatal(err)
		}
	} else {
		if !confirm("Confirm that the target USB armory is plugged to this computer in pairing mode? (See https://github.com/f-secure-foundry/armory-drive#firmware-update for help)") {
			log.Fatal("Goodbye")
		}
	}

	log.Printf("The USB armory should now have a blinking blue LED to indicate pairing mode, a new drive should appear on your system")
	mountPoint := prompt("Please specify the path of the newly appeared drive")

	imx = append(imx, sig...)
	imx = append(imx, ota...)

	log.Printf("Copying firmware to USB armory in pairing mode at %s", mountPoint)
	if err = os.WriteFile(path.Join(mountPoint, OTAName), imx, 0600); err != nil {
		log.Fatal(err)
	}

	log.Printf("Please eject the drive mounted at %s to flash the firmware. Wait for the white LED to turn on and then off for the update to complete.", mountPoint)
	log.Printf("Once the update is complete unplug the USB armory and restore eMMC boot mode (See https://github.com/f-secure-foundry/usbarmory/wiki/Boot-Modes-(Mk-II) for help)")
	log.Printf("\nYou can now use your new Armory Drive installation!")
}

//case confirm("Would you like to use F-Secure signed releases, enabling Secure Boot on the USB armory with F-Secure public keys as required?"):
//	switch {
//	case confirm("Would you like to use sign releases on your own, enabling Secure Boot on the USB armory with your own public keys as required?"):
//	default:
//		log.Fatal("Goodbye")
//	}
