package repo

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFile(t *testing.T) {
	root, err := Root()
	if err != nil {
		t.Fatal(err)
	}

	// Relative to the repository root.
	got, err := File("build/repo/repo.go")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(root, "build/repo/repo.go"); got != want {
		t.Errorf("File = %q, want %q", got, want)
	}

	// Relative to the working directory (the package dir under `go test`).
	if _, err := File("repo_test.go"); err != nil {
		t.Error(err)
	}

	// Absolute.
	if got, err := File(filepath.Join(root, "go.mod")); err != nil || got != filepath.Join(root, "go.mod") {
		t.Errorf("File(abs) = %q, %v", got, err)
	}

	if _, err := File("does/not/exist"); err == nil {
		t.Error("expected an error for a missing file")
	}
}

func TestFindBinary(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "some-binary")
	if err := os.WriteFile(bin, nil, 0o600); err != nil {
		t.Fatal(err)
	}

	t.Setenv(BinDirEnv, dir)

	got, ok := FindBinary("some-binary")
	if !ok || got != bin {
		t.Errorf("FindBinary = %q, %v; want %q, true", got, ok, bin)
	}

	if _, ok := FindBinary("no-such-binary"); ok {
		t.Error("found a binary that does not exist")
	}
}
