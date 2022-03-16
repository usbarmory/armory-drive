// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	_ "unsafe"

	"github.com/usbarmory/tamago/dma"
)

// Override usbarmory pkg ramSize and `mem` allocation, as this application
// requires large DMA descriptors.

//go:linkname ramSize runtime.ramSize
var ramSize uint32 = 0x10000000 // 256MB
// 2nd half of external RAM (256MB)
var dmaStart uint32 = 0x90000000

// 256MB
var dmaSize = 0x10000000

func init() {
	dma.Init(dmaStart, dmaSize)
}
