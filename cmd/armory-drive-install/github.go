// Copyright (c) F-Secure Corporation
// https://foundry.f-secure.com
//
// Use of this source code is governed by the license
// that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/google/go-github/v34/github"
	"golang.org/x/oauth2"
)

const (
	org         = "f-secure-foundry"
	releaseRepo = "armory-drive"
	logRepo     = "armory-drive-log"

	checkpointPath      = "log/"
	keysPath            = "keys/"
	logKeyName          = "armory-drive-log.pub"
	frKeyName           = "armory-drive.pub"
	checkpointCachePath = "armory-drive-install.lastCheckpoint"
)

type releaseAssets struct {
	// firmware binary
	imx []byte
	// secure boot fuse table
	srk []byte
	// secure boot signature for eMMC boot mode
	csf []byte
	// secure boot signature for serial download mode
	sdp []byte
	// firmware transparency proof
	log []byte

	// manifest authentication key
	frPub []byte
	// transparency log authentication key
	logPub []byte
}

func (a *releaseAssets) complete() bool {
	return (len(a.imx) > 0 &&
		len(a.srk) > 0 &&
		len(a.csf) > 0 &&
		len(a.sdp) > 0 &&
		len(a.log) > 0)
}

func githubClient() (*github.Client, bool) {
	var httpClient *http.Client

	// A GITHUB_TOKEN environment variable can be set to avoid GitHub API
	// rate limiting.
	token := os.Getenv("GITHUB_TOKEN")

	if len(token) == 0 {
		return github.NewClient(nil), false
	}

	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)

	httpClient = oauth2.NewClient(context.Background(), ts)

	return github.NewClient(httpClient), true
}

func downloadRelease(version string) (a *releaseAssets, err error) {
	var release *github.RepositoryRelease

	ctx := context.Background()
	client, auth := githubClient()

	if version == "latest" {
		release, _, err = client.Repositories.GetLatestRelease(ctx, org, releaseRepo)
	} else {
		release, _, err = client.Repositories.GetReleaseByTag(ctx, org, releaseRepo, version)
	}

	if err != nil {
		return
	}

	if !auth {
		// If we do not have a GitHub API token make unauthenticated
		// downloads.
		client = nil
	}

	a = &releaseAssets{}

	for _, asset := range release.Assets {
		switch *asset.Name {
		case "armory-drive.imx":
			if a.imx, err = downloadAsset("binary release", release, asset, client); err != nil {
				return
			}
		case "armory-drive.srk":
			if a.srk, err = downloadAsset("SRK table hash", release, asset, client); err != nil {
				return
			}
		case "armory-drive.csf":
			if a.csf, err = downloadAsset("HAB signature", release, asset, client); err != nil {
				return
			}
		case "armory-drive.sdp":
			if a.sdp, err = downloadAsset("recovery signature", release, asset, client); err != nil {
				return
			}
		case "armory-drive.proofbundle":
			if a.log, err = downloadAsset("proof bundle", release, asset, client); err != nil {
				return
			}
		}
	}

	if !a.complete() {
		return nil, errors.New("incomplete release")
	}

	log.Printf("\nDownloaded verified release assets")

	if len(conf.frPublicKey) > 0 {
		log.Printf("Using %s as manifest authentication key", conf.frPublicKey)
		a.frPub, err = os.ReadFile(conf.frPublicKey)
	} else {
		a.frPub, err = downloadKey("manifest authentication key", keysPath+frKeyName, client)
	}

	if err != nil {
		return nil, fmt.Errorf("could not load key, %v", err)
	}

	if len(conf.logPublicKey) > 0 {
		log.Printf("Using %s as log authentication key", conf.logPublicKey)
		a.logPub, err = os.ReadFile(conf.logPublicKey)
	} else {
		a.logPub, err = downloadKey("transparency log authentication key", keysPath+logKeyName, client)
	}

	if err != nil {
		return nil, fmt.Errorf("could not load key, %v", err)
	}

	if err := verifyRelease(release, a); err != nil {
		return nil, fmt.Errorf("invalid release: %v", err)
	}

	return
}

func logFetcher(ctx context.Context, path string) (buf []byte, err error) {
	client, _ := githubClient()

	opts := &github.RepositoryContentGetOptions{
		Ref: conf.branch,
	}

	res, _, err := client.Repositories.DownloadContents(ctx, org, logRepo, checkpointPath+path, opts)

	if err != nil {
		return
	}

	return io.ReadAll(res)
}

func downloadAsset(tag string, release *github.RepositoryRelease, asset *github.ReleaseAsset, client *github.Client) ([]byte, error) {
	log.Printf("\nFound %s", tag)
	log.Printf("  Tag:    %s", release.GetTagName())
	log.Printf("  Author: %s", asset.GetUploader().GetLogin())
	log.Printf("  Date:   %s", asset.CreatedAt)
	log.Printf("  Link:   %s", release.GetHTMLURL())
	log.Printf("  URL:    %s", asset.GetBrowserDownloadURL())

	log.Printf("Downloading %s %d bytes...", asset.GetName(), asset.GetSize())

	if client != nil {
		res, _, err := client.Repositories.DownloadReleaseAsset(context.Background(), org, releaseRepo, asset.GetID(), http.DefaultClient)

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

func downloadKey(tag string, path string, client *github.Client) ([]byte, error) {
	log.Printf("Downloading %s from %s", tag, fmt.Sprintf("%s/%s/%s", org, logRepo, path))

	if client != nil {
		res, _, err := client.Repositories.DownloadContents(context.Background(), org, logRepo, path, nil)

		if err != nil {
			return nil, err
		}

		return io.ReadAll(res)
	}

	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", org, logRepo, conf.branch, path)

	res, err := http.Get(url)

	if err != nil {
		return nil, err
	}

	return io.ReadAll(res.Body)
}
