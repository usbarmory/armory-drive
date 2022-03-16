// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"log"
	"os"

	"github.com/usbarmory/crucible/hab"
)

func checkHABArguments() {
	if len(conf.srkKey) > 0 && len(conf.srkCrt) > 0 && len(conf.table) > 0 && conf.index > 0 {
		return
	}

	log.Fatal(secureBootHelp)
}

func genCerts() (CSFKeyPEMBlock, CSFCertPEMBlock, IMGKeyPEMBlock, IMGCertPEMBlock []byte, err error) {
	var signingKey *rsa.PrivateKey

	SRKKeyPEMBlock, err := os.ReadFile(conf.srkKey)

	if err != nil {
		return
	}

	SRKCertPEMBlock, err := os.ReadFile(conf.srkCrt)

	if err != nil {
		return
	}

	caKey, _ := pem.Decode(SRKKeyPEMBlock)

	if caKey == nil {
		err = errors.New("failed to parse SRK key PEM")
		return
	}

	caCert, _ := pem.Decode(SRKCertPEMBlock)

	if caCert == nil {
		err = errors.New("failed to parse SRK certificate PEM")
		return
	}

	ca, err := x509.ParseCertificate(caCert.Bytes)

	if err != nil {
		return
	}

	caPriv, err := x509.ParsePKCS8PrivateKey(caKey.Bytes)

	if err != nil {
		return
	}

	switch k := caPriv.(type) {
	case *rsa.PrivateKey:
		signingKey = k
	default:
		err = errors.New("failed to parse SRK key")
		return
	}

	log.Printf("generating and signing CSF keypair")
	CSFKeyPEMBlock, CSFCertPEMBlock, err = hab.NewCertificate("CSF", hab.DEFAULT_KEY_LENGTH, hab.DEFAULT_KEY_EXPIRY, ca, signingKey)

	if err != nil {
		return
	}

	log.Printf("generating and signing IMG keypair")
	IMGKeyPEMBlock, IMGCertPEMBlock, err = hab.NewCertificate("IMG", hab.DEFAULT_KEY_LENGTH, hab.DEFAULT_KEY_EXPIRY, ca, signingKey)

	return
}

func sign(assets *releaseAssets) (err error) {
	opts := hab.SignOptions{
		Index:  conf.index,
		DCD:    hab.DCD_OFFSET,
		Engine: hab.HAB_ENG_SW,
	}

	if opts.Table, err = os.ReadFile(conf.table); err != nil {
		return
	}

	log.Printf("generating ephemeral CSF/IMG certificates")
	opts.CSFKeyPEMBlock, opts.CSFCertPEMBlock, opts.IMGKeyPEMBlock, opts.IMGCertPEMBlock, err = genCerts()

	if err != nil {
		return
	}

	// On user signed releases we disable OTA authentication to
	// simplify key management. This has no security impact as the
	// executable is authenticated at boot using secure boot.
	assets.log = nil
	assets.imx = clearFRPublicKey(assets.imx, assets.frPub)

	log.Printf("generating HAB signatures")
	if assets.csf, err = hab.Sign(assets.imx, opts); err != nil {
		return
	}

	opts.SDP = true

	log.Printf("generating HAB recovery signatures")
	if assets.sdp, err = hab.Sign(assets.imx, opts); err != nil {
		return
	}

	return
}
