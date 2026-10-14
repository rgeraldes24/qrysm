// Command gen regenerates the generated Go code checked into the repository
// without Bazel.
//
//	go run ./build/gen [ssz]
//
// With no arguments every kind is generated.
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type kind string

const (
	kindSSZ kind = "ssz"
)

var allKinds = []kind{kindSSZ}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "gen: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	kinds := allKinds
	if len(args) > 0 {
		kinds = make([]kind, len(args))
		for i, a := range args {
			kinds[i] = kind(a)
		}
	}

	if err := chdirRepoRoot(); err != nil {
		return fmt.Errorf("chdir repo root: %w", err)
	}

	for _, k := range kinds {
		fmt.Printf("==> gen %s\n", k)
		if err := genKind(k); err != nil {
			return fmt.Errorf("gen %s: %w", k, err)
		}
	}

	return nil
}

func genKind(k kind) error {
	switch k {
	case kindSSZ:
		return genSSZ()
	default:
		return fmt.Errorf("unknown kind %q (want %s)", k, kindSSZ)
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
