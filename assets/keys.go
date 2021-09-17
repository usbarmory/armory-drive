// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package assets

import (
	"crypto/sha256"
	_ "embed"
)

//go:generate go run embed_keys.go

// SRKSize represents the Secure Boot SRK hash size in bytes.
const SRKSize = 32

// SRKHash represents the Secure Boot SRK fuse table, this value is the output
// of DummySRKHash().
var SRKHash = []byte{
	0x63, 0x0d, 0xcd, 0x29, 0x66, 0xc4, 0x33, 0x66, 0x91, 0x12, 0x54, 0x48, 0xbb, 0xb2, 0x5b, 0x4f,
	0xf4, 0x12, 0xa4, 0x9c, 0x73, 0x2d, 0xb2, 0xc8, 0xab, 0xc1, 0xb8, 0x58, 0x1b, 0xd7, 0x10, 0xdd,
}

// Revision represents the firmware version.
var Revision string

// DefaultLogOrigin contains the default Firmware Transparency log origin name.
const DefaultLogOrigin = "Armory Drive Prod 1"

// DummySRKHash generates a known placeholder for the SRK hash to allow its
// identification and replacement within the binary, by `armory-drive-install`,
// with F-Secure or user secure boot key information.
func DummySRKHash() []byte {
	var dummySRK []byte

	for i := 0; i < SRKSize; i++ {
		dummySRK = append(dummySRK, byte(i))
	}

	dummySRKHash := sha256.Sum256(dummySRK)

	return dummySRKHash[:]
}
