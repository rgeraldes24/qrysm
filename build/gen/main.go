// Command gen regenerates the generated Go code checked into the repository
// without Bazel.
//
//	make gen [proto ssz mocks] [mode=force|no-force]
//
// With no arguments every kind is generated, in that order: the SSZ code is
// generated from the .pb.go files. A kind whose inputs and outputs have not
// changed since its last run on this checkout is skipped (see cache.go),
// unless mode=force.
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

func run(args []string) error {
	fs := flag.NewFlagSet("gen", flag.ContinueOnError)
	modeFlag := fs.String("mode", string(modeNoForce), "force|no-force: force ignores the cache and regenerates everything")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse flags: %w", err)
	}

	m, err := parseMode(*modeFlag)
	if err != nil {
		return err
	}

	kinds := allKinds
	if rest := fs.Args(); len(rest) > 0 {
		kinds = make([]kind, len(rest))
		for i, a := range rest {
			kinds[i] = kind(a)
		}
	}

	if err := chdirRepoRoot(); err != nil {
		return fmt.Errorf("chdir repo root: %w", err)
	}

	cache := loadCache()
	for _, k := range kinds {
		if m != modeForce {
			want, err := manifest(k)
			if err != nil {
				return fmt.Errorf("manifest %s: %w", k, err)
			}

			if cache.Kinds[string(k)] == want {
				fmt.Printf("==> gen %s (up to date, skipped)\n", k)
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
		return fmt.Errorf("unknown kind %q (want %s|%s|%s)", k, kindProto, kindSSZ, kindMocks)
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
	cmd := exec.Command(name, args...)
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

// goimports runs the goimports of the golang.org/x/tools version go.mod
// selects.
func goimports(paths ...string) error {
	return sh("go", append([]string{"run", "golang.org/x/tools/cmd/goimports", "-w"}, paths...)...)
}

func gofmtSimplify(paths ...string) error {
	return sh("gofmt", append([]string{"-s", "-w"}, paths...)...)
}
