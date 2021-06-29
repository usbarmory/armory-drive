// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package ota

import (
	"encoding/json"
	"errors"

	"github.com/f-secure-foundry/armory-drive/assets"
	"github.com/f-secure-foundry/armory-drive-log/api"
)

func verify(imx []byte, proof []byte) (pb *api.ProofBundle, err error) {
	if len(assets.FRPublicKey) == 0 || len(assets.LogPublicKey) == 0 {
		return
	}

	pb = &api.ProofBundle{}

	if err = json.Unmarshal(proof, &pb); err != nil {
		return
	}

	return nil, errors.New("TOOD")
}
