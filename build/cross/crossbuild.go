package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// Config is the cross-build configuration, read from the environment the
// Makefile exports (see the `dist` target).
type Config struct {
	Go           string              // GO
	Dist         string              // DIST
	Tag          string              // GIT_TAG (falls back to `git describe`, then "Unknown")
	Binaries     []string            // CROSS_BINARIES
	Targets      []Target            // CROSS_TARGETS
	CommonCFlags string              // CGO_CFLAGS_COMMON
	Arm64CFlags  string              // CGO_CFLAGS_LINUX_ARM64
	Ldflags      string              // LDFLAGS
	Tagflag      string              // TAGFLAG (e.g. "-tags=develop", or empty)
	BinTags      map[string][]string // BIN_TAGS ("<bin>=<tag>[,<tag>...] ..."): per-binary build tags
	Mode         string              // BUILD_MODE (dev|release) — display only, for the progress line
}

// OS is a target operating system (the GOOS value).
type OS string

const (
	Linux   OS = "linux"
	Darwin  OS = "darwin"
	Windows OS = "windows"
)

// Arch is a target architecture (the GOARCH value).
type Arch string

const (
	AMD64 Arch = "amd64"
	ARM64 Arch = "arm64"
)

// Target is a platform to build for. Triple is the zig target for Linux
// (it pins the glibc version), informative otherwise.
type Target struct {
	OS     OS
	Arch   Arch
	Triple string
}

// Build cross-compiles every (binary × target) artifact.
func (c Config) Build() error {
	for _, t := range c.Targets {
		if (t.OS == Darwin || t.OS == Windows) && runtime.GOOS != string(Linux) {
			return fmt.Errorf("'%s' targets require a Linux x86_64 host (osxcross/mingw-w64), current host: %s/%s", t.OS, runtime.GOOS, runtime.GOARCH)
		}
	}

	zig, err := provision("install-zig.sh")
	if err != nil {
		return fmt.Errorf("provision zig: %w", err)
	}

	if err := os.MkdirAll(c.Dist, 0o750); err != nil {
		return fmt.Errorf("create dist dir: %w", err)
	}

	total := len(c.Targets) * len(c.Binaries)
	n := 0
	osxcross := ""

	for _, t := range c.Targets {
		cc, cxx, extra, pathPrefix, err := c.toolchain(t, zig, &osxcross)
		if err != nil {
			return fmt.Errorf("toolchain: %w", err)
		}

		b := builder{cfg: c, t: t, cc: cc, cxx: cxx, pathPrefix: pathPrefix}
		for _, bin := range c.Binaries {
			out := filepath.Join(c.Dist, ArtifactName(bin, c.Tag, t))
			n++

			fmt.Printf("[%d/%d] → %s/%s  %s  (%s)\n", n, total, t.OS, t.Arch, bin, c.Mode)
			if err := b.compile(out, BinaryPackage(bin), c.tagFlag(bin), c.cgoCFlags(extra)); err != nil {
				return fmt.Errorf("compile %s for %s/%s: %w", bin, t.OS, t.Arch, err)
			}
		}
	}

	fmt.Printf("✅ cross: built %d/%d artifact(s) → %s/\n", n, total, c.Dist)

	return nil
}

// BinaryPackage returns the main package of a binary: ./cmd/<bin>, except for
// the deposit CLI.
func BinaryPackage(bin string) string {
	if bin == "deposit" {
		return "./cmd/staking-deposit-cli/deposit"
	}

	return "./cmd/" + bin
}

// ArtifactName is the released file name of a binary, as qrysm.sh downloads it.
func ArtifactName(bin, tag string, t Target) string {
	ext := ""
	if t.OS == Windows {
		ext = ".exe"
	}

	return fmt.Sprintf("%s-%s-%s-%s%s", bin, tag, t.OS, t.Arch, ext)
}

// cgoCFlags assembles the CGO_CFLAGS value for one artifact.
func (c Config) cgoCFlags(extra ...string) string {
	return strings.Join(strings.Fields(c.CommonCFlags+" "+strings.Join(extra, " ")), " ")
}

// tagFlag merges TAGFLAG with the binary's own build tags.
func (c Config) tagFlag(bin string) string {
	tags := strings.Split(strings.TrimPrefix(c.Tagflag, "-tags="), ",")
	tags = append(tags, c.BinTags[bin]...)

	var kept []string
	for _, t := range tags {
		if t = strings.TrimSpace(t); t != "" {
			kept = append(kept, t)
		}
	}

	if len(kept) == 0 {
		return ""
	}

	return "-tags=" + strings.Join(kept, ",")
}

