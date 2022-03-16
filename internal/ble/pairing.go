// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package ble

import (
	"encoding/binary"

	"github.com/usbarmory/armory-drive/api"
	"github.com/usbarmory/armory-drive/internal/crypto"

	"github.com/skip2/go-qrcode"
)

const pairingCodeSize = 117

func (b *BLE) PairingMode() (code []byte, err error) {
	// Generate a new UA longterm key, it will be saved only on successful
	// pairings.
	if err = b.Keyring.NewLongtermKey(); err != nil {
		return
	}

	key, err := b.Keyring.Export(crypto.UA_LONGTERM_KEY, false)

	if err != nil {
		return
	}

	b.pairingMode = true
	b.pairingNonce = binary.BigEndian.Uint64(crypto.Rand(8))

	pb := &api.PairingQRCode{
		BLEName: b.name,
		Nonce:   b.pairingNonce,
		PubKey:  key,
	}

	if err = b.signPairingCode(pb); err != nil {
		return
	}

	qr, err := qrcode.New(string(pb.Bytes()), qrcode.Medium)

	if err != nil {
		return
	}

	return qr.PNG(pairingCodeSize)
}

func (b *BLE) signPairingCode(qr *api.PairingQRCode) (err error) {
	var data []byte

	nonce := make([]byte, 8)
	binary.BigEndian.PutUint64(nonce, qr.Nonce)

	data = append(data, []byte(qr.BLEName)...)
	data = append(data, nonce...)
	data = append(data, qr.PubKey...)

	qr.Signature, err = b.Keyring.SignECDSA(data, false)

	return
}
