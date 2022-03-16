// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package ums

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io/ioutil"
	"os"

	"github.com/usbarmory/armory-drive/assets"
	"github.com/usbarmory/armory-drive/internal/crypto"

	"github.com/usbarmory/tamago/soc/imx6/usdhc"

	"github.com/mitchellh/go-fs"
	"github.com/mitchellh/go-fs/fat"
)

const readme = `
Please download the F-Secure Armory Drive application from the iOS App Store
and scan file QR.png
`

// pairing disk paths (8.3 format)
const (
	codePath       = "QR.PNG"
	readmePath     = "README.TXT"
	versionPath    = "VERSION.TXT"
	checkpointPath = "LASTCHKP.BIN"
)

const (
	blockSize = 512

	pairingDiskPath   = "pairing.disk"
	pairingDiskOffset = 2048 * blockSize
	pairingDiskBlocks = 16800

	bootSignature = 0xaa55
)

type MBR struct {
	Bootstrap     [446]byte
	Partitions    [4]Partition
	BootSignature uint16
}

type Partition struct {
	Status   byte
	FirstCHS [3]byte
	Type     byte
	LastCHS  [3]byte
	FirstLBA [4]byte
	Sectors  [4]byte
}

func (mbr *MBR) Bytes() []byte {
	buf := new(bytes.Buffer)
	binary.Write(buf, binary.LittleEndian, mbr)
	return buf.Bytes()
}

type PairingDisk struct {
	Data []byte
}

func (q *PairingDisk) Detect() error {
	return nil
}

func (q *PairingDisk) Info() (info usdhc.CardInfo) {
	info.SD = true
	info.BlockSize = blockSize
	info.Blocks = pairingDiskBlocks

	return
}

func (q *PairingDisk) ReadBlocks(lba int, buf []byte) (err error) {
	start := lba * blockSize
	end := start + len(buf)

	if end > len(q.Data) {
		return errors.New("read operation exceeds disk size")
	}

	copy(buf[:], q.Data[start:end])

	return
}

func (q *PairingDisk) WriteBlocks(lba int, buf []byte) (err error) {
	start := lba * blockSize

	if start+len(buf) > len(q.Data) {
		return errors.New("write operation exceeds disk size")
	}

	copy(q.Data[start:], buf)

	return
}

func Pairing(code []byte, keyring *crypto.Keyring) (card *PairingDisk) {
	img, err := os.OpenFile(pairingDiskPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600)

	if err != nil {
		panic(err)
	}

	if err = img.Truncate(pairingDiskBlocks * blockSize); err != nil {
		panic(err)
	}

	dev, err := fs.NewFileDisk(img)

	if err != nil {
		panic(err)
	}

	conf := &fat.SuperFloppyConfig{
		FATType: fat.FAT16,
		Label:   VendorID,
		OEMName: VendorID,
	}

	if err = fat.FormatSuperFloppy(dev, conf); err != nil {
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

	if len(code) > 0 {
		if err = addFile(root, codePath, code); err != nil {
			panic(err)
		}

		_ = addFile(root, readmePath, []byte(readme))
	}

	_ = addFile(root, versionPath, []byte(assets.Revision))

	if pb := keyring.Conf.ProofBundle; pb != nil {
		if err = addFile(root, checkpointPath, pb.NewCheckpoint); err != nil {
			panic(err)
		}
	}

	img.Close()

	partitionData, err := ioutil.ReadFile(img.Name())

	if err != nil {
		panic(err)
	}

	// go-fs implements a partition-less msdos floppy, therefore we must
	// move its partition in a partitioned disk.
	partition := Partition{
		FirstCHS: [3]byte{0x00, 0x21, 0x18},
		Type:     0x06,
		LastCHS:  [3]byte{0x01, 0x2a, 0xc7},
		FirstLBA: [4]byte{0x00, 0x08, 0x00, 0x00},
		Sectors:  [4]byte{0xa0, 0x39, 0x00, 0x00},
	}

	mbr := &MBR{}
	mbr.Partitions[0] = partition
	mbr.BootSignature = bootSignature

	data := mbr.Bytes()
	data = append(data, make([]byte, pairingDiskOffset-blockSize)...)
	data = append(data, partitionData...)

	card = &PairingDisk{
		Data: data,
	}

	return
}

func addFile(root fs.Directory, path string, data []byte) (err error) {
	entry, err := root.AddFile(path)

	if err != nil {
		return
	}

	file, err := entry.File()

	if err != nil {
		return
	}

	_, err = file.Write(data)

	return
}
