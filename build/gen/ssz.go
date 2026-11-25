package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	fastsszModule = "github.com/prysmaticlabs/fastssz"

	// fastsszPatch is applied to sszgen by Bazel (go_deps.module_override in
	// MODULE.bazel); apply it here too so both generate the same code.
	fastsszPatch = "third_party/com_github_prysmaticlabs_fastssz-validation.patch"
)

type network string

const (
	networkMainnet network = "mainnet"
	networkMinimal network = "minimal"
)

var networks = []network{networkMainnet, networkMinimal}

func genSSZ() error {
	targets, err := loadSSZTargets()
	if err != nil {
		return fmt.Errorf("load SSZ targets: %w", err)
	}

	tmpRoot, err := os.MkdirTemp("", "gen-ssz-")
	if err != nil {
		return fmt.Errorf("mkdirTemp: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpRoot) }()

	sszgen, err := buildSSZGen(tmpRoot)
	if err != nil {
		return fmt.Errorf("build sszgen: %w", err)
	}

	for _, t := range targets {
		out := filepath.ToSlash(filepath.Join(t.pkg, t.out))
		fmt.Printf("  %s\n", out)

		generated := make(map[network][]byte, len(networks))
		for _, n := range networks {
			data, err := runSSZGen(sszgen, filepath.Join(tmpRoot, string(n)), n, t)
			if err != nil {
				return fmt.Errorf("sszgen %s (%s): %w", out, n, err)
			}

			generated[n] = data
		}

		if err := writeNetworkFiles(out, generated[networkMainnet], generated[networkMinimal]); err != nil {
			return fmt.Errorf("write %s: %w", out, err)
		}
	}

	return nil
}

// writeNetworkFiles writes a config-invariant output once, untagged, or splits
// it into a //go:build !minimal file and a //go:build minimal twin.
func writeNetworkFiles(out string, mainnet, minimal []byte) error {
	twin := minimalTwin(out)
	if bytes.Equal(mainnet, minimal) {
		if err := os.Remove(twin); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove: %w", err)
		}

		return os.WriteFile(out, mainnet, 0o644) // #nosec G306 -- source file, same mode as before
	}

	if err := os.WriteFile(out, tagged("!minimal", mainnet), 0o644); err != nil { // #nosec G306 -- source file
		return fmt.Errorf("writeFile: %w", err)
	}

	return os.WriteFile(twin, tagged("minimal", minimal), 0o644) // #nosec G306 -- source file
}

// buildSSZGen builds the go.mod pinned sszgen with the same patch Bazel applies.
func buildSSZGen(tmpRoot string) (string, error) {
	modDir, err := goModuleDir(fastsszModule)
	if err != nil {
		return "", fmt.Errorf("module dir: %w", err)
	}

	src := filepath.Join(tmpRoot, "fastssz")
	if err := copyTree(modDir, src); err != nil {
		return "", fmt.Errorf("copy %s: %w", fastsszModule, err)
	}

	patch, err := filepath.Abs(fastsszPatch)
	if err != nil {
		return "", fmt.Errorf("abs: %w", err)
	}

	if err := shInDir(src, nil, "patch", "-s", "-p1", "-i", patch); err != nil {
		return "", fmt.Errorf("apply %s: %w", fastsszPatch, err)
	}

	bin := filepath.Join(tmpRoot, "bin", "sszgen")
	if err := shInDir(src, []string{"GOFLAGS=-mod=mod", "GOWORK=off"}, "go", "build", "-o", bin, "./sszgen"); err != nil {
		return "", fmt.Errorf("build: %w", err)
	}

	return bin, nil
}

// runSSZGen generates the SSZ code of one target for one configuration and
// returns it.
func runSSZGen(sszgen, stageRoot string, n network, t sszTarget) ([]byte, error) {
	path, err := stagePkg(stageRoot, t.pkg, n)
	if err != nil {
		return nil, fmt.Errorf("stage %s: %w", t.pkg, err)
	}

	includes := make([]string, 0, len(t.includes))
	for _, inc := range t.includes {
		dir, err := stagePkg(stageRoot, inc, n)
		if err != nil {
			return nil, fmt.Errorf("stage %s: %w", inc, err)
		}

		includes = append(includes, dir)
	}

	out := filepath.Join(stageRoot, "out", t.pkg, t.out)
	if err := os.MkdirAll(filepath.Dir(out), 0o750); err != nil {
		return nil, fmt.Errorf("mkdir all: %w", err)
	}

	args := []string{"--path=" + path, "--output=" + out}
	if len(includes) > 0 {
		args = append(args, "--include="+strings.Join(includes, ","))
	}

	if len(t.objs) > 0 {
		args = append(args, "--objs="+strings.Join(t.objs, ","))
	}

	if len(t.excludeObjs) > 0 {
		args = append(args, "--exclude-objs="+strings.Join(t.excludeObjs, ","))
	}

	if err := sh(sszgen, args...); err != nil {
		return nil, err
	}

	data, err := os.ReadFile(out) // #nosec G304 -- under our own temp dir
	if err != nil {
		return nil, fmt.Errorf("readFile: %w", err)
	}

	return setSourceHash(data, path)
}

var buildTagHeader = regexp.MustCompile(`\A//go:build [^\n]*\n\n`)

// stagePkg copies, once per configuration, the Go files sszgen must see for
// the package dir: for a proto package, the *.pb.go files of that
// configuration with their //go:build header removed (exactly what the proto
// plugins output, and what Bazel hands to sszgen); for any other package, its
// non-test Go files. It returns the staged directory.
func stagePkg(stageRoot, dir string, n network) (string, error) {
	dst := filepath.Join(stageRoot, "src", dir)
	if _, err := os.Stat(dst); err == nil {
		return dst, nil
	}

	files, err := sszInputFiles(dir, n)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(dst, 0o750); err != nil {
		return "", fmt.Errorf("mkdir all: %w", err)
	}

	for _, f := range files {
		data, err := os.ReadFile(f) // #nosec G304 -- repo-relative source file
		if err != nil {
			return "", fmt.Errorf("readFile: %w", err)
		}

		name := filepath.Base(f)
		if strings.HasSuffix(name, ".minimal.pb.go") {
			name = strings.TrimSuffix(name, ".minimal.pb.go") + ".pb.go"
		}

		if err := os.WriteFile(filepath.Join(dst, name), buildTagHeader.ReplaceAll(data, nil), 0o600); err != nil {
			return "", fmt.Errorf("writeFile: %w", err)
		}
	}

	return dst, nil
}

// sszInputFiles returns the repo-relative Go files of dir that sszgen reads
// for configuration n.
func sszInputFiles(dir string, n network) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("readDir %s: %w", dir, err)
	}

	names := make(map[string]bool, len(entries))
	hasPB := false
	for _, e := range entries {
		if e.IsDir() {
			continue
		}

		names[e.Name()] = true
		hasPB = hasPB || strings.HasSuffix(e.Name(), ".pb.go")
	}

	var out []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		if hasPB {
			if !strings.HasSuffix(name, ".pb.go") {
				continue
			}

			isTwin := strings.HasSuffix(name, ".minimal.pb.go")
			hasTwin := !isTwin && names[minimalTwin(name)]
			if (n == networkMinimal && hasTwin) || (n == networkMainnet && isTwin) {
				continue
			}
		}

		out = append(out, filepath.ToSlash(filepath.Join(dir, name)))
	}

	sort.Strings(out)

	return out, nil
}

