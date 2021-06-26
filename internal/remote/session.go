// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package remote

import (
	"sync"
	"time"
)

type Session struct {
	sync.Mutex

	PeerName string

	Last   int64
	Skew   time.Duration
	Active bool

	PairingMode  bool
	PairingNonce uint64

	Data []byte
}

func (s *Session) Reset() {
	s.Active = false
	s.Data = nil
}

func (s *Session) Time() int64 {
	return time.Now().Add(s.Skew).UnixNano() / (1000 * 1000)
}
