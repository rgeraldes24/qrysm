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

// This file implements the functions of bazel.go for `go build` and `go test`:
// files resolve against the working directory and the module root, binaries
// come from $QRYSM_BIN or dist/ (where `make build` puts them), and temporary
// directories are the OS ones.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// BuiltWithBazel returns true iff this library was built with Bazel.
func BuiltWithBazel() bool {
	return false
}

// moduleRoot walks up from the current working directory to find the module
// root (the directory containing go.mod).
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

// testdataRoot is where external test data (Bazel http_archive data deps)
// would live outside Bazel. None is fetched yet (TQ-4).
func testdataRoot() string {
	if root, err := moduleRoot(); err == nil {
		return filepath.Join(root, "third_party", "testdata")
	}

	return ""
}

// searchBases is the ordered list of directories Runfile resolves relative
// paths against.
func searchBases() []string {
	var bases []string
	if cwd, err := os.Getwd(); err == nil {
		bases = append(bases, cwd)
	}

	if root, err := moduleRoot(); err == nil {
		bases = append(bases, root)
	}

	if td := testdataRoot(); td != "" {
		bases = append(bases, td)
	}

	return bases
}

// FindBinary looks for a built binary by name, first in $QRYSM_BIN, then in
// the repository's dist/ directory (populated by `make build`).
func FindBinary(_, name string) (string, bool) {
	var candidates []string
	if dir := os.Getenv("QRYSM_BIN"); dir != "" {
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
// working directory, the module root and the test-data root. An absolute path
// that does not exist is also tried relative to those, as rules_go resolves
// "/testing/..." against the workspace. It returns an error if the file or
// directory cannot be found.
func Runfile(path string) (string, error) {
	if filepath.IsAbs(path) {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}

		path = strings.TrimLeft(path, string(filepath.Separator))
	}

	for _, base := range searchBases() {
		candidate := filepath.Join(base, path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("runfile %s: could not locate file", path)
}

// RunfilesPath returns the module root.
func RunfilesPath() (string, error) {
	return moduleRoot()
}

// ListRunfiles returns every file under the test-data root, or none when there
// is no test data.
func ListRunfiles() ([]RunfileEntry, error) {
	root := testdataRoot()
	if root == "" {
		return nil, errors.New("could not locate the module root")
	}

	if _, err := os.Stat(root); errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}

	var entries []RunfileEntry
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk dir: %w", err)
		}

		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(root, p)
		if err != nil {
			return fmt.Errorf("rel: %w", err)
		}

		entries = append(entries, RunfileEntry{Path: p, ShortPath: filepath.ToSlash(rel)})

		return nil
	})
	if err != nil {
		return nil, err
	}

	return entries, nil
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
