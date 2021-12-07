// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package ble

import (
	"crypto/aes"
	"encoding/binary"
	"errors"
	"log"
	"time"

	"github.com/f-secure-foundry/armory-drive/api"
	"github.com/f-secure-foundry/armory-drive/assets"
	"github.com/f-secure-foundry/armory-drive/internal/crypto"

	"github.com/f-secure-foundry/tamago/board/f-secure/usbarmory/mark-two"

	"google.golang.org/protobuf/proto"
)

func (b *BLE) parseEnvelope(buf []byte) (msg *api.Message, err error) {
	env := &api.Envelope{}

	if err = proto.Unmarshal(buf, env); err != nil {
		return
	}

	msg = &api.Message{}

	if err = proto.Unmarshal(env.Message, msg); err != nil {
		return
	}

	if msg.OpCode == api.OpCode_SESSION {
		b.Keyring.ClearSessionKeys()
		b.session.Reset()
	}

	if !b.pairingMode && b.Keyring.MobileLongterm != nil {
		if err = b.verifyEnvelope(env); err != nil {
			return
		}
	}

	if msg.OpCode != api.OpCode_PAIR && msg.OpCode != api.OpCode_SESSION {
		if err = b.decryptPayload(msg); err != nil {
			return
		}
	}

	if msg.Timestamp <= b.session.Last {
		return nil, errors.New("invalid timestamp")
	}

	b.session.Last = msg.Timestamp

	return
}

func (b *BLE) handleEnvelope(req []byte) (res []byte) {
	resMsg := &api.Message{
		Timestamp: b.session.Time(),
		Response:  true,
		OpCode:    api.OpCode_NULL,
	}

	defer func() {
		var err error

		if resMsg.OpCode != api.OpCode_PAIR && resMsg.OpCode != api.OpCode_SESSION {
			if err = b.encryptPayload(resMsg); err != nil {
				return
			}
		}

		resEnv := &api.Envelope{
			Message: resMsg.Bytes(),
		}

		if err = b.signEnvelope(resEnv); err != nil {
			return
		}

		if resMsg.OpCode == api.OpCode_SESSION && resMsg.Error == 0 {
			b.session.Active = true
		}

		res = resEnv.Bytes()
	}()

	reqMsg, err := b.parseEnvelope(req)

	if err != nil {
		resMsg.Error = api.ErrorCode_INVALID_MESSAGE
		return
	}

	resMsg.OpCode = reqMsg.OpCode
	b.handleMessage(reqMsg, resMsg)

	return
}

func (b *BLE) handleMessage(reqMsg *api.Message, resMsg *api.Message) {
	switch {
	case b.pairingMode:
		if reqMsg.OpCode != api.OpCode_PAIR {
			resMsg.Error = api.ErrorCode_INVALID_MESSAGE
			return
		}

		b.pair(reqMsg, resMsg)
		return
	case b.Keyring.MobileLongterm == nil:
		resMsg.Error = api.ErrorCode_INVALID_MESSAGE
		return
	case reqMsg.OpCode == api.OpCode_SESSION:
		b.newSession(reqMsg, resMsg)
		return
	case !b.session.Active:
		resMsg.Error = api.ErrorCode_INVALID_SESSION
		return
	}

	switch reqMsg.OpCode {
	case api.OpCode_UNLOCK:
		b.unlock(reqMsg, resMsg)
	case api.OpCode_LOCK:
		b.lock(reqMsg, resMsg)
	case api.OpCode_STATUS:
		b.status(reqMsg, resMsg)
	case api.OpCode_CONFIGURATION:
		b.configuration(reqMsg, resMsg)
	default:
		resMsg.Error = api.ErrorCode_INVALID_MESSAGE
	}
}

