package main

import (
	"errors"
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

// genSSZ generates the SSZ code of every target from the checked-in .pb.go
// files: the mainnet ones, and for the minimal configuration the
// .minimal.pb.go twins where they exist. Run it after genProto.
func genSSZ() error {
	tmpRoot, err := os.MkdirTemp("", "gen-ssz-")
	if err != nil {
		return fmt.Errorf("mkdirTemp: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpRoot) }()

	sszgen, err := buildSSZGen(tmpRoot)
	if err != nil {
		return fmt.Errorf("build sszgen: %w", err)
	}

	for _, t := range sszTargets {
		if err := genSSZTarget(sszgen, t); err != nil {
			return fmt.Errorf("%s: %w", t.pkg, err)
		}
	}

	return nil
}

// genSSZTarget writes the target's generated.ssz.go, untagged when both
// configurations generate the same code, otherwise tagged "!minimal" next to a
// generated.minimal.ssz.go tagged "minimal".
func genSSZTarget(sszgen string, t sszTarget) error {
	out := filepath.Join(t.pkg, sszOut)
	twin := filepath.Join(t.pkg, minimalTwin(sszOut))
	fmt.Printf("generating %s\n", out)

	mainnet, err := sszgenOne(sszgen, t, false)
	if err != nil {
		return fmt.Errorf("mainnet: %w", err)
	}

	minimal, err := sszgenOne(sszgen, t, true)
	if err != nil {
		return fmt.Errorf("minimal: %w", err)
	}

	if mainnet == minimal {
		if err := os.Remove(twin); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove: %w", err)
		}

		return writeSSZ(out, []byte(mainnet))
	}

	if err := writeSSZ(out, withBuildTag("!minimal", []byte(mainnet))); err != nil {
		return err
	}

	return writeSSZ(twin, withBuildTag("minimal", []byte(minimal)))
}

func writeSSZ(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0o644); err != nil { // #nosec G306 -- checked-in source file
		return fmt.Errorf("writeFile: %w", err)
	}

	return nil
}

// sszgenOne runs sszgen on the .pb.go files of the target's package and
// returns the generated code without its "// Hash:" line, which only records
// a digest of the inputs and would change with every .pb.go header.
func sszgenOne(sszgen string, t sszTarget, minimal bool) (string, error) {
	stage := filepath.Join(t.pkg, ".sszgen_tmp")
	if err := stagePbgo(t.pkg, stage, minimal); err != nil {
		return "", fmt.Errorf("stagePbgo: %w", err)
	}

	defer unstage(stage)

	inc := slices.Clone(t.libInc)
	for _, p := range t.protoInc {
		istage := filepath.Join(p, ".sszinc_tmp")
		if err := stagePbgo(p, istage, minimal); err != nil {
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

// stagePbgo copies the .pb.go files of pkgDir into stageDir, so sszgen parses
// the message types and nothing else of the package. With minimal, a file's
// .minimal.pb.go twin is copied in its place when it has one.
func stagePbgo(pkgDir, stageDir string, minimal bool) error {
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

		src := filepath.Join(pkgDir, name)
		if minimal {
			twin := filepath.Join(pkgDir, minimalTwin(name))
			if _, err := os.Stat(twin); err == nil {
				src = twin
			}
		}

		data, err := os.ReadFile(src) // #nosec G304 -- src from a controlled ReadDir of repo proto packages
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
