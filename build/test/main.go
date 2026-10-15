// Command test runs the unit tests.
//
//	make test [mainnet minimal race] [mode=no-race|race] [pkgs=...]
//
// Each pass runs `go test` through gotestsum, which reruns a failed test up to
// four more times, so a test fails the pass only if it fails five times in a
// row. A test that panics cannot be rerun.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
)

// kind is a unit-test pass.
type kind string

const (
	mainnet kind = "mainnet"
	minimal kind = "minimal"
	race    kind = "race"
)

// validKinds are the test passes, in canonical run order.
var validKinds = []kind{mainnet, minimal, race}

const (
	// gotestsum is pinned and run with `go run pkg@version`, which builds it
	// from its own module graph instead of qrysm's.
	gotestsumPkg = "gotest.tools/gotestsum@v1.13.0"

	totalRuns = 5
	rerunMax  = 1000

	// testTimeout bounds one package's test binary; beacon-chain/blockchain
	// takes about 25 minutes.
	testTimeout = "60m"
)

// minimalPkgs are the packages with tests of the minimal configuration. Their
// test files that only one configuration runs carry a "minimal" or "!minimal"
// build constraint.
var minimalPkgs = []string{
	"./beacon-chain/core/helpers",
	"./beacon-chain/rpc/qrysm/v1alpha1/beacon",
	"./beacon-chain/rpc/qrysm/v1alpha1/validator",
	"./beacon-chain/state/state-native",
	"./config/fieldparams",
	"./config/params",
}

// raceTarget is a package, or the tests in some of its files, that also run
// with the race detector.
type raceTarget struct {
	pkg   string
	files []string // nil for the whole package
}

var raceTargets = []raceTarget{
	{pkg: "./beacon-chain/blockchain", files: []string{"chain_info_norace_test.go", "receive_block_test.go", "service_norace_test.go"}},
	{pkg: "./beacon-chain/core/helpers"},
	{pkg: "./beacon-chain/sync/initial-sync", files: []string{
		"blocks_fetcher_test.go", "blocks_fetcher_utils_test.go", "blocks_queue_test.go",
		"fsm_test.go", "initial_sync_test.go", "round_robin_test.go",
	}},
	{pkg: "./config/params"},
}

// excludeRe matches the packages dropped from the mainnet pass: the end-to-end
// tests, which `make e2e` runs.
var excludeRe = regexp.MustCompile(`/testing/endtoend(/|$)`)

func joinKinds(kinds []kind) string {
	ss := make([]string, len(kinds))
	for i, k := range kinds {
		ss[i] = string(k)
	}

	return strings.Join(ss, " ")
}

var errTestsFailed = errors.New("tests failed")

func main() {
	if err := run(); err != nil {
		if !errors.Is(err, errTestsFailed) {
			fmt.Fprintln(os.Stderr, "test:", err)
		}

		os.Exit(1)
	}
}

func run() error {
	raceFlag := flag.Bool("race", false, "run the selected pass(es) with the race detector")
	pkgsFlag := flag.String("pkgs", "", "space-separated package patterns to restrict the passes to; a pattern starting with '-' excludes")
	flag.Parse()

	goBin := env("GO", "go")
	kinds, err := selectKinds(flag.Args())
	if err != nil {
		return err
	}

	filter, err := newPkgFilter(goBin, *pkgsFlag)
	if err != nil {
		return err
	}

	failed := false
	for _, k := range kinds {
		passes, err := passSpecs(goBin, k)
		if err != nil {
			return fmt.Errorf("pass spec for %s: %w", k, err)
		}

		for _, p := range passes {
			pkgs := filter.apply(p.pkgs)
			if len(pkgs) == 0 {
				continue
			}

			testFlags := append([]string{p.tags, "-timeout=" + testTimeout}, p.extra...)
			if *raceFlag && !slices.Contains(testFlags, "-race") {
				testFlags = append(testFlags, "-race")
			}

			fmt.Printf("\n=== %s (%d packages: go test %s) ===\n", p.name, len(pkgs), strings.Join(testFlags, " "))
			if err := gotestsum(goBin, pkgs, testFlags); err != nil {
				failed = true
			}
		}
	}

	fmt.Println()
	if failed {
		fmt.Printf("Some test failed all %d attempts, or a package failed to build or panicked\n", totalRuns)
		return errTestsFailed
	}

	fmt.Printf("All tests passed (%s); a test listed under \"=== FAIL\" above was a flake that passed within %d attempts\n", joinKinds(kinds), totalRuns)

	return nil
}

// pass is one `go test` invocation.
type pass struct {
	name  string
	pkgs  []string
	tags  string
	extra []string
}

func passSpecs(goBin string, k kind) ([]pass, error) {
	switch k {
	case mainnet:
		pkgs, err := mainnetPackages(goBin)
		return []pass{{name: "mainnet pass", pkgs: pkgs, tags: "-tags=develop"}}, err

	case minimal:
		pkgs, err := goList(goBin, minimalPkgs...)
		return []pass{{name: "minimal pass", pkgs: pkgs, tags: "-tags=develop,minimal"}}, err

	case race:
		var passes []pass
		for _, t := range raceTargets {
			pkgs, err := goList(goBin, t.pkg)
			if err != nil {
				return nil, err
			}

			extra := []string{"-race"}
			if t.files != nil {
				re, err := testsRunRegexp(t.pkg, t.files)
				if err != nil {
					return nil, err
				}

				extra = append(extra, "-run="+re)
			}

			passes = append(passes, pass{name: "race pass: " + t.pkg, pkgs: pkgs, tags: "-tags=develop", extra: extra})
		}

		return passes, nil
	}

	return nil, fmt.Errorf("unknown pass %q", k)
}

