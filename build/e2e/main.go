// Command e2e runs the end-to-end tests (usually through
// `make e2e`).
//
//	go run ./build/e2e [<scenario>|<suite>...]
//
// It builds the binaries the devnet needs into $DIST (default: dist), then
// runs each selected scenario's test with `go test`. The test harness finds the
// binaries there (see build/repo.FindBinary).
package main

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/theQRL/qrysm/build/repo"
)

type (
	kind  string
	suite string
)

const (
	kindMinimal         kind = "minimal"
	kindSlashing        kind = "slashing"
	kindSlasher         kind = "slasher"
	kindBuilder         kind = "builder"
	kindValidatorREST   kind = "validator-rest"
	kindScenario        kind = "scenario"
	kindMainnet         kind = "mainnet"
	kindMainnetRelease  kind = "mainnet-release"
	kindScenarioMainnet kind = "scenario-mainnet"
)

const (
	suitePresubmit     suite = "presubmit"
	suitePostsubmit    suite = "postsubmit"
	suiteScenarioTests suite = "scenario_tests"
)

const e2eTimeout = "60m"

const logDirPrefix = "qrysm-e2e-logs-"

type spec struct {
	run         string // anchored -run regexp
	minimal     bool
	needQrysmSh bool
}

var kinds = map[kind]spec{
	kindMinimal:         {run: "^TestEndToEnd_MinimalConfig$", minimal: true},
	kindSlashing:        {run: "^TestEndToEnd_Slasher_MinimalConfig$", minimal: true},
	kindSlasher:         {run: "^TestEndToEnd_SlasherSimulator$", minimal: true},
	kindBuilder:         {run: "^TestEndToEnd_MinimalConfig_WithBuilder$", minimal: true},
	kindValidatorREST:   {run: "^TestEndToEnd_MinimalConfig_ValidatorRESTApi$", minimal: true},
	kindScenario:        {run: "^TestEndToEnd_MultiScenarioRun_Minimal$", minimal: true},
	kindMainnet:         {run: "^TestEndToEnd_MainnetConfig$"},
	kindMainnetRelease:  {run: "^TestEndToEnd_MainnetConfig_ValidatorAtCurrentRelease$", needQrysmSh: true},
	kindScenarioMainnet: {run: "^TestEndToEnd_MultiScenarioRun$"},
}

// suites group the scenarios run together. Keep in
// sync with the E2E_SUITE_* variables of the Makefile.
var suites = map[suite][]kind{
	suitePresubmit:     {kindMinimal, kindSlashing, kindSlasher},
	suitePostsubmit:    {kindBuilder, kindMainnet, kindMainnetRelease},
	suiteScenarioTests: {kindScenarioMainnet, kindScenario, kindValidatorREST},
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "❌ e2e:", err)
		os.Exit(1)
	}
}

func run() error {
	goBin := env("GO", "go")
	dist, err := filepath.Abs(env("DIST", "dist"))
	if err != nil {
		return fmt.Errorf("resolving dist path: %w", err)
	}

	label, targets, err := selectTargets(os.Args[1:])
	if err != nil {
		return fmt.Errorf("selecting e2e targets: %w", err)
	}

	if err := os.MkdirAll(dist, 0o750); err != nil {
		return fmt.Errorf("creating dist dir: %w", err)
	}

	// Kill any devnet processes left over from a previous run, then wipe the
	// data directories they left behind.
	cleanupStaleProcs(dist)
	cleanupStaleData()

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, shutdownSignals...)
	go func() {
		<-sigc
		procMu.Lock()
		if currentProc != nil {
			if err := killProcGroup(currentProc.Pid); err != nil {
				fmt.Fprintln(os.Stderr, "e2e: tearing down devnet process group:", err)
			}
		}
		procMu.Unlock()

		fmt.Fprintln(os.Stderr, "\n❌ e2e: interrupted — devnet torn down")
		os.Exit(130)
	}()

	// The execution client, at the go-qrl version pinned by go.mod.
	if err := goBuild(goBin, dist, "gqrl", "github.com/theQRL/go-qrl/cmd/gqrl", ""); err != nil {
		return fmt.Errorf("building gqrl: %w", err)
	}

	built := map[string]bool{}
	for i, k := range targets {
		cfg := kinds[k]
		binTags, testTags := "", "develop"
		if cfg.minimal {
			binTags, testTags = "minimal", "develop,minimal"
		}

		if !built[binTags] {
			if err := buildQrysmBins(goBin, dist, binTags); err != nil {
				return fmt.Errorf("building qrysm binaries: %w", err)
			}

			built[binTags] = true
		}

		if cfg.needQrysmSh {
			if err := provisionQrysmSh(dist); err != nil {
				return fmt.Errorf("provisioning qrysm.sh: %w", err)
			}
		}

		logDir, err := os.MkdirTemp("", logDirPrefix)
		if err != nil {
			return fmt.Errorf("creating log directory: %w", err)
		}

		fmt.Fprintf(os.Stderr, "e2e: [%d/%d] %s (run=%s, tags=%s)\n  %s=%s\n  E2E_LOG_PATH=%s\n",
			i+1, len(targets), k, cfg.run, testTags, repo.BinDirEnv, dist, logDir)

		args := []string{"test", "-tags=" + testTags, "-run", cfg.run, "-timeout", e2eTimeout, "-v", "-count=1", "./testing/endtoend"}
		if err := runGoTest(goBin, args, []string{repo.BinDirEnv + "=" + dist, "E2E_LOG_PATH=" + logDir}); err != nil {
			return fmt.Errorf("scenario %s failed (logs: %s): %w", k, logDir, err)
		}
	}

	fmt.Printf("✅ e2e: %s passed (%d scenarios)\n", label, len(targets))

	return nil
}

