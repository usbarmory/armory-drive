// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package ble

import (
	"github.com/usbarmory/armory-drive/api"
)

func (b *BLE) verifyEnvelope(env *api.Envelope) (err error) {
	return b.Keyring.VerifyECDSA(env.Message, env.Signature, b.session.Active)
}

func (b *BLE) signEnvelope(env *api.Envelope) (err error) {
	env.Signature, err = b.Keyring.SignECDSA(env.Message, b.session.Active)
	return
}

func (b *BLE) encryptPayload(msg *api.Message) (err error) {
	msg.Payload, err = b.Keyring.EncryptOFB(msg.Payload)
	return
}

func (b *BLE) decryptPayload(msg *api.Message) (err error) {
	msg.Payload, err = b.Keyring.DecryptOFB(msg.Payload)
	return
}
