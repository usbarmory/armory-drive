// Copyright (c) WithSecure Corporation
// https://foundry.withsecure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package api

import (
	"google.golang.org/protobuf/proto"
)

func (msg *Message) Bytes() (buf []byte) {
	buf, _ = proto.Marshal(msg)
	return
}

func (env *Envelope) Bytes() (buf []byte) {
	buf, _ = proto.Marshal(env)
	return
}

func (kex *KeyExchange) Bytes() (buf []byte) {
	buf, _ = proto.Marshal(kex)
	return
}

func (qr *PairingQRCode) Bytes() (buf []byte) {
	buf, _ = proto.Marshal(qr)
	return
}

func (status *Status) Bytes() (buf []byte) {
	buf, _ = proto.Marshal(status)
	return
}
