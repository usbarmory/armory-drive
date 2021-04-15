// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"math/big"
	"sync"

	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/xts"

	"github.com/f-secure-foundry/tamago/dma"
	"github.com/f-secure-foundry/tamago/soc/imx6/dcp"
)

const (
	// flag to select DCP for on supported block ciphers
	DCP = true
	// flag to allow DCP, when flagged, for XTS computation
	DCPXTS = false
	// flag to select DCP for ESSIV computation
	DCPIV = false

	// key derivation iteration count
	PBKDF2_ITER = 4096

	// DEK key derivation diversifier
	DEK_DIV = "floppyDEK"
	// ESSIV key derivation diversifier
	ESSIV_DIV = "floppyESSIV"
	// SNVS key derivation diversifier
	SNVS_DIV = "floppySNVS"
)

// flag to select ESSIV on AES-128 CBC ciphers
var ESSIV bool

// IV buffer
var iv = make([]byte, aes.BlockSize)

// IV encryption IV for ESSIV computation and IV reset
var zero = make([]byte, aes.BlockSize)

var cipherFn func(buf []byte, lba int, blocks int, blockSize int, enc bool, wg *sync.WaitGroup)

func init() {
	dcp.Init()
}

func deriveKey(diversifier []byte, index int, export bool) (key []byte, err error) {
	if index == BLOCK_KEY {
		var armoryLongterm []byte

		// We want to diversify block cipher key derivation across different
		// pairings, to do so we combine the diversifier with the UA long term
		// public key, which is recreated at each pairing.
		armoryLongterm, err = keyring.Export(UA_LONGTERM_KEY, false)

		if err != nil {
			return
		}

		diversifier = append(diversifier, armoryLongterm...)

		// We re-use the ESSIV "salt" (unfortunate name collision here, it's
		// not actually the PBKDF2 salt, or a salt at all) as it is random and
		// unknown, the PBKDF2 salt is random but known (as it should be).
		diversifier = pbkdf2.Key(keyring.salt, diversifier, PBKDF2_ITER, aes.BlockSize, sha256.New)
	}

	// It is advised to use only deterministic input data for key
	// derivation, therefore we use the empty allocated IV before it being
	// filled.
	iv := make([]byte, aes.BlockSize)

	if export {
		key, err = dcp.DeriveKey(diversifier, iv, -1)
	} else {
		_, err = dcp.DeriveKey(diversifier, iv, index)
	}

	if err != nil {
		return
	}

	if export {
		err = dcp.SetKey(index, key)
	}

	return
}

func setCipher(kind Cipher, diversifier []byte) (err error) {
	var dek []byte

	// We need to zero out the IV buffer when switching away from ESSIV, as
	// it fills its entirety.
	ESSIV = false
	copy(iv, zero)

	switch kind {
	case Cipher_AES128_CBC_PLAIN, Cipher_AES128_CBC_ESSIV:
		if kind == Cipher_AES128_CBC_ESSIV {
			ESSIV = true
		}

		if DCP {
			_, err = deriveKey(diversifier, BLOCK_KEY, false)

			cipherFn = cipherDCP
		} else {
			dek, err = deriveKey(diversifier, BLOCK_KEY, true)

			if err != nil {
				return
			}

			keyring.cb, err = aes.NewCipher(dek)

			cipherFn = cipherAES
		}

		if err != nil {
			return
		}

		if ESSIV && !DCPIV {
			keyring.cbiv, err = aes.NewCipher(keyring.salt)
		}
	case Cipher_AES128_XTS_PLAIN, Cipher_AES256_XTS_PLAIN:
		var size int
		cbxts := aes.NewCipher

		if kind == Cipher_AES256_XTS_PLAIN {
			size = 32 * 2
		} else {
			size = 16 * 2

			if DCPXTS && DCP {
				cbxts = NewDCPCipher
			}
		}

		dek, err = deriveKey(diversifier, BLOCK_KEY, true)

		if err != nil {
			return
		}

		dk := pbkdf2.Key(dek, keyring.salt, PBKDF2_ITER, size, sha256.New)
		keyring.cbxts, err = xts.NewCipher(cbxts, dk)

		if err != nil {
			return
		}

		cipherFn = cipherXTS
	case Cipher_NONE:
		deriveKey(zero, BLOCK_KEY, false)
		keyring.cb = nil
		keyring.cbxts = nil
	default:
		err = errors.New("unsupported cipher")
	}

	return
}

