package main

// This file is the single point where build/gen reads its generation config
// from the Bazel files, which stay the source of truth while Bazel builds the
// repository:
//
//   - proto/ssz_proto_library.bzl -> the mainnet/minimal SSZ substitution dicts
//   - proto/**/BUILD.bazel         -> the proto packages and their plugins
//                                     (go_proto_library, go_proto_compiler)
//                                     and the SSZ targets (ssz_gen_marshal)
//
// Nothing else in build/gen hardcodes this config.

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bazelbuild/buildtools/build"
)

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

// evalStringList resolves an expression to a flat slice of strings.
func evalStringList(e build.Expr, env map[string]build.Expr) ([]string, error) {
	switch v := e.(type) {
	case *build.StringExpr:
		return []string{v.Value}, nil
	case *build.ListExpr:
		var out []string
		for _, item := range v.List {
			sub, err := evalStringList(item, env)
			if err != nil {
				return nil, fmt.Errorf("list item: %w", err)
			}

			out = append(out, sub...)
		}

		return out, nil
	case *build.Ident:
		ref, ok := env[v.Name]
		if !ok {
			return nil, fmt.Errorf("unresolved identifier %q", v.Name)
		}

		return evalStringList(ref, env)
	case *build.BinaryExpr:
		if v.Op != "+" {
			return nil, fmt.Errorf("unsupported binary op %q", v.Op)
		}

		left, err := evalStringList(v.X, env)
		if err != nil {
			return nil, fmt.Errorf("left side of +: %w", err)
		}

		right, err := evalStringList(v.Y, env)
		if err != nil {
			return nil, fmt.Errorf("right side of +: %w", err)
		}

		return append(left, right...), nil
	default:
		return nil, fmt.Errorf("unsupported expression %T", e)
	}
}

const sszProtoLibraryBzl = "proto/ssz_proto_library.bzl"

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

// The compilers go_proto_library rules name, by label name. The options of
// cast_grpc and of the default grpc-gateway compiler are those of the
// go_proto_compiler rules in the protoc-gen-go-cast and grpc-gateway modules.
var (
	compilerModes = map[string]string{
		"go_cast":                  modeCast,
		"cast_proto_compiler":      modeCast,
		"go_cast_grpc":             modeCastGRPC,
		"cast_grpc_proto_compiler": modeCastGRPC,
		"go_proto":                 modeStock,
		"proto_compiler":           modeStock,
	}
	gatewayCompilers = map[string]bool{
		"go_gen_grpc_gateway":         true,
		"grpc_gateway_proto_compiler": true,
	}
)

// loadProtoPkgs returns the proto package directories that define a
// go_proto_library, sorted, with the plugin their .pb.go files come from and
// the options of the grpc-gateway plugin when they have .pb.gw.go files.
func loadProtoPkgs() ([]protoPkg, error) {
	files, err := buildBazelFiles()
	if err != nil {
		return nil, fmt.Errorf("build bazel files: %w", err)
	}

	var pkgs []protoPkg
	for _, path := range files {
		f, err := parseBazel(path)
		if err != nil {
			return nil, fmt.Errorf("parse bazel: %w", err)
		}

		if len(f.Rules("go_proto_library")) == 0 {
			continue
		}

		p, err := protoPkgFromFile(filepath.ToSlash(filepath.Dir(path)), f)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}

		pkgs = append(pkgs, p)
	}

	return pkgs, nil
}

// protoPkgFromFile reads how a package's Go code is generated from the
// go_proto_library rules its go_library embeds, directly or through another
// go_proto_library. A package can define go_proto_library rules nothing
// embeds (proto/qrl/ext has a cast one next to the stock one it uses); they
// generate nothing that is checked in.
func protoPkgFromFile(dir string, f *build.File) (protoPkg, error) {
	protoRules := make(map[string]*build.Rule)
	for _, r := range f.Rules("go_proto_library") {
		protoRules[r.Name()] = r
	}

	localCompilers := make(map[string]*build.Rule)
	for _, r := range f.Rules("go_proto_compiler") {
		localCompilers[r.Name()] = r
	}

	var used []*build.Rule
	seen := make(map[string]bool)
	var visit func(labels []string)
	visit = func(labels []string) {
		for _, l := range labels {
			r, ok := protoRules[strings.TrimPrefix(l, ":")]
			if !ok || seen[r.Name()] {
				continue
			}

			seen[r.Name()] = true
			used = append(used, r)
			visit(r.AttrStrings("embed"))
		}
	}

	for _, lib := range f.Rules("go_library") {
		visit(lib.AttrStrings("embed"))
	}

	if len(used) == 0 {
		used = f.Rules("go_proto_library")
	}

	p := protoPkg{dir: dir}
	for _, r := range used {
		labels := r.AttrStrings("compilers")
		if c := r.AttrString("compiler"); c != "" {
			labels = append(labels, c)
		}

		if len(labels) == 0 {
			// The default compiler of go_proto_library.
			labels = []string{"@io_bazel_rules_go//proto:go_proto"}
		}

		for _, l := range labels {
			mode, gateway, err := compilerKind(l, localCompilers)
			if err != nil {
				return protoPkg{}, fmt.Errorf("%s: %w", r.Name(), err)
			}

			switch {
			case gateway != nil && p.gateway != nil:
				return protoPkg{}, fmt.Errorf("%s: a second grpc-gateway compiler", r.Name())
			case gateway != nil:
				p.gateway = gateway
			case p.mode != "" && p.mode != mode:
				return protoPkg{}, fmt.Errorf("%s: compiler %s conflicts with mode %s", r.Name(), l, p.mode)
			default:
				p.mode = mode
			}
		}
	}

	if p.mode == "" {
		return protoPkg{}, fmt.Errorf("no go_proto_library generates the .pb.go files")
	}

	return p, nil
}

