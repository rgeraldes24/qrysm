// Command test runs the unit tests without Bazel (usually through `make test`).
//
// Tests run under gotestsum, which re-runs failing tests: like Bazel's flaky
// tests, a test only fails the run if it fails all its attempts.
//
//	go run ./build/test [-race] [mainnet] [mainnet-spectest] [minimal] [minimal-spectest]
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// kind is a unit-test pass.
type kind string

const (
	mainnet         kind = "mainnet"
	mainnetSpectest kind = "mainnet-spectest"
	minimal         kind = "minimal"
	minimalSpectest kind = "minimal-spectest"
)

// validKinds are the test passes, in canonical run order.
var validKinds = []kind{mainnet, mainnetSpectest, minimal, minimalSpectest}

const (
	// gotestsum is run at a pinned version, outside of the module graph.
	gotestsumPkg = "gotest.tools/gotestsum@v1.13.0"

	totalRuns = 5
	rerunMax  = 1000

	// testTimeout is Bazel's "eternal" test timeout: the chain simulations of
	// beacon-chain/blockchain exceed go test's 10m default.
	testTimeout = "60m"
)

// minimalPkgs are the packages with tests Bazel runs with the minimal
// configuration (go_test rules with qrl_network = "minimal"), spec tests
// excepted.
var minimalPkgs = []string{
	"./beacon-chain/core/helpers",
	"./beacon-chain/rpc/qrysm/v1alpha1/beacon",
	"./beacon-chain/rpc/qrysm/v1alpha1/validator",
	"./beacon-chain/state/state-native",
	"./config/fieldparams",
	"./config/params",
}

// excludeRe matches the packages dropped from the mainnet pass: E2E (run by
// `make e2e`), all spec tests (which run in their own passes), and the
// packages only tested with the minimal configuration.
var excludeRe = regexp.MustCompile(strings.Join([]string{
	`/testing/endtoend`,
	`/testing/spectest/`,
	`/beacon-chain/rpc/qrysm/v1alpha1/beacon$`,
}, "|"))

// minimalSpectestRe matches the minimal-config spec tests, dropped from the
// mainnet spectest pass because they run in the minimal-spectest pass.
var minimalSpectestRe = regexp.MustCompile(`/testing/spectest/minimal`)

var errTestsFailed = errors.New("tests failed")

func main() {
	if err := run(); err != nil {
		if !errors.Is(err, errTestsFailed) {
			fmt.Fprintln(os.Stderr, "❌ test:", err)
		}

		os.Exit(1)
	}
}

func run() error {
	race := flag.Bool("race", false, "run the selected pass(es) with the race detector")
	flag.Parse()

	goBin := env("GO", "go")
	gotestsumFlags := []string{
		"--format=pkgname",
		"--no-color=false",
		"--hide-summary=skipped",
		fmt.Sprintf("--rerun-fails=%d", totalRuns-1),
		fmt.Sprintf("--rerun-fails-max-failures=%d", rerunMax),
	}

	kinds, err := selectKinds(flag.Args(), *race)
	if err != nil {
		return fmt.Errorf("selecting test passes: %w", err)
	}

	failed := false
	for _, k := range kinds {
		pkgs, header, tagFlag, err := passSpec(goBin, k)
		if err != nil {
			return fmt.Errorf("pass spec for %s: %w", k, err)
		}

		testFlags := []string{tagFlag, "-timeout=" + testTimeout}
		if *race {
			testFlags = append(testFlags, "-race")
		}

		fmt.Printf("\n%s\n", header)
		if err := gotestsum(goBin, gotestsumFlags, pkgs, testFlags); err != nil {
			failed = true
		}
	}

	fmt.Println()
	if failed {
		fmt.Printf("❌ Some failure: a test failed all %d attempts\n", totalRuns)
		return errTestsFailed
	}

	suffix := ""
	if *race {
		suffix = " with -race"
	}

	fmt.Printf(
		"✅ All tests passed (%s%s - any test in the `=== Failed` section above was a flake recovered within %d attempts)\n",
		joinKinds(kinds), suffix, totalRuns,
	)

	return nil
}

func passSpec(goBin string, k kind) (pkgs []string, header, tagFlag string, err error) {
	switch k {
	case mainnet:
		pkgs, err = filteredPackages(goBin, excludeRe, "./...")
		return pkgs, "=== mainnet pass (excluding spectests) ===", "-tags=develop", err

	case mainnetSpectest:
		pkgs, err = filteredPackages(goBin, minimalSpectestRe, "./testing/spectest/...")
		return pkgs, "=== mainnet spectest pass ===", "-tags=develop", err

	case minimal:
		pkgs, err = goList(goBin, minimalPkgs...)
		return pkgs, "=== minimal pass (-tags=minimal, excluding spectests) ===", "-tags=develop,minimal", err

	case minimalSpectest:
		pkgs, err = goList(goBin, "./testing/spectest/minimal/...")
		return pkgs, "=== minimal spectest pass (-tags=minimal) ===", "-tags=develop,minimal", err
	}

	return nil, "", "", fmt.Errorf("unknown pass %q", k)
}

// filteredPackages is `go list <pattern>` minus the packages matching exclude.
func filteredPackages(goBin string, exclude *regexp.Regexp, pattern string) ([]string, error) {
	all, err := goList(goBin, pattern)
	if err != nil {
		return nil, err
	}

	pkgs := all[:0]
	for _, p := range all {
		if !exclude.MatchString(p) {
			pkgs = append(pkgs, p)
		}
	}

	return pkgs, nil
}

func selectKinds(args []string, race bool) ([]kind, error) {
	if len(args) == 0 {
		if race {
			return []kind{mainnet, mainnetSpectest}, nil
		}

		return validKinds, nil
	}

	kinds := make([]kind, 0, len(args))
	for _, arg := range args {
		k := kind(arg)
		if !slices.Contains(validKinds, k) {
			return nil, fmt.Errorf("not a test pass: %s (passes: %s)", arg, joinKinds(validKinds))
		}

		if !slices.Contains(kinds, k) {
			kinds = append(kinds, k)
		}
	}

	return kinds, nil
}

func gotestsum(goBin string, gotestsumFlags, pkgs, testFlags []string) error {
	args := append([]string{"run", gotestsumPkg}, gotestsumFlags...)
	args = append(args, "--packages="+strings.Join(pkgs, " "), "--")
	args = append(args, testFlags...)

	cmd := exec.Command(goBin, args...) // #nosec G204 -- goBin is the go toolchain; arguments are fixed or come from `go list`
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
	sc.Buffer(make([]byte, 1024*1024), 1024*1024) // spec-test package lines can be long
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

// goList runs `go list <patterns>` and returns the non-empty import paths.
func goList(goBin string, patterns ...string) ([]string, error) {
	out, err := exec.Command(goBin, append([]string{"list"}, patterns...)...).Output() // #nosec G204 -- see gotestsum
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

func joinKinds(kinds []kind) string {
	ss := make([]string, len(kinds))
	for i, k := range kinds {
		ss[i] = string(k)
	}

	return strings.Join(ss, " ")
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
