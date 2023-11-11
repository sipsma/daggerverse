package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/chainguard-dev/go-apk/pkg/apk"
)

const (
	alpineVersion     = "v3.18"
	alpineRepository  = "https://dl-cdn.alpinelinux.org/alpine"
	alpineReleasesURL = "https://alpinelinux.org/releases.json"
)

type Apk struct{}

func (m *Apk) Container(ctx context.Context, pkgs []string) (*Container, error) {
	repo := apk.NewRepositoryFromComponents(
		alpineRepository,
		alpineVersion,
		"main",
		"", //TODO: apkarch(), but including here breaks it below in GetRepositoryIndexes?
	)

	keys, err := m.keys()
	if err != nil {
		return nil, fmt.Errorf("failed to get alpine keys: %w", err)
	}

	indexes, err := apk.GetRepositoryIndexes(ctx, []string{repo.URI}, keys, apkarch())
	if err != nil {
		return nil, fmt.Errorf("failed to get alpine indexes: %w", err)
	}

	pkgResolver := apk.NewPkgResolver(ctx, indexes)

	// TODO: these base packages should be skippable if desired
	pkgs = append([]string{"alpine-baselayout", "busybox"}, pkgs...)

	repoPkgs, conflicts, err := pkgResolver.GetPackagesWithDependencies(ctx, pkgs)
	if err != nil {
		return nil, fmt.Errorf("failed to get alpine packages: %w", err)
	}
	if len(conflicts) > 0 {
		return nil, fmt.Errorf("failed to get alpine packages: %v", conflicts)
	}

	setupBase := dag.Container().From("alpine:" + alpineVersion[1:])

	ctr := dag.Container()

	for _, pkg := range repoPkgs {
		url := pkg.URL()
		mntPath := filepath.Join("/mnt", filepath.Base(url))
		outDir := "/out"

		unpacked := setupBase.
			WithMountedFile(mntPath, dag.HTTP(url)).
			WithMountedDirectory(outDir, dag.Directory()).
			WithWorkdir(outDir).
			WithExec([]string{"tar", "-xf", mntPath})

		// TODO: check if there's any other important script files
		// TODO: make scripts optional too?

		// TODO: setup caching such that it works nicely w/ remote cache; wrapping in a function
		// may do it once we have better cache control. Right now we need to always pull the
		// packages to read the entries.
		// TODO: the above could also make the parallelization easier

		entries, err := unpacked.Directory(outDir).Entries(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get alpine package entries: %w", err)
		}
		rmFiles := []string{}
		var preInstallFile, postInstallFile, triggerFile *File
		for _, entry := range entries {
			if !strings.HasPrefix(entry, ".") {
				continue
			}
			rmFiles = append(rmFiles, filepath.Join(outDir, entry))
			switch entry {
			case ".pre-install":
				preInstallFile = unpacked.File(filepath.Join(outDir, entry))
			case ".post-install":
				postInstallFile = unpacked.File(filepath.Join(outDir, entry))
			case ".trigger":
				triggerFile = unpacked.File(filepath.Join(outDir, entry))
			}
		}

		// TODO: squash layers or otherwise fix
		pkgDir := unpacked.WithExec(append([]string{"rm"}, rmFiles...)).Directory(outDir)

		if preInstallFile != nil {
			ctr = ctr.
				WithMountedFile("/tmp/script", preInstallFile).
				WithExec([]string{"/tmp/script"}).
				WithoutMount("/tmp/script")
		}
		ctr = ctr.WithDirectory("/", pkgDir)
		if postInstallFile != nil {
			ctr = ctr.
				WithMountedFile("/tmp/script", postInstallFile).
				WithExec([]string{"/tmp/script"}).
				WithoutMount("/tmp/script")
		}
		// TODO: not sure if this is actually when trigger should run?
		if triggerFile != nil {
			ctr = ctr.
				WithMountedFile("/tmp/script", triggerFile).
				WithExec([]string{"/tmp/script"}).
				WithoutMount("/tmp/script")
		}

		// TODO:
		// ctr = ctr.WithDirectory(filepath.Join("/mnt", pkg.Name), unpacked.Directory(outDir))
	}

	return ctr, nil
}

func (m *Apk) Debug(ctx context.Context, pkgs []string) (*Container, error) {
	ctr, err := m.Container(ctx, pkgs)
	if err != nil {
		return nil, err
	}

	return dag.Container().From("alpine:"+alpineVersion[1:]).
		WithMountedDirectory("/mnt", ctr.Rootfs()), nil
}

func (m *Apk) keys() (map[string][]byte, error) {
	releases, err := m.releases()
	if err != nil {
		return nil, fmt.Errorf("failed to get alpine releases: %w", err)
	}
	branch := releases.GetReleaseBranch(alpineVersion)
	if branch == nil {
		return nil, fmt.Errorf("failed to get alpine branch for version %s", alpineVersion)
	}
	urls := branch.KeysFor(apkarch(), time.Now())

	keys := make(map[string][]byte)
	for _, u := range urls {
		res, err := http.Get(u)
		if err != nil {
			return nil, fmt.Errorf("failed to get alpine key at %s: %w", u, err)
		}
		defer res.Body.Close()
		if res.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("unable to get alpine key at %s: %v", u, res.Status)
		}
		keyBytes, err := io.ReadAll(res.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read alpine key at %s: %w", u, err)
		}
		keys[filepath.Base(u)] = keyBytes
	}
	return keys, nil
}

func (m *Apk) releases() (*apk.Releases, error) {
	res, err := http.Get(alpineReleasesURL)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unable to get alpine releases at %s: %v", alpineReleasesURL, res.Status)
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read alpine releases: %w", err)
	}
	var releases apk.Releases
	if err := json.Unmarshal(b, &releases); err != nil {
		return nil, fmt.Errorf("failed to unmarshal alpine releases: %w", err)
	}

	return &releases, nil
}

func apkarch() string {
	return apk.ArchToAPK(runtime.GOARCH)
}
