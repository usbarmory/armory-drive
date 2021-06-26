// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"github.com/skip2/go-qrcode"
)

const pairingCodeSize = 117

func newPairingCode() (code []byte, err error) {
	// Generate a new UA longterm key, it will be saved only on successful
	// pairings.
	err = keyring.NewLongtermKey()

	if err != nil {
		return
	}

	key, err := keyring.Export(UA_LONGTERM_KEY, false)

	if err != nil {
		return
	}

	pb := &PairingQRCode{
		BLEName: session.PeerName,
		Nonce:   session.PairingNonce,
		PubKey:  key,
	}

	err = pb.Sign()

	if err != nil {
		return
	}

	qr, err := qrcode.New(string(pb.Bytes()), qrcode.Medium)

	if err != nil {
		return
	}

	return qr.PNG(pairingCodeSize)
}
