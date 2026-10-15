package bazel

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/theQRL/qrysm/testing/require"
)

func TestModuleRoot(t *testing.T) {
	root, err := moduleRoot()
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(root, "go.mod"))
	require.NoError(t, err)
	_, err = os.Stat(filepath.Join(root, "build", "bazel", "runfiles.go"))
	require.NoError(t, err)

	t.Chdir(t.TempDir())
	_, err = moduleRoot()
	require.ErrorContains(t, "could not locate go.mod", err)
	require.Equal(t, "", testdataRoot())
}

func TestFindBinary(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "beacon-chain")
	require.NoError(t, os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755)) // #nosec G306 -- test fake must be executable
	t.Setenv("QRYSM_BIN", dir)

	got, ok := FindBinary("cmd/beacon-chain", "beacon-chain")
	require.Equal(t, true, ok)
	require.Equal(t, bin, got)

	_, ok = FindBinary("", "no-such-binary")
	require.Equal(t, false, ok)
}

func TestRunfile(t *testing.T) {
	// Relative to the working directory (this package).
	got, err := Runfile("runfiles.go")
	require.NoError(t, err)
	require.Equal(t, true, filepath.IsAbs(got))

	// Relative to the module root.
	got, err = Runfile("go.mod")
	require.NoError(t, err)
	root, err := moduleRoot()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "go.mod"), got)

	// Absolute.
	got, err = Runfile(filepath.Join(root, "go.mod"))
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "go.mod"), got)

	// A workspace-absolute path, as the e2e static files are named.
	got, err = Runfile("/build/bazel/runfiles.go")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(root, "build", "bazel", "runfiles.go"), got)

	_, err = Runfile("no/such/file")
	require.ErrorContains(t, "could not locate file", err)
	_, err = Runfile(filepath.Join(root, "no-such-file"))
	require.ErrorContains(t, "could not locate file", err)
}

func TestListRunfiles(t *testing.T) {
	// The repository has no third_party/testdata, so there is nothing to list.
	entries, err := ListRunfiles()
	require.NoError(t, err)

	root, err := moduleRoot()
	require.NoError(t, err)
	if _, err := os.Stat(filepath.Join(root, "third_party", "testdata")); os.IsNotExist(err) {
		require.Equal(t, 0, len(entries))
	}
}

func TestTmpDirs(t *testing.T) {
	require.Equal(t, os.TempDir(), TestTmpDir())
}

func TestTestDataPath(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.Equal(t, filepath.Join(cwd, "testdata", "a.txt"), TestDataPath(t, "a.txt"))
}
