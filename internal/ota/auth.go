// Copyright (c) WithSecure Corporation
// https://foundry.withsecure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

// +build !disable_fr_auth

package ota

import (
	_ "embed"
)

const DisableAuth = false

// FRPublicKey represents the firmware releases manifest authentication key.
//go:embed armory-drive.pub
var FRPublicKey []byte

// LogPublicKey represents the firmware releases transparency log.
// authentication key.
//go:embed armory-drive-log.pub
var LogPublicKey []byte
