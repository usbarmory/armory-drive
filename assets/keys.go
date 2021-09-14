// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package assets

import (
	"crypto/sha256"
)

//go:generate go run embed_keys.go

// SRKSize represents the Secure Boot SRK hash size in bytes
const SRKSize = 32

// SRKHash represents the Secure Boot SRK fuse table
var SRKHash []byte

// FRPublicKey represents the firmware releases manifest authentication key
var FRPublicKey []byte

// LogPublicKey represents the firmware releases transparency log
// authentication key
var LogPublicKey []byte

// Revision represents the firmware version
var Revision string

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
