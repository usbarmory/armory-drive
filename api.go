// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"crypto/aes"
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"

	"github.com/f-secure-foundry/tamago/board/f-secure/usbarmory/mark-two"
)

func parseEnvelope(buf []byte) (msg *Message, err error) {
	env := &Envelope{}
	err = proto.Unmarshal(buf, env)

	if err != nil {
		return
	}

	msg = &Message{}
	err = proto.Unmarshal(env.Message, msg)

	if err != nil {
		return
	}

	if msg.OpCode == OpCode_SESSION {
		keyring.Reset()
		session.Reset()
	}

	if !session.PairingMode && keyring.MobileLongterm != nil {
		err = env.Verify()

		if err != nil {
			return
		}
	}

	if msg.OpCode != OpCode_PAIR && msg.OpCode != OpCode_SESSION {
		err = msg.Decrypt()

		if err != nil {
			return
		}
	}

	if msg.Timestamp <= session.Last {
		return nil, errors.New("invalid timestamp")
	}

	session.Last = msg.Timestamp

	return
}

func buildResponse(opCode OpCode) (msg *Message) {
	return &Message{
		Timestamp: session.Time(),
		Response:  true,
		OpCode:    opCode,
	}
}

func handleEnvelope(req []byte) (res []byte) {
	resMsg := buildResponse(0)

	defer func() {
		var err error

		if resMsg.OpCode != OpCode_PAIR && resMsg.OpCode != OpCode_SESSION {
			err = resMsg.Encrypt()

			if err != nil {
				return
			}
		}

		resEnv := &Envelope{
			Message: resMsg.Bytes(),
		}

		err = resEnv.Sign()

		if err != nil {
			return
		}

		if resMsg.OpCode == OpCode_SESSION && resMsg.Error == 0 {
			session.Active = true
		}

		res = resEnv.Bytes()
	}()

	reqMsg, err := parseEnvelope(req)

	if err != nil {
		resMsg.Error = ErrorCode_INVALID_MESSAGE
		return
	}

	resMsg.OpCode = reqMsg.OpCode
	handleMessage(reqMsg, resMsg)

	return
}

func handleMessage(reqMsg *Message, resMsg *Message) {
	if session.PairingMode {
		if reqMsg.OpCode != OpCode_PAIR {
			resMsg.Error = ErrorCode_INVALID_MESSAGE
			return
		}

		pair(reqMsg, resMsg)
		return
	}

	if keyring.MobileLongterm == nil {
		resMsg.Error = ErrorCode_INVALID_MESSAGE
		return
	}

	if reqMsg.OpCode == OpCode_SESSION {
		newSession(reqMsg, resMsg)
		return
	}

	if !session.Active {
		resMsg.Error = ErrorCode_INVALID_SESSION
		return
	}

	switch reqMsg.OpCode {
	case OpCode_UNLOCK:
		unlock(reqMsg, resMsg)
	case OpCode_LOCK:
		lock(reqMsg, resMsg)
	case OpCode_STATUS:
		status(reqMsg, resMsg)
	case OpCode_CONFIGURATION:
		configuration(reqMsg, resMsg)
	default:
		resMsg.Error = ErrorCode_INVALID_MESSAGE
	}
}

func pair(reqMsg *Message, resMsg *Message) {
	keyExchange := &KeyExchange{}
	err := proto.Unmarshal(reqMsg.Payload, keyExchange)

	if err != nil {
		resMsg.Error = ErrorCode_INVALID_MESSAGE
		return
	}

	defer func() {
		if err != nil {
			resMsg.Error = ErrorCode_PAIRING_KEY_NEGOTIATION_FAILED
		}
	}()

	if keyExchange.Nonce != session.PairingNonce {
		err = errors.New("nonce mismatch")
		return
	}

	err = keyring.Import(MD_LONGTERM_KEY, false, keyExchange.Key)

	if err != nil {
		return
	}

	// At this point pairing is considered successful, therefore overwrite
	// previous keyring with the newly generated UA longterm key.
	keyring.Init(true)

	// Save the received MD longterm key in persistent storage.
	conf.MobileLongterm = keyExchange.Key
	err = conf.save()

	pairingComplete <- true
}

