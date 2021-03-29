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

	"github.com/flynn/hid"
)

const warningSignedFirmware = `
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

const warningUnsignedFirmware = `
████████████████████████████████████████████████████████████████████████████████

                 ***  Armory Drive Programming Utility  ***
                 ***           READ CAREFULLY           ***

This will provision unsigned Armory Drive firmware on your USB armory.

This firmware *cannot guarantee device security* as hardware bound key material
will use *default test keys*, lacking protection for stored armory communication
keys and leaving data encryption key freshness only to the mobile application.

Unsigned releases are therefore recommended exclusively for test/evaluation purposes
and are *not recommended for protection of sensitive data*.

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

func main() {
	flag.Parse()

	// FIXME: TODO
	//if !confirm("Are you installing Armory Drive for the first time on this unit, or recovering a non working unit?")
	//}

	if !confirm("Would you like to proceed?") {
		log.Fatal("Goodbye")
	}

	// FIXME: TODO
	imx, _, _, err := downloadLatestRelease()

	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Downloaded firmware with SHA256 %x", sha256.Sum256(imx))

	log.Printf("Flashing firmware to target USB armory")

	err = imxLoad(imx)

	if err != nil {
		log.Fatal(err)
	}
}
