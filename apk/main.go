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

	goapk "github.com/chainguard-dev/go-apk/pkg/apk"
)

const (
	alpineVersion     = "v3.18"
	alpineRepository  = "https://dl-cdn.alpinelinux.org/alpine"
	alpineReleasesURL = "https://alpinelinux.org/releases.json"
	wolfiRepository = "https://packages.wolfi.dev/os"
	wolfiKey = "https://packages.wolfi.dev/os/wolfi-signing.rsa.pub"
	wolfiBase = "cgr.dev/chainguard/wolfi-base"
)

type Apk struct{
	Wolfi bool
}

func (m *Apk) LegacyBuild(ctx context.Context, pkgs []string) *Container {
	return dag.Container().From("alpine:" + alpineVersion[1:]).
		WithExec(append([]string{"apk", "add"}, pkgs...))
}

func (m *Apk) Scan(ctx context.Context, pkgs []string, legacy Optional[bool], wolfi Optional[bool]) (string, error) {
	var ctr *Container
	if legacy.GetOr(false) {
		ctr = m.LegacyBuild(ctx, pkgs)
	} else {
		var err error
		ctr, err = m.Build(ctx, pkgs, Opt(false), wolfi)
		if err != nil {
			return "", err
		}
	}
	return dag.Trivy().ScanContainer(ctx, ctr)
}

func (m *Apk) Build(
	ctx context.Context,
	pkgs []string,
	debugPkgs Optional[bool],
	wolfi Optional[bool],
) (*Container, error) {
	// TODO: what should this repo interface look like?
	m.Wolfi = wolfi.GetOr(false)
	repo := goapk.NewRepositoryFromComponents(
		alpineRepository,
		alpineVersion,
		"main",
		"",
	)
	basePkgs := []string{"alpine-baselayout", "alpine-release", "busybox"}
	builderBase := "busybox:latest"

	if m.Wolfi {
		repo.URI = wolfiRepository
		basePkgs = []string{"wolfi-baselayout", "busybox"}
		//builderBase = wolfiBase
	}

	keys, err := m.keys()
	if err != nil {
		return nil, fmt.Errorf("failed to get alpine keys: %w", err)
	}

	indexes, err := goapk.GetRepositoryIndexes(ctx, []string{repo.URI}, keys, apkarch())
	if err != nil {
		return nil, fmt.Errorf("failed to get alpine indexes: %w", err)
	}

	pkgResolver := goapk.NewPkgResolver(ctx, indexes)

	pkgs = append(basePkgs, pkgs...)

	repoPkgs, conflicts, err := pkgResolver.GetPackagesWithDependencies(ctx, pkgs)
	fmt.Printf("conflicts: %v\n", conflicts)
	if err != nil {
		return nil, fmt.Errorf("failed to get alpine packages: %w", err)
	}
	// TODO: i dont think we need to error here
	//if len(conflicts) > 0 {
	//	return nil, fmt.Errorf("failed to get alpine packages with conflicts: %v", conflicts)
	//}

	setupBase := dag.Container().From(builderBase)

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
			rmFiles = append(rmFiles, entry)
			switch entry {
			case ".pre-install":
				preInstallFile = unpacked.File(filepath.Join(outDir, entry))
			case ".post-install":
				postInstallFile = unpacked.File(filepath.Join(outDir, entry))
			case ".trigger":
				triggerFile = unpacked.File(filepath.Join(outDir, entry))
			}
		}

		pkgDir := unpacked.Directory(outDir)

		if preInstallFile != nil {
			ctr = ctr.
				WithMountedFile("/tmp/script", preInstallFile).
				WithExec([]string{"/tmp/script"}).
				WithoutMount("/tmp/script")
		}
		ctr = ctr.WithDirectory("/", pkgDir, ContainerWithDirectoryOpts{
			Exclude: rmFiles,
		})
		if postInstallFile != nil {
			ctr = ctr.
				WithMountedFile("/tmp/script", postInstallFile).
				WithExec([]string{"/tmp/script"}).
				WithoutMount("/tmp/script")
		}
		if triggerFile != nil {
			ctr = ctr.
				WithMountedFile("/tmp/script", triggerFile).
				// TODO: triggers failing for git package on wolfi
//				WithExec([]string{"/tmp/script"}).
				WithoutMount("/tmp/script")
		}

		if debugPkgs.GetOr(false) {
			ctr = ctr.WithDirectory(filepath.Join("/debug", pkg.Name), pkgDir)
		}
	}

	return ctr, nil
}

func (m *Apk) Debug(ctx context.Context, pkgs []string) (*Container, error) {
	ctr, err := m.Build(ctx, pkgs, Opt(false), Opt(false))
	if err != nil {
		return nil, err
	}

	return dag.Container().From("alpine:"+alpineVersion[1:]).
		WithMountedDirectory("/mnt", ctr.Rootfs()), nil
}

func (m *Apk) keys() (map[string][]byte, error) {
	var urls []string
	if m.Wolfi {
		urls = []string{wolfiKey}
	} else {
		releases, err := m.releases()
		if err != nil {
			return nil, fmt.Errorf("failed to get alpine releases: %w", err)
		}
		branch := releases.GetReleaseBranch(alpineVersion)
		if branch == nil {
			return nil, fmt.Errorf("failed to get alpine branch for version %s", alpineVersion)
		}
		urls = branch.KeysFor(apkarch(), time.Now())
	}

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

func (m *Apk) releases() (*goapk.Releases, error) {
	res, err := http.Get(alpineReleasesURL)
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unable to get alpine releases at %s: %v", alpineReleasesURL, res.Status)
	}
	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read alpine releases: %w", err)
	}
	var releases goapk.Releases
	if err := json.Unmarshal(b, &releases); err != nil {
		return nil, fmt.Errorf("failed to unmarshal alpine releases: %w", err)
	}

	return &releases, nil
}

func apkarch() string {
	return goapk.ArchToAPK(runtime.GOARCH)
}