// compilerKind classifies a compiler label: either the mode of a .pb.go
// plugin, or the options of the grpc-gateway plugin.
func compilerKind(label string, local map[string]*build.Rule) (mode string, gateway []string, err error) {
	name := label
	if i := strings.LastIndexByte(name, ':'); i >= 0 {
		name = name[i+1:]
	}

	if !strings.Contains(label, "//") {
		r, ok := local[name]
		if !ok {
			return "", nil, fmt.Errorf("compiler %q is not defined in the package", label)
		}

		if !strings.HasSuffix(r.AttrString("plugin"), ":protoc-gen-grpc-gateway") {
			return "", nil, fmt.Errorf("compiler %q runs an unsupported plugin %q", label, r.AttrString("plugin"))
		}

		opts := r.AttrStrings("options")
		if opts == nil {
			opts = []string{}
		}

		return "", opts, nil
	}

	if gatewayCompilers[name] {
		return "", defaultGatewayOptions, nil
	}

	mode, ok := compilerModes[name]
	if !ok {
		return "", nil, fmt.Errorf("unsupported compiler %q", label)
	}

	return mode, nil, nil
}

// loadSSZTargets returns the SSZ generation targets read from the
// ssz_gen_marshal rules across proto/**/BUILD.bazel, in sorted path order.
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

		rules := f.Rules("ssz_gen_marshal")
		if len(rules) == 0 {
			continue
		}

		if len(rules) > 1 {
			// Every target writes the package's generated.ssz.go.
			return nil, fmt.Errorf("%s: %d ssz_gen_marshal rules, want at most one", path, len(rules))
		}

		env := topLevelAssignments(f)
		pkg := filepath.ToSlash(filepath.Dir(path))
		t, err := sszTargetFromRule(rules[0], env, pkg)
		if err != nil {
			return nil, fmt.Errorf("%s: %s: %w", path, rules[0].Name(), err)
		}

		targets = append(targets, t)
	}

	return targets, nil
}

// sszTargetFromRule converts a single ssz_gen_marshal rule into an sszTarget.
func sszTargetFromRule(r *build.Rule, env map[string]build.Expr, pkg string) (sszTarget, error) {
	objs, err := attrStringList(r, env, "objs")
	if err != nil {
		return sszTarget{}, fmt.Errorf("objs: %w", err)
	}

	exclude, err := attrStringList(r, env, "exclude_objs")
	if err != nil {
		return sszTarget{}, fmt.Errorf("exclude_objs: %w", err)
	}

	includes, err := attrStringList(r, env, "includes")
	if err != nil {
		return sszTarget{}, fmt.Errorf("includes: %w", err)
	}

	libInc, protoInc := splitIncludes(includes)

	return sszTarget{
		pkg:      pkg,
		libInc:   libInc,
		protoInc: protoInc,
		objs:     objs,
		exclude:  exclude,
	}, nil
}

// attrStringList resolves a rule attribute to a string slice, or nil if absent.
func attrStringList(r *build.Rule, env map[string]build.Expr, key string) ([]string, error) {
	e := r.Attr(key)
	if e == nil {
		return nil, nil
	}

	return evalStringList(e, env)
}

// splitIncludes maps Bazel include labels (e.g. "//consensus-types/primitives")
// to the two buckets the sszgen invocation expects: proto packages (whose
// .pb.go must be staged) and plain Go package directories.
func splitIncludes(labels []string) (libInc, protoInc []string) {
	for _, l := range labels {
		path := strings.TrimPrefix(l, "//")
		if i := strings.IndexByte(path, ':'); i >= 0 {
			path = path[:i]
		}

		if strings.HasPrefix(path, "proto/") {
			protoInc = append(protoInc, path)
			continue
		}

		libInc = append(libInc, path)
	}

	return libInc, protoInc
}
