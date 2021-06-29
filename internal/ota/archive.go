// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package ota

import (
	"archive/zip"
	"bytes"
	"errors"
	"io"
)

const imxFileName = "armory-drive-signed.imx"
const proofFileName = "armory-drive.release"

func extract(buf []byte) (imx []byte, proof []byte, err error) {
	r := bytes.NewReader(buf)

	reader, err := zip.NewReader(r, r.Size())

	if err != nil {
		return
	}

	imxFile, err := reader.Open(imxFileName)

	if err != nil {
		return nil, nil, errors.New("invalid update file, missing imx file")
	}
	defer func() { _ = imxFile.Close() }() // make errcheck happy

	if imx, err = io.ReadAll(imxFile); err != nil {
		return nil, nil, errors.New("invalid update file, could not read imx file")
	}

	proofFile, err := reader.Open(proofFileName)

	if err != nil {
		return nil, nil, errors.New("invalid update file, missing proof file")
	}
	defer func() { _ = proofFile.Close() }() // make errcheck happy

	if proof, err = io.ReadAll(proofFile); err != nil {
		return nil, nil, errors.New("invalid update file, could not read proof file")
	}

	return
}
