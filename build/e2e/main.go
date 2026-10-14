// Command e2e runs the end-to-end tests of testing/endtoend:
//
//	make e2e [<scenario>...]
//
// It builds the binaries the scenarios start (beacon-chain, validator,
// bootnode, and gqrl from the go-qrl version go.mod selects) into dist/, then
// runs each scenario's test, one at a time, with QRYSM_BIN pointing at them.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"sync"
	"syscall"
)

// scenario is an end-to-end test of testing/endtoend.
type scenario struct {
	run     string // anchored -run regexp
	minimal bool   // run with -tags minimal, against binaries built with it
}

// scenarios are the e2e tests that need nothing but the binaries built here.
// The others also need web3signer, which was never provided, or a builder.
var scenarios = map[string]scenario{
	"minimal":  {run: "^TestEndToEnd_MinimalConfig$", minimal: true},
	"slashing": {run: "^TestEndToEnd_Slasher_MinimalConfig$", minimal: true},
	"slasher":  {run: "^TestEndToEnd_SlasherSimulator$", minimal: true},
}

// defaultScenarios run when none is named.
var defaultScenarios = []string{"minimal"}

const (
	e2eTimeout = "60m"
	gqrlPkg    = "github.com/theQRL/go-qrl/cmd/gqrl"
)

// binaries are built into dist/ for the scenarios; the e2e components find them
// there through build/bazel.FindBinary.
var binaries = []struct{ name, pkg string }{
	{"beacon-chain", "./cmd/beacon-chain"},
	{"validator", "./cmd/validator"},
	{"bootnode", "./tools/bootnode"},
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "e2e:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	goBin := env("GO", "go")
	dist, err := filepath.Abs(env("DIST", "dist"))
	if err != nil {
		return fmt.Errorf("resolving dist path: %w", err)
	}

	names, err := selectScenarios(args)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dist, 0o750); err != nil {
		return fmt.Errorf("creating %s: %w", dist, err)
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigc
		killCurrent()
		fmt.Fprintln(os.Stderr, "\ne2e: interrupted, devnet torn down")
		os.Exit(130)
	}()

	// gqrl is built inside the qrysm module so that go.mod's replace of
	// github.com/theQRL/go-qrl applies (`go install pkg@version` ignores it).
	if err := goBuild(goBin, filepath.Join(dist, "gqrl"), gqrlPkg, ""); err != nil {
		return err
	}

	built := map[bool]bool{}
	for i, name := range names {
		sc := scenarios[name]
		testTags := "develop"
		if sc.minimal {
			testTags = "develop,minimal"
		}

		if !built[sc.minimal] {
			tags := ""
			if sc.minimal {
				tags = "minimal"
			}

			for _, b := range binaries {
				if err := goBuild(goBin, filepath.Join(dist, b.name), b.pkg, tags); err != nil {
					return err
				}
			}

			built[sc.minimal] = true
		}

		logDir, err := os.MkdirTemp("", "qrysm-e2e-logs-")
		if err != nil {
			return fmt.Errorf("creating log directory: %w", err)
		}

		// The devnet keeps its data under the test's temporary directory
		// (build/bazel.TestTmpDir is os.TempDir); a fresh one
		// per scenario keeps runs apart.
		dataDir, err := os.MkdirTemp("", "qrysm-e2e-data-")
		if err != nil {
			return fmt.Errorf("creating data directory: %w", err)
		}

		fmt.Fprintf(os.Stderr, "e2e: [%d/%d] %s (run=%s, tags=%s)\n  QRYSM_BIN=%s\n  E2E_LOG_PATH=%s\n  TMPDIR=%s\n",
			i+1, len(names), name, sc.run, testTags, dist, logDir, dataDir)

		args := []string{"test", "-tags=" + testTags, "-run", sc.run, "-timeout", e2eTimeout, "-v", "-count=1", "./testing/endtoend"}
		if err := runGoTest(goBin, args, []string{"QRYSM_BIN=" + dist, "E2E_LOG_PATH=" + logDir, "TMPDIR=" + dataDir}); err != nil {
			return fmt.Errorf("scenario %s failed (logs: %s, data: %s): %w", name, logDir, dataDir, err)
		}

		if err := os.RemoveAll(dataDir); err != nil {
			fmt.Fprintf(os.Stderr, "e2e: removing %s: %v\n", dataDir, err)
		}
	}

	fmt.Printf("e2e: %s passed\n", strings.Join(names, " "))

	return nil
}

func selectScenarios(args []string) ([]string, error) {
	if len(args) == 0 {
		return defaultScenarios, nil
	}

	var names []string
	for _, a := range args {
		if _, ok := scenarios[a]; !ok {
			return nil, fmt.Errorf("unknown scenario %q (one of: %s)", a, scenarioNames())
		}

		if !slices.Contains(names, a) {
			names = append(names, a)
		}
	}

	return names, nil
}

func scenarioNames() string {
	names := make([]string, 0, len(scenarios))
	for n := range scenarios {
		names = append(names, n)
	}

	sort.Strings(names)

	return strings.Join(names, " ")
}

func goBuild(goBin, out, pkg, tags string) error {
	args := []string{"build", "-o", out}
	if tags != "" {
		args = append(args, "-tags="+tags)
	}

	args = append(args, pkg)
	fmt.Fprintf(os.Stderr, "e2e: %s %s\n", goBin, strings.Join(args, " "))
	cmd := exec.Command(goBin, args...) // #nosec G204 -- goBin is the go toolchain; this is a test orchestration tool
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("building %s: %w", pkg, err)
	}

	return nil
}

var (
	procMu      sync.Mutex
	currentProc *os.Process
)

// runGoTest runs `go test` in its own process group and waits for it, then
// kills what is left of the group: the devnet processes the test started.
func runGoTest(goBin string, args, extraEnv []string) error {
	cmd := exec.Command(goBin, args...) // #nosec G204 -- goBin is the go toolchain; this is a test orchestration tool
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
	killCurrent()

	return err
}

func killCurrent() {
	procMu.Lock()
	defer procMu.Unlock()

	if currentProc == nil {
		return
	}

	if err := killProcGroup(currentProc.Pid); err != nil && !errors.Is(err, os.ErrProcessDone) {
		fmt.Fprintln(os.Stderr, "e2e: reaping devnet process group:", err)
	}

	currentProc = nil
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