// mainnetPackages is `go list ./...` minus the excludeRe packages.
func mainnetPackages(goBin string) ([]string, error) {
	all, err := goList(goBin, "./...")
	if err != nil {
		return nil, fmt.Errorf("listing all packages: %w", err)
	}

	pkgs := all[:0]
	for _, p := range all {
		if !excludeRe.MatchString(p) {
			pkgs = append(pkgs, p)
		}
	}

	return pkgs, nil
}

// testsRunRegexp returns a -run pattern that matches exactly the Test
// functions declared in files of the package directory dir.
func testsRunRegexp(dir string, files []string) (string, error) {
	var names []string
	fset := token.NewFileSet()
	for _, name := range files {
		f, err := parser.ParseFile(fset, filepath.Join(dir, name), nil, parser.SkipObjectResolution)
		if err != nil {
			return "", fmt.Errorf("parse %s: %w", name, err)
		}

		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if ok && fn.Recv == nil && strings.HasPrefix(fn.Name.Name, "Test") && fn.Name.Name != "TestMain" {
				names = append(names, fn.Name.Name)
			}
		}
	}

	if len(names) == 0 {
		return "", fmt.Errorf("no tests in %s %v", dir, files)
	}

	sort.Strings(names)

	return "^(" + strings.Join(names, "|") + ")$", nil
}

func selectKinds(args []string) ([]kind, error) {
	if len(args) == 0 {
		return validKinds, nil
	}

	kinds := make([]kind, 0, len(args))
	for _, arg := range args {
		k := kind(arg)
		if !slices.Contains(validKinds, k) {
			return nil, fmt.Errorf("not a test pass: %s (passes: %s)", arg, joinKinds(validKinds))
		}

		kinds = append(kinds, k)
	}

	return kinds, nil
}

// pkgFilter restricts the passes to the packages matched by its include
// patterns (all when there are none) and not matched by its exclude patterns.
type pkgFilter struct {
	include map[string]bool
	exclude map[string]bool
}

func newPkgFilter(goBin, spec string) (pkgFilter, error) {
	var inc, exc []string
	for _, p := range strings.Fields(spec) {
		if rest, ok := strings.CutPrefix(p, "-"); ok {
			exc = append(exc, rest)
		} else {
			inc = append(inc, p)
		}
	}

	var f pkgFilter
	set := func(patterns []string) (map[string]bool, error) {
		if len(patterns) == 0 {
			return nil, nil
		}

		pkgs, err := goList(goBin, patterns...)
		if err != nil {
			return nil, err
		}

		m := make(map[string]bool, len(pkgs))
		for _, p := range pkgs {
			m[p] = true
		}

		return m, nil
	}

	var err error
	if f.include, err = set(inc); err != nil {
		return f, err
	}

	f.exclude, err = set(exc)

	return f, err
}

func (f pkgFilter) apply(pkgs []string) []string {
	var out []string
	for _, p := range pkgs {
		if (f.include == nil || f.include[p]) && !f.exclude[p] {
			out = append(out, p)
		}
	}

	return out
}

func gotestsum(goBin string, pkgs, testFlags []string) error {
	args := []string{
		"run", gotestsumPkg,
		"--format=pkgname",
		"--hide-summary=skipped",
		fmt.Sprintf("--rerun-fails=%d", totalRuns-1),
		fmt.Sprintf("--rerun-fails-max-failures=%d", rerunMax),
		"--packages=" + strings.Join(pkgs, " "),
		"--",
	}
	args = append(args, testFlags...)

	cmd := exec.Command(goBin, args...) // #nosec G204 -- goBin is the go toolchain binary; this is a test orchestration tool
	cmd.Stderr = os.Stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("getting gotestsum stdout: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting gotestsum: %w", err)
	}

	streamProgress(stdout, len(pkgs))

	return cmd.Wait()
}

var statusIcon = regexp.MustCompile(`[✓✖∅↻]`)

// streamProgress copies r to stdout, prefixing each package-status line with a
// right-aligned "[count/total]" running counter.
func streamProgress(r io.Reader, total int) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	width := len(strconv.Itoa(total))
	count := 0

	for sc.Scan() {
		line := sc.Text()
		if !statusIcon.MatchString(line) {
			fmt.Println(line)
			continue
		}

		count++
		fmt.Printf("[%*d/%d] %s\n", width, count, total, line)
	}
}

// goList runs `go list <patterns>` and returns the import paths.
func goList(goBin string, patterns ...string) ([]string, error) {
	cmd := exec.Command(goBin, append([]string{"list"}, patterns...)...) // #nosec G204 -- goBin is the go toolchain binary
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("go list %s: %w\n%s", strings.Join(patterns, " "), err, ee.Stderr)
		}

		return nil, fmt.Errorf("go list %s: %w", strings.Join(patterns, " "), err)
	}

	var pkgs []string
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if line != "" {
			pkgs = append(pkgs, line)
		}
	}

	return pkgs, nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
