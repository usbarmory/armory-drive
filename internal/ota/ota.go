// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package ota

import (
	"os"
	"io/ioutil"
	"log"
	"runtime"
	"time"

	"github.com/f-secure-foundry/tamago/board/f-secure/usbarmory/mark-two"

	"github.com/mitchellh/go-fs"
	"github.com/mitchellh/go-fs/fat"
)

const updatePath  = "UPDATE.ZIP"

func Check(buf []byte, path string, off int) {
	img, err := os.OpenFile(path, os.O_RDWR|os.O_TRUNC, 0600)

	if err != nil {
		panic(err)
	}

	_, err = img.Write(buf[off:])

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
		if entry.Name() == updatePath {
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

	log.Println("extracting OTA file")

	imx, _, err := extract(buf)

	if err != nil {
		panic(err)
	}

	// if !ota.Verify(hash, proof) {
	//	panic("invalid firmware signature")
	// }

	//conf.UpdateProofBundle(proof)

	log.Println("flashing IMX file")

	err = usbarmory.MMC.WriteBlocks(2, imx)

	if err != nil {
		panic(err)
	}

	log.Println("OTA complete")

	exit <- true

	usbarmory.LED("blue", false)
	usbarmory.LED("white", false)
}
