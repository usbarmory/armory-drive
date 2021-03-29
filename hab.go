// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code uuis governed by the license
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"fmt"
	"log"
	_ "unsafe"

	"github.com/f-secure-foundry/armory-drive/assets"

	"github.com/f-secure-foundry/tamago/soc/imx6"

	"github.com/f-secure-foundry/crucible/otp"
	"github.com/f-secure-foundry/crucible/util"
)

// HAB SRK hash size in bytes
const srkSize = 32

func initializeHAB() {
	switch {
	case imx6.SNVS():
		// unit is already Secure Booted
		return
	case len(assets.SRKHash) != srkSize:
		// image is not signed
		return
	case !bytes.Equal(assets.SRKHash, make([]byte, len(assets.SRKHash))):
		// no Secure Boot and empty SRK hash means this is a test unit
		return
	default:
		hab(assets.SRKHash)
	}

	imx6.Reboot()
}

func fuse(name string, bank int, word int, off int, size int, val []byte) {
	log.Printf("fusing %s bank:%d word:%d off:%d size:%d val:%x", name, bank, word, off, size, val)

	if err := otp.BlowOCOTP(bank, word, off, size, val); err != nil {
		panic(err)
	}

	if res, err := otp.ReadOCOTP(bank, word, off, size); err != nil || !bytes.Equal(val, res) {
		panic(fmt.Sprintf("readback error for %s, val:%x res:%x err:%v\n", name, val, res, err))
	}
}

// The function activates secure boot by following the procedure described at:
//   https://github.com/f-secure-foundry/usbarmory/wiki/Secure-boot-(Mk-II)#activating-hab
//
// IMPORTANT: enabling secure boot functionality on the USB armory SoC, unlike
// similar features on modern PCs, is an irreversible action that permanently
// fuses verification keys hashes on the device. This means that any errors in
// the process or loss of the signing PKI will result in a bricked device
// incapable of executing unsigned code. This is a security feature, not a bug.
func hab(srk []byte) {
	// Enable High Assurance Boot (i.e. secure boot)

	if len(assets.SRKHash) != srkSize {
		panic("fatal error, invalid SRK hash")
	}

	// fuse HAB public keys hash
	fuse("SRK_HASH", 3, 0, 0, 256, util.SwitchEndianness(srk))

	// lock HAB public keys hash
	fuse("SRK_LOCK", 0, 0, 14, 1, []byte{1})

	// set device in Closed Configuration (IMX6ULRM Table 8-2, p245)
	fuse("SEC_CONFIG", 0, 6, 0, 2, []byte{0b11})

	// disable NXP reserved mode (IMX6ULRM 8.2.6, p244)
	fuse("DIR_BT_DIS", 0, 6, 3, 1, []byte{1})

	// Disable debugging features (IMX6ULRM Table 5-9, p216)

	// disable Secure JTAG controller
	fuse("SJC_DISABLE", 0, 6, 20, 1, []byte{1})

	// disable JTAG debug mode
	fuse("JTAG_SMODE", 0, 6, 22, 2, []byte{0b11})

	// disable HAB ability to enable JTAG
	fuse("JTAG_HEO", 0, 6, 27, 1, []byte{1})

	// disable tracing
	fuse("KTE", 0, 6, 26, 1, []byte{1})

	// Further reduce the attack surface

	// disable Serial Download Protocol (SDP) READ_REGISTER command (IMX6ULRM 8.9.3, p310)
	fuse("SDP_READ_DISABLE", 0, 6, 18, 1, []byte{1})

	// disable SDP over UART (IMX6ULRM 8.9, p305)
	fuse("UART_SERIAL_DOWNLOAD_DISABLE", 0, 7, 4, 1, []byte{1})
}
