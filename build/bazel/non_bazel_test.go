//go:build !bazel

package bazel

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuiltWithBazel_NonBazel(t *testing.T) {
	if BuiltWithBazel() {
		t.Error("built with Bazel")
	}
}

func TestRunfile_NonBazel(t *testing.T) {
	root, err := moduleRoot()
	if err != nil {
		t.Fatal(err)
	}

	// Relative to the module root.
	got, err := Runfile("build/bazel/non_bazel.go")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "build/bazel/non_bazel.go"); got != want {
		t.Errorf("Runfile = %q, want %q", got, want)
	}

	// Relative to the working directory (the package dir under `go test`).
	if _, err := Runfile("non_bazel_test.go"); err != nil {
		t.Error(err)
	}

	// Absolute.
	if got, err := Runfile(filepath.Join(root, "go.mod")); err != nil || got != filepath.Join(root, "go.mod") {
		t.Errorf("Runfile(abs) = %q, %v", got, err)
	}

	if _, err := Runfile("does/not/exist"); err == nil {
		t.Error("expected an error for a missing runfile")
	}
}

func TestFindBinary_NonBazel(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "some-binary")
	if err := os.WriteFile(bin, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv(BinDirEnv, dir)

	got, ok := FindBinary("cmd/some-binary", "some-binary")
	if !ok || got != bin {
		t.Errorf("FindBinary = %q, %v; want %q, true", got, ok, bin)
	}

	if _, ok := FindBinary("", "no-such-binary"); ok {
		t.Error("found a binary that does not exist")
	}
}
