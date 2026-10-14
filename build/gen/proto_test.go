package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/theQRL/qrysm/testing/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/pluginpb"
)

func TestDownloadVerified(t *testing.T) {
	body := []byte("the archive bytes")
	sum := fmt.Sprintf("%x", sha256.Sum256(body))
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/missing" {
			http.NotFound(w, r)
			return
		}

		_, _ = w.Write(body)
	}))
	defer srv.Close()

	got, err := downloadVerified(srv.URL, sum)
	require.NoError(t, err)
	require.DeepEqual(t, body, got)

	_, err = downloadVerified(srv.URL, strings.Repeat("0", 64))
	require.ErrorContains(t, "sha256 mismatch", err)

	_, err = downloadVerified(srv.URL+"/missing", sum)
	require.ErrorContains(t, "404", err)
}

func TestExtractZipFile(t *testing.T) {
	zipFile := func(t *testing.T, name, content string) *zip.File {
		t.Helper()
		var buf bytes.Buffer
		zw := zip.NewWriter(&buf)
		w, err := zw.Create(name)
		require.NoError(t, err)
		_, err = w.Write([]byte(content))
		require.NoError(t, err)
		require.NoError(t, zw.Close())

		zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
		require.NoError(t, err)

		return zr.File[0]
	}

	dest := t.TempDir()
	require.NoError(t, extractZipFile(zipFile(t, "x/google/api/http.proto", "syntax = \"proto3\";\n"), dest, "google/api/http.proto"))
	got, err := os.ReadFile(filepath.Join(dest, "google", "api", "http.proto"))
	require.NoError(t, err)
	require.Equal(t, "syntax = \"proto3\";\n", string(got))

	err = extractZipFile(zipFile(t, "evil", "x"), dest, "../evil")
	require.ErrorContains(t, "escapes destination directory", err)
}

func TestCompileDescriptors(t *testing.T) {
	stage := t.TempDir()
	writeRepoFile(t, stage, "common.proto", "syntax = \"proto3\";\npackage common;\n// C is common.\nmessage C { int32 x = 1; }\n")
	writeRepoFile(t, stage, "a.proto", "syntax = \"proto3\";\npackage a;\nimport \"common.proto\";\nimport \"google/protobuf/empty.proto\";\nmessage A { common.C c = 1; google.protobuf.Empty e = 2; }\n")
	writeRepoFile(t, stage, "b.proto", "syntax = \"proto3\";\npackage b;\nimport \"common.proto\";\nmessage B { common.C c = 1; }\n")

	got, err := compileDescriptors([]string{stage, t.TempDir()}, []string{"a.proto", "b.proto"})
	require.NoError(t, err)

	var names []string
	for _, fd := range got {
		names = append(names, fd.GetName())
		// rules_go hands the plugins descriptors without source info.
		require.Equal(t, true, fd.SourceCodeInfo == nil)
	}

	// Imports come before the files that import them, each exactly once.
	require.DeepEqual(t, []string{"common.proto", "google/protobuf/empty.proto", "a.proto", "b.proto"}, names)
	// protoc sets json_name on every field; the generated raw descriptors
	// depend on it.
	require.Equal(t, "x", got[0].GetMessageType()[0].GetField()[0].GetJsonName())
}

