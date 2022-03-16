// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"log"

	"github.com/usbarmory/armory-drive/assets"
)

func fixupSRKHash(buf []byte, srk []byte) []byte {
	dummySRKHash := assets.DummySRKHash()

	if !bytes.Contains(buf, dummySRKHash) {
		log.Fatal("could not locate dummy SRK hash")
	}

	buf = bytes.ReplaceAll(buf, dummySRKHash, srk)

	if bytes.Contains(buf, dummySRKHash) || !bytes.Contains(buf, srk) {
		log.Fatal("could not set SRK hash")
	}

	return buf
}

func clearFRPublicKey(buf []byte, key []byte) []byte {
	if !bytes.Contains(buf, key) {
		log.Fatal("could not locate OTA public key")
	}

	buf = bytes.ReplaceAll(buf, key, make([]byte, len(key)))

	if bytes.Contains(buf, key) {
		log.Fatal("could not clear OTA public key")
	}

	return buf
}
