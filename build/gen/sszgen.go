package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const (
	fastsszModule = "github.com/prysmaticlabs/fastssz"

	// fastsszPatch hardens the code sszgen generates: it rejects set padding
	// bits in bitvectors, encodings of 2^32 bytes or more and out-of-bounds
	// offsets, validates bitlists, and reuses the destination slices. The
	// generated SSZ code relies on it, so sszgen is never built without it.
	fastsszPatch = "third_party/com_github_prysmaticlabs_fastssz-validation.patch"
)

// buildSSZGen builds sszgen from the fastssz version selected by go.mod with
// fastsszPatch applied, and returns the path of the binary. The module is
// copied out of the module cache, patched, and substituted with a replace
// directive in a temporary copy of go.mod, so sszgen and its dependencies are
// built at the versions go.mod selects.
func buildSSZGen(tmpRoot string) (string, error) {
	src, err := moduleDir(fastsszModule)
	if err != nil {
		return "", fmt.Errorf("locate %s: %w", fastsszModule, err)
	}

	patched := filepath.Join(tmpRoot, "fastssz")
	if err := copyTree(src, patched); err != nil {
		return "", fmt.Errorf("copy %s: %w", fastsszModule, err)
	}

	if err := applyPatch(patched, fastsszPatch); err != nil {
		return "", fmt.Errorf("apply %s: %w", fastsszPatch, err)
	}

	modFile, err := writeReplacedModFile(tmpRoot, fastsszModule, patched)
	if err != nil {
		return "", fmt.Errorf("write go.mod: %w", err)
	}

	bin := filepath.Join(tmpRoot, "bin", "sszgen")
	fmt.Printf("building sszgen from %s with %s\n", src, fastsszPatch)
	if err := sh("go", "build", "-modfile="+modFile, "-o", bin, fastsszModule+"/sszgen"); err != nil {
		return "", fmt.Errorf("build sszgen: %w", err)
	}

	return bin, nil
}

// moduleDir downloads, if needed, the version of the module that go.mod
// selects and returns its directory in the module cache.
func moduleDir(path string) (string, error) {
	cmd := exec.Command("go", "mod", "download", "-json", path)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("go mod download %s: %w: %s", path, err, stderr.String())
	}

	var m struct {
		Dir   string
		Error string
	}
	if err := json.Unmarshal(out, &m); err != nil {
		return "", fmt.Errorf("decode go mod download output: %w", err)
	}

	if m.Error != "" {
		return "", fmt.Errorf("go mod download %s: %s", path, m.Error)
	}

	if m.Dir == "" {
		return "", fmt.Errorf("go mod download %s: no directory", path)
	}

	return m.Dir, nil
}

// copyTree copies the regular files under src to dst with writable
// permissions (the module cache is read-only). The vendor directory is
// skipped: a replaced module is built with the main module's dependencies.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("filepath.Rel: %w", err)
		}

		target := filepath.Join(dst, rel)
		if d.IsDir() {
			if rel == "vendor" || rel == ".git" {
				return filepath.SkipDir
			}

			return os.MkdirAll(target, 0o750)
		}

		if !d.Type().IsRegular() {
			return nil
		}

		data, err := os.ReadFile(path) // #nosec G304 -- path is under the module cache directory of a pinned module
		if err != nil {
			return fmt.Errorf("readFile: %w", err)
		}

		return os.WriteFile(target, data, 0o600)
	})
}

// applyPatch applies a -p1 unified diff to dir with git apply, which rejects
// any hunk that does not match exactly. Some hunks of fastsszPatch carry no
// context lines, hence --unidiff-zero. GIT_CEILING_DIRECTORIES keeps git from
// treating a repository above dir as the one to patch.
func applyPatch(dir, patch string) error {
	abs, err := filepath.Abs(patch)
	if err != nil {
		return fmt.Errorf("filepath.Abs: %w", err)
	}

	env := []string{"GIT_CEILING_DIRECTORIES=" + filepath.Dir(dir)}

	return shInDir(dir, env, "git", "apply", "-p1", "--unidiff-zero", "--whitespace=nowarn", abs)
}

// writeReplacedModFile writes a copy of go.mod (and go.sum) to dir with module
// replaced by the directory replacement, for use with go build -modfile.
func writeReplacedModFile(dir, module, replacement string) (string, error) {
	mod, err := os.ReadFile("go.mod")
	if err != nil {
		return "", fmt.Errorf("readFile: %w", err)
	}

	if bytes.Contains(mod, []byte("replace "+module+" ")) {
		return "", fmt.Errorf("go.mod already replaces %s", module)
	}

	sum, err := os.ReadFile("go.sum")
	if err != nil {
		return "", fmt.Errorf("readFile: %w", err)
	}

	modFile := filepath.Join(dir, "gen.mod")
	mod = append(mod, []byte(fmt.Sprintf("\nreplace %s => %s\n", module, replacement))...)
	if err := os.WriteFile(modFile, mod, 0o600); err != nil {
		return "", fmt.Errorf("writeFile: %w", err)
	}

	if err := os.WriteFile(strings.TrimSuffix(modFile, ".mod")+".sum", sum, 0o600); err != nil {
		return "", fmt.Errorf("writeFile: %w", err)
	}

	return modFile, nil
}
