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

// loadCache reads cacheFile, or returns an empty cache.
func loadCache() genCache {
	cache := genCache{Version: cacheVersion, Kinds: map[string]string{}}

	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return cache
	}

	if err := json.Unmarshal(data, &cache); err == nil && cache.Version == cacheVersion && cache.Kinds != nil {
		return cache
	}

	return genCache{Version: cacheVersion, Kinds: map[string]string{}}
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

	specific, err := specificFiles(k)
	if err != nil {
		return nil, fmt.Errorf("specificFiles %s: %w", k, err)
	}

	return append(common, specific...), nil
}

func specificFiles(k kind) ([]string, error) {
	switch k {
	case kindProto:
		return protoFiles()
	case kindSSZ:
		return sszFiles()
	case kindMocks:
		return mockFiles()
	default:
		return nil, fmt.Errorf("unknown kind %q", k)
	}
}

// commonFiles are the inputs of every kind: the pinned tool versions and the
// generator itself.
func commonFiles() ([]string, error) {
	files := []string{"go.mod", "go.sum"}

	gen, err := goFiles("build/gen")
	if err != nil {
		return nil, err
	}

	return append(files, gen...), nil
}

// protoFiles are the protos and every generated file of the proto packages.
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
		if strings.HasSuffix(name, ".proto") || isGeneratedProto(name) {
			files = append(files, filepath.ToSlash(path))
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walkDir proto: %w", err)
	}

	return files, nil
}

// sszFiles are the .pb.go files (with their minimal twins) and Go packages
// sszgen reads, the patch it is built with, and the outputs.
func sszFiles() ([]string, error) {
	files := []string{fastsszPatch}
	for _, t := range sszTargets {
		for _, dir := range append([]string{t.pkg}, t.protoInc...) {
			pbs, err := pbgoFiles(dir)
			if err != nil {
				return nil, fmt.Errorf("pbgoFiles %s: %w", dir, err)
			}

			files = append(files, pbs...)
		}

		for _, dir := range t.libInc {
			lib, err := goFiles(dir)
			if err != nil {
				return nil, err
			}

			files = append(files, lib...)
		}

		files = append(files, filepath.ToSlash(filepath.Join(t.pkg, sszOut)), filepath.ToSlash(filepath.Join(t.pkg, minimalTwin(sszOut))))
	}

	return files, nil
}

// mockFiles are the sources of the mocks and the mocks.
func mockFiles() ([]string, error) {
	var files []string
	for _, g := range mockGroups {
		for _, m := range g.reflect {
			src, err := goFiles(strings.TrimPrefix(m.importPath, goModulePrefix))
			if err != nil {
				return nil, err
			}

			files = append(files, src...)
			files = append(files, m.dest)
		}

		for _, m := range g.source {
			files = append(files, m.source, m.dest)
		}
	}

	return files, nil
}

// pbgoFiles returns the .pb.go files in dir, minimal twins included.
func pbgoFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("readDir %s: %w", dir, err)
	}

	var out []string
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".pb.go") {
			continue
		}

		out = append(out, filepath.ToSlash(filepath.Join(dir, entry.Name())))
	}

	return out, nil
}

// goFiles returns the non-test .go files in dir.
func goFiles(dir string) ([]string, error) {
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
