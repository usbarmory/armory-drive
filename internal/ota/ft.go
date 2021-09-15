// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package ota

import (
	"bytes"
	"encoding/json"
	"errors"

	"github.com/f-secure-foundry/armory-drive-log/api"
	"github.com/f-secure-foundry/armory-drive-log/api/verify"
	"github.com/f-secure-foundry/armory-drive/assets"

	"github.com/f-secure-foundry/tamago/soc/imx6/dcp"

	"golang.org/x/mod/sumdb/note"
)

// proofEnabled returns whether OTA updates should be verified or not. The
// verification happens only on firmware images compiled with the necessary key
// material and when no fixup is present.
//
// Key material is cleared on unsigned/test images only while a zero fixup is
// performed only on user signed images as they cannot be authenticated with
// F-Secure own keys.
func proofEnabled() bool {
	return len(assets.LogPublicKey) != 0 && len(assets.FRPublicKey) != 0 &&
		!bytes.Equal(assets.FRPublicKey, make([]byte, len(assets.FRPublicKey)))
}

func verifyProof(imx []byte, csf []byte, proof []byte, oldProof *api.ProofBundle) (pb *api.ProofBundle, err error) {
	if len(proof) == 0 {
		return nil, errors.New("missing proof")
	}

	pb = &api.ProofBundle{}

	if err = json.Unmarshal(proof, pb); err != nil {
		return
	}

	logSigV, err := note.NewVerifier(string(assets.LogPublicKey))

	if err != nil {
		return
	}

	frSigV, err := note.NewVerifier(string(assets.FRPublicKey))

	if err != nil {
		return
	}

	var oldCP api.Checkpoint

	if oldProof != nil {
		verifiers := note.VerifierList(logSigV)

		if n, _ := note.Open(oldProof.NewCheckpoint, verifiers); n != nil {
			if err = oldCP.Unmarshal([]byte(n.Text)); err != nil {
				return
			}
		}
	}

	imxHash, err := dcp.Sum256(imx)

	if err != nil {
		return
	}

	csfHash, err := dcp.Sum256(csf)

	if err != nil {
		return
	}

	hashes := map[string][]byte{
		imxPath: imxHash[:],
		csfPath: csfHash[:],
	}

	if err = verify.Bundle(*pb, oldCP, logSigV, frSigV, hashes, assets.LogOrigin); err != nil {
		return
	}

	// leaf hashes are not needed so we can save space
	pb.LeafHashes = nil

	return
}