var hashLine = regexp.MustCompile(`(?m)^// Hash: [0-9a-f]{64}$`)

// setSourceHash rewrites the "// Hash:" header of sszgen's output. sszgen
// hashes its input files in Go map order, so its own value is not
// reproducible as soon as a package has more than one file; use the same
// algorithm over the files sorted by name instead.
func setSourceHash(data []byte, dir string) ([]byte, error) {
	pkgs, err := parser.ParseDir(token.NewFileSet(), dir, nil, parser.AllErrors)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", dir, err)
	}

	files := make(map[string]*ast.File)
	for name, pkg := range pkgs {
		if strings.HasSuffix(name, "_test") || name == "ignore" {
			continue
		}

		files = pkg.Files
	}

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}

	sort.Strings(names)

	var content strings.Builder
	for _, name := range names {
		var buf bytes.Buffer
		if err := format.Node(&buf, token.NewFileSet(), files[name]); err != nil {
			return nil, fmt.Errorf("format %s: %w", name, err)
		}

		content.Write(buf.Bytes())
	}

	sum := sha256.Sum256([]byte(content.String()))
	if !hashLine.Match(data) {
		return nil, fmt.Errorf("no hash line in sszgen output")
	}

	return hashLine.ReplaceAll(data, []byte("// Hash: "+hex.EncodeToString(sum[:]))), nil
}

// copyTree copies the regular files of src under dst with writable modes.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return fmt.Errorf("rel: %w", err)
		}

		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o750)
		}

		if !d.Type().IsRegular() {
			return nil
		}

		data, err := os.ReadFile(path) // #nosec G304 -- file of a Go module cache dir
		if err != nil {
			return fmt.Errorf("readFile: %w", err)
		}

		return os.WriteFile(target, data, 0o600)
	})
}
