// Command dist builds the release binaries for every platform:
//
//	make dist [<bin>...] [platform=<os>/<arch>,...] [cgo=1]
//
// Each binary is written to dist/<bin>-<tag>-<os>-<arch>[.exe] next to a
// .sha256 file in `sha256sum` format, the names the release job uploads and
// qrysm.sh downloads. The build is reproducible: -trimpath, no symbol table,
// and the version stamped from git with the build time taken from
// SOURCE_DATE_EPOCH (default: the commit time), so two builds of a commit
// give the same bytes.
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"
)

const versionPkg = "github.com/theQRL/qrysm/runtime/version"

// defaultPlatforms are the platforms of a release.
var defaultPlatforms = []string{"linux/amd64", "linux/arm64", "darwin/amd64", "darwin/arm64", "windows/amd64"}

// binary is a binary to build: name=package[:tag,tag...] on the command line.
type binary struct {
	name, pkg string
	tags      []string
}

func parseBinary(s string) (binary, error) {
	name, rest, ok := strings.Cut(s, "=")
	if !ok || name == "" || rest == "" {
		return binary{}, fmt.Errorf("binary %q: want name=package[:tags]", s)
	}

	pkg, tags, _ := strings.Cut(rest, ":")
	b := binary{name: name, pkg: pkg}
	if tags != "" {
		b.tags = strings.Split(tags, ",")
	}

	return b, nil
}

// stamp is the version information linked into the binaries.
type stamp struct {
	commit, tag string
	epoch       int64
}

// date formats the build time as hack/workspace_status.sh does
// (`date --rfc-3339=seconds --utc`).
func (s stamp) date() string {
	return time.Unix(s.epoch, 0).UTC().Format("2006-01-02 15:04:05-07:00")
}

func (s stamp) ldflags() string {
	x := func(name, value string) string {
		return fmt.Sprintf("-X '%s.%s=%s'", versionPkg, name, value)
	}

	return strings.Join([]string{
		"-s", "-w",
		x("gitCommit", s.commit),
		x("gitTag", s.tag),
		x("buildDate", s.date()),
		x("buildDateUnix", strconv.FormatInt(s.epoch, 10)),
	}, " ")
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "dist:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("dist", flag.ContinueOnError)
	out := fs.String("out", "dist", "output directory")
	platforms := fs.String("platforms", strings.Join(defaultPlatforms, ","), "comma-separated <os>/<arch> list")
	cgo := fs.Bool("cgo", false, "build with cgo (native platform only)")
	tag := fs.String("tag", "", "version tag (default: the latest tag reachable from HEAD, else the short commit hash)")
	goBin := fs.String("go", "go", "go command")
	if err := fs.Parse(args); err != nil {
		return err
	}

	var bins []binary
	for _, a := range fs.Args() {
		b, err := parseBinary(a)
		if err != nil {
			return err
		}

		bins = append(bins, b)
	}

	if len(bins) == 0 {
		return errors.New("no binary to build")
	}

	platformsSet := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "platforms" {
			platformsSet = true
		}
	})

	plats, err := selectPlatforms(*platforms, platformsSet, *cgo, runtime.GOOS+"/"+runtime.GOARCH)
	if err != nil {
		return err
	}

	st, err := gitStamp(*tag)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(*out, 0o750); err != nil {
		return fmt.Errorf("mkdir %s: %w", *out, err)
	}

	fmt.Printf("version %s, commit %s, build date %s\n", st.tag, st.commit, st.date())
	for _, p := range plats {
		goos, goarch, _ := strings.Cut(p, "/")
		for _, b := range bins {
			path, err := build(*goBin, *out, b, goos, goarch, *cgo, st)
			if err != nil {
				return err
			}

			if err := writeSHA256(path); err != nil {
				return err
			}
		}
	}

	return nil
}

