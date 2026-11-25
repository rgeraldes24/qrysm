//go:build !bazel

package main

// These tests check that qrysm-vet does not drift from Bazel's nogo while both
// coexist. They read the Bazel files of the repository. Remove them along with
// Bazel.

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

const repoRoot = "../.."

func parseRootBuild(t *testing.T) *build.File {
	data, err := os.ReadFile(filepath.Join(repoRoot, "BUILD.bazel"))
	require.NoError(t, err)

	f, err := build.ParseBuild("BUILD.bazel", data)
	require.NoError(t, err)

	return f
}

func rule(t *testing.T, f *build.File, kind, name string) *build.Rule {
	for _, r := range f.Rules(kind) {
		if r.Name() == name {
			return r
		}
	}

	t.Fatalf("%s rule %q not found", kind, name)

	return nil
}

// stringList returns the strings of a top-level `name = [...]` assignment, or
// of the list literals of a `name = [...] + ...` one.
func stringList(t *testing.T, f *build.File, name string) []string {
	for _, stmt := range f.Stmt {
		assign, ok := stmt.(*build.AssignExpr)
		if !ok {
			continue
		}

		if lhs, ok := assign.LHS.(*build.Ident); !ok || lhs.Name != name {
			continue
		}

		rhs := assign.RHS
		if bin, ok := rhs.(*build.BinaryExpr); ok {
			rhs = bin.X
		}

		list, ok := rhs.(*build.ListExpr)
		require.Equal(t, true, ok, "%s is not a list", name)

		var out []string
		for _, e := range list.List {
			if s, ok := e.(*build.StringExpr); ok {
				out = append(out, s.Value)
			}
		}

		return out
	}

	t.Fatalf("%s not found", name)

	return nil
}

// TestAnalyzersMatchNogo checks that qrysm-vet runs the same analyzers as the
// `nogo` rule of the root BUILD.bazel.
func TestAnalyzersMatchNogo(t *testing.T) {
	f := parseRootBuild(t)
	nogo := rule(t, f, "nogo", "nogo")

	// Analyzer packages of the nogo rule, including those of its select()
	// default branch, but not the select() keys.
	var bazelPkgs []string
	build.Walk(nogo.Attr("deps"), func(e build.Expr, _ []build.Expr) {
		s, ok := e.(*build.StringExpr)
		if !ok {
			return
		}

		if strings.HasPrefix(s.Value, "//tools/analyzers/") || strings.HasPrefix(s.Value, "@org_golang_x_tools//") {
			bazelPkgs = append(bazelPkgs, importPath(t, s.Value))
		}
	})

	// Analyzer packages imported by analyzers.go.
	src, err := parser.ParseFile(token.NewFileSet(), "analyzers.go", nil, parser.ImportsOnly)
	require.NoError(t, err)

	var goPkgs []string
	for _, imp := range src.Imports {
		path, err := strconv.Unquote(imp.Path.Value)
		require.NoError(t, err)

		if strings.HasPrefix(path, modulePath+"/tools/analyzers/") ||
			strings.HasPrefix(path, "golang.org/x/tools/go/analysis/passes/") {
			goPkgs = append(goPkgs, path)
		}
	}

	slices.Sort(bazelPkgs)
	slices.Sort(goPkgs)
	require.DeepEqual(t, bazelPkgs, goPkgs)

	require.DeepEqual(t, stringList(t, f, "STATICCHECK_ANALYZERS"), staticcheckChecks)
}

// TestExclusionsMatchNogo checks that qrysm-vet adds to nogo_config.json the
// exclusions the nogo_config_exclude rule of the root BUILD.bazel adds.
func TestExclusionsMatchNogo(t *testing.T) {
	f := parseRootBuild(t)
	r := rule(t, f, "nogo_config_exclude", "nogo_config_with_excludes")

	require.DeepEqual(t, r.AttrStrings("exclude_files"), excludedFiles)
	require.Equal(t, "NOGO_CONFIG_EXCLUDE_CHECKS", r.Attr("checks").(*build.Ident).Name)

	// The rule also excludes the upper-cased STATICCHECK_ANALYZERS (the `+ [...]`
	// part of NOGO_CONFIG_EXCLUDE_CHECKS), which loadConfigs does as well.
	require.DeepEqual(t, append([]string{"*"}, excludedChecks...), stringList(t, f, "NOGO_CONFIG_EXCLUDE_CHECKS"))
}

