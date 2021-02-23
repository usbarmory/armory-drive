// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/binary"
	"regexp"
	"runtime"
	"time"

	"github.com/f-secure-foundry/tamago/board/f-secure/usbarmory/mark-two"
)

var BLEStartupPattern = regexp.MustCompile(`(\+STARTUP)`)
var BLENamePattern = regexp.MustCompile(`\+UBTLN:"([^"]+)"`)

func txPacket(ble *usbarmory.ANNA, buf []byte) {
	// detect USB armory Mk II β errata fix
	if ble.UART.Flow {
		ble.UART.Write(buf)
		return
	}

	for i := 0; i < len(buf); i++ {
		for !ble.RTS() {
		}

		ble.UART.Tx(buf[i])
	}
}

func rxPackets(ble *usbarmory.ANNA) {
	var pkt []byte
	var length uint16

	next := true

	for {
		// detect USB armory Mk II β errata fix
		if ble.UART.Flow {
			buf := make([]byte, 1024)
			n := ble.UART.Read(buf)

			if n != 0 {
				pkt = append(pkt, buf[:n]...)
			}
		} else {
			ble.CTS(true)
			c, ok := ble.UART.Rx()
			ble.CTS(false)

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
				handleEvent(ble, pkt[3:length-1])
			}

			pkt = pkt[length:]
			next = true
		}
	}
}

func rxATResponse(ble *usbarmory.ANNA, pattern *regexp.Regexp) (match [][]byte) {
	var buf []byte

	for len(match) == 0 {
		ble.CTS(true)

		c, ok := ble.UART.Rx()

		if !ok {
			continue
		}

		ble.CTS(false)

		buf = append(buf, c)
		match = pattern.FindSubmatch(buf)
	}

	return
}

func startBLE(dataMode bool) {
	usbarmory.BLE.Init()
	time.Sleep(usbarmory.RESET_GRACE_TIME)

	rxATResponse(usbarmory.BLE, BLEStartupPattern)

	usbarmory.BLE.UART.Write([]byte("AT+UBTLN?\r"))
	m := rxATResponse(usbarmory.BLE, BLENamePattern)
	remote.name = string(m[1])

	if !dataMode {
		usbarmory.LED("blue", true)
		return
	}

	// enter data mode
	usbarmory.BLE.UART.Write([]byte("ATO2\r"))
	usbarmory.LED("blue", true)

	go func() {
		rxPackets(usbarmory.BLE)
	}()
}