func parsePlatforms(s string) ([]string, error) {
	var out []string
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		goos, goarch, ok := strings.Cut(p, "/")
		if !ok || goos == "" || goarch == "" {
			return nil, fmt.Errorf("platform %q: want <os>/<arch>", p)
		}

		if !slices.Contains(out, p) {
			out = append(out, p)
		}
	}

	if len(out) == 0 {
		return nil, errors.New("no platform")
	}

	return out, nil
}

// artifactName is the release asset name of a binary.
func artifactName(bin, tag, goos, goarch string) string {
	name := fmt.Sprintf("%s-%s-%s-%s", bin, tag, goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}

	return name
}

func build(goBin, out string, b binary, goos, goarch string, cgo bool, st stamp) (string, error) {
	path := filepath.Join(out, artifactName(b.name, st.tag, goos, goarch))
	args := []string{"build", "-trimpath", "-ldflags=" + st.ldflags(), "-o", path}
	if len(b.tags) > 0 {
		args = append(args, "-tags="+strings.Join(b.tags, ","))
	}

	args = append(args, b.pkg)

	cgoEnabled := "0"
	if cgo {
		cgoEnabled = "1"
	}

	fmt.Printf("%s/%s CGO_ENABLED=%s go %s\n", goos, goarch, cgoEnabled, strings.Join(args, " "))
	cmd := exec.Command(goBin, args...) // #nosec G204 -- goBin is the go toolchain binary; this is a release build tool
	cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED="+cgoEnabled, "SOURCE_DATE_EPOCH="+strconv.FormatInt(st.epoch, 10))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("build %s for %s/%s: %w", b.name, goos, goarch, err)
	}

	return path, nil
}

// writeSHA256 writes <path>.sha256 as `sha256sum <name>` would.
func writeSHA256(path string) error {
	f, err := os.Open(path) // #nosec G304 -- path is a binary this tool just built
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hash %s: %w", path, err)
	}

	line := hex.EncodeToString(h.Sum(nil)) + "  " + filepath.Base(path) + "\n"

	return os.WriteFile(path+".sha256", []byte(line), 0o644) // #nosec G306 -- release checksum file
}

// gitStamp reads the version information from git. SOURCE_DATE_EPOCH, when
// set, overrides the build time, which otherwise is the commit time.
func gitStamp(tag string) (stamp, error) {
	commit, err := git("rev-parse", "HEAD")
	if err != nil {
		return stamp{}, err
	}

	if tag == "" {
		if tag, err = git("describe", "--tags", "--abbrev=0"); err != nil {
			// No tag reachable (e.g. a shallow clone): name the build after
			// the commit, as `git describe --always` would.
			if tag, err = git("rev-parse", "--short=12", "HEAD"); err != nil {
				return stamp{}, err
			}

			fmt.Fprintf(os.Stderr, "dist: no tag reachable from HEAD, using %s (pass tag=... to set one)\n", tag)
		}
	}

	epochStr := os.Getenv("SOURCE_DATE_EPOCH")
	if epochStr == "" {
		if epochStr, err = git("log", "-1", "--format=%ct", "HEAD"); err != nil {
			return stamp{}, err
		}
	}

	epoch, err := strconv.ParseInt(epochStr, 10, 64)
	if err != nil {
		return stamp{}, fmt.Errorf("SOURCE_DATE_EPOCH %q: %w", epochStr, err)
	}

	return stamp{commit: commit, tag: tag, epoch: epoch}, nil
}

func git(args ...string) (string, error) {
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(ee.Stderr)))
		}

		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}

	return strings.TrimSpace(string(out)), nil
}

// selectPlatforms returns the platforms to build. A cgo build can only target
// the native platform, which is also its default when -platforms is not given.
func selectPlatforms(platforms string, platformsSet, cgo bool, native string) ([]string, error) {
	if cgo && !platformsSet {
		platforms = native
	}

	plats, err := parsePlatforms(platforms)
	if err != nil {
		return nil, err
	}

	if cgo {
		for _, p := range plats {
			if p != native {
				return nil, fmt.Errorf("cgo=1 builds the native platform %s only, not %s", native, p)
			}
		}
	}

	return plats, nil
}