func TestLoadConfigsExcludesGeneratedFiles(t *testing.T) {
	configs, err := loadConfigs(rawConfig)
	require.NoError(t, err)

	for _, analyzer := range []string{"errcheck", "printf", "SA4006"} {
		cfg := configFor(configs, analyzer)
		require.NotNil(t, cfg, analyzer)

		for _, file := range []string{
			"proto/qrysm/v1alpha1/beacon_block.pb.go",
			"proto/qrysm/v1alpha1/beacon_block.minimal.pb.go",
			"proto/qrysm/v1alpha1/node.pb.gw.go",
			"proto/qrysm/v1alpha1/generated.ssz.go",
			"proto/qrysm/v1alpha1/generated.minimal.ssz.go",
			"testing/mock/node_service_mock.go",
		} {
			require.Equal(t, false, cfg.keep(file), "%s: %s", analyzer, file)
		}

		require.Equal(t, true, cfg.keep("beacon-chain/node/node.go"), analyzer)
	}

	// Analyzers outside the config and the excluded checks report everywhere.
	require.Equal(t, (*config)(nil), configFor(configs, "sigchanyzer"))
}

// importPath maps a nogo dependency label to its Go import path.
func importPath(t *testing.T, label string) string {
	pkg, _, _ := strings.Cut(label, ":")
	switch {
	case strings.HasPrefix(pkg, "//"):
		return modulePath + "/" + strings.TrimPrefix(pkg, "//")
	case strings.HasPrefix(pkg, "@org_golang_x_tools//"):
		return "golang.org/x/tools/" + strings.TrimPrefix(pkg, "@org_golang_x_tools//")
	}

	t.Fatalf("unexpected nogo dependency %q", label)

	return ""
}

// TestLintPassesMatchBazel checks that the Makefile's lint passes list the
// packages of the Bazel targets built with a non-default configuration.
func TestLintPassesMatchBazel(t *testing.T) {
	passes := map[string][]string{} // Makefile variable -> packages
	add := func(v, pkg string) {
		if !slices.Contains(passes[v], pkg) {
			passes[v] = append(passes[v], pkg)
		}
	}

	err := filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			switch d.Name() {
			case ".git", "testdata", "third_party", "node_modules", "dist":
				return filepath.SkipDir
			}

			// Like the go command, ignore dot directories (e.g. other worktrees).
			if strings.HasPrefix(d.Name(), "bazel-") || (path != repoRoot && strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}

			return nil
		}

		if d.Name() != "BUILD.bazel" {
			return nil
		}

		data, err := os.ReadFile(path) // #nosec G304 -- BUILD.bazel file of the repository
		if err != nil {
			return err
		}

		f, err := build.ParseBuild(path, data)
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(repoRoot, filepath.Dir(path))
		if err != nil {
			return err
		}

		pkg := "./" + filepath.ToSlash(rel)
		for _, r := range f.Rules("") {
			if r.AttrString("qrl_network") == "minimal" {
				add("VET_MINIMAL", pkg)
			}

			gotags := r.AttrStrings("gotags")
			if slices.Contains(gotags, "develop") {
				add("VET_DEVELOP", pkg)
			}

			if slices.Contains(gotags, "noMainnetGenesis") {
				add("VET_NO_MAINNET_GENESIS", pkg)
			}
		}

		return nil
	})
	require.NoError(t, err)

	vars := makefileVars(t, filepath.Join(repoRoot, "Makefile"))
	for v, pkgs := range passes {
		slices.Sort(pkgs)
		require.DeepEqual(t, pkgs, vars[v], v)
	}
}

// makefileVars returns the `NAME := values...` assignments of a Makefile, joining continued lines.
func makefileVars(t *testing.T, path string) map[string][]string {
	data, err := os.ReadFile(path) // #nosec G304 -- the repository Makefile
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