// ConfigFromEnv builds a Config from the environment the Makefile exports.
func ConfigFromEnv() Config {
	cfg := Config{
		Go:           env("GO", "go"),
		Dist:         env("DIST", "dist"),
		Tag:          gitTag(),
		Binaries:     strings.Fields(env("CROSS_BINARIES", "beacon-chain validator client-stats qrysmctl deposit")),
		CommonCFlags: env("CGO_CFLAGS_COMMON", "-O2"),
		Arm64CFlags:  os.Getenv("CGO_CFLAGS_LINUX_ARM64"),
		Ldflags:      os.Getenv("LDFLAGS"),
		Tagflag:      os.Getenv("TAGFLAG"),
		BinTags:      ParseBinTags(os.Getenv("BIN_TAGS")),
		Mode:         env("BUILD_MODE", "dev"),
	}

	cfg.Targets = defaultTargets
	if spec := os.Getenv("CROSS_TARGETS"); spec != "" {
		cfg.Targets = parseTargets(spec)
	}

	return cfg
}

// ParseBinTags parses "<bin>=<tag>[,<tag>...] ..." into a map.
func ParseBinTags(spec string) map[string][]string {
	out := make(map[string][]string)
	for _, field := range strings.Fields(spec) {
		bin, tags, ok := strings.Cut(field, "=")
		if !ok || bin == "" {
			fmt.Fprintf(os.Stderr, "cross: skipping malformed BIN_TAGS entry %q\n", field)
			continue
		}

		out[bin] = append(out[bin], strings.Split(tags, ",")...)
	}

	return out
}

func (c Config) toolchain(t Target, zig string, osxcross *string) (cc, cxx, extra, pathPrefix string, err error) {
	switch t.OS {
	case Linux:
		cc = zig + " cc -target " + t.Triple
		cxx = zig + " c++ -target " + t.Triple
		if t.Arch == ARM64 {
			extra = c.Arm64CFlags
		}

	case Darwin:
		if *osxcross == "" {
			if *osxcross, err = provision("install-osxcross.sh"); err != nil {
				return
			}
		}

		pathPrefix = *osxcross + string(os.PathListSeparator)
		cc, cxx = filepath.Join(*osxcross, "oa64-clang"), filepath.Join(*osxcross, "oa64-clang++")
		if t.Arch == AMD64 {
			cc, cxx = filepath.Join(*osxcross, "o64-clang"), filepath.Join(*osxcross, "o64-clang++")
		}

	case Windows:
		if _, err = provision("install-mingw.sh"); err != nil {
			return
		}

		cc, cxx = "x86_64-w64-mingw32-gcc", "x86_64-w64-mingw32-g++"

	default:
		err = fmt.Errorf("unknown target OS %q", t.OS)
	}

	return
}

type builder struct {
	cfg        Config
	t          Target
	cc, cxx    string
	pathPrefix string
}

func (b builder) compile(out, pkg, tagFlag, cgoCFlags string) error {
	args := []string{"build"}
	if tagFlag != "" {
		args = append(args, tagFlag)
	}

	args = append(args, "-trimpath", "-ldflags", b.cfg.Ldflags, "-o", out, pkg)

	// #nosec G204 -- build-time tooling: the compiler and its flags come from the Makefile environment.
	cmd := exec.Command(b.cfg.Go, args...)
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
	cmd.Env = append(os.Environ(),
		"GOOS="+string(b.t.OS),
		"GOARCH="+string(b.t.Arch),
		"CGO_ENABLED=1",
		"CC="+b.cc,
		"CXX="+b.cxx,
		"CGO_CFLAGS="+cgoCFlags,
	)

	if b.pathPrefix != "" {
		cmd.Env = append(cmd.Env, "PATH="+b.pathPrefix+os.Getenv("PATH"))
	}

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run: %w", err)
	}

	return nil
}

func provision(script string) (string, error) {
	// #nosec G204 -- build-time tooling: `script` is one of the in-repo literals passed by callers.
	cmd := exec.Command(filepath.Join("tools", "cross-toolchain", script))
	cmd.Stderr = os.Stderr

	path, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("%s: %w", script, err)
	}

	return strings.TrimSpace(string(path)), nil
}

// defaultTargets is the build matrix used when CROSS_TARGETS is unset.
var defaultTargets = []Target{
	{Linux, AMD64, "x86_64-linux-gnu.2.31"},
	{Linux, ARM64, "aarch64-linux-gnu.2.31"},
	{Darwin, AMD64, "x86_64-macos"},
	{Darwin, ARM64, "aarch64-macos"},
	{Windows, AMD64, "x86_64-windows-gnu"},
}

// parseTargets parses "<os>/<arch>/<triple> ..." into targets.
func parseTargets(spec string) []Target {
	fields := strings.Fields(spec)

	targets := make([]Target, 0, len(fields))
	for _, field := range fields {
		parts := strings.SplitN(field, "/", 3)
		if len(parts) != 3 {
			fmt.Fprintf(os.Stderr, "cross: skipping malformed target %q\n", field)
			continue
		}

		targets = append(targets, Target{OS: OS(parts[0]), Arch: Arch(parts[1]), Triple: parts[2]})
	}

	return targets
}

func gitTag() string {
	if tag := os.Getenv("GIT_TAG"); tag != "" {
		return tag
	}

	out, err := exec.Command("git", "describe", "--tags", "--abbrev=0").Output()
	if err != nil {
		return "Unknown"
	}

	return strings.TrimSpace(string(out))
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
