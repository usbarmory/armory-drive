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
	MMC_CONF_BLOCK  = 2097152
	CONF_MIN_BLOCKS = 2
	CONF_MAX_BLOCKS = 4
)

var conf *PersistentConfiguration

type PersistentConfiguration struct {
	// serialized long term BLE peer authentication keys
	ArmoryLongterm []byte
	MobileLongterm []byte

	// BLE API Configuration
	Settings *Configuration

	// Transparency Log Checkpoint
	Checkpoint []byte
}

func (conf *PersistentConfiguration) save() (err error) {
	blockSize := usbarmory.MMC.Info().BlockSize

	buf := new(bytes.Buffer)
	err = gob.NewEncoder(buf).Encode(conf)

	if err != nil {
		return
	}

	snvs, err := encryptSNVS(buf.Bytes(), CONF_MAX_BLOCKS*blockSize)

	if err != nil {
		return
	}

	return usbarmory.MMC.WriteBlocks(MMC_CONF_BLOCK, snvs)
}

func load(lba int, blocks int) (conf *PersistentConfiguration, err error) {
	blockSize := usbarmory.MMC.Info().BlockSize
	snvs := make([]byte, blocks*blockSize)
	err = usbarmory.MMC.ReadBlocks(lba, snvs)

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

func LoadConfiguration() (conf *PersistentConfiguration, err error) {
	// support changes in configuration size over time
	for blocks := CONF_MAX_BLOCKS; blocks >= CONF_MIN_BLOCKS; blocks-- {
		conf, err = load(MMC_CONF_BLOCK, blocks)

		if err == nil {
			return
		}
	}

	return
}
