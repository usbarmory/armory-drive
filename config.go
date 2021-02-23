// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/gob"

	"github.com/f-secure-foundry/tamago/board/f-secure/usbarmory/mark-two"
)

const (
	MMC_CONF_BLOCK   = 2097152
	MMC_CONF_SECTORS = 2
)

var conf *PersistentConfiguration

type PersistentConfiguration struct {
	// serialized long term BLE peer authentication keys
	ArmoryLongterm []byte
	MobileLongterm []byte

	// BLE API Configuration
	Settings *Configuration
}

func (conf *PersistentConfiguration) save() (err error) {
	blockSize := usbarmory.MMC.Info().BlockSize

	buf := new(bytes.Buffer)
	err = gob.NewEncoder(buf).Encode(conf)

	if err != nil {
		return
	}

	snvs, err := encryptSNVS(buf.Bytes(), MMC_CONF_SECTORS*blockSize)

	if err != nil {
		return
	}

	return usbarmory.MMC.WriteBlocks(MMC_CONF_BLOCK, snvs)
}

func LoadConfiguration() (conf *PersistentConfiguration, err error) {
	blockSize := usbarmory.MMC.Info().BlockSize

	snvs := make([]byte, blockSize*MMC_CONF_SECTORS)
	err = usbarmory.MMC.ReadBlocks(MMC_CONF_BLOCK, snvs)

	if err != nil {
		return
	}

	buf, err := decryptSNVS(snvs)

	if err != nil {
		return
	}

	conf = &PersistentConfiguration{}
	err = gob.NewDecoder(bytes.NewBuffer(buf)).Decode(conf)

	return
}
