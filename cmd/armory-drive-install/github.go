// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"

	"github.com/google/go-github/v34/github"
)

const org = "f-secure-foundry"
const repo = "armory-drive"

func downloadLatestRelease() (imx []byte, csf []byte, sig []byte, sdp []byte, err error) {
	var release *github.RepositoryRelease

	client := github.NewClient(nil)

	if conf.releaseVersion == "latest" {
		release, _, err = client.Repositories.GetLatestRelease(context.Background(), org, repo)
	} else {
		release, _, err = client.Repositories.GetReleaseByTag(context.Background(), org, repo, conf.releaseVersion)
	}

	if err != nil {
		return
	}

	tagName := release.GetTagName()

	for _, asset := range release.Assets {
		switch *asset.Name {
		case tagName + ".imx":
			if imx, err = download("release", release, asset); err != nil {
				return
			}
		case tagName + ".csf":
			if csf, err = download("HAB signature", release, asset); err != nil {
				return
			}
		case tagName + ".sig":
			if sig, err = download("OTA signature", release, asset); err != nil {
				return
			}
		case tagName + ".sdp":
			if sdp, err = download("recovery signature", release, asset); err != nil {
				return
			}
		}
	}

	if len(imx) == 0 {
		err = fmt.Errorf("could not find %s release for github.com/%s/%s", conf.releaseVersion, org, repo)
		return
	}

	if len(csf) == 0 {
		err = fmt.Errorf("could not find %s HAB signature for github.com/%s/%s", conf.releaseVersion, org, repo)
		return
	}

	if len(sig) == 0 {
		err = fmt.Errorf("could not find %s OTA signature for github.com/%s/%s", conf.releaseVersion, org, repo)
		return
	}

	if len(sdp) == 0 {
		err = fmt.Errorf("could not find %s recovery signature for github.com/%s/%s", conf.releaseVersion, org, repo)
		return
	}

	return
}

func download(tag string, release *github.RepositoryRelease, asset *github.ReleaseAsset) ([]byte, error) {
	log.Printf("Found %s", tag)
	log.Printf("  Tag:    %s", release.GetTagName())
	log.Printf("  Author: %s", asset.GetUploader().Login)
	log.Printf("  Date:   %s", asset.CreatedAt)
	log.Printf("  Link:   %s", release.GetHTMLURL())
	log.Printf("  URL:    %s", asset.GetBrowserDownloadURL())

	log.Printf("Downloading %s %d MiB bytes...", asset.GetName(), asset.GetSize()/(1024*1024))

	res, err := http.Get(asset.GetBrowserDownloadURL())

	if err != nil {
		return nil, err
	}

	return io.ReadAll(res.Body)
}
