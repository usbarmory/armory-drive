// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io/ioutil"
	"os"

	"github.com/mitchellh/go-fs"
	"github.com/mitchellh/go-fs/fat"
	"github.com/skip2/go-qrcode"

	"github.com/f-secure-foundry/tamago/soc/imx6/usdhc"
)

const README = `
Please download the F-Secure Armory application from the iOS App Store and scan file QR.png
`

const (
	QR_DISK_PATH       = "qr.disk"
	QR_DISK_BLOCK_SIZE = 512
	QR_DISK_SECTORS    = 16800
	QR_CODE_PATH       = "QR.png"
	QR_CODE_SIZE       = 117

	BOOT_SIGNATURE = 0xaa55

	QR_PARTITION_OFFSET = 2048 * 512
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

type QRCard struct {
	Card
	diskData []byte
}

func (q *QRCard) Detect() error {
	return nil
}

func (q *QRCard) Info() (info usdhc.CardInfo) {
	info.SD = true
	info.BlockSize = QR_DISK_BLOCK_SIZE
	info.Blocks = QR_DISK_SECTORS

	return
}

func (q *QRCard) ReadBlocks(lba int, buf []byte) (err error) {
	start := lba * QR_DISK_BLOCK_SIZE
	end := start + len(buf)

	if end > len(q.diskData) {
		return errors.New("read operation exceeds disk size")
	}

	copy(buf[:], q.diskData[start:end])

	return
}

func (q *QRCard) WriteBlocks(lba int, buf []byte) (err error) {
	start := lba * QR_DISK_BLOCK_SIZE

	if start+len(buf) > len(q.diskData) {
		return errors.New("write operation exceeds disk size")
	}

	copy(q.diskData[start:], buf)

	return
}

func QRFS() (card *QRCard) {
	code, err := QR()

	if err != nil {
		panic(err)
	}

	img, err := os.OpenFile(QR_DISK_PATH, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0600)

	if err != nil {
		panic(err)
	}

	err = img.Truncate(QR_DISK_SECTORS * QR_DISK_BLOCK_SIZE)

	if err != nil {
		panic(err)
	}

	dev, err := fs.NewFileDisk(img)

	if err != nil {
		panic(err)
	}

	conf := &fat.SuperFloppyConfig{
		FATType: fat.FAT16,
		Label:   "F-Secure",
		OEMName: "F-Secure",
	}

	err = fat.FormatSuperFloppy(dev, conf)

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

	err = addFile(root, "README.txt", []byte(README))

	if err != nil {
		panic(err)
	}

	err = addFile(root, QR_CODE_PATH, code)

	if err != nil {
		panic(err)
	}

	_ = addFile(root, "VERSION.txt", []byte(Revision))

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
	mbr.BootSignature = BOOT_SIGNATURE

	diskData := mbr.Bytes()
	diskData = append(diskData, make([]byte, QR_PARTITION_OFFSET-512)...)
	diskData = append(diskData, partitionData...)

	card = &QRCard{
		diskData: diskData,
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

func QR() (code []byte, err error) {
	// Generate a new UA longterm key, it will be saved only on successful
	// pairings.
	err = keyring.NewLongtermKey()

	if err != nil {
		return
	}

	key, err := keyring.Export(UA_LONGTERM_KEY, false)

	if err != nil {
		return
	}

	pb := &PairingQRCode{
		BLEName: remote.name,
		Nonce:   remote.pairingNonce,
		PubKey:  key,
	}

	err = pb.Sign()

	if err != nil {
		return
	}

	qr, err := qrcode.New(string(pb.Bytes()), qrcode.Medium)

	if err != nil {
		return
	}

	return qr.PNG(QR_CODE_SIZE)
}