// equivalent to aes-cbc-essiv:md5
func essiv(buf []byte, iv []byte) (err error) {
	if DCPIV {
		err = dcp.Encrypt(buf, ESSIV_KEY, iv)
	} else {
		encrypter := cipher.NewCBCEncrypter(keyring.cbiv, iv)
		encrypter.CryptBlocks(buf, buf)
	}

	return
}

// equivalent to aes-cbc-plain (hw)
func cipherDCP(buf []byte, lba int, blocks int, blockSize int, enc bool, wg *sync.WaitGroup) {
	payloads, ivs := dma.Reserve(blocks*aes.BlockSize, 4)
	defer dma.Release(payloads)

	for i := 0; i < blocks; i++ {
		off := i * aes.BlockSize

		// fill unused 64-bits as reserved buffers are not initialized
		binary.BigEndian.PutUint64(ivs[off+8:], 0)
		binary.BigEndian.PutUint64(ivs[off:], uint64(lba+i))

		if ESSIV {
			if err := essiv(ivs[off:], zero); err != nil {
				panic(err)
			}
		}
	}

	err := dcp.CipherChain(buf, ivs, blocks, blockSize, BLOCK_KEY, enc)

	if err != nil {
		panic(err)
	}

	if wg != nil {
		wg.Done()
	}
}

// equivalent to aes-cbc-plain (sw)
func cipherAES(buf []byte, lba int, blocks int, blockSize int, enc bool, wg *sync.WaitGroup) {
	for i := 0; i < blocks; i++ {
		start := i * blockSize
		end := start + blockSize
		slice := buf[start:end]

		binary.BigEndian.PutUint64(iv, uint64(lba+i))

		if ESSIV {
			if err := essiv(iv, zero); err != nil {
				panic(err)
			}
		}

		var mode cipher.BlockMode

		if enc {
			mode = cipher.NewCBCEncrypter(keyring.cb, iv)
		} else {
			mode = cipher.NewCBCDecrypter(keyring.cb, iv)
		}

		mode.CryptBlocks(slice, slice)
	}

	if wg != nil {
		wg.Done()
	}
}

// equivalent to aes-xts-plain64 (sw)
func cipherXTS(buf []byte, lba int, blocks int, blockSize int, enc bool, wg *sync.WaitGroup) {
	for i := 0; i < blocks; i++ {
		start := i * blockSize
		end := start + blockSize
		slice := buf[start:end]

		if enc {
			keyring.cbxts.Encrypt(slice, slice, uint64(lba+i))
		} else {
			keyring.cbxts.Decrypt(slice, slice, uint64(lba+i))
		}
	}

	if wg != nil {
		wg.Done()
	}
}

func encryptSNVS(input []byte, length int) (output []byte, err error) {
	block, err := aes.NewCipher(keyring.snvs)

	if err != nil {
		return
	}

	iv := rng(aes.BlockSize)
	// pad to block size, accounting for IV and HMAC length
	length -= len(iv) + sha256.Size

	if len(input) < length {
		pad := make([]byte, length-len(input))
		input = append(input, pad...)
	}

	output = iv

	mac := hmac.New(sha256.New, keyring.snvs)
	mac.Write(iv)

	stream := cipher.NewOFB(block, iv)
	output = append(output, make([]byte, len(input))...)

	stream.XORKeyStream(output[len(iv):], input)
	mac.Write(output[len(iv):])

	output = append(output, mac.Sum(nil)...)

	return
}

