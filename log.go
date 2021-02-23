// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"log"
)

// initialized at compile time (see Makefile)
var Build string
var Revision string

func init() {
	log.SetFlags(0)
}
