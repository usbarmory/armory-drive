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
	"github.com/f-secure-foundry/armory-drive/internal/crypto"

	"github.com/f-secure-foundry/tamago/soc/imx6/dcp"

	"golang.org/x/mod/sumdb/note"
)

func verifyProof(imx []byte, csf []byte, proof []byte, keyring *crypto.Keyring) (err error) {
	if len(proof) == 0 {
		return errors.New("missing proof")
	}

	pb := &api.ProofBundle{}

	if err = json.Unmarshal(proof, pb); err != nil {
		return
	}

	var oldCP api.Checkpoint

	if keyring.Conf.ProofBundle != nil {
		if err = oldCP.Unmarshal(keyring.Conf.ProofBundle.NewCheckpoint); err != nil {
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

	keyring.Conf.ProofBundle = pb
	keyring.Save()

	return
}
