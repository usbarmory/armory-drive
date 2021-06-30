// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package ota

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
)

const (
	imxPath   = "armory-drive.imx"
	csfPath   = "armory-drive.csf"
	proofPath = "armory-drive.release"
)

func open(reader *zip.Reader, p string) (buf []byte, err error) {
	f, err := reader.Open(p)

	if err != nil {
		return nil, fmt.Errorf("could not open %s", p)
	}
	defer f.Close()

	return io.ReadAll(f)
}

func extract(buf []byte) (imx []byte, csf []byte, proof []byte, err error) {
	r := bytes.NewReader(buf)

	reader, err := zip.NewReader(r, r.Size())

	if err != nil {
		return
	}

	if imx, err = open(reader, imxPath); err != nil {
		err = fmt.Errorf("could not open %s, %v", imxPath, err)
		return
	}

	if csf, err = open(reader, csfPath); err != nil {
		err = fmt.Errorf("could not open %s, %v", csfPath, err)
		return
	}

	if proof, err = open(reader, proofPath); err != nil {
		err = fmt.Errorf("could not open %s, %v", proofPath, err)
		return
	}

	return
}
