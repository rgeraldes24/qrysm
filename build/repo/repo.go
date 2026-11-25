// Package repo locates the files of the repository (test fixtures, static
// files, external test data) and the binaries built from it, wherever the
// code using it runs from within the repository (e.g. a package directory
// under `go test`).
package repo

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// BinDirEnv names the environment variable FindBinary looks binaries up in
// first.
const BinDirEnv = "QRYSM_BIN"

// TestDataDir is the repository-relative directory holding external test data.
const TestDataDir = "third_party/testdata"

// Root returns the repository root: the closest directory at or above the
// working directory holding a go.mod file.
func Root() (string, error) {
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

// searchBases is the ordered list of directories File resolves relative paths
// against: the working directory, the repository root, then the external test
// data directory.
func searchBases() []string {
	var bases []string
	if cwd, err := os.Getwd(); err == nil {
		bases = append(bases, cwd)
	}

	if root, err := Root(); err == nil {
		bases = append(bases, root, filepath.Join(root, TestDataDir))
	}

	return bases
}

// File resolves path against, in order: an absolute path, the current working
// directory, the repository root, and the external test data directory. It
// returns an error if the file or directory cannot be found.
func File(path string) (string, error) {
	if filepath.IsAbs(path) {
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("file not found: %s", path)
		}

		return path, nil
	}

	for _, base := range searchBases() {
		candidate := filepath.Join(base, path)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("file not found: %s", path)
}

// FindBinary looks for a built binary by name, first in $QRYSM_BIN, then in
// the repository's dist/ directory (populated by `make build` and `make e2e`).
func FindBinary(name string) (string, bool) {
	var candidates []string
	if dir := os.Getenv(BinDirEnv); dir != "" {
		candidates = append(candidates, filepath.Join(dir, name))
	}

	if root, err := Root(); err == nil {
		candidates = append(candidates, filepath.Join(root, "dist", name))
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, true
		}
	}

	return "", false
}

// TestDataFile is a file of the external test data directory.
type TestDataFile struct {
	// Path is the absolute path to the file.
	Path string
	// ShortPath is the path relative to the external test data directory,
	// using forward slashes.
	ShortPath string
}

// ListTestData returns every file of the external test data directory.
func ListTestData() ([]TestDataFile, error) {
	root, err := Root()
	if err != nil {
		return nil, err
	}

	dir := filepath.Join(root, TestDataDir)

	var files []TestDataFile
	err = filepath.WalkDir(dir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return fmt.Errorf("rel: %w", err)
		}

		files = append(files, TestDataFile{Path: p, ShortPath: filepath.ToSlash(rel)})

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", dir, err)
	}

	return files, nil
}
