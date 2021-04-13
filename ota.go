// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/f-secure-foundry/armory-drive/assets"

	"github.com/f-secure-foundry/armory-drive/minisign"
	"github.com/f-secure-foundry/tamago/board/f-secure/usbarmory/mark-two"

	"github.com/mitchellh/go-fs"
	"github.com/mitchellh/go-fs/fat"
)

const sigLimit = 1024

const OTAName = "UA-DRIVE.OTA"

func ota() {
	img, err := os.OpenFile(QR_DISK_PATH, os.O_RDWR|os.O_TRUNC, 0600)

	if err != nil {
		panic(err)
	}

	card := cards[0].(*QRCard)
	_, err = img.Write(card.diskData[QR_PARTITION_OFFSET:])

	if err != nil {
		panic(err)
	}

	img.Seek(0, 0)

	dev, err := fs.NewFileDisk(img)

	if err != nil {
		panic(err)
	}

	f, err := fat.New(dev)

	if err != nil {
		panic(err)
	}

	root, err := f.RootDir()

	if err != nil {
		panic(err)
	}

	for _, entry := range root.Entries() {
		if entry.Name() == OTAName {
			update(entry)
			return
		}
	}
}

func update(entry fs.DirectoryEntry) {
	var exit = make(chan bool)

	file, err := entry.File()

	if err != nil {
		panic(err)
	}

	go func() {
		var on bool

		for {
			select {
			case <-exit:
				usbarmory.LED("white", false)
				return
			default:
			}

			on = !on
			usbarmory.LED("white", on)

			runtime.Gosched()
			time.Sleep(1 * time.Second)
		}
	}()

	buf, err := ioutil.ReadAll(file)

	if err != nil {
		panic(err)
	}

	valid, bin, err := verify(buf)

	if err != nil {
		panic(err)
	}

	if !valid {
		panic("invalid firmware signature")
	}

	err = usbarmory.MMC.WriteBlocks(2, bin)

	if err != nil {
		panic(err)
	}

	exit <- true

	usbarmory.LED("blue", false)
	usbarmory.LED("white", false)
}

func verify(buf []byte) (valid bool, bin []byte, err error) {
	if bytes.Equal(assets.OTAPublicKey, make([]byte, len(assets.OTAPublicKey))) {
		// If there is no valid OTA public key we assume that no OTA
		// signature is present.
		return true, buf, nil
	}

	if len(buf) < sigLimit {
		return false, nil, errors.New("invalid signature")
	}

	sig, off, err := minisign.DecodeSignature(string(buf[0:sigLimit]))

	if err != nil {
		return false, nil, fmt.Errorf("invalid signature, %v", err)
	}

	pub, err := minisign.NewPublicKey(string(assets.OTAPublicKey))

	if err != nil {
		return false, nil, fmt.Errorf("invalid public key, %v", err)
	}

	// The fat package from go-fs reads back files adding padding to 4096,
	// invalidating the signature. Therefore we receive a hint on the
	// actual payload length in an untrusted comment.
	n, err := strconv.Atoi(strings.TrimPrefix(sig.UntrustedComment, "untrusted comment: "))

	if err != nil {
		panic(err)
	}

	if n > len(buf[off:]) {
		panic("invalid signature")
	}

	bin = buf[off : off+n]
	valid, err = pub.Verify(bin, sig)

	return
}
