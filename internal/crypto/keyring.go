// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package crypto

import (
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"errors"
	"io"
	"sync"

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
	// FDE function
	Cipher func(buf []byte, lba int, blocks int, blockSize int, enc bool, wg *sync.WaitGroup)

	// Configuration instance
	Conf *PersistentConfiguration

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
}

func (k *Keyring) Init(overwrite bool) (err error) {
	// derive persistent storage encryption key
	if k.snvs, err = k.deriveKey([]byte(SNVS_DIV), SNVS_KEY, true); err != nil {
		return
	}

	err = k.Load()

	if err != nil || overwrite {
		err = k.reset()

		if err != nil {
			return
		}
	}

	err = k.Import(UA_LONGTERM_KEY, true, k.Conf.ArmoryLongterm)

	if err != nil {
		return
	}

	// we might not be paired yet, so ignore errors
	k.Import(MD_LONGTERM_KEY, false, k.Conf.MobileLongterm)

	// Derive salt, used for ESSIV computation as well as BLOCK_KEY derivation.
	if k.salt, err = k.deriveKey([]byte(ESSIV_DIV), ESSIV_KEY, true); err != nil {
		return
	}

	return
}

func (k *Keyring) Export(index int, private bool) ([]byte, error) {
	var pubKey *ecdsa.PublicKey
	var privKey *ecdsa.PrivateKey

	switch index {
	case UA_LONGTERM_KEY:
		privKey = k.ArmoryLongterm
	case UA_EPHEMERAL_KEY:
		privKey = k.armoryEphemeral
	case MD_LONGTERM_KEY:
		pubKey = k.MobileLongterm
	case MD_EPHEMERAL_KEY:
		pubKey = k.mobileEphemeral
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

func (k *Keyring) Import(index int, private bool, der []byte) (err error) {
	var pubKey *ecdsa.PublicKey
	var privKey *ecdsa.PrivateKey

	if private {
		privKey, err = x509.ParseECPrivateKey(der)
	} else {
		var pk interface{}

		pk, err = x509.ParsePKIXPublicKey(der)

		if err == nil {
			switch key := pk.(type) {
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
		k.ArmoryLongterm = privKey
	case MD_LONGTERM_KEY:
		k.MobileLongterm = pubKey
	case MD_EPHEMERAL_KEY:
		k.mobileEphemeral = pubKey
	default:
		return errors.New("invalid key index")
	}

	return
}

func (k *Keyring) NewLongtermKey() (err error) {
	k.ArmoryLongterm, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	return
}

func (k *Keyring) NewSessionKeys(nonce []byte) (err error) {
	k.armoryEphemeral, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	if err != nil {
		return
	}

	peerX := k.mobileEphemeral.X
	peerY := k.mobileEphemeral.Y
	privX := k.armoryEphemeral.D.Bytes()

	length := (k.mobileEphemeral.Params().BitSize + 7) >> 3
	k.preMaster = make([]byte, length)

	s, _ := k.mobileEphemeral.ScalarMult(peerX, peerY, privX)
	shared := s.Bytes()

	copy(k.preMaster[len(k.preMaster)-len(shared):], shared)

	hkdf := hkdf.New(sha256.New, k.preMaster, nonce, nil)

	k.sessionKey = make([]byte, 32)
	_, err = io.ReadFull(hkdf, k.sessionKey)

	return
}

func (k *Keyring) ClearSessionKeys() {
	k.sessionKey = []byte{}
	k.armoryEphemeral = nil
	k.mobileEphemeral = nil
}
