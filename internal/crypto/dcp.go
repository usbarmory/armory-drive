// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package crypto

import (
	"crypto/aes"
	"crypto/cipher"

	"github.com/usbarmory/tamago/soc/imx6/dcp"
)

type dcpCipher struct {
	keyIndex int
}

// NewDCPKeyRAMCipher creates and returns a new cipher.Block. The keyIndex
// argument represents a key RAM slot, set with either dcp.DeriveKey() or
// dcp.SetKey(), for hardware accelerated AES-128 encryption.
func newDCPKeyRAMCipher(keyIndex int) (c cipher.Block, err error) {
	c = &dcpCipher{
		keyIndex: keyIndex,
	}

	return
}

// NewDCPCipher creates and returns a new cipher.Block. The key argument should
// be a 16 bytes AES key for hardware accelerated AES-128.
//
// The passed key is placed in DCP RAM slot 0 for use.
func newDCPCipher(key []byte) (c cipher.Block, err error) {
	c = &dcpCipher{
		keyIndex: BLOCK_KEY,
	}

	return c, dcp.SetKey(BLOCK_KEY, key)
}

// BlockSize returns the AES block size in bytes.
func (c *dcpCipher) BlockSize() int {
	return aes.BlockSize
}

// Encrypt performs in-place buffer encryption using AES-128-CBC.
func (c *dcpCipher) Encrypt(_ []byte, buf []byte) {
	dcp.Encrypt(buf, c.keyIndex, zero)
}

// Decrypt performs in-place buffer decryption using AES-128-CBC.
func (c *dcpCipher) Decrypt(_ []byte, buf []byte) {
	dcp.Decrypt(buf, c.keyIndex, zero)
}
