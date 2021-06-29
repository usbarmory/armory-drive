// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package ota

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/f-secure-foundry/armory-drive/assets"
	"github.com/f-secure-foundry/armory-drive-log/api"

	"github.com/f-secure-foundry/tamago/soc/imx6/dcp"
)

func compareHash(buf []byte, s string) (valid bool) {
	sum, err := dcp.Sum256(buf)

	if err != nil {
		return false
	}

	hash, err := hex.DecodeString(s)

	if err != nil {
		return false
	}

	return bytes.Equal(sum[:], hash)
}

func verify(imx []byte, csf []byte, proof []byte) (pb *api.ProofBundle, err error) {
	if len(assets.FRPublicKey) == 0 || len(assets.LogPublicKey) == 0 {
		return nil, errors.New("missing OTA authentication keys")
	}

	if len(proof) == 0 {
		return nil, errors.New("missing proof")
	}

	pb = &api.ProofBundle{}

	if err = json.Unmarshal(proof, &pb); err != nil {
		return
	}

	// compareHash(imx, pb.FirmwareRelease().ArtifactSHA256[imxFileName])
	// compareHash(csf, pb.FirmwareRelease().ArtifactSHA256[csfFileName])

	return nil, errors.New("TOOD")
}
