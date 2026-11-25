// Copyright 2015 The Cockroach Authors.
//
// Use of this software is governed by the Business Source License
// included in the file licenses/BSL.txt.
//
// As of the Change Date specified in that file, in accordance with
// the Business Source License, use of this software will be governed
// by the Apache License, Version 2.0, included in the file
// licenses/APL.txt.

//go:build !bazel

package bazel

// This file implements the package for plain `go build` / `go test` builds:
// the runfiles Bazel used to stage are looked up in the source tree instead,
// and binaries in the directory `make build` / `make e2e` write to.

import (
	"fmt"
	"os"
	"path/filepath"
)

// BinDirEnv names the environment variable FindBinary looks binaries up in
// first.
const BinDirEnv = "QRYSM_BIN"

// BuiltWithBazel returns true iff this library was built with Bazel.
func BuiltWithBazel() bool {
	return false
}

// moduleRoot walks up from the current working directory to the module root
// (the directory containing go.mod). `go test` runs in the directory of the
// package under test, so this reliably finds the repository root.
func moduleRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not locate go.mod above %q", dir)
		}

		dir = parent
	}
}

// searchBases is the ordered list of directories Runfile resolves relative
// paths against: the working directory, the module root, then the directory
// holding external test data (third_party/testdata).
func searchBases() []string {
	var bases []string
	if cwd, err := os.Getwd(); err == nil {
		bases = append(bases, cwd)
	}

	if root, err := moduleRoot(); err == nil {
		bases = append(bases, root, filepath.Join(root, "third_party", "testdata"))
	}

	return bases
}

// FindBinary looks for a built binary by name, first in $QRYSM_BIN, then in
// the repository's dist/ directory (populated by `make build` and `make e2e`).
// The package argument only matters to Bazel.
func FindBinary(_, name string) (string, bool) {
	var candidates []string
	if dir := os.Getenv(BinDirEnv); dir != "" {
		candidates = append(candidates, filepath.Join(dir, name))
	}

	if root, err := moduleRoot(); err == nil {
		candidates = append(candidates, filepath.Join(root, "dist", name))
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, true
		}
	}

	return "", false
}

// Runfile resolves path against, in order: an absolute path, the current
// working directory, the module root, and the external test-data directory.
// It returns an error if the file or directory cannot be found.
func Runfile(path string) (string, error) {
	if filepath.IsAbs(path) {
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("runfile not found: %s", path)
		}

		return path, nil
	}

	for _, base := range searchBases() {
		candidate := filepath.Join(base, path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("runfile not found: %s", path)
}

// RunfilesPath returns the module root.
func RunfilesPath() (string, error) {
	return moduleRoot()
}

// TestTmpDir returns the OS temporary directory.
func TestTmpDir() string {
	return os.TempDir()
}

// NewTmpDir creates a new temporary directory with the given prefix. The
// caller is responsible for cleaning it up.
func NewTmpDir(prefix string) (string, error) {
	return os.MkdirTemp("", prefix)
}

// RelativeTestTargetPath has no meaning outside Bazel.
func RelativeTestTargetPath() string {
	return ""
}

// SetGoEnv is a no-op outside Bazel: the Go toolchain is already on PATH.
func SetGoEnv() {}
