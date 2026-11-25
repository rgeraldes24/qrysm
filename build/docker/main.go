// Command docker assembles the qrysm OCI container images from the binaries
// `make dist` cross-built into $DIST, and writes them to $DIST as tarballs
// (`docker load -i <file>`).
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	dockerfile = "tools/docker/Dockerfile"
	builder    = "qrysm-builder"

	beaconChain = "beacon-chain"
	validator   = "validator"
	allTools    = "alltools"
)

// allToolsBinaries are the binaries of the alltools image (see the alltools
// stage of tools/docker/Dockerfile).
var allToolsBinaries = []string{beaconChain, "client-stats", "deposit", "qrysmctl", validator}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "❌ docker:", err)
		os.Exit(1)
	}
}

func run() error {
	dist := env("DIST", "dist")
	binTag := env("GIT_TAG", gitDescribe())
	imageTag := env("DOCKER_TAG", binTag)
	registry := env("DOCKER_REGISTRY", "index.docker.io/qrledger/qrysm")
	images := strings.Fields(env("DOCKER_IMAGES", beaconChain+" "+validator+" "+allTools))
	arches := strings.Fields(env("DOCKER_ARCHES", "amd64 arm64"))

	if err := ensureBuilder(); err != nil {
		return fmt.Errorf("ensure builder: %w", err)
	}

	total := len(images) * len(arches)
	n := 0
	for _, arch := range arches {
		for _, image := range images {
			n++

			args, err := buildArgs(image, binTag, arch)
			if err != nil {
				return err
			}

			for _, bin := range imageBinaries(image) {
				binPath := filepath.Join(dist, fmt.Sprintf("%s-%s-linux-%s", bin, binTag, arch))
				if _, err := os.Stat(binPath); err != nil {
					return fmt.Errorf("image %s needs %s: %w", image, binPath, err)
				}
			}

			// All images share one repository; the image name prefixes the tag
			// (see hack/build_and_upload_docker.sh).
			ref := fmt.Sprintf("%s:%s-%s", registry, image, imageTag)
			out := filepath.Join(dist, fmt.Sprintf("%s-%s-linux-%s.tar", image, binTag, arch))

			args = append(args,
				"-t", ref,
				"--output", "type=docker,dest="+out,
				"-f", dockerfile, ".",
			)

			fmt.Fprintf(os.Stderr, "[%d/%d] → docker/%s  %s  → %s\n", n, total, arch, image, out)

			// #nosec G204 -- build-time tooling: the tag, registry and image/arch names come from the Makefile environment.
			cmd := exec.Command("docker", args...)
			cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
			if err := cmd.Run(); err != nil {
				return fmt.Errorf("docker buildx %s (linux/%s): %w%s", image, arch, err, emulationHint(arch))
			}
		}
	}

	fmt.Fprintf(os.Stderr, "✅ docker: wrote %d image(s) -> %s/  (docker load -i <file>)\n", n, dist)

	return nil
}

// buildArgs returns the `docker buildx build` arguments selecting the image's
// Dockerfile stage.
func buildArgs(image, binTag, arch string) ([]string, error) {
	args := []string{
		"buildx", "--builder", builder, "build",
		"--platform", "linux/" + arch,
		"--build-arg", "TAG=" + binTag,
	}

	switch image {
	case beaconChain, validator:
		return append(args, "--target", "single", "--build-arg", "BIN="+image), nil
	case allTools:
		return append(args, "--target", "alltools"), nil
	default:
		return nil, fmt.Errorf("no image defined for %q (want %s|%s|%s)", image, beaconChain, validator, allTools)
	}
}

// imageBinaries returns the binaries an image packages.
func imageBinaries(image string) []string {
	if image == allTools {
		return allToolsBinaries
	}

	return []string{image}
}

func ensureBuilder() error {
	if exec.Command("docker", "buildx", "inspect", builder).Run() == nil {
		return nil
	}

	fmt.Fprintf(os.Stderr, "docker: creating buildx builder %q (docker-container driver)\n", builder)
	cmd := exec.Command("docker", "buildx", "create", "--name", builder, "--driver", "docker-container")
	cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("create buildx builder: %w", err)
	}

	return nil
}

func emulationHint(arch string) string {
	if arch == runtime.GOARCH {
		return ""
	}

	return fmt.Sprintf("\n\nbuilding linux/%s on a %s host runs the image's rootfs stage under "+
		"emulation. If you see \"exec format error\", register QEMU once with:\n"+
		"    docker run --privileged --rm tonistiigi/binfmt --install %s", arch, runtime.GOARCH, arch)
}

func gitDescribe() string {
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
