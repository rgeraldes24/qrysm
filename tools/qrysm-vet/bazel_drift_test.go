package main

import (
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/bazelbuild/buildtools/build"
	"github.com/theQRL/qrysm/testing/require"
)

// rootBuild parses the root BUILD.bazel.
func rootBuild(t *testing.T) *build.File {
	t.Helper()
	data, err := os.ReadFile("../../BUILD.bazel")
	require.NoError(t, err)
	f, err := build.ParseBuild("BUILD.bazel", data)
	require.NoError(t, err)

	return f
}

func rootRule(t *testing.T, f *build.File, kind, name string) *build.Rule {
	t.Helper()
	for _, r := range f.Rules(kind) {
		if r.Name() == name {
			return r
		}
	}

	t.Fatalf("%s rule %q not found in the root BUILD.bazel", kind, name)
	return nil
}

// TestAnalyzersMatchNogo checks that qrysm-vet runs the analyzers of the
// `nogo` rule of the root BUILD.bazel. Remove it along with Bazel.
func TestAnalyzersMatchNogo(t *testing.T) {
	f := rootBuild(t)
	nogo := rootRule(t, f, "nogo", "nogo")

	var bazelPkgs []string
	build.Walk(nogo.Attr("deps"), func(e build.Expr, _ []build.Expr) {
		s, ok := e.(*build.StringExpr)
		if !ok {
			return
		}

		pkg, _, _ := strings.Cut(s.Value, ":")
		switch {
		case strings.HasPrefix(pkg, "//tools/analyzers/"):
			bazelPkgs = append(bazelPkgs, "github.com/theQRL/qrysm/"+strings.TrimPrefix(pkg, "//"))
		case strings.HasPrefix(pkg, "@org_golang_x_tools//"):
			bazelPkgs = append(bazelPkgs, "golang.org/x/tools/"+strings.TrimPrefix(pkg, "@org_golang_x_tools//"))
		}
	})

	src, err := parser.ParseFile(token.NewFileSet(), "analyzers.go", nil, parser.ImportsOnly)
	require.NoError(t, err)

	var goPkgs []string
	for _, imp := range src.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		require.NoError(t, err)

		if strings.HasPrefix(path, "github.com/theQRL/qrysm/tools/analyzers/") ||
			strings.HasPrefix(path, "golang.org/x/tools/go/analysis/passes/") {
			goPkgs = append(goPkgs, path)
		}
	}

	slices.Sort(bazelPkgs)
	slices.Sort(goPkgs)
	require.DeepEqual(t, bazelPkgs, goPkgs)
}

// TestExclusionsMatchNogo checks globalExcludes and excludeChecks against the
// nogo_config_exclude rule of the root BUILD.bazel. Remove it along with Bazel.
func TestExclusionsMatchNogo(t *testing.T) {
	f := rootBuild(t)
	rule := rootRule(t, f, "nogo_config_exclude", "nogo_config_with_excludes")

	bazelExcludes := rule.AttrStrings("exclude_files")
	for _, e := range bazelExcludes {
		require.Equal(t, true, slices.Contains(globalExcludes, e), "%q is not in globalExcludes", e)
	}

	var checks []string
	for _, stmt := range f.Stmt {
		assign, ok := stmt.(*build.AssignExpr)
		if !ok {
			continue
		}

		if lhs, ok := assign.LHS.(*build.Ident); !ok || lhs.Name != "NOGO_CONFIG_EXCLUDE_CHECKS" {
			continue
		}

		// The list literal, before `+ [sa.upper() ...]`.
		bin, ok := assign.RHS.(*build.BinaryExpr)
		require.Equal(t, true, ok, "NOGO_CONFIG_EXCLUDE_CHECKS changed shape")
		list, ok := bin.X.(*build.ListExpr)
		require.Equal(t, true, ok, "NOGO_CONFIG_EXCLUDE_CHECKS changed shape")
		for _, e := range list.List {
			if s := e.(*build.StringExpr).Value; s != "*" {
				checks = append(checks, s)
			}
		}
	}

	require.DeepEqual(t, checks, excludeChecks)
}

// TestLintPassesMatchBazel checks that the Makefile's develop and minimal lint
// passes list the packages of the Bazel targets built in those
// configurations. Remove it along with Bazel.
func TestLintPassesMatchBazel(t *testing.T) {
	const root = "../.."

	var develop, minimal []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "testdata" || strings.HasPrefix(d.Name(), "bazel-") {
				return filepath.SkipDir
			}

			return nil
		}

		if d.Name() != "BUILD.bazel" {
			return nil
		}

		data, err := os.ReadFile(path) // #nosec G304 -- a BUILD.bazel file of the repository
		if err != nil {
			return err
		}

		f, err := build.ParseBuild(path, data)
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(root, filepath.Dir(path))
		if err != nil {
			return err
		}

		pkg := "./" + filepath.ToSlash(rel)
		for _, r := range f.Rules("") {
			if r.AttrString("qrl_network") == "minimal" && !slices.Contains(minimal, pkg) {
				minimal = append(minimal, pkg)
			}

			if slices.Contains(r.AttrStrings("gotags"), "develop") && !slices.Contains(develop, pkg) {
				develop = append(develop, pkg)
			}
		}

		return nil
	})
	require.NoError(t, err)

	slices.Sort(develop)
	slices.Sort(minimal)

	vars := makefileVars(t, filepath.Join(root, "Makefile"))
	require.DeepEqual(t, develop, vars["VET_DEVELOP"])
	require.DeepEqual(t, minimal, vars["VET_MINIMAL"])
}

// makefileVars returns the `NAME := values...` assignments of a Makefile,
// joining continued lines.
func makefileVars(t *testing.T, path string) map[string][]string {
	data, err := os.ReadFile(path) // #nosec G304 -- the repository's Makefile
	require.NoError(t, err)

	vars := make(map[string][]string)
	text := strings.ReplaceAll(string(data), "\\\n", " ")
	for line := range strings.SplitSeq(text, "\n") {
		name, value, ok := strings.Cut(line, ":=")
		name = strings.TrimSpace(name)
		if !ok || name == "" || strings.ContainsAny(name, " \t#") {
			continue
		}

		vars[name] = strings.Fields(value)
	}

	return vars
}
