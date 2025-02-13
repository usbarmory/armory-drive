// Copyright (c) WithSecure Corporation
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	_ "unsafe"

	"github.com/usbarmory/tamago/dma"
)

// Override standard memory allocation, as this application requires large DMA
// descriptors.

//go:linkname ramSize runtime.ramSize
var ramSize uint = 0x10000000 // 256MB
// 2nd half of external RAM (256MB)
var dmaStart uint = 0x90000000

// 256MB
var dmaSize = 0x10000000

func init() {
	dma.Init(dmaStart, dmaSize)
}
