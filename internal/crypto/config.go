// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package crypto

import (
	"bytes"
	"encoding/gob"

	logapi "github.com/usbarmory/armory-drive-log/api"
	"github.com/usbarmory/armory-drive/api"

	"github.com/usbarmory/tamago/board/f-secure/usbarmory/mark-two"
)

const (
	MMC_CONF_BLOCK = 2097152
	CONF_BLOCKS_V1 = 2
	CONF_BLOCKS_V2 = 2048
)

type PersistentConfiguration struct {
	// serialized long term BLE peer authentication keys
	ArmoryLongterm []byte
	MobileLongterm []byte

	// BLE API Configuration
	Settings *api.Configuration

	// Transparency Log Checkpoint
	ProofBundle *logapi.ProofBundle
}

func (k *Keyring) reset() (err error) {
	var armoryLongterm []byte

	if k.ArmoryLongterm == nil {
		err = k.NewLongtermKey()

		if err != nil {
			return
		}
	}

	armoryLongterm, err = k.Export(UA_LONGTERM_KEY, true)

	if err != nil {
		return
	}

	k.Conf = &PersistentConfiguration{
		ArmoryLongterm: armoryLongterm,
		Settings: &api.Configuration{
			Cipher: api.Cipher_AES128_CBC_PLAIN,
		},
	}

	return k.Save()
}

func (k *Keyring) loadAt(lba int, blocks int) (err error) {
	blockSize := usbarmory.MMC.Info().BlockSize
	snvs := make([]byte, blocks*blockSize)

	if err = usbarmory.MMC.ReadBlocks(lba, snvs); err != nil {
		return
	}

	buf, err := k.decryptSNVS(snvs)

	if err != nil {
		return
	}

	k.Conf = &PersistentConfiguration{}
	err = gob.NewDecoder(bytes.NewBuffer(buf)).Decode(k.Conf)

	return
}

func (k *Keyring) Load() (err error) {
	if err = k.loadAt(MMC_CONF_BLOCK, CONF_BLOCKS_V2); err == nil {
		return
	}

	err = k.loadAt(MMC_CONF_BLOCK, CONF_BLOCKS_V1)

	return
}

func (k *Keyring) Save() (err error) {
	blockSize := usbarmory.MMC.Info().BlockSize

	buf := new(bytes.Buffer)
	err = gob.NewEncoder(buf).Encode(k.Conf)

	if err != nil {
		return
	}

	snvs, err := k.encryptSNVS(buf.Bytes(), CONF_BLOCKS_V2*blockSize)

	if err != nil {
		return
	}

	return usbarmory.MMC.WriteBlocks(MMC_CONF_BLOCK, snvs)
}
