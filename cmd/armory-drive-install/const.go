// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

const (
	unsigned = iota
	signedByFSecure
	signedByUser
)

const (
	otaPath = "UPDATE.ZIP"
	imxPath = "armory-drive.imx"
	csfPath = "armory-drive.csf"
	logPath = "armory-drive.log"
)

const usage = `Usage: habtool [OPTIONS]
  -h    show this help

  -I    first time install
  -R    recovery install
  -U int
        upgrade (unsigned: 0, F-Secure keys: 1, user keys: 2) (default -1)

  -C string
        SRK private key in PEM format
  -c string
        SRK public key in PEM format
  -t string
        SRK table
  -T string
        SRK table hash
  -x int
        index for SRK key (default -1)

  -p string
        transparency log authentication key
  -f string
        manifest authentication key
  -l string
        firmware transparency log origin (default "Armory Drive Prod 2")
  -b string
        release branch (default "master")
  -r string
        release version (default "latest")
`

const welcome = `
Welcome to the Armory Drive installer!

For more information or support on Armory Drive see:
  https://github.com/f-secure-foundry/armory-drive/wiki

This program will install or upgrade Armory Drive on your USB armory.`

const secureBootNotice = `
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

████████████████████████████████████████████████████████████████████████████████
`

const userSignedFirmwareWarning = `
████████████████████████████████████████████████████████████████████████████████

                 ***  Armory Drive Programming Utility  ***
                 ***           READ CAREFULLY           ***

This will provision user signed Armory Drive firmware on your USB armory. By
doing so, secure boot will be activated on the USB armory with permanent OTP
fusing of your own public secure boot keys.

Fusing OTP's is an **irreversible** action that permanently fuses values on the
device. This means that your USB armory will be able to only execute firmware
signed with your own secure boot keys after programming is completed.

In other words your USB armory will stop acting as a generic purpose device and
will be converted to *exclusive use of your own signed firmware releases*.

████████████████████████████████████████████████████████████████████████████████
`

const secureBootHelp = `
████████████████████████████████████████████████████████████████████████████████

To sign releases on your own the installer needs access to your SRK private
(-C) and public (-c) keys, the SRK table (-t), the SRK table hash (-T) and the
SRK keypair index (-x) within the table.

These flags must passed to the installer when signing your own releases (launch
installer with -h flag to see all availale flags).

If you want to use previously generated  secure boot keys, please set these
flags according to your environment.

If you have not yet generated secure boot keys, you can do so with the
following tool:

  https://github.com/f-secure-foundry/crucible/tree/master/cmd/habtool

Example:

  # SRK keys generation
  habtool -C SRK_1_key.pem -c SRK_1_crt.pem
  habtool -C SRK_2_key.pem -c SRK_2_crt.pem
  habtool -C SRK_3_key.pem -c SRK_3_crt.pem
  habtool -C SRK_4_key.pem -c SRK_4_crt.pem

  # SRK table and table hash generation
  habtool -1 SRK_1_crt.pem -2 SRK_2_crt.pem -3 SRK_3_crt.pem -4 SRK_4_crt.pem -t SRK_1_2_3_4_table.bin -o SRK_1_2_3_4_fuse.bin

  # installer invocation with generated key material
  armory-drive-install -C SRK_1_key.pem -c SRK_1_crt.pem -t SRK_1_2_3_4_table.bin -T SRK_1_2_3_4_fuse.bin -x 1

████████████████████████████████████████████████████████████████████████████████
`
