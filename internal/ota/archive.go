// Copyright (c) WithSecure Corporation
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
	imxPath = "armory-drive.imx"
	csfPath = "armory-drive.csf"
	logPath = "armory-drive.log"
)

func open(reader *zip.Reader, p string) (buf []byte, err error) {
	f, err := reader.Open(p)

	if err != nil {
		return
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

	if len(imx) == 0 {
		err = fmt.Errorf("could not open %s, empty file", imxPath)
		return
	}

	if csf, err = open(reader, csfPath); err != nil {
		err = fmt.Errorf("could not open %s, %v", csfPath, err)
		return
	}

	if len(csf) == 0 {
		err = fmt.Errorf("could not open %s, empty file", csfPath)
		return
	}

	if proofEnabled() {
		if proof, err = open(reader, logPath); err != nil {
			err = fmt.Errorf("could not open %s, %v", logPath, err)
			return
		}

		if len(proof) == 0 {
			err = fmt.Errorf("could not open %s, empty file", logPath)
			return
		}
	}

	return
}
