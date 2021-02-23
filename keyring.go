// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"io"

	"golang.org/x/crypto/hkdf"
	"golang.org/x/crypto/xts"
)

// DCP key RAM indices
const (
	BLOCK_KEY = iota
	ESSIV_KEY
	SNVS_KEY
)

// BLE key indices
const (
	UA_LONGTERM_KEY = iota
	UA_EPHEMERAL_KEY
	MD_LONGTERM_KEY
	MD_EPHEMERAL_KEY
)

type Keyring struct {
	// CPU bound ESSIV cipher
	cbiv cipher.Block
	// CPU bound block cipher
	cb cipher.Block
	// CPU bound xts block cipher
	cbxts *xts.Cipher

	// IV encryption key for ESSIV computation
	salt []byte
	// persistent storage encryption key
	snvs []byte

	// long term BLE peer authentication keys
	ArmoryLongterm *ecdsa.PrivateKey
	MobileLongterm *ecdsa.PublicKey

	// ephemeral BLE peer session keys
	armoryEphemeral *ecdsa.PrivateKey
	mobileEphemeral *ecdsa.PublicKey

	// BLE shared pre-master secret
	preMaster []byte
	// BLE shared session key
	sessionKey []byte
}

var keyring = &Keyring{}

func (keyring *Keyring) Init(overwrite bool) (err error) {
	// derive persistent storage encryption key
	if keyring.snvs, err = deriveKey([]byte(SNVS_DIV), SNVS_KEY, true); err != nil {
		return
	}

	conf, err = LoadConfiguration()

	if err != nil || overwrite {
		var armoryLongterm []byte

		if keyring.ArmoryLongterm == nil {
			err = keyring.NewLongtermKey()

			if err != nil {
				return
			}
		}

		armoryLongterm, err = keyring.Export(UA_LONGTERM_KEY, true)

		if err != nil {
			return
		}

		conf = &PersistentConfiguration{
			ArmoryLongterm: armoryLongterm,
			Settings: &Configuration{
				Cipher: Cipher_AES128_CBC_PLAIN,
			},
		}

		err = conf.save()

		if err != nil {
			return
		}
	}

	err = keyring.Import(UA_LONGTERM_KEY, true, conf.ArmoryLongterm)

	if err != nil {
		return
	}

	// we might not be paired yet, so ignore errors
	keyring.Import(MD_LONGTERM_KEY, false, conf.MobileLongterm)

	// Derive salt, used for ESSIV computation as well as BLOCK_KEY derivation.
	if keyring.salt, err = deriveKey([]byte(ESSIV_DIV), ESSIV_KEY, true); err != nil {
		return
	}

	return
}

func (keyring *Keyring) Export(index int, private bool) ([]byte, error) {
	var pubKey *ecdsa.PublicKey
	var privKey *ecdsa.PrivateKey

	switch index {
	case UA_LONGTERM_KEY:
		privKey = keyring.ArmoryLongterm
	case UA_EPHEMERAL_KEY:
		privKey = keyring.armoryEphemeral
	case MD_LONGTERM_KEY:
		pubKey = keyring.MobileLongterm
	case MD_EPHEMERAL_KEY:
		pubKey = keyring.mobileEphemeral
	default:
		return nil, errors.New("invalid key index")
	}

	if !private && pubKey == nil && privKey != nil {
		pubKey = &privKey.PublicKey
	}

	if private {
		if privKey == nil {
			return nil, errors.New("invalid key")
		}

		return x509.MarshalECPrivateKey(privKey)
	} else {
		if pubKey == nil {
			return nil, errors.New("invalid key")
		}

		return x509.MarshalPKIXPublicKey(pubKey)
	}
}

func (keyring *Keyring) Import(index int, private bool, der []byte) (err error) {
	var pubKey *ecdsa.PublicKey
	var privKey *ecdsa.PrivateKey

	if private {
		privKey, err = x509.ParseECPrivateKey(der)
	} else {
		var k interface{}

		k, err = x509.ParsePKIXPublicKey(der)

		if err == nil {
			switch key := k.(type) {
			case *ecdsa.PublicKey:
				pubKey = key
			default:
				return errors.New("incompatible key type")
			}
		}
	}

	if err != nil {
		return
	}

	switch index {
	case UA_LONGTERM_KEY:
		keyring.ArmoryLongterm = privKey
	case MD_LONGTERM_KEY:
		keyring.MobileLongterm = pubKey
	case MD_EPHEMERAL_KEY:
		keyring.mobileEphemeral = pubKey
	default:
		return errors.New("invalid key index")
	}

	return
}

func (keyring *Keyring) NewLongtermKey() (err error) {
	keyring.ArmoryLongterm, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	return
}

func (keyring *Keyring) NewSessionKeys(nonce []byte) (err error) {
	keyring.armoryEphemeral, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	if err != nil {
		return
	}

	peerX := keyring.mobileEphemeral.X
	peerY := keyring.mobileEphemeral.Y
	privX := keyring.armoryEphemeral.D.Bytes()

	length := (keyring.mobileEphemeral.Params().BitSize + 7) >> 3
	keyring.preMaster = make([]byte, length)

	s, _ := keyring.mobileEphemeral.ScalarMult(peerX, peerY, privX)
	shared := s.Bytes()

	copy(keyring.preMaster[len(keyring.preMaster)-len(shared):], shared)

	hkdf := hkdf.New(sha256.New, keyring.preMaster, nonce, nil)

	keyring.sessionKey = make([]byte, 32)
	_, err = io.ReadFull(hkdf, keyring.sessionKey)

	return
}