func decryptSNVS(input []byte) (output []byte, err error) {
	if len(input) < aes.BlockSize {
		return nil, errors.New("invalid length for decrypt")
	}

	iv := input[0:aes.BlockSize]
	input = input[aes.BlockSize:]

	block, err := aes.NewCipher(keyring.snvs)

	if err != nil {
		return
	}

	mac := hmac.New(sha256.New, keyring.snvs)
	mac.Write(iv)

	if len(input) < mac.Size() {
		return nil, errors.New("invalid length for decrypt")
	}

	inputMac := input[len(input)-mac.Size():]
	mac.Write(input[0 : len(input)-mac.Size()])

	if !hmac.Equal(inputMac, mac.Sum(nil)) {
		return nil, errors.New("invalid HMAC")
	}

	stream := cipher.NewOFB(block, iv)
	output = make([]byte, len(input)-mac.Size())

	stream.XORKeyStream(output, input[0:len(input)-mac.Size()])

	return
}

func encryptOFB(plaintext []byte) (ciphertext []byte, err error) {
	if !remote.session {
		return nil, errors.New("invalid session")
	}

	block, err := aes.NewCipher(keyring.sessionKey)

	if err != nil {
		return
	}

	in := bytes.NewReader(plaintext)
	out := new(bytes.Buffer)

	iv := rng(aes.BlockSize)
	stream := cipher.NewOFB(block, iv)
	reader := &cipher.StreamReader{S: stream, R: in}

	_, err = io.Copy(out, reader)

	if err == nil {
		ciphertext = iv
		ciphertext = append(ciphertext, out.Bytes()...)
	}

	return
}

func decryptOFB(ciphertext []byte) (plaintext []byte, err error) {
	if !remote.session {
		return nil, errors.New("invalid session")
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("invalid message")
	}

	block, err := aes.NewCipher(keyring.sessionKey)

	if err != nil {
		return
	}

	iv := ciphertext[0:aes.BlockSize]
	in := bytes.NewReader(ciphertext[aes.BlockSize:])
	out := new(bytes.Buffer)

	stream := cipher.NewOFB(block, iv)
	writer := &cipher.StreamWriter{S: stream, W: out}

	_, err = io.Copy(writer, in)

	if err == nil {
		plaintext = out.Bytes()
	}

	return
}

func signECDSA(data []byte) (sig *Signature, err error) {
	var sigKey *ecdsa.PrivateKey

	if remote.session {
		sigKey = keyring.armoryEphemeral
	} else {
		sigKey = keyring.ArmoryLongterm
	}

	h := sha256.New()
	h.Write(data)
	sum := h.Sum(nil)

	r, s, err := ecdsa.Sign(rand.Reader, sigKey, sum)

	if err != nil {
		return
	}

	sig = &Signature{
		Data: sum,
		R:    r.Bytes(),
		S:    s.Bytes(),
	}

	return
}

func verifyECDSA(data []byte, sig *Signature) (err error) {
	var verKey *ecdsa.PublicKey

	if remote.session {
		verKey = keyring.mobileEphemeral
	} else {
		verKey = keyring.MobileLongterm
	}

	h := sha256.New()
	h.Write(data)

	if !bytes.Equal(sig.Data, h.Sum(nil)) {
		return errors.New("signature error, data mismatch")
	}

	R := big.NewInt(0)
	S := big.NewInt(0)

	R.SetBytes(sig.R)
	S.SetBytes(sig.S)

	valid := ecdsa.Verify(verKey, sig.Data, R, S)

	if !valid {
		return errors.New("signature error, invalid")
	}

	return
}

func rng(n int) []byte {
	buf := make([]byte, n)
	_, err := rand.Read(buf)

	if err != nil {
		panic(err)
	}

	return buf
}
