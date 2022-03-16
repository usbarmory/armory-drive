// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package ota

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"time"

	"github.com/usbarmory/armory-drive-log/api"
	"github.com/usbarmory/armory-drive/internal/crypto"

	"github.com/usbarmory/tamago/board/f-secure/usbarmory/mark-two"

	"github.com/mitchellh/go-fs"
	"github.com/mitchellh/go-fs/fat"
)

const updatePath = "UPDATE.ZIP"

func Check(buf []byte, path string, off int, keyring *crypto.Keyring) {
	img, err := os.OpenFile(path, os.O_RDWR|os.O_TRUNC, 0600)

	if err != nil {
		panic(err)
	}

	if _, err = img.Write(buf[off:]); err != nil {
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
		if entry.Name() == updatePath {
			update(entry, keyring)
			return
		}
	}
}

func update(entry fs.DirectoryEntry, keyring *crypto.Keyring) {
	var err error
	var exit = make(chan bool)

	defer func() {
		exit <- true
	}()

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

				if err != nil {
					log.Printf("firmware update error, %v", err)
					usbarmory.LED("blue", true)
				}

				return
			default:
			}

			on = !on
			usbarmory.LED("white", on)

			runtime.Gosched()
			time.Sleep(100 * time.Millisecond)
		}
	}()

	buf, err := ioutil.ReadAll(file)

	if err != nil {
		err = errors.New("could not read update")
		return
	}

	imx, csf, proof, err := extract(buf)

	if err != nil {
		err = fmt.Errorf("could not extract archive, %v", err)
		return
	}

	if len(proof) > 0 {
		var pb *api.ProofBundle

		// firmware authentication
		pb, err = verifyProof(imx, csf, proof, keyring.Conf.ProofBundle)

		if err != nil {
			err = fmt.Errorf("could not verify proof, %v", err)
			return
		}

		keyring.Conf.ProofBundle = pb
		keyring.Save()
	}

	// append HAB signature
	imx = append(imx, csf...)

	if err = usbarmory.MMC.WriteBlocks(2, imx); err != nil {
		err = fmt.Errorf("could not write to MMC, %v", err)
		return
	}

	log.Println("firmware update complete")

	usbarmory.LED("blue", false)
	usbarmory.LED("white", false)
}
