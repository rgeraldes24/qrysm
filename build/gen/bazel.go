package main

// This file is the single point where build/gen reads its generation config
// from the Bazel files, which are the source of truth while both build systems
// coexist:
//
//   - proto/ssz_proto_library.bzl -> the mainnet/minimal SSZ substitution dicts
//   - proto/**/BUILD.bazel        -> the proto packages, their protoc plugins
//                                    (go_proto_library / go_proto_compiler) and
//                                    the SSZ targets (ssz_gen_marshal)
//
// Nothing else in build/gen hardcodes this config. When Bazel is eventually
// removed, only this file needs to change.

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bazelbuild/buildtools/build"
)

const sszProtoLibraryBzl = "proto/ssz_proto_library.bzl"

// Options of the go_gen_grpc_gateway compiler declared by grpc-gateway's own
// protoc-gen-grpc-gateway/BUILD.bazel.
var defaultGatewayOptions = []string{"logtostderr=true", "allow_repeated_fields_in_body=true"}

// parseBazel reads and parses a BUILD.bazel or .bzl file.
func parseBazel(path string) (*build.File, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- path is a repo-relative Bazel file
	if err != nil {
		return nil, fmt.Errorf("readFile: %w", err)
	}

	f, err := build.Parse(path, data)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	return f, nil
}

// topLevelAssignments collects the file's top-level `name = <expr>` statements.
func topLevelAssignments(f *build.File) map[string]build.Expr {
	env := make(map[string]build.Expr)
	for _, stmt := range f.Stmt {
		assign, ok := stmt.(*build.AssignExpr)
		if !ok || assign.Op != "=" {
			continue
		}

		if lhs, ok := assign.LHS.(*build.Ident); ok {
			env[lhs.Name] = assign.RHS
		}
	}

	return env
}

// loadSSZDicts returns the mainnet and minimal SSZ-size substitution maps.
func loadSSZDicts() (mainnet, minimal map[string]string, err error) {
	f, err := parseBazel(sszProtoLibraryBzl)
	if err != nil {
		return nil, nil, err
	}

	env := topLevelAssignments(f)

	mainnet, err = stringDict(env, "mainnet")
	if err != nil {
		return nil, nil, fmt.Errorf("string dict: %w", err)
	}

	minimal, err = stringDict(env, "minimal")
	if err != nil {
		return nil, nil, fmt.Errorf("string dict: %w", err)
	}

	return mainnet, minimal, nil
}

// stringDict resolves a top-level `name = {...}` assignment to a string map.
func stringDict(env map[string]build.Expr, name string) (map[string]string, error) {
	e, ok := env[name]
	if !ok {
		return nil, fmt.Errorf("%s: dict %q not found", sszProtoLibraryBzl, name)
	}

	dict, ok := e.(*build.DictExpr)
	if !ok {
		return nil, fmt.Errorf("%s: %q is not a dict (%T)", sszProtoLibraryBzl, name, e)
	}

	out := make(map[string]string, len(dict.List))
	for _, kv := range dict.List {
		key, ok := kv.Key.(*build.StringExpr)
		if !ok {
			return nil, fmt.Errorf("%s: %q has a non-string key", sszProtoLibraryBzl, name)
		}

		val, ok := kv.Value.(*build.StringExpr)
		if !ok {
			return nil, fmt.Errorf("%s: %q[%q] has a non-string value", sszProtoLibraryBzl, name, key.Value)
		}

		out[key.Value] = val.Value
	}

	return out, nil
}

// buildBazelFiles returns every proto/**/BUILD.bazel path, sorted.
func buildBazelFiles() ([]string, error) {
	var paths []string
	err := filepath.WalkDir("proto", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return fmt.Errorf("walk dir: %w", err)
		}

		if !d.IsDir() && d.Name() == "BUILD.bazel" {
			paths = append(paths, filepath.ToSlash(path))
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walkDir: %w", err)
	}

	sort.Strings(paths)

	return paths, nil
}

// protoPkg describes how one proto package is compiled to Go.
type protoPkg struct {
	// mode is the protoc-gen-go flavour generating the *.pb.go files
	// (cast, cast_grpc or stock).
	mode string
	// gateway is true when the package also has a grpc-gateway
	// go_proto_library generating *.pb.gw.go files.
	gateway bool
	// gatewayOpts are the grpc-gateway plugin options.
	gatewayOpts []string
}

