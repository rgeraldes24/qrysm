package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/theQRL/qrysm/testing/require"
)

func TestLoadCache(t *testing.T) {
	for _, tc := range []struct {
		name string
		raw  string
		want map[string]string
	}{
		{name: "absent", want: map[string]string{}},
		{name: "valid", raw: `{"version":1,"kinds":{"proto":"abc","ssz":"def"}}`, want: map[string]string{"proto": "abc", "ssz": "def"}},
		{name: "other version", raw: `{"version":999,"kinds":{"proto":"abc"}}`, want: map[string]string{}},
		{name: "corrupt", raw: `{`, want: map[string]string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.raw != "" {
				writeRepoFile(t, dir, cacheFile, tc.raw)
			}
			t.Chdir(dir)

			got := loadCache()
			require.Equal(t, cacheVersion, got.Version)
			require.DeepEqual(t, tc.want, got.Kinds)
		})
	}
}

func TestStoreCache(t *testing.T) {
	t.Chdir(t.TempDir())

	require.NoError(t, storeCache(genCache{Kinds: map[string]string{"mocks": "deadbeef"}}))

	got := loadCache()
	require.Equal(t, cacheVersion, got.Version)
	require.DeepEqual(t, map[string]string{"mocks": "deadbeef"}, got.Kinds)
}

func TestFileSum(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, dir, "f.txt", "hello world\n")

	got, err := fileSum(filepath.Join(dir, "f.txt"))
	require.NoError(t, err)
	want := sha256.Sum256([]byte("hello world\n"))
	require.Equal(t, hex.EncodeToString(want[:]), got)

	got, err = fileSum(filepath.Join(dir, "missing"))
	require.NoError(t, err)
	require.Equal(t, "absent", got)

	_, err = fileSum(dir)
	require.ErrorContains(t, "readFile", err)
}

func TestManifest(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, dir, "go.mod", "module x\n")
	writeRepoFile(t, dir, "go.sum", "")
	writeRepoFile(t, dir, "build/gen/main.go", "package main\n")
	writeRepoFile(t, dir, "proto/x/a.proto", "syntax = \"proto3\";\n")
	writeRepoFile(t, dir, "proto/x/a.pb.go", "package x\n")
	writeRepoFile(t, dir, "proto/x/BUILD.bazel", "")
	writeRepoFile(t, dir, "proto/x/hand.go", "package x\n")
	writeRepoFile(t, dir, sszProtoLibraryBzl, "mainnet = {}\n")
	t.Chdir(dir)

	first, err := manifest(kindProto)
	require.NoError(t, err)
	second, err := manifest(kindProto)
	require.NoError(t, err)
	require.Equal(t, first, second)

	// A hand-written file is neither input nor output.
	writeRepoFile(t, dir, "proto/x/hand.go", "package x\n\nvar _ = 1\n")
	same, err := manifest(kindProto)
	require.NoError(t, err)
	require.Equal(t, first, same)

	// Inputs, outputs and the generator invalidate it.
	for _, rel := range []string{"proto/x/a.proto", "proto/x/a.pb.go", "go.mod", "build/gen/main.go"} {
		writeRepoFile(t, dir, rel, "changed "+rel)
		changed, err := manifest(kindProto)
		require.NoError(t, err)
		require.NotEqual(t, first, changed, rel)
		first = changed
	}

	// So does a new generated file, such as a minimal twin.
	writeRepoFile(t, dir, "proto/x/a.minimal.pb.go", "package x\n")
	changed, err := manifest(kindProto)
	require.NoError(t, err)
	require.NotEqual(t, first, changed)
	require.NoError(t, os.Remove(filepath.Join(dir, "proto/x/a.minimal.pb.go")))
}

// TestManifest_Repo computes every kind's manifest over the repository, so a
// generator change that leaves a kind's file set unreadable fails here.
func TestManifest_Repo(t *testing.T) {
	t.Chdir("../..")

	for _, k := range allKinds {
		files, err := kindFiles(k)
		require.NoError(t, err)
		require.NotEmpty(t, files)
		for _, f := range files {
			if _, err := os.Stat(f); err != nil && !os.IsNotExist(err) {
				t.Fatalf("%s: %v", f, err)
			}
		}
	}

	files, err := kindFiles(kindSSZ)
	require.NoError(t, err)
	for _, want := range []string{fastsszPatch, "consensus-types/primitives/epoch.go", "proto/engine/v1/execution_engine.minimal.pb.go"} {
		found := false
		for _, f := range files {
			found = found || f == want
		}
		require.Equal(t, true, found, want)
	}
}

func TestDedupeSorted(t *testing.T) {
	require.DeepEqual(t, []string{"a", "b", "c"}, dedupeSorted([]string{"c", "a", "b", "a"}))
}
