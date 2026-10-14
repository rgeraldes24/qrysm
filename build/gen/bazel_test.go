package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/bazelbuild/buildtools/build"
	"github.com/theQRL/qrysm/testing/require"
)

// parseContent parses Bazel source into a *build.File, failing the test on error.
func parseContent(t *testing.T, content string) *build.File {
	t.Helper()
	f, err := build.Parse("test.bzl", []byte(content))
	require.NoError(t, err)

	return f
}

// writeRepoFile writes content to rel under dir, creating parent directories.
func writeRepoFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func TestParseBazel(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "BUILD.bazel")
	writeRepoFile(t, dir, "BUILD.bazel", `ssz_gen_marshal(
    name = "ssz_generated_files",
    objs = ["A"],
)
`)

	f, err := parseBazel(path)
	require.NoError(t, err)
	require.Equal(t, path, f.Path)

	rules := f.Rules("ssz_gen_marshal")
	require.Equal(t, 1, len(rules))
	require.Equal(t, "ssz_generated_files", rules[0].Name())

	_, err = parseBazel(filepath.Join(dir, "missing"))
	require.ErrorContains(t, "readFile", err)
}

func TestTopLevelAssignments(t *testing.T) {
	env := topLevelAssignments(parseContent(t, `
name = "value"
items = ["a", "b"]
items += ["c"]
`))

	require.Equal(t, 2, len(env))
	str, ok := env["name"].(*build.StringExpr)
	require.Equal(t, true, ok)
	require.Equal(t, "value", str.Value)

	// The "+=" statement is skipped, so only the original "=" binding remains.
	list, ok := env["items"].(*build.ListExpr)
	require.Equal(t, true, ok)
	require.Equal(t, 2, len(list.List))
}

func TestEvalStringList(t *testing.T) {
	eval := func(t *testing.T, src string) ([]string, error) {
		t.Helper()
		env := topLevelAssignments(parseContent(t, src))
		e, ok := env["target"]
		require.Equal(t, true, ok)

		return evalStringList(e, env)
	}

	for _, tc := range []struct {
		name string
		src  string
		want []string
		err  string
	}{
		{name: "string", src: `target = "a"`, want: []string{"a"}},
		{name: "list", src: `target = ["a", "b"]`, want: []string{"a", "b"}},
		{name: "identifier", src: "base = [\"a\"]\ntarget = base", want: []string{"a"}},
		{name: "concatenation", src: "base = [\"a\"]\ntarget = base + [\"b\"] + [\"c\"]", want: []string{"a", "b", "c"}},
		{name: "unresolved identifier", src: `target = missing`, err: `unresolved identifier "missing"`},
		{name: "unsupported operator", src: `target = ["a"] - ["b"]`, err: `unsupported binary op "-"`},
		{name: "unsupported expression", src: `target = 42`, err: "unsupported expression"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := eval(t, tc.src)
			if tc.err != "" {
				require.ErrorContains(t, tc.err, err)
				return
			}

			require.NoError(t, err)
			require.DeepEqual(t, tc.want, got)
		})
	}
}

func TestBuildBazelFiles(t *testing.T) {
	dir := t.TempDir()
	// Created out of order, with non-matching siblings interleaved.
	writeRepoFile(t, dir, "proto/zeta/BUILD.bazel", "")
	writeRepoFile(t, dir, "proto/alpha/BUILD.bazel", "")
	writeRepoFile(t, dir, "proto/BUILD.bazel", "")
	writeRepoFile(t, dir, "proto/alpha/notes.txt", "")
	writeRepoFile(t, dir, "proto/beta/BUILD", "")
	writeRepoFile(t, dir, "other/BUILD.bazel", "")
	t.Chdir(dir)

	got, err := buildBazelFiles()
	require.NoError(t, err)
	require.DeepEqual(t, []string{
		"proto/BUILD.bazel",
		"proto/alpha/BUILD.bazel",
		"proto/zeta/BUILD.bazel",
	}, got)
}

