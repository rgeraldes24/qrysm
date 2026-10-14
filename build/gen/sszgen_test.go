package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theQRL/qrysm/testing/require"
)

func TestCopyTree(t *testing.T) {
	src := t.TempDir()
	writeRepoFile(t, src, "go.mod", "module x\n")
	writeRepoFile(t, src, "sszgen/main.go", "package main\n")
	writeRepoFile(t, src, "vendor/modules.txt", "")
	require.NoError(t, os.Chmod(filepath.Join(src, "sszgen", "main.go"), 0o444))

	dst := filepath.Join(t.TempDir(), "copy")
	require.NoError(t, copyTree(src, dst))

	got, err := os.ReadFile(filepath.Join(dst, "sszgen", "main.go"))
	require.NoError(t, err)
	require.Equal(t, "package main\n", string(got))

	// The copy is writable, so the patch can be applied to it.
	require.NoError(t, os.WriteFile(filepath.Join(dst, "sszgen", "main.go"), nil, 0o600))

	_, err = os.Stat(filepath.Join(dst, "vendor"))
	require.Equal(t, true, os.IsNotExist(err))
}

func TestApplyPatch(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, dir, "mod/a.go", "line 1\nline 2\nline 3\n")
	// A hunk without context lines, as fastsszPatch has.
	writeRepoFile(t, dir, "fix.patch", `diff --git a/a.go b/a.go
--- a/a.go
+++ b/a.go
@@ -2 +2 @@
-line 2
+line two
`)

	require.NoError(t, applyPatch(filepath.Join(dir, "mod"), filepath.Join(dir, "fix.patch")))
	got, err := os.ReadFile(filepath.Join(dir, "mod", "a.go"))
	require.NoError(t, err)
	require.Equal(t, "line 1\nline two\nline 3\n", string(got))

	// The file no longer matches, so applying the patch again fails.
	require.ErrorContains(t, "git apply", applyPatch(filepath.Join(dir, "mod"), filepath.Join(dir, "fix.patch")))
}

func TestWriteReplacedModFile(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, dir, "go.mod", "module x\n\nrequire example.com/m v1.0.0\n")
	writeRepoFile(t, dir, "go.sum", "example.com/m v1.0.0 h1:x\n")
	t.Chdir(dir)

	out := t.TempDir()
	modFile, err := writeReplacedModFile(out, "example.com/m", "/tmp/m")
	require.NoError(t, err)
	require.Equal(t, filepath.Join(out, "gen.mod"), modFile)

	mod, err := os.ReadFile(modFile)
	require.NoError(t, err)
	require.Equal(t, true, strings.HasSuffix(string(mod), "\nreplace example.com/m => /tmp/m\n"))

	sum, err := os.ReadFile(filepath.Join(out, "gen.sum"))
	require.NoError(t, err)
	require.Equal(t, "example.com/m v1.0.0 h1:x\n", string(sum))

	writeRepoFile(t, dir, "go.mod", "module x\n\nreplace example.com/m => ../m\n")
	_, err = writeReplacedModFile(out, "example.com/m", "/tmp/m")
	require.ErrorContains(t, "already replaces", err)
}

// TestFastsszPatchApplies applies the checked-in patch to the fastssz version
// go.mod selects, so a fastssz upgrade that breaks the patch fails here and
// not only in make gen.
func TestFastsszPatchApplies(t *testing.T) {
	t.Chdir("../..")

	src, err := moduleDir(fastsszModule)
	require.NoError(t, err)

	dst := filepath.Join(t.TempDir(), "fastssz")
	require.NoError(t, copyTree(src, dst))
	require.NoError(t, applyPatch(dst, fastsszPatch))

	main, err := os.ReadFile(filepath.Join(dst, "sszgen", "main.go"))
	require.NoError(t, err)
	require.StringContains(t, "func (v *Value) validateBitvector(", string(main))
}
