package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/theQRL/qrysm/testing/require"
)

// writeRepoFile writes content to rel under dir, creating parent directories.
func writeRepoFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

// TestProtoPkgsCoverRepo checks that protoPkgs lists exactly the directories
// holding .proto files, so a new proto package is not silently left out.
func TestProtoPkgsCoverRepo(t *testing.T) {
	t.Chdir("../..")

	dirs := map[string]bool{}
	require.NoError(t, filepath.WalkDir("proto", func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".proto") {
			dirs[filepath.ToSlash(filepath.Dir(path))] = true
		}

		return err
	}))

	var want, got []string
	for d := range dirs {
		want = append(want, d)
	}

	for _, p := range protoPkgs {
		got = append(got, p.dir)
		require.NotEmpty(t, p.mode, p.dir)
	}

	sort.Strings(want)
	require.Equal(t, true, sort.StringsAreSorted(got), "protoPkgs must stay sorted")
	require.DeepEqual(t, want, got)
}

func TestSSZTargets(t *testing.T) {
	t.Chdir("../..")

	for _, tt := range sszTargets {
		require.Equal(t, true, slices.ContainsFunc(protoPkgs, func(p protoPkg) bool { return p.dir == tt.pkg }), tt.pkg)
		for _, dir := range append(slices.Clone(tt.libInc), tt.protoInc...) {
			_, err := os.Stat(dir)
			require.NoError(t, err)
		}
		require.NotEmpty(t, tt.objs)
	}
}

func TestSSZSizes(t *testing.T) {
	keys := func(m map[string]string) []string {
		var out []string
		for k := range m {
			out = append(out, k)
		}

		sort.Strings(out)

		return out
	}

	require.DeepEqual(t, keys(sszSizes["mainnet"]), keys(sszSizes["minimal"]))
	// The qrl/service gateway options extend, and do not alias, the defaults.
	require.Equal(t, 2, len(defaultGatewayOptions))
}