func TestRunPlugin(t *testing.T) {
	root := t.TempDir()
	plant := func(t *testing.T, name string, resp *pluginpb.CodeGeneratorResponse) string {
		t.Helper()
		data, err := proto.Marshal(resp)
		require.NoError(t, err)
		respFile := filepath.Join(root, name+".bin")
		require.NoError(t, os.WriteFile(respFile, data, 0o600))
		plugin := filepath.Join(root, name)
		script := "#!/bin/sh\ncat > /dev/null\ncat " + respFile + "\n"
		require.NoError(t, os.WriteFile(plugin, []byte(script), 0o755)) // #nosec G306 -- test fake must be executable

		return plugin
	}

	name, content := "proto/foo/foo.pb.go", "package foo\n"
	ok := plant(t, "ok", &pluginpb.CodeGeneratorResponse{File: []*pluginpb.CodeGeneratorResponse_File{{Name: &name, Content: &content}}})
	out := filepath.Join(root, "out")
	require.NoError(t, runPlugin(ok, request(nil, "", nil), out))
	got, err := os.ReadFile(filepath.Join(out, "proto", "foo", "foo.pb.go"))
	require.NoError(t, err)
	require.Equal(t, content, string(got))

	msg := "bad input"
	failing := plant(t, "failing", &pluginpb.CodeGeneratorResponse{Error: &msg})
	require.ErrorContains(t, "plugin reported error: bad input", runPlugin(failing, request(nil, "", nil), out))

	escape := "../escape.go"
	escaping := plant(t, "escaping", &pluginpb.CodeGeneratorResponse{File: []*pluginpb.CodeGeneratorResponse_File{{Name: &escape, Content: &content}}})
	require.ErrorContains(t, "escapes the output directory", runPlugin(escaping, request(nil, "", nil), out))
}

func TestPluginForMode(t *testing.T) {
	const binDir = "/tmp/bin"
	opts := []string{"paths=source_relative"}

	for _, tc := range []struct {
		mode, plugin, param string
	}{
		{modeCast, "protoc-gen-go-cast", "paths=source_relative"},
		{modeCastGRPC, "protoc-gen-go-cast", "plugins=grpc,silent=true,paths=source_relative"},
		{modeStock, "protoc-gen-go", "paths=source_relative"},
	} {
		plugin, param, err := pluginForMode(tc.mode, binDir, opts)
		require.NoError(t, err)
		require.Equal(t, filepath.Join(binDir, tc.plugin), plugin)
		require.Equal(t, tc.param, param)
	}

	_, _, err := pluginForMode("bogus", binDir, opts)
	require.ErrorContains(t, "unknown proto plugin mode", err)
}

func TestStageProtos(t *testing.T) {
	root := t.TempDir()
	writeRepoFile(t, root, "proto/qrl/types.proto", "// FOO and F\nAA BB\n")
	writeRepoFile(t, root, "proto/qrl/notes.txt", "FOO")
	t.Chdir(root)

	// Keys are applied longest-first: "FOO" -> "Bar" before "F" -> "X".
	dict := map[string]string{"FOO": "Bar", "F": "X", "AA": "1", "BB": "2"}
	stage := filepath.Join(root, "stage")
	require.NoError(t, stageProtos(stage, dict))

	got, err := os.ReadFile(filepath.Join(stage, "proto", "qrl", "types.proto"))
	require.NoError(t, err)
	require.Equal(t, "// Bar and X\n1 2\n", string(got))

	_, err = os.Stat(filepath.Join(stage, "proto", "qrl", "notes.txt"))
	require.Equal(t, true, os.IsNotExist(err))
}

func TestBuildMMAP(t *testing.T) {
	stage := t.TempDir()
	writeRepoFile(t, stage, "proto/testing/test.proto", "")
	writeRepoFile(t, stage, "proto/qrl/v1/a.proto", "")

	got, err := buildMMAP(stage)
	require.NoError(t, err)
	require.Equal(t, ",Mproto/qrl/v1/a.proto=github.com/theQRL/qrysm/proto/qrl/v1"+
		",Mproto/testing/test.proto=github.com/theQRL/qrysm/proto/testing"+
		",Mgoogle/api/annotations.proto=google.golang.org/genproto/googleapis/api/annotations"+
		",Mgoogle/api/http.proto=google.golang.org/genproto/googleapis/api/annotations", got)
}

func TestPkgProtos(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, dir, "zeta.proto", "")
	writeRepoFile(t, dir, "alpha.proto", "")
	writeRepoFile(t, dir, "notes.txt", "")
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "nested.proto"), 0o700))

	got, err := pkgProtos(dir)
	require.NoError(t, err)
	require.DeepEqual(t, []string{
		filepath.ToSlash(filepath.Join(dir, "alpha.proto")),
		filepath.ToSlash(filepath.Join(dir, "zeta.proto")),
	}, got)

	_, err = pkgProtos(filepath.Join(dir, "missing"))
	require.ErrorContains(t, "readDir", err)
}

