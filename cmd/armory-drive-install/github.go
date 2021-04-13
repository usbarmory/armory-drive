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
	"os"

	"github.com/google/go-github/v34/github"
	"golang.org/x/oauth2"
)

const org = "f-secure-foundry"
const repo = "armory-drive"

func downloadLatestRelease() (imx []byte, csf []byte, sig []byte, sdp []byte, err error) {
	var release *github.RepositoryRelease
	var httpClient *http.Client

	// A GITHUB_TOKEN environment variable can be set to avoid GitHub API
	// rate limiting.
	token := os.Getenv("GITHUB_TOKEN")

	if len(token) > 0 {
		ts := oauth2.StaticTokenSource(
			&oauth2.Token{AccessToken: token},
		)

		httpClient = oauth2.NewClient(context.Background(), ts)
	}

	client := github.NewClient(httpClient)

	if conf.releaseVersion == "latest" {
		release, _, err = client.Repositories.GetLatestRelease(context.Background(), org, repo)
	} else {
		release, _, err = client.Repositories.GetReleaseByTag(context.Background(), org, repo, conf.releaseVersion)
	}

	if err != nil {
		return
	}

	if len(token) == 0 {
		// If we do not have a GitHub API token make unauthenticated
		// downloads.
		client = nil
	}

	for _, asset := range release.Assets {
		switch *asset.Name {
		case "armory-drive.imx":
			if imx, err = download("binary release", release, asset, client); err != nil {
				return
			}
		case "armory-drive.csf":
			if csf, err = download("HAB signature", release, asset, client); err != nil {
				return
			}
		case "armory-drive.sig":
			if sig, err = download("OTA signature", release, asset, client); err != nil {
				return
			}
		case "armory-drive.sdp":
			if sdp, err = download("recovery signature", release, asset, client); err != nil {
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

func download(tag string, release *github.RepositoryRelease, asset *github.ReleaseAsset, client *github.Client) ([]byte, error) {
	log.Printf("Found %s", tag)
	log.Printf("  Tag:    %s", release.GetTagName())
	log.Printf("  Author: %s", asset.GetUploader().GetLogin())
	log.Printf("  Date:   %s", asset.CreatedAt)
	log.Printf("  Link:   %s", release.GetHTMLURL())
	log.Printf("  URL:    %s", asset.GetBrowserDownloadURL())

	log.Printf("Downloading %s %d bytes...", asset.GetName(), asset.GetSize())

	if client != nil {
		res, _, err := client.Repositories.DownloadReleaseAsset(context.Background(), org, repo, asset.GetID(), http.DefaultClient)

		if err != nil {
			return nil, err
		}

		return io.ReadAll(res)
	}

	res, err := http.Get(asset.GetBrowserDownloadURL())

	if err != nil {
		return nil, err
	}

	return io.ReadAll(res.Body)
}
