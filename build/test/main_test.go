package main

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theQRL/qrysm/testing/require"
)

func TestSelectKinds(t *testing.T) {
	got, err := selectKinds(nil)
	require.NoError(t, err)
	require.DeepEqual(t, validKinds, got)

	got, err = selectKinds([]string{"minimal", "mainnet"})
	require.NoError(t, err)
	require.DeepEqual(t, []kind{minimal, mainnet}, got)

	_, err = selectKinds([]string{"spectest"})
	require.ErrorContains(t, "not a test pass: spectest", err)
}

func TestPkgFilter(t *testing.T) {
	pkgs := []string{"a", "b", "c"}
	require.DeepEqual(t, pkgs, pkgFilter{}.apply(pkgs))
	require.DeepEqual(t, []string{"a", "c"}, pkgFilter{exclude: map[string]bool{"b": true}}.apply(pkgs))
	require.DeepEqual(t, []string{"b"}, pkgFilter{include: map[string]bool{"b": true, "c": true}, exclude: map[string]bool{"c": true}}.apply(pkgs))
}

func TestNewPkgFilter(t *testing.T) {
	t.Chdir("../..")

	f, err := newPkgFilter("go", "./build/... -./build/gen")
	require.NoError(t, err)
	got := f.apply([]string{
		"github.com/theQRL/qrysm/build/bazel",
		"github.com/theQRL/qrysm/build/gen",
		"github.com/theQRL/qrysm/config/params",
	})
	require.DeepEqual(t, []string{"github.com/theQRL/qrysm/build/bazel"}, got)
}

func TestTestsRunRegexp(t *testing.T) {
	dir := t.TempDir()
	write := func(name, src string) {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600))
	}
	write("a_test.go", "package x\n\nimport \"testing\"\n\nfunc TestB(t *testing.T) {}\nfunc TestMain(m *testing.M) {}\nfunc helper() {}\n")
	write("b_test.go", "package x\n\nimport \"testing\"\n\nfunc TestA(t *testing.T) {}\nfunc BenchmarkA(b *testing.B) {}\n")
	write("c_test.go", "package x\n")

	got, err := testsRunRegexp(dir, []string{"a_test.go", "b_test.go"})
	require.NoError(t, err)
	require.Equal(t, "^(TestA|TestB)$", got)

	_, err = testsRunRegexp(dir, []string{"c_test.go"})
	require.ErrorContains(t, "no tests", err)
}

// TestRaceTargets checks that the files the race pass names exist and hold
// tests, so a renamed file fails here and not in CI.
func TestRaceTargets(t *testing.T) {
	t.Chdir("../..")

	for _, rt := range raceTargets {
		_, err := os.Stat(rt.pkg)
		require.NoError(t, err)
		if rt.files != nil {
			_, err := testsRunRegexp(rt.pkg, rt.files)
			require.NoError(t, err, rt.pkg)
		}
	}
}

func TestExcludeRe(t *testing.T) {
	require.Equal(t, true, excludeRe.MatchString("github.com/theQRL/qrysm/testing/endtoend"))
	require.Equal(t, true, excludeRe.MatchString("github.com/theQRL/qrysm/testing/endtoend/components"))
	require.Equal(t, false, excludeRe.MatchString("github.com/theQRL/qrysm/testing/endtoendx"))
}

func TestStreamProgress(t *testing.T) {
	r, w, err := os.Pipe()
	require.NoError(t, err)
	stdout := os.Stdout
	os.Stdout = w

	streamProgress(strings.NewReader("✓  pkg/a (1s)\nnoise\n✖  pkg/b (2s)\n"), 12)

	os.Stdout = stdout
	require.NoError(t, w.Close())
	var buf bytes.Buffer
	_, err = io.Copy(&buf, r)
	require.NoError(t, err)
	require.Equal(t, "[ 1/12] ✓  pkg/a (1s)\nnoise\n[ 2/12] ✖  pkg/b (2s)\n", buf.String())
}