func TestMinimalTwin(t *testing.T) {
	require.Equal(t, "proto/x/a.minimal.pb.go", minimalTwin("proto/x/a.pb.go"))
	require.Equal(t, "proto/x/a.minimal.pb.gw.go", minimalTwin("proto/x/a.pb.gw.go"))
	require.Equal(t, "generated.minimal.ssz.go", minimalTwin("generated.ssz.go"))
}

func TestWriteProtoOutputs(t *testing.T) {
	root := t.TempDir()
	// Stale files of an earlier run: one whose proto went away, one twin that
	// is no longer needed. Hand-written files stay.
	writeRepoFile(t, root, "proto/x/gone.pb.go", "stale")
	writeRepoFile(t, root, "proto/x/same.minimal.pb.go", "stale")
	writeRepoFile(t, root, "proto/x/hand.go", "package x\n")
	writeRepoFile(t, root, "proto/x/sub/keep.pb.go", "other package")

	outMain, outMin := filepath.Join(root, "main"), filepath.Join(root, "min")
	writeRepoFile(t, outMain, "proto/x/same.pb.go", "same\n")
	writeRepoFile(t, outMin, "proto/x/same.pb.go", "same\n")
	writeRepoFile(t, outMain, "proto/x/diff.pb.go", "mainnet\n")
	writeRepoFile(t, outMin, "proto/x/diff.pb.go", "minimal\n")
	writeRepoFile(t, outMain, "proto/x/svc.pb.gw.go", "gw\n")
	writeRepoFile(t, outMin, "proto/x/svc.pb.gw.go", "gw\n")
	t.Chdir(root)

	written, err := writeProtoOutputs([]protoPkg{{dir: "proto/x", mode: modeCast}}, outMain, outMin)
	require.NoError(t, err)
	require.DeepEqual(t, []string{"proto/x/diff.pb.go", "proto/x/diff.minimal.pb.go", "proto/x/same.pb.go", "proto/x/svc.pb.gw.go"}, written)

	read := func(rel string) string {
		data, err := os.ReadFile(rel)
		require.NoError(t, err)
		return string(data)
	}
	require.Equal(t, "//go:build !minimal\n\nmainnet\n", read("proto/x/diff.pb.go"))
	require.Equal(t, "//go:build minimal\n\nminimal\n", read("proto/x/diff.minimal.pb.go"))
	require.Equal(t, "same\n", read("proto/x/same.pb.go"))
	require.Equal(t, "package x\n", read("proto/x/hand.go"))
	require.Equal(t, "other package", read("proto/x/sub/keep.pb.go"))

	for _, gone := range []string{"proto/x/gone.pb.go", "proto/x/same.minimal.pb.go"} {
		_, err := os.Stat(gone)
		require.Equal(t, true, os.IsNotExist(err), gone)
	}

	require.NoError(t, applyGenModes(written))
	mode := func(rel string) os.FileMode {
		info, err := os.Stat(rel)
		require.NoError(t, err)
		return info.Mode().Perm()
	}
	require.Equal(t, os.FileMode(0o755), mode("proto/x/diff.pb.go"))
	require.Equal(t, os.FileMode(0o755), mode("proto/x/svc.pb.gw.go"))
	require.Equal(t, os.FileMode(0o644), mode("proto/x/diff.minimal.pb.go"))

	// A file only one configuration generated is an error.
	writeRepoFile(t, outMain, "proto/x/only.pb.go", "x")
	_, err = writeProtoOutputs([]protoPkg{{dir: "proto/x", mode: modeCast}}, outMain, outMin)
	require.ErrorContains(t, "mainnet generated 4 files, minimal 3", err)
}
