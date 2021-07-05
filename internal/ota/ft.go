// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package ota

import (
	"encoding/json"
	"errors"

	"github.com/f-secure-foundry/armory-drive-log/api"
	"github.com/f-secure-foundry/armory-drive-log/api/verify"
	"github.com/f-secure-foundry/armory-drive/assets"

	"github.com/f-secure-foundry/tamago/soc/imx6/dcp"

	"golang.org/x/mod/sumdb/note"
)

func verifyProof(imx []byte, csf []byte, proof []byte, oldProof *api.ProofBundle) (pb *api.ProofBundle, err error) {
	if len(proof) == 0 {
		return nil, errors.New("missing proof")
	}

	if err = json.Unmarshal(proof, pb); err != nil {
		return
	}

	var oldCP api.Checkpoint

	if oldProof != nil {
		if err = oldCP.Unmarshal(oldProof.NewCheckpoint); err != nil {
			return
		}
	}

	logSigV, err := note.NewVerifier(string(assets.LogPublicKey))

	if err != nil {
		return
	}

	frSigV, err := note.NewVerifier(string(assets.FRPublicKey))

	if err != nil {
		return
	}

	firmwareHash, err := dcp.Sum256(imx)

	if err != nil {
		return
	}

	// TODO: verify csf

	if err = verify.Bundle(*pb, oldCP, logSigV, frSigV, firmwareHash[:]); err != nil {
		return
	}

	// leaf hashes are not needed so we can save space
	pb.LeafHashes = nil

	return
}
