// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

// +build linux,ignore

package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strconv"
)

const OTAPublicKeyFileName = "armory-drive-minisign.pub"
const SRKHashFileName = "SRK_1_2_3_4_fuse.bin"

func init() {
	log.SetFlags(0)
	log.SetOutput(os.Stdout)
}

func main() {
	var err error

	var OTAPublicKey []byte
	var SRKHash []byte

	if p := os.Getenv("OTA_KEYS"); len(p) > 0 {
		OTAPublicKey, err = os.ReadFile(path.Join(p, OTAPublicKeyFileName))

		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Fatal("OTA_PUBLIC environment variable must be defined (see README.md)")
	}

	if p := os.Getenv("HAB_KEYS"); len(p) > 0 {
		SRKHash, err = ioutil.ReadFile(path.Join(p, SRKHashFileName))

		if err != nil {
			log.Fatal(err)
		}
	} else {
		log.Fatal("SRK_HASH environment variable must be defined (see README.md)")
	}

	out, err := os.Create("tmp-provisioning.go")

	if err != nil {
		log.Fatal(err)
	}

	out.WriteString(`
package assets

func init() {
`)
	out.WriteString(fmt.Sprintf("\tOTAPublicKey = []byte(%s)\n", strconv.Quote(string(OTAPublicKey))))
	out.WriteString(fmt.Sprintf("\tSRKHash = []byte(%s)\n", strconv.Quote(string(SRKHash))))

	out.WriteString(`
}
`)
}
