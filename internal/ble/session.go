// Copyright (c) WithSecure Corporation
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package ble

import (
	"sync"
	"time"
)

type Session struct {
	sync.Mutex

	Last   int64
	Skew   time.Duration
	Active bool
	Data   []byte
}

func (s *Session) Reset() {
	s.Active = false
	s.Data = nil
}

func (s *Session) Time() int64 {
	return time.Now().Add(s.Skew).UnixNano() / (1000 * 1000)
}