func (b *BLE) pair(reqMsg *api.Message, resMsg *api.Message) {
	keyExchange := &api.KeyExchange{}
	err := proto.Unmarshal(reqMsg.Payload, keyExchange)

	if err != nil {
		resMsg.Error = api.ErrorCode_INVALID_MESSAGE
		return
	}

	defer func() {
		if err != nil {
			log.Printf("err: %v", err)
			resMsg.Error = api.ErrorCode_PAIRING_KEY_NEGOTIATION_FAILED
		}
	}()

	if keyExchange.Nonce != b.pairingNonce {
		err = errors.New("nonce mismatch")
		return
	}

	// At this point pairing is considered successful, therefore overwrite
	// previous keyring with the newly generated UA longterm key.
	b.Keyring.Init(true)

	// Import the MD longterm key.
	if err = b.Keyring.Import(crypto.MD_LONGTERM_KEY, false, keyExchange.Key); err != nil {
		return
	}

	// Save the received MD longterm key in persistent storage.
	b.Keyring.Conf.MobileLongterm = keyExchange.Key
	err = b.Keyring.Save()

	b.Drive.PairingComplete <- true
}

func (b *BLE) newSession(reqMsg *api.Message, resMsg *api.Message) {
	keyExchange := &api.KeyExchange{}
	err := proto.Unmarshal(reqMsg.Payload, keyExchange)

	if err != nil {
		resMsg.Error = api.ErrorCode_INVALID_MESSAGE
		return
	}

	defer func() {
		if err != nil {
			// invalidate previous session on any error
			b.Keyring.ClearSessionKeys()
			b.session.Reset()
			resMsg.Error = api.ErrorCode_SESSION_KEY_NEGOTIATION_FAILED
		}
	}()

	if err = b.Keyring.Import(crypto.MD_EPHEMERAL_KEY, false, keyExchange.Key); err != nil {
		return
	}

	nonce := crypto.Rand(8)

	if err = b.Keyring.NewSessionKeys(nonce); err != nil {
		return
	}

	key, err := b.Keyring.Export(crypto.UA_EPHEMERAL_KEY, false)

	if err != nil {
		return
	}

	b.session.Skew = time.Until(time.Unix(0, reqMsg.Timestamp*1000*1000))

	keyExchange = &api.KeyExchange{
		Key:   key,
		Nonce: binary.BigEndian.Uint64(nonce),
	}

	resMsg.Timestamp = b.session.Time()
	resMsg.Payload = keyExchange.Bytes()
}

func (b *BLE) unlock(reqMsg *api.Message, resMsg *api.Message) {
	keyExchange := &api.KeyExchange{}
	err := proto.Unmarshal(reqMsg.Payload, keyExchange)

	b.session.Lock()

	defer func() {
		b.Drive.Ready = (err == nil)
		usbarmory.LED("white", b.Drive.Ready)

		// rate limit unlock operation
		time.Sleep(1 * time.Second)
		b.session.Unlock()
	}()

	if err != nil {
		resMsg.Error = api.ErrorCode_INVALID_MESSAGE
		return
	}

	defer func() {
		if err != nil {
			resMsg.Error = api.ErrorCode_UNLOCK_FAILED
		}
	}()

	if len(keyExchange.Key) < aes.BlockSize {
		resMsg.Error = api.ErrorCode_INVALID_MESSAGE
		return
	}

	err = b.Keyring.SetCipher(b.Keyring.Conf.Settings.Cipher, keyExchange.Key)
}

func (b *BLE) lock(reqMsg *api.Message, resMsg *api.Message) {
	if err := b.Drive.Lock(); err != nil {
		resMsg.Error = api.ErrorCode_GENERIC_ERROR
	}
}

func (b *BLE) status(reqMsg *api.Message, resMsg *api.Message) {
	s := &api.Status{
		Version:       assets.Revision,
		Capacity:      b.Drive.Capacity(),
		Locked:        !b.Drive.Ready,
		Configuration: b.Keyring.Conf.Settings,
	}

	resMsg.Payload = s.Bytes()
}

func (b *BLE) configuration(reqMsg *api.Message, resMsg *api.Message) {
	settings := &api.Configuration{}
	err := proto.Unmarshal(reqMsg.Payload, settings)

	if err != nil || b.Drive.Ready {
		resMsg.Error = api.ErrorCode_INVALID_MESSAGE
		return
	}

	b.Keyring.Conf.Settings = settings
	b.Keyring.Save()
}
