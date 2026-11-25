// Command gen generates the proto, SSZ and mock files without Bazel.
//
// Usage (from the repository root, usually through `make gen`):
//
//	go run ./build/gen [--mode=force|no-force] [proto] [ssz] [mocks]
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type kind string

const (
	kindProto kind = "proto"
	kindSSZ   kind = "ssz"
	kindMocks kind = "mocks"
)

// allKinds is also the generation order: SSZ reads the generated .pb.go files.
var allKinds = []kind{kindProto, kindSSZ, kindMocks}

type mode string

const (
	modeNoForce mode = "no-force"
	modeForce   mode = "force"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "gen: %v\n", err)
		os.Exit(1)
	}
}

func parseMode(s string) (mode, error) {
	switch m := mode(s); m {
	case modeNoForce, modeForce:
		return m, nil
	default:
		return "", fmt.Errorf("invalid mode %q (want %s|%s)", s, modeNoForce, modeForce)
	}
}

func parseKinds(args []string) ([]kind, error) {
	if len(args) == 0 {
		return allKinds, nil
	}

	want := make(map[kind]bool, len(args))
	for _, a := range args {
		k := kind(a)
		switch k {
		case kindProto, kindSSZ, kindMocks:
			want[k] = true
		default:
			return nil, fmt.Errorf("unknown kind %q (want %s|%s|%s)", a, kindProto, kindSSZ, kindMocks)
		}
	}

	// Keep the canonical order whatever the command line order is.
	kinds := make([]kind, 0, len(want))
	for _, k := range allKinds {
		if want[k] {
			kinds = append(kinds, k)
		}
	}

	return kinds, nil
}

func run(args []string) error {
	fs := flag.NewFlagSet("gen", flag.ContinueOnError)
	modeFlag := fs.String("mode", string(modeNoForce), "force|no-force: force ignores the cache and regenerates everything")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	m, err := parseMode(*modeFlag)
	if err != nil {
		return fmt.Errorf("parse mode: %w", err)
	}

	kinds, err := parseKinds(fs.Args())
	if err != nil {
		return err
	}

	if err := chdirRepoRoot(); err != nil {
		return fmt.Errorf("chdir repo root: %w", err)
	}

	force := m == modeForce
	cache := loadCache()
	for _, k := range kinds {
		if !force {
			want, err := manifest(k)
			if err != nil {
				return fmt.Errorf("manifest %s: %w", k, err)
			}

			if cache.Kinds[string(k)] == want {
				continue
			}
		}

		fmt.Printf("==> gen %s\n", k)
		if err := genKind(k); err != nil {
			return fmt.Errorf("gen %s: %w", k, err)
		}

		// Recompute after generation: the outputs are part of the manifest.
		got, err := manifest(k)
		if err != nil {
			return fmt.Errorf("manifest %s: %w", k, err)
		}

		cache.Kinds[string(k)] = got
		if err := storeCache(cache); err != nil {
			return fmt.Errorf("store cache: %w", err)
		}
	}

	return nil
}

func genKind(k kind) error {
	switch k {
	case kindProto:
		return genProto()
	case kindSSZ:
		return genSSZ()
	case kindMocks:
		return genMocks()
	default:
		return fmt.Errorf("unknown kind %q", k)
	}
}

// chdirRepoRoot walks up from the working directory to the module root (the
// directory containing go.mod) and chdirs there, so every path below is
// repo-relative regardless of where the command was invoked.
func chdirRepoRoot() error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getwd: %w", err)
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return os.Chdir(dir)
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return fmt.Errorf("go.mod not found above %s", dir)
		}

		dir = parent
	}
}

func sh(name string, args ...string) error {
	return shInDir("", nil, name, args...)
}

func shInDir(dir string, env []string, name string, args ...string) error {
	cmd := exec.Command(name, args...) // #nosec G204 -- fixed tool names with repo-controlled arguments
	cmd.Dir = dir
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}

	return nil
}

// goimports runs the goimports version pinned by go.mod (golang.org/x/tools).
func goimports(paths ...string) error {
	return sh("go", append([]string{"run", "golang.org/x/tools/cmd/goimports", "-w"}, paths...)...)
}

func gofmtSimplify(paths ...string) error {
	return sh("gofmt", append([]string{"-s", "-w"}, paths...)...)
}

// goBuildTools builds the given main packages, at the versions pinned by
// go.mod, into a new directory under tmpRoot and returns that directory.
func goBuildTools(tmpRoot string, pkgs ...string) (string, error) {
	binDir := filepath.Join(tmpRoot, "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		return "", fmt.Errorf("mkdir all: %w", err)
	}

	args := append([]string{"build", "-o", binDir + string(os.PathSeparator)}, pkgs...)
	if err := sh("go", args...); err != nil {
		return "", err
	}

	return binDir, nil
}
