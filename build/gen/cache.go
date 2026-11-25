package main

// This file implements a content-based cache that lets `make gen` skip a kind
// (proto / ssz / mocks) when none of the files that can affect its output have
// changed since the last successful run on this checkout.
//
// For each kind we build a manifest: a sorted (repo-relative-path, sha256) list
// over that kind's input AND output files, then a single SHA256 over the list.
// The manifest is compared against the value stored in .gen-cache.json. A match
// means nothing relevant changed -> skip. Outputs are part of the manifest, so a
// manual edit or deletion of a generated file invalidates the cache too.

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	cacheFile    = ".gen-cache.json"
	cacheVersion = 1
)

// genCache is the on-disk cache: one manifest hash per kind. Version lets us
// invalidate the whole cache when the manifest scheme changes.
type genCache struct {
	Version int               `json:"version"`
	Kinds   map[string]string `json:"kinds"`
}

func loadCache() genCache {
	empty := genCache{Version: cacheVersion, Kinds: map[string]string{}}

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return empty
	}

	var c genCache
	if err := json.Unmarshal(data, &c); err != nil || c.Version != cacheVersion || c.Kinds == nil {
		return empty
	}

	return c
}

func storeCache(c genCache) error {
	c.Version = cacheVersion

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	if err := os.WriteFile(cacheFile, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("writeFile: %w", err)
	}

	return nil
}

// manifest returns the content hash of every file that determines kind k's
// output (the common toolchain files plus k's specific inputs and outputs).
func manifest(k kind) (string, error) {
	files, err := kindFiles(k)
	if err != nil {
		return "", fmt.Errorf("kindFiles %s: %w", k, err)
	}

	h := sha256.New()
	for _, f := range dedupeSorted(files) {
		sum, err := fileSum(f)
		if err != nil {
			return "", fmt.Errorf("fileSum %s: %w", f, err)
		}

		if _, err := fmt.Fprintf(h, "%s %s\n", f, sum); err != nil {
			return "", fmt.Errorf("fmt.Fprintf: %w", err)
		}
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func fileSum(path string) (string, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path comes from the repo-relative gen file sets
	if err != nil {
		if os.IsNotExist(err) {
			return "absent", nil
		}

		return "", fmt.Errorf("readFile %s: %w", path, err)
	}

	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:]), nil
}

func kindFiles(k kind) ([]string, error) {
	common, err := commonFiles()
	if err != nil {
		return nil, fmt.Errorf("commonFiles: %w", err)
	}

	var specific []string
	switch k {
	case kindProto:
		specific, err = protoFiles()
	case kindSSZ:
		specific, err = sszFiles()
	case kindMocks:
		specific, err = mockFiles()
	default:
		return nil, fmt.Errorf("unknown kind %q", k)
	}

	if err != nil {
		return nil, fmt.Errorf("files of %s: %w", k, err)
	}

	return append(common, specific...), nil
}

func commonFiles() ([]string, error) {
	files := []string{"go.mod", "go.sum"}

	entries, err := os.ReadDir("build/gen")
	if err != nil {
		return nil, fmt.Errorf("readDir build/gen: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") {
			continue
		}

		files = append(files, "build/gen/"+entry.Name())
	}

	return files, nil
}

func protoFiles() ([]string, error) {
	var files []string
	err := filepath.WalkDir("proto", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walkDir proto: %w", err)
		}

		if d.IsDir() {
			return nil
		}

		name := d.Name()
		if strings.HasSuffix(name, ".proto") || name == "BUILD.bazel" || isProtoOutput(name) {
			files = append(files, filepath.ToSlash(path))
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walkDir proto: %w", err)
	}

	return append(files, sszProtoLibraryBzl), nil
}

func sszFiles() ([]string, error) {
	targets, err := loadSSZTargets()
	if err != nil {
		return nil, fmt.Errorf("loadSSZTargets: %w", err)
	}

	files, err := buildBazelFiles()
	if err != nil {
		return nil, fmt.Errorf("buildBazelFiles: %w", err)
	}

	files = append(files, fastsszPatch)
	for _, t := range targets {
		for _, dir := range append([]string{t.pkg}, t.includes...) {
			srcs, err := goPkgFiles(dir)
			if err != nil {
				return nil, fmt.Errorf("goPkgFiles %s: %w", dir, err)
			}

			files = append(files, srcs...)
		}

		out := filepath.ToSlash(filepath.Join(t.pkg, t.out))
		files = append(files, out, minimalTwin(out))
	}

	return files, nil
}

func mockFiles() ([]string, error) {
	var files []string
	for _, g := range mockGroups() {
		for _, m := range g.reflect {
			goFiles, err := goPkgFiles(strings.TrimPrefix(m.importPath, modulePath+"/"))
			if err != nil {
				return nil, fmt.Errorf("goPkgFiles %s: %w", m.importPath, err)
			}

			files = append(files, goFiles...)
			files = append(files, m.dest)
		}

		for _, m := range g.source {
			files = append(files, m.source, m.dest)
		}
	}

	return files, nil
}

// goPkgFiles returns the non-test Go files of dir.
func goPkgFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("readDir %s: %w", dir, err)
	}

	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		out = append(out, filepath.ToSlash(filepath.Join(dir, name)))
	}

	return out, nil
}

func dedupeSorted(in []string) []string {
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if seen[s] {
			continue
		}

		seen[s] = true
		out = append(out, s)
	}

	sort.Strings(out)

	return out
}
