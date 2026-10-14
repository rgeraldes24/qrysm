package main

// This file is the single point where build/gen reads its generation config
// from the Bazel files, which stay the source of truth while Bazel builds the
// repository:
//
//   - proto/**/BUILD.bazel -> the SSZ targets (ssz_gen_marshal)
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
