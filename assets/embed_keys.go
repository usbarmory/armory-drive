// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

// +build linux,ignore

package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/f-secure-foundry/armory-drive/assets"
)

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stdout)
}

func main() {
	var err error

	var OTAPublicKey []byte

	if p := os.Getenv("OTA_KEY"); len(p) > 0 {
		pub, err := os.ReadFile(p)

		if err != nil {
			log.Fatal(err)
		}

		// remove untrusted comment
		pub = bytes.TrimRight(pub, "\n\r")
		lines := bytes.Split(pub, []byte("\n"))
		OTAPublicKey = lines[len(lines)-1]

		if len(OTAPublicKey) == 0 {
			log.Fatalf("could not parse %s", p)
		}
	}

	out, err := os.Create("tmp_keys.go")

	if err != nil {
		log.Fatal(err)
	}

	out.WriteString(`
package assets

func init() {
`)
	out.WriteString(fmt.Sprintf("\tOTAPublicKey = []byte(%s)\n", strconv.Quote(string(OTAPublicKey))))
	out.WriteString(fmt.Sprintf("\tSRKHash = []byte(%s)\n", strconv.Quote(string(assets.DummySRKHash()))))
	out.WriteString(`
}
`)
}
