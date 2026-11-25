package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// modulePath is the module whose packages -deps reports on.
const modulePath = "github.com/theQRL/qrysm"

const depsFlag = "deps"

func init() {
	// Parsed by hand in main (unitchecker parses the flags only once analyzers run), but
	// registered so that `go vet` / `go fix` accept it.
	flag.Bool(depsFlag, false, "also report on (and fix) the dependencies within "+modulePath)
}

// depsEnabled reports whether the -deps flag is set in args (the tool's arguments).
func depsEnabled(args []string) bool {
	return boolFlagSet(args, depsFlag)
}

// fixEnabled reports whether unitchecker's -fix flag is set in args (the tool's arguments).
func fixEnabled(args []string) bool {
	return boolFlagSet(args, "fix")
}

func boolFlagSet(args []string, name string) bool {
	return slices.ContainsFunc(args, func(arg string) bool {
		arg = strings.TrimLeft(arg, "-")
		return arg == name || arg == name+"=true"
	})
}

// rewriteConfig rewrites the package config that `go vet` / `go fix` pass as the last argument
// (`qrysm-vet [flags] <dir>/vet.cfg`):
//   - the config has no Go language version, so packages are type-checked without one (see also
//     the file versions hidden by install). This matters to the analyzers reading the package
//     version, e.g. httpmux.
//   - with deps, a dependency within this module, analyzed only for its facts by default, is
//     also reported on, so that every package a binary or test depends on is analyzed in its
//     configuration (e.g. under `-tags=minimal`), whereas `go vet` only reports on the listed
//     packages, together with their tests (which may not build in that configuration).
func rewriteConfig(args []string, deps bool) error {
	if len(args) < 2 || !strings.HasSuffix(args[len(args)-1], ".cfg") {
		return nil
	}

	path := args[len(args)-1]
	data, err := os.ReadFile(path) // #nosec G304 -- path is the package config written by `go vet` / `go fix`
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	var cfg map[string]json.RawMessage
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}

	cfg["GoVersion"] = json.RawMessage(`""`)

	if deps {
		var pkg struct{ ModulePath string }
		if err := json.Unmarshal(data, &pkg); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}

		if pkg.ModulePath == modulePath {
			cfg["VetxOnly"] = json.RawMessage(`false`)
		}
	}

	data, err = json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", path, err)
	}

	rewritten := filepath.Join(filepath.Dir(path), "qrysm-vet.cfg")
	if err := os.WriteFile(rewritten, data, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", rewritten, err)
	}

	args[len(args)-1] = rewritten

	return nil
}