func selectTargets(args []string) (string, []kind, error) {
	var names []string
	for _, arg := range args {
		if arg != "" && !strings.HasPrefix(arg, "-") {
			names = append(names, arg)
		}
	}

	if len(names) == 0 {
		names = []string{string(suitePresubmit)}
	}

	var targets []kind
	seen := map[kind]bool{}
	for _, name := range names {
		expanded, err := resolveTarget(name)
		if err != nil {
			return "", nil, err
		}

		for _, k := range expanded {
			if !seen[k] {
				seen[k] = true
				targets = append(targets, k)
			}
		}
	}

	return strings.Join(names, " "), targets, nil
}

// resolveTarget expands a single suite or scenario name into its kinds.
func resolveTarget(name string) ([]kind, error) {
	if members, ok := suites[suite(name)]; ok {
		return members, nil
	}

	if k := kind(name); kinds[k].run != "" {
		return []kind{k}, nil
	}

	return nil, fmt.Errorf("unknown e2e target %q (scenarios: %s; suites: %s)", name, kindNames(), suiteNames())
}

func kindNames() string {
	names := make([]string, 0, len(kinds))
	for k := range kinds {
		names = append(names, string(k))
	}

	sort.Strings(names)

	return strings.Join(names, " ")
}

func suiteNames() string {
	return strings.Join([]string{string(suitePresubmit), string(suitePostsubmit), string(suiteScenarioTests)}, " ")
}

var (
	procMu      sync.Mutex
	currentProc *os.Process
)

func runGoTest(goBin string, args, extraEnv []string) error {
	cmd := exec.Command(goBin, args...) // #nosec G204 -- goBin is the go toolchain; arguments are fixed
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), extraEnv...)

	setNewProcGroup(cmd)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting go test: %w", err)
	}

	procMu.Lock()
	currentProc = cmd.Process
	procMu.Unlock()

	err := cmd.Wait()

	// Reap devnet children the test may have orphaned (e.g. on a -timeout panic).
	if killErr := killProcGroup(cmd.Process.Pid); killErr != nil {
		fmt.Fprintln(os.Stderr, "e2e: reaping devnet process group:", killErr)
	}

	procMu.Lock()
	currentProc = nil
	procMu.Unlock()

	return err
}

func cleanupStaleProcs(dist string) {
	var killed bool
	for _, name := range []string{"bootnode", "beacon-chain", "validator", "gqrl"} {
		// #nosec G204 -- dist is a developer-controlled path, not external input.
		if err := exec.Command("pkill", "-f", filepath.Join(dist, name)).Run(); err == nil {
			killed = true
		}
	}

	if killed {
		fmt.Fprintln(os.Stderr, "e2e: cleaned up stale devnet process(es) from a previous run")
	}
}

// cleanupStaleData removes the devnet data directories left behind by a
// previous run (testing/endtoend/params puts them under the temp dir).
func cleanupStaleData() {
	matches, err := filepath.Glob(filepath.Join(os.TempDir(), "shard-*"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "e2e: globbing stale devnet data:", err)
		return
	}

	var removed bool
	for _, dir := range matches {
		if err := os.RemoveAll(dir); err != nil {
			fmt.Fprintf(os.Stderr, "e2e: removing stale devnet data %s: %v\n", dir, err)
			continue
		}

		removed = true
	}

	if removed {
		fmt.Fprintln(os.Stderr, "e2e: cleaned up stale devnet data from a previous run")
	}
}

func buildQrysmBins(goBin, dist, tags string) error {
	fmt.Fprintf(os.Stderr, "e2e: building binaries (tags=%q) → %s\n", tags, dist)
	for _, b := range []struct{ name, pkg string }{
		{"beacon-chain", "./cmd/beacon-chain"},
		{"validator", "./cmd/validator"},
		{"bootnode", "./tools/bootnode"},
	} {
		if err := goBuild(goBin, dist, b.name, b.pkg, tags); err != nil {
			return err
		}
	}

	return nil
}

// goBuild compiles pkg into dist/name with the given build tags (cgo enabled,
// as the qrysm binaries and gqrl need it).
func goBuild(goBin, dist, name, pkg, tags string) error {
	args := []string{"build", "-trimpath"}
	if tags != "" {
		args = append(args, "-tags="+tags)
	}

	args = append(args, "-o", filepath.Join(dist, name), pkg)
	cmd := exec.Command(goBin, args...) // #nosec G204 -- goBin is the go toolchain; arguments are fixed
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(), "CGO_ENABLED=1")
	fmt.Fprintf(os.Stderr, "  build %s\n", name)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("building %s: %w", name, err)
	}

	return nil
}

// provisionQrysmSh makes the repository's qrysm.sh wrapper available to the
// harness as the "qrysm_sh" binary.
func provisionQrysmSh(dist string) error {
	wd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("resolving working directory: %w", err)
	}

	src, dst := filepath.Join(wd, "qrysm.sh"), filepath.Join(dist, "qrysm_sh")
	if _, err := os.Stat(src); err != nil {
		return fmt.Errorf("stat %s: %w", src, err)
	}

	_ = os.Remove(dst)
	if err := os.Symlink(src, dst); err != nil {
		return fmt.Errorf("symlink %s -> %s: %w", dst, src, err)
	}

	return nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