func TestLoadSSZTargets(t *testing.T) {
	t.Run("reads every ssz_gen_marshal rule in path order", func(t *testing.T) {
		dir := t.TempDir()
		writeRepoFile(t, dir, "proto/bbb/BUILD.bazel", `
ssz_gen_marshal(
    name = "ssz_generated_files",
    objs = ["B"],
)
`)
		writeRepoFile(t, dir, "proto/aaa/BUILD.bazel", `
COMMON = ["//consensus-types/primitives"]
ssz_gen_marshal(
    name = "ssz_generated_files",
    go_proto = ":go_proto",
    objs = ["A1", "A2"],
    exclude_objs = ["Skip"],
    includes = COMMON + ["//proto/engine/v1:engine"],
)
`)
		writeRepoFile(t, dir, "proto/ccc/BUILD.bazel", `go_library(name = "ccc")`)
		t.Chdir(dir)

		got, err := loadSSZTargets()
		require.NoError(t, err)
		require.DeepEqual(t, []sszTarget{
			{
				pkg:      "proto/aaa",
				libInc:   []string{"consensus-types/primitives"},
				protoInc: []string{"proto/engine/v1"},
				objs:     []string{"A1", "A2"},
				exclude:  []string{"Skip"},
			},
			{
				pkg:  "proto/bbb",
				objs: []string{"B"},
			},
		}, got)
	})

	t.Run("rejects two targets in one package", func(t *testing.T) {
		dir := t.TempDir()
		writeRepoFile(t, dir, "proto/aaa/BUILD.bazel", `
ssz_gen_marshal(name = "a", objs = ["A"])
ssz_gen_marshal(name = "b", objs = ["B"])
`)
		t.Chdir(dir)

		_, err := loadSSZTargets()
		require.ErrorContains(t, "2 ssz_gen_marshal rules", err)
	})
}

func TestSplitIncludes(t *testing.T) {
	libInc, protoInc := splitIncludes([]string{
		"//consensus-types/primitives",
		"//proto/engine/v1:engine",
		"//math:go_default_library",
		"//proto/engine/v1",
	})
	require.DeepEqual(t, []string{"consensus-types/primitives", "math"}, libInc)
	require.DeepEqual(t, []string{"proto/engine/v1", "proto/engine/v1"}, protoInc)
}

// TestLoadSSZTargets_Repo checks the targets read from the repository's own
// BUILD files, so a BUILD change that build/gen cannot read fails here.
func TestLoadSSZTargets_Repo(t *testing.T) {
	t.Chdir("../..")

	targets, err := loadSSZTargets()
	require.NoError(t, err)

	var pkgs []string
	for _, tt := range targets {
		pkgs = append(pkgs, tt.pkg)
		require.NotEmpty(t, tt.objs)
	}

	require.DeepEqual(t, []string{"proto/engine/v1", "proto/qrl/v1", "proto/qrysm/v1alpha1"}, pkgs)
}

func TestLoadSSZDicts(t *testing.T) {
	dir := t.TempDir()
	writeRepoFile(t, dir, sszProtoLibraryBzl, `
mainnet = {
    "foo.size": "1",
    "bar.size": "2",
}
minimal = {
    "foo.size": "3",
}
`)
	t.Chdir(dir)

	mainnet, minimal, err := loadSSZDicts()
	require.NoError(t, err)
	require.DeepEqual(t, map[string]string{"foo.size": "1", "bar.size": "2"}, mainnet)
	require.DeepEqual(t, map[string]string{"foo.size": "3"}, minimal)
}

func TestStringDict(t *testing.T) {
	for _, tc := range []struct {
		name, src, err string
	}{
		{name: "absent", src: `d = {}`, err: `dict "missing" not found`},
		{name: "not a dict", src: `missing = "x"`, err: `"missing" is not a dict`},
		{name: "non-string key", src: `missing = {1: "a"}`, err: "non-string key"},
		{name: "non-string value", src: `missing = {"a": 1}`, err: "non-string value"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := stringDict(topLevelAssignments(parseContent(t, tc.src)), "missing")
			require.ErrorContains(t, tc.err, err)
		})
	}
}