func newSession(reqMsg *Message, resMsg *Message) {
	keyExchange := &KeyExchange{}
	err := proto.Unmarshal(reqMsg.Payload, keyExchange)

	if err != nil {
		resMsg.Error = ErrorCode_INVALID_MESSAGE
		return
	}

	defer func() {
		if err != nil {
			// invalidate previous session on any error
			keyring.Reset()
			session.Reset()
			resMsg.Error = ErrorCode_SESSION_KEY_NEGOTIATION_FAILED
		}
	}()

	err = keyring.Import(MD_EPHEMERAL_KEY, false, keyExchange.Key)

	if err != nil {
		return
	}

	nonce := rng(8)
	err = keyring.NewSessionKeys(nonce)

	if err != nil {
		return
	}

	key, err := keyring.Export(UA_EPHEMERAL_KEY, false)

	if err != nil {
		return
	}

	session.Skew = time.Until(time.Unix(0, reqMsg.Timestamp*1000*1000))

	keyExchange = &KeyExchange{
		Key:   key,
		Nonce: binary.BigEndian.Uint64(nonce),
	}

	resMsg.Timestamp = session.Time()
	resMsg.Payload = keyExchange.Bytes()
}

func unlock(reqMsg *Message, resMsg *Message) {
	keyExchange := &KeyExchange{}
	err := proto.Unmarshal(reqMsg.Payload, keyExchange)

	session.Lock()

	defer func() {
		ready = (err == nil)
		usbarmory.LED("white", ready)

		// rate limit unlock operation
		time.Sleep(1 * time.Second)
		session.Unlock()
	}()

	if err != nil {
		resMsg.Error = ErrorCode_INVALID_MESSAGE
		return
	}

	defer func() {
		if err != nil {
			resMsg.Error = ErrorCode_UNLOCK_FAILED
		}
	}()

	if len(keyExchange.Key) < aes.BlockSize {
		resMsg.Error = ErrorCode_INVALID_MESSAGE
		return
	}

	err = setCipher(conf.Settings.Cipher, keyExchange.Key)
}

func lock(reqMsg *Message, resMsg *Message) {
	var err error

	// no matter what, we invalidate the drive
	ready = false
	err = setCipher(Cipher_NONE, zero)

	usbarmory.LED("white", ready)

	if err != nil {
		resMsg.Error = ErrorCode_GENERIC_ERROR
	}
}

func status(reqMsg *Message, resMsg *Message) {
	var capacity uint64

	if len(cards) > 0 {
		info := cards[0].Info()
		capacity = uint64(info.Blocks) * uint64(info.BlockSize)
	}

	s := &Status{
		Version:       fmt.Sprintf("%s", Revision),
		Capacity:      capacity,
		Locked:        !ready,
		Configuration: conf.Settings,
	}

	resMsg.Payload = s.Bytes()
}

func configuration(reqMsg *Message, resMsg *Message) {
	settings := &Configuration{}
	err := proto.Unmarshal(reqMsg.Payload, settings)

	if err != nil || ready {
		resMsg.Error = ErrorCode_INVALID_MESSAGE
		return
	}

	conf.Settings = settings
	conf.save()
}

func (msg *Message) Bytes() (buf []byte) {
	buf, _ = proto.Marshal(msg)
	return
}

func (msg *Message) Encrypt() (err error) {
	msg.Payload, err = encryptOFB(msg.Payload)
	return
}

func (msg *Message) Decrypt() (err error) {
	msg.Payload, err = decryptOFB(msg.Payload)
	return
}

func (env *Envelope) Bytes() (buf []byte) {
	buf, _ = proto.Marshal(env)
	return
}

func (env *Envelope) Sign() (err error) {
	env.Signature, err = signECDSA(env.Message)
	return
}

func (env *Envelope) Verify() (err error) {
	return verifyECDSA(env.Message, env.Signature)
}

func (kex *KeyExchange) Bytes() (buf []byte) {
	buf, _ = proto.Marshal(kex)
	return
}

func (qr *PairingQRCode) Bytes() (buf []byte) {
	buf, _ = proto.Marshal(qr)
	return
}

func (status *Status) Bytes() (buf []byte) {
	buf, _ = proto.Marshal(status)
	return
}

func (qr *PairingQRCode) Sign() (err error) {
	var data []byte

	nonce := make([]byte, 8)
	binary.BigEndian.PutUint64(nonce, qr.Nonce)

	data = append(data, []byte(qr.BLEName)...)
	data = append(data, nonce...)
	data = append(data, qr.PubKey...)

	qr.Signature, err = signECDSA(data)

	return
}
