// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package ble

import (
	"bytes"
	"encoding/binary"
	"regexp"
	"runtime"
	"time"

	"github.com/usbarmory/armory-drive/internal/crypto"
	"github.com/usbarmory/armory-drive/internal/ums"

	"github.com/usbarmory/tamago/board/f-secure/usbarmory/mark-two"
)

var BLEStartupPattern = regexp.MustCompile(`(\+STARTUP)`)
var BLENamePattern = regexp.MustCompile(`\+UBTLN:"([^"]+)"`)

type eventHandler func([]byte) []byte

type BLE struct {
	Drive   *ums.Drive
	Keyring *crypto.Keyring

	name    string
	session *Session

	pairingMode  bool
	pairingNonce uint64

	anna *usbarmory.ANNA
	data []byte
}

func (b *BLE) txPacket(buf []byte) {
	// detect USB armory Mk II β errata fix
	if b.anna.UART.Flow {
		b.anna.UART.Write(buf)
		return
	}

	for i := 0; i < len(buf); i++ {
		for !b.anna.RTS() {
		}

		b.anna.UART.Tx(buf[i])
	}
}

func (b *BLE) rxPackets() {
	var pkt []byte
	var length uint16

	next := true

	for {
		// detect USB armory Mk II β errata fix
		if b.anna.UART.Flow {
			buf := make([]byte, 1024)
			n := b.anna.UART.Read(buf)

			if n != 0 {
				pkt = append(pkt, buf[:n]...)
			}
		} else {
			b.anna.CTS(true)
			c, ok := b.anna.UART.Rx()
			b.anna.CTS(false)

			if ok {
				pkt = append(pkt, c)
			}
		}

		// look for the beginning of packet
		if next {
			i := bytes.IndexByte(pkt, EDM_START)

			if i < 0 {
				pkt = []byte{}

				runtime.Gosched()
				continue
			}

			pkt = pkt[i:]
			next = false
			length = 0
		}

		if len(pkt) >= 3 && length == 0 {
			length = binary.BigEndian.Uint16(pkt[1:3])

			if length == 0 || length > PAYLOAD_MAX_LENGTH {
				pkt = []byte{}
				next = true
			} else {
				// from payload length to packet length
				length += 4
			}
		} else if length != 0 && len(pkt) >= int(length) {
			if pkt[length-1] == EDM_STOP {
				b.handleEvent(pkt[3 : length-1])
			}

			pkt = pkt[length:]
			next = true
		}
	}
}

func (b *BLE) rxATResponse(pattern *regexp.Regexp) (match [][]byte) {
	var buf []byte

	for len(match) == 0 {
		b.anna.CTS(true)

		c, ok := b.anna.UART.Rx()

		if !ok {
			continue
		}

		b.anna.CTS(false)

		buf = append(buf, c)
		match = pattern.FindSubmatch(buf)
	}

	return
}

func (b *BLE) Init() (err error) {
	b.anna = usbarmory.BLE

	if err = b.anna.Init(); err != nil {
		return
	}

	time.Sleep(usbarmory.RESET_GRACE_TIME)
	b.rxATResponse(BLEStartupPattern)

	b.anna.UART.Write([]byte("AT+UBTLN?\r"))
	m := b.rxATResponse(BLENamePattern)

	b.name = string(m[1])
	b.session = &Session{}

	// enter data mode
	b.anna.UART.Write([]byte("ATO2\r"))

	usbarmory.LED("blue", true)

	go func() {
		b.rxPackets()
	}()

	return
}