func TestProtoPkgFromFile(t *testing.T) {
	for _, tc := range []struct {
		name, src string
		want      protoPkg
		err       string
	}{
		{
			name: "cast grpc with the default gateway compiler",
			src: `
go_proto_library(name = "go_proto", compilers = ["@com_github_prysmaticlabs_protoc_gen_go_cast//:go_cast_grpc"])
go_proto_library(
    name = "go_grpc_gateway_library",
    compilers = ["@com_github_grpc_ecosystem_grpc_gateway_v2//protoc-gen-grpc-gateway:go_gen_grpc_gateway"],
    embed = [":go_proto"],
)
go_library(name = "lib", embed = [":go_grpc_gateway_library"])
`,
			want: protoPkg{dir: "proto/x", mode: modeCastGRPC, gateway: defaultGatewayOptions},
		},
		{
			name: "gateway compiler defined in the package",
			src: `
go_proto_library(name = "go_proto", compilers = ["@com_github_prysmaticlabs_protoc_gen_go_cast//:go_cast_grpc"])
go_proto_compiler(
    name = "gw_compiler",
    options = ["allow_delete_body=true"],
    plugin = "@com_github_grpc_ecosystem_grpc_gateway_v2//protoc-gen-grpc-gateway:protoc-gen-grpc-gateway",
)
go_proto_library(name = "gw", compilers = ["gw_compiler"], embed = [":go_proto"])
go_library(name = "lib", embed = [":gw"])
`,
			want: protoPkg{dir: "proto/x", mode: modeCastGRPC, gateway: []string{"allow_delete_body=true"}},
		},
		{
			name: "only the embedded library counts",
			src: `
go_proto_library(name = "go_proto", compilers = ["@com_github_prysmaticlabs_protoc_gen_go_cast//:go_cast"])
go_proto_library(name = "ext_go_proto")
go_library(name = "lib", embed = [":ext_go_proto"])
`,
			want: protoPkg{dir: "proto/x", mode: modeStock},
		},
		{
			name: "no go_library: every rule counts",
			src:  `go_proto_library(name = "go_proto", compiler = "//:cast_proto_compiler")`,
			want: protoPkg{dir: "proto/x", mode: modeCast},
		},
		{
			name: "unsupported compiler",
			src:  `go_proto_library(name = "go_proto", compilers = ["@io_bazel_rules_go//proto:go_grpc"])`,
			err:  `unsupported compiler "@io_bazel_rules_go//proto:go_grpc"`,
		},
		{
			name: "undefined local compiler",
			src:  `go_proto_library(name = "go_proto", compilers = ["missing"])`,
			err:  `compiler "missing" is not defined in the package`,
		},
		{
			name: "conflicting modes",
			src: `
go_proto_library(name = "a", compilers = ["//:cast_proto_compiler"])
go_proto_library(name = "b", compilers = ["//:cast_grpc_proto_compiler"])
`,
			err: "conflicts with mode",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := protoPkgFromFile("proto/x", parseContent(t, tc.src))
			if tc.err != "" {
				require.ErrorContains(t, tc.err, err)
				return
			}

			require.NoError(t, err)
			require.DeepEqual(t, tc.want, got)
		})
	}
}

// TestLoadProtoPkgs_Repo checks the proto packages read from the repository's
// own BUILD files.
func TestLoadProtoPkgs_Repo(t *testing.T) {
	t.Chdir("../..")

	pkgs, err := loadProtoPkgs()
	require.NoError(t, err)

	allowDeleteBody := []string{"logtostderr=true", "allow_repeated_fields_in_body=true", "allow_delete_body=true"}
	require.DeepEqual(t, []protoPkg{
		{dir: "proto/engine/v1", mode: modeCastGRPC},
		{dir: "proto/qrl/ext", mode: modeStock},
		{dir: "proto/qrl/service", mode: modeCastGRPC, gateway: allowDeleteBody},
		{dir: "proto/qrl/v1", mode: modeCastGRPC, gateway: defaultGatewayOptions},
		{dir: "proto/qrysm/v1alpha1", mode: modeCastGRPC, gateway: defaultGatewayOptions},
		{dir: "proto/qrysm/v1alpha1/validator-client", mode: modeCastGRPC, gateway: defaultGatewayOptions},
		{dir: "proto/testing", mode: modeCast},
	}, pkgs)

	mainnet, minimal, err := loadSSZDicts()
	require.NoError(t, err)
	require.Equal(t, len(mainnet), len(minimal))
}
