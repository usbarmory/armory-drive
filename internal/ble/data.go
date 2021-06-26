// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package ble

import (
	"bytes"
	"encoding/binary"
)

const (
	// Packet                      (4 bytes + []byte) max: 251
	//   Data Event | Data Command (3 bytes + []byte) max: 247
	//      Fragment               (2 bytes + []byte) max: 244
	//        protobuf                                max: 242

	PAYLOAD_MAX_LENGTH  = 247
	FRAGMENT_MAX_LENGTH = 244
	PROTOBUF_MAX_LENGTH = 242

	EDM_START = 0xAA
	EDM_STOP  = 0x55

	DATA_EVENT   = 0x31
	DATA_COMMAND = 0x36
)

type Packet struct {
	Start   uint8
	Length  uint16
	Payload []byte
	Stop    uint8
}

func (pkt *Packet) SetDefaults() {
	pkt.Start = EDM_START
	pkt.Stop = EDM_STOP
}

func (pkt *Packet) SetPayload(buf []byte) {
	pkt.Length = uint16(len(buf))
	pkt.Payload = buf
}

func (pkt *Packet) Bytes() []byte {
	buf := new(bytes.Buffer)

	binary.Write(buf, binary.BigEndian, pkt.Start)
	binary.Write(buf, binary.BigEndian, pkt.Length)
	buf.Write(pkt.Payload)
	binary.Write(buf, binary.BigEndian, pkt.Stop)

	return buf.Bytes()
}

type Data struct {
	Kind      uint16
	ChannelId uint8
	Data      []byte
}

func (cmd *Data) SetDefaults() {
	cmd.Kind = DATA_COMMAND
}

func (cmd *Data) Bytes() []byte {
	buf := new(bytes.Buffer)

	binary.Write(buf, binary.BigEndian, cmd.Kind)
	binary.Write(buf, binary.BigEndian, cmd.ChannelId)
	buf.Write(cmd.Data)

	return buf.Bytes()
}

type Fragment struct {
	Total uint8
	Seq   uint8
	Data  []byte
}

func (frg *Fragment) Parse(data []byte) {
	frg.Total = uint8(data[0])
	frg.Seq = uint8(data[1])
	frg.Data = data[2:]
}

func (frg *Fragment) Bytes() []byte {
	buf := new(bytes.Buffer)

	buf.WriteByte(frg.Total)
	buf.WriteByte(frg.Seq)
	buf.Write(frg.Data)

	return buf.Bytes()
}

func handleFragment(data []byte) (event []byte) {
	fragment := &Fragment{}
	fragment.Parse(data)

	if fragment.Total == 1 {
		return fragment.Data
	}

	if fragment.Seq > 1 && len(data) == 0 || fragment.Seq > fragment.Total {
		data = nil
		return
	}

	if fragment.Seq == 1 {
		data = make([]byte, fragment.Total*PROTOBUF_MAX_LENGTH)
	}

	data = append(data, fragment.Data...)

	if fragment.Seq == fragment.Total {
		event = data
		data = nil
	}

	return
}

func (b *BLE) handleEvent(buf []byte) {
	var fragments [][]byte

	if len(buf) < 3+2 {
		return
	}

	// decode Data Event
	kind := binary.BigEndian.Uint16(buf[0:2])
	channel := buf[2]
	data := buf[3:]

	if kind != DATA_EVENT {
		return
	}

	event := handleFragment(data)

	if len(event) == 0 {
		return
	}

	res := b.eventHandler(event)

	for i := 0; i < len(res); i += PROTOBUF_MAX_LENGTH {
		if i+PROTOBUF_MAX_LENGTH > len(res) {
			fragments = append(fragments, res[i:])
		} else {
			fragments = append(fragments, res[i:PROTOBUF_MAX_LENGTH])
		}
	}

	for i := range fragments {
		fragment := &Fragment{
			Total: uint8(len(fragments)),
			Seq:   uint8(i + 1),
			Data:  fragments[i],
		}

		// prepare Data Command
		payload := &Data{}
		payload.SetDefaults()
		payload.ChannelId = channel
		payload.Data = fragment.Bytes()

		// prepare response Packet
		pkt := &Packet{}
		pkt.SetDefaults()
		pkt.SetPayload(payload.Bytes())

		b.txPacket(pkt.Bytes())
	}
}