// loadProtoPkgs returns the proto package directories that define a
// go_proto_library, mapped to how they are compiled.
func loadProtoPkgs() (map[string]protoPkg, error) {
	files, err := buildBazelFiles()
	if err != nil {
		return nil, fmt.Errorf("build bazel files: %w", err)
	}

	pkgs := make(map[string]protoPkg)
	for _, path := range files {
		f, err := parseBazel(path)
		if err != nil {
			return nil, fmt.Errorf("parse bazel: %w", err)
		}

		rules := f.Rules("go_proto_library")
		if len(rules) == 0 {
			continue
		}

		localCompilers := make(map[string]*build.Rule)
		for _, r := range f.Rules("go_proto_compiler") {
			localCompilers[r.Name()] = r
		}

		var (
			pkg     protoPkg
			hasMode bool
		)
		for _, r := range rules {
			if opts, ok := gatewayOptions(r, localCompilers); ok {
				pkg.gateway = true
				pkg.gatewayOpts = opts
				continue
			}

			// Packages may declare several plain go_proto_library rules for the
			// same protos (e.g. a cast and a stock flavour); the first one wins.
			if !hasMode {
				pkg.mode = compilerMode(r)
				hasMode = true
			}
		}

		if !hasMode {
			return nil, fmt.Errorf("%s: no non-gateway go_proto_library", path)
		}

		pkgs[filepath.ToSlash(filepath.Dir(path))] = pkg
	}

	return pkgs, nil
}

// compilerLabels returns the compiler label(s) of a go_proto_library rule.
func compilerLabels(r *build.Rule) []string {
	labels := r.AttrStrings("compilers")
	if s := r.AttrString("compiler"); s != "" {
		labels = append(labels, s)
	}

	return labels
}

// gatewayOptions reports whether the rule runs the grpc-gateway plugin and,
// if so, with which options. A compiler label either is grpc-gateway's own
// go_gen_grpc_gateway, or names a go_proto_compiler of the same BUILD file
// whose plugin is protoc-gen-grpc-gateway.
func gatewayOptions(r *build.Rule, localCompilers map[string]*build.Rule) ([]string, bool) {
	for _, l := range compilerLabels(r) {
		if strings.HasSuffix(l, "go_gen_grpc_gateway") || strings.HasSuffix(l, ":grpc_gateway_proto_compiler") {
			return defaultGatewayOptions, true
		}

		c, ok := localCompilers[strings.TrimPrefix(l, ":")]
		if ok && strings.Contains(c.AttrString("plugin"), "protoc-gen-grpc-gateway") {
			return c.AttrStrings("options"), true
		}
	}

	return nil, false
}

// compilerMode maps a go_proto_library rule's compiler label(s) to a protoc
// plugin mode.
func compilerMode(r *build.Rule) string {
	labels := compilerLabels(r)

	for _, l := range labels {
		if strings.Contains(l, "grpc") {
			return modeCastGRPC
		}
	}

	for _, l := range labels {
		if strings.Contains(l, "cast") {
			return modeCast
		}
	}

	return modeStock
}

// sszTarget describes one ssz_gen_marshal rule.
type sszTarget struct {
	pkg         string   // proto package dir, e.g. "proto/qrysm/v1alpha1"
	out         string   // generated file, relative to pkg
	includes    []string // repo-relative package dirs sszgen resolves external types from
	objs        []string // objs attr (empty: every struct)
	excludeObjs []string // exclude_objs attr
}

// loadSSZTargets returns the SSZ generation targets read from the
// ssz_gen_marshal rules across proto/**/BUILD.bazel, in (sorted path, file)
// order.
func loadSSZTargets() ([]sszTarget, error) {
	files, err := buildBazelFiles()
	if err != nil {
		return nil, fmt.Errorf("build bazel files: %w", err)
	}

	var targets []sszTarget
	for _, path := range files {
		f, err := parseBazel(path)
		if err != nil {
			return nil, fmt.Errorf("parse bazel: %w", err)
		}

		pkg := filepath.ToSlash(filepath.Dir(path))
		for _, r := range f.Rules("ssz_gen_marshal") {
			if r.AttrString("go_proto") == "" {
				return nil, fmt.Errorf("%s: %s: only go_proto based ssz_gen_marshal rules are supported", path, r.Name())
			}

			var includes []string
			for _, l := range r.AttrStrings("includes") {
				dir, err := labelPkgDir(l)
				if err != nil {
					return nil, fmt.Errorf("%s: %s: %w", path, r.Name(), err)
				}

				includes = append(includes, dir)
			}

			targets = append(targets, sszTarget{
				pkg:         pkg,
				out:         "generated.ssz.go",
				includes:    includes,
				objs:        r.AttrStrings("objs"),
				excludeObjs: r.AttrStrings("exclude_objs"),
			})
		}
	}

	return targets, nil
}

// labelPkgDir maps an absolute label of the main repository
// ("//a/b", "//a/b:c") to its repo-relative package dir ("a/b").
func labelPkgDir(label string) (string, error) {
	if !strings.HasPrefix(label, "//") {
		return "", fmt.Errorf("unsupported label %q (want //pkg[:target])", label)
	}

	pkg, _, _ := strings.Cut(strings.TrimPrefix(label, "//"), ":")

	return pkg, nil
}
