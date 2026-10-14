package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// sszOut is the checked-in name of the file an ssz_gen_marshal target
// generates.
const sszOut = "generated.ssz.go"

type sszTarget struct {
	pkg      string
	libInc   []string
	protoInc []string
	objs     []string
	exclude  []string
}

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
		fmt.Printf("generating %s/%s\n", t.pkg, sszOut)
		out, err := sszgenOne(sszgen, t)
		if err != nil {
			return fmt.Errorf("%s: %w", t.pkg, err)
		}

		if err := os.WriteFile(filepath.Join(t.pkg, sszOut), []byte(out), 0o644); err != nil { // #nosec G306 -- checked-in source file
			return fmt.Errorf("writeFile: %w", err)
		}
	}

	return nil
}

// sszgenOne runs sszgen on the .pb.go files of the target's package and
// returns the generated code without its "// Hash:" line, which only records
// a digest of the inputs and would change with every .pb.go header.
func sszgenOne(sszgen string, t sszTarget) (string, error) {
	stage := filepath.Join(t.pkg, ".sszgen_tmp")
	if err := stagePbgo(t.pkg, stage); err != nil {
		return "", fmt.Errorf("stagePbgo: %w", err)
	}

	defer unstage(stage)

	inc := slices.Clone(t.libInc)
	for _, p := range t.protoInc {
		istage := filepath.Join(p, ".sszinc_tmp")
		if err := stagePbgo(p, istage); err != nil {
			return "", fmt.Errorf("stagePbgo: %w", err)
		}

		defer unstage(istage)
		inc = append(inc, istage)
	}

	tmp, err := os.CreateTemp("", "sszgen-*.go")
	if err != nil {
		return "", fmt.Errorf("createTemp: %w", err)
	}

	tmpName := tmp.Name()
	if err := tmp.Close(); err != nil {
		return "", fmt.Errorf("close: %w", err)
	}

	defer func() { _ = os.Remove(tmpName) }()

	args := []string{"--output=" + tmpName, "--path=" + stage, "--objs=" + strings.Join(t.objs, ",")}
	if len(inc) > 0 {
		args = append(args, "--include="+strings.Join(inc, ","))
	}

	if len(t.exclude) > 0 {
		args = append(args, "--exclude-objs="+strings.Join(t.exclude, ","))
	}

	if err := sh(sszgen, args...); err != nil {
		return "", fmt.Errorf("sszgen: %w", err)
	}

	data, err := os.ReadFile(tmpName) // #nosec G304 -- tmpName is our own os.CreateTemp output
	if err != nil {
		return "", fmt.Errorf("readFile: %w", err)
	}

	return stripHashLine(string(data)), nil
}

func stripHashLine(s string) string {
	var b strings.Builder
	for _, line := range strings.SplitAfter(s, "\n") {
		if strings.HasPrefix(line, "// Hash: ") {
			continue
		}

		b.WriteString(line)
	}

	return b.String()
}

// stagePbgo copies the mainnet .pb.go files of pkgDir into stageDir, so
// sszgen parses the message types and nothing else of the package.
func stagePbgo(pkgDir, stageDir string) error {
	if err := os.MkdirAll(stageDir, 0o750); err != nil {
		return fmt.Errorf("mkdirAll: %w", err)
	}

	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return fmt.Errorf("readDir: %w", err)
	}

	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".pb.go") || strings.HasSuffix(name, ".minimal.pb.go") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(pkgDir, name)) // #nosec G304 -- pkgDir/name from a controlled ReadDir of repo proto packages
		if err != nil {
			return fmt.Errorf("readFile: %w", err)
		}

		if err := os.WriteFile(filepath.Join(stageDir, name), data, 0o600); err != nil {
			return fmt.Errorf("writeFile: %w", err)
		}
	}

	return nil
}

func unstage(dir string) { _ = os.RemoveAll(dir) }
