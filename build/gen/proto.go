package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/protoutil"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

const (
	modulePath = "github.com/theQRL/qrysm"

	// The googleapis commit providing google/api/{annotations,http}.proto.
	googleapisPin    = "64926d52febbf298cb82a8f472ade4a3969ba922"
	googleapisSHA256 = "9d1a930e767c93c825398b8f8692eca3fe353b9aaadedfbcf1fca2282c85df88"

	// grpcGatewayModule provides proto/gateway/event_source.proto. Its version
	// (and its replacement) are pinned by go.mod.
	grpcGatewayModule = "github.com/grpc-ecosystem/grpc-gateway/v2"

	// gatewayPlaceholder is written for the .pb.gw.go file of a proto file
	// without any HTTP bound service, for which the grpc-gateway plugin
	// generates nothing: every proto of a gateway package has a .pb.gw.go.
	gatewayPlaceholder = "//go:build ignore\n// +build ignore\n\npackage ignore\n"
)

var googleapisProtos = []string{
	"google/api/annotations.proto",
	"google/api/http.proto",
}

const (
	modeCast     = "cast"
	modeCastGRPC = "cast_grpc"
	modeStock    = "stock"
)

// protoPlugins are built at the versions pinned by go.mod.
var protoPlugins = []string{
	"github.com/prysmaticlabs/protoc-gen-go-cast",
	"google.golang.org/protobuf/cmd/protoc-gen-go",
	grpcGatewayModule + "/protoc-gen-grpc-gateway",
}

func protoPkgDirs(pkgs map[string]protoPkg) []string {
	dirs := make([]string, 0, len(pkgs))
	for dir := range pkgs {
		dirs = append(dirs, dir)
	}

	sort.Strings(dirs)

	return dirs
}

// isProtoOutput reports whether path is a file written by the proto generator.
func isProtoOutput(path string) bool {
	return strings.HasSuffix(path, ".pb.go") || strings.HasSuffix(path, ".pb.gw.go")
}

// minimalTwin returns the name of the minimal-config twin of a generated file:
// foo.pb.go -> foo.minimal.pb.go, foo.pb.gw.go -> foo.minimal.pb.gw.go,
// generated.ssz.go -> generated.minimal.ssz.go.
func minimalTwin(path string) string {
	for _, suffix := range []string{".pb.gw.go", ".pb.go", ".ssz.go"} {
		if base, ok := strings.CutSuffix(path, suffix); ok {
			return base + ".minimal" + suffix
		}
	}

	return path
}

func genProto() error {
	mainnet, minimal, pkgs := sszMainnet, sszMinimal, protoPkgs

	tmpRoot, err := os.MkdirTemp("", "gen-proto-")
	if err != nil {
		return fmt.Errorf("mkdirTemp: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpRoot) }()

	googleapisInc, err := fetchGoogleapis(filepath.Join(tmpRoot, "googleapis"))
	if err != nil {
		return fmt.Errorf("fetch googleapis: %w", err)
	}

	gatewayInc, err := goModuleDir(grpcGatewayModule)
	if err != nil {
		return fmt.Errorf("grpc-gateway module dir: %w", err)
	}

	fmt.Println("building protoc plugins")
	binDir, err := goBuildTools(tmpRoot, protoPlugins...)
	if err != nil {
		return fmt.Errorf("build proto plugins: %w", err)
	}

	includes := []string{googleapisInc, gatewayInc}

	outMain := filepath.Join(tmpRoot, "mainnet")
	if err := generateNetwork(mainnet, outMain, binDir, includes, pkgs); err != nil {
		return fmt.Errorf("generate mainnet: %w", err)
	}

	outMin := filepath.Join(tmpRoot, "minimal")
	if err := generateNetwork(minimal, outMin, binDir, includes, pkgs); err != nil {
		return fmt.Errorf("generate minimal: %w", err)
	}

	// Remove stale outputs (e.g. twins of a file that no longer differs between
	// configurations, or files of a deleted .proto) before writing new ones.
	if err := removeProtoOutputs(protoPkgDirs(pkgs)); err != nil {
		return fmt.Errorf("remove stale outputs: %w", err)
	}

	// Write each mainnet output back to its source-relative path. When the
	// mainnet and minimal generated code is byte-identical the proto is
	// config-invariant and is written once, untagged. When they differ -- a
	// field's Go type changed (e.g. Bitvector128 vs Bitvector16) or an
	// ssz-size/ssz-max struct tag changed with the configuration -- the file is
	// split into a //go:build !minimal file plus a <name>.minimal.pb.go twin.
	// Keeping both variants in the tree lets the SSZ generator read the correct
	// sizes for each configuration, and lets `go build -tags minimal` select
	// the right sources with no build-time codegen.
	err = filepath.WalkDir(outMain, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !isProtoOutput(path) {
			return err
		}

		rel, err := filepath.Rel(outMain, path)
		if err != nil {
			return fmt.Errorf("filepath.Rel: %w", err)
		}
		rel = filepath.ToSlash(rel)

		mainData, err := os.ReadFile(path) // #nosec G304 -- path is under our own temp out dir
		if err != nil {
			return fmt.Errorf("readFile: %w", err)
		}

		minData, err := os.ReadFile(filepath.Join(outMin, rel)) // #nosec G304 -- under our own temp out dir
		if err != nil {
			return fmt.Errorf("readFile: %w", err)
		}

		if bytes.Equal(mainData, minData) {
			return os.WriteFile(rel, mainData, 0o600)
		}

		if err := os.WriteFile(rel, tagged("!minimal", mainData), 0o600); err != nil {
			return fmt.Errorf("writeFile: %w", err)
		}

		return os.WriteFile(minimalTwin(rel), tagged("minimal", minData), 0o600)
	})
	if err != nil {
		return err
	}

	if err := goimports("proto"); err != nil {
		return fmt.Errorf("goimports: %w", err)
	}

	if err := gofmtSimplify("proto"); err != nil {
		return fmt.Errorf("gofmtSimplify: %w", err)
	}

	return applyGenModes(protoPkgDirs(pkgs))
}

// removeProtoOutputs deletes the generated files of the given package dirs.
func removeProtoOutputs(dirs []string) error {
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("readDir %s: %w", dir, err)
		}

		for _, e := range entries {
			if e.IsDir() || !isProtoOutput(e.Name()) {
				continue
			}

			if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
				return fmt.Errorf("remove: %w", err)
			}
		}
	}

	return nil
}

// The generated *.pb.go and *.pb.gw.go files have always been checked in with
// the 0755 mode. That makes no sense for Go source files, but it is kept to
// avoid a mode diff across the whole proto tree.
func applyGenModes(dirs []string) error {
	for _, dir := range dirs {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return fmt.Errorf("readDir %s: %w", dir, err)
		}

		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !isProtoOutput(name) {
				continue
			}

			mode := os.FileMode(0o755)
			if strings.Contains(name, ".minimal.") {
				mode = 0o644
			}

			if err := os.Chmod(filepath.Join(dir, name), mode); err != nil {
				return fmt.Errorf("chmod %s: %w", name, err)
			}
		}
	}

	return nil
}

func tagged(tag string, data []byte) []byte {
	return append([]byte("//go:build "+tag+"\n\n"), data...)
}

// goModuleDir returns the on-disk directory of a go.mod dependency,
// downloading it first if needed.
func goModuleDir(module string) (string, error) {
	out, err := exec.Command("go", "mod", "download", "-json", module).Output() // #nosec G204 -- fixed module path
	if err != nil {
		return "", fmt.Errorf("go mod download %s: %w", module, err)
	}

	dir, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", module).Output() // #nosec G204 -- fixed module path
	if err != nil {
		return "", fmt.Errorf("go list -m %s: %w (%s)", module, err, out)
	}

	d := strings.TrimSpace(string(dir))
	if d == "" {
		return "", fmt.Errorf("module %s has no directory", module)
	}

	return d, nil
}

func fetchGoogleapis(dest string) (string, error) {
	url := fmt.Sprintf("https://github.com/googleapis/googleapis/archive/%s.zip", googleapisPin)

	data, err := downloadVerified(url, googleapisSHA256)
	if err != nil {
		return "", fmt.Errorf("download googleapis archive: %w", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open zip: %w", err)
	}

	stripPrefix := fmt.Sprintf("googleapis-%s/", googleapisPin)

	want := make(map[string]bool, len(googleapisProtos))
	for _, p := range googleapisProtos {
		want[p] = true
	}

	found := make(map[string]bool, len(want))
	for _, f := range zr.File {
		rel := strings.TrimPrefix(f.Name, stripPrefix)
		if !want[rel] {
			continue
		}

		if err := extractZipFile(f, dest, rel); err != nil {
			return "", fmt.Errorf("extract %s: %w", rel, err)
		}

		found[rel] = true
	}

	for _, p := range googleapisProtos {
		if !found[p] {
			return "", fmt.Errorf("googleapis archive missing %s", p)
		}
	}

	return dest, nil
}

func downloadVerified(url, wantSHA256 string) ([]byte, error) {
	resp, err := http.Get(url) // #nosec G107 -- url is built from a pinned constant
	if err != nil {
		return nil, fmt.Errorf("http get: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http get %s: %s", url, resp.Status)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("readAll: %w", err)
	}

	sum := sha256.Sum256(data)
	if got := hex.EncodeToString(sum[:]); got != wantSHA256 {
		return nil, fmt.Errorf("sha256 mismatch for %s: got %s, want %s", url, got, wantSHA256)
	}

	return data, nil
}

func extractZipFile(f *zip.File, destDir, rel string) error {
	dst := filepath.Join(destDir, filepath.FromSlash(rel))
	if !strings.HasPrefix(dst, filepath.Clean(destDir)+string(os.PathSeparator)) {
		return fmt.Errorf("archive entry %q escapes destination directory", rel)
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return fmt.Errorf("mkdirAll: %w", err)
	}

	rc, err := f.Open()
	if err != nil {
		return fmt.Errorf("open: %w", err)
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc) // #nosec G110 -- archive is SHA-256 verified
	if err != nil {
		return fmt.Errorf("readAll: %w", err)
	}

	return os.WriteFile(dst, data, 0o600)
}

func generateNetwork(dict map[string]string, outDir, binDir string, includes []string, pkgs map[string]protoPkg) error {
	tmp, err := os.MkdirTemp("", "gen-net-")
	if err != nil {
		return fmt.Errorf("mkdirTemp: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmp) }()

	stage := filepath.Join(tmp, "stage")
	if err := stageProtos(stage, dict); err != nil {
		return fmt.Errorf("stageProtos: %w", err)
	}

	mmap, err := buildMMAP(stage)
	if err != nil {
		return fmt.Errorf("buildMMAP: %w", err)
	}

	pkgFiles := make(map[string][]string, len(pkgs))
	var allProtos []string
	for _, dir := range protoPkgDirs(pkgs) {
		protos, err := pkgProtos(dir)
		if err != nil {
			return fmt.Errorf("pkgProtos: %w", err)
		}

		pkgFiles[dir] = protos
		allProtos = append(allProtos, protos...)
	}

	protoFiles, err := compileDescriptors(append([]string{stage}, includes...), allProtos)
	if err != nil {
		return fmt.Errorf("compileDescriptors: %w", err)
	}

	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return fmt.Errorf("mkdir all: %w", err)
	}

	for _, dir := range protoPkgDirs(pkgs) {
		pkg, protos := pkgs[dir], pkgFiles[dir]
		opt := "paths=source_relative" + mmap

		fmt.Printf("  %s (%s): %d files\n", dir, pkg.mode, len(protos))
		plugin, param, err := pluginForMode(pkg.mode, binDir, opt)
		if err != nil {
			return fmt.Errorf("pluginForMode: %w", err)
		}

		if err := runPlugin(plugin, protos, param, protoFiles, outDir); err != nil {
			return fmt.Errorf("runPlugin %s: %w", filepath.Base(plugin), err)
		}

		if !pkg.gateway() {
			continue
		}

		fmt.Printf("  %s (grpc-gateway): %d files\n", dir, len(protos))
		gwParam := strings.Join(append(slices.Clone(pkg.gatewayOpts), opt), ",")
		gwPlugin := filepath.Join(binDir, "protoc-gen-grpc-gateway")
		if err := runPlugin(gwPlugin, protos, gwParam, protoFiles, outDir); err != nil {
			return fmt.Errorf("runPlugin %s: %w", filepath.Base(gwPlugin), err)
		}

		for _, p := range protos {
			gw := filepath.Join(outDir, strings.TrimSuffix(p, ".proto")+".pb.gw.go")
			if _, err := os.Stat(gw); err == nil {
				continue
			}

			if err := os.WriteFile(gw, []byte(gatewayPlaceholder), 0o600); err != nil {
				return fmt.Errorf("writeFile: %w", err)
			}
		}
	}

	return nil
}

func compileDescriptors(importPaths, files []string) ([]*descriptorpb.FileDescriptorProto, error) {
	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(&protocompile.SourceResolver{
			ImportPaths: importPaths,
		}),
		SourceInfoMode: protocompile.SourceInfoNone,
	}

	linked, err := compiler.Compile(context.Background(), files...)
	if err != nil {
		return nil, fmt.Errorf("compile: %w", err)
	}

	var (
		out   []*descriptorpb.FileDescriptorProto
		visit func(fd protoreflect.FileDescriptor)
	)

	seen := make(map[string]bool)
	visit = func(fd protoreflect.FileDescriptor) {
		if seen[fd.Path()] {
			return
		}

		seen[fd.Path()] = true

		imports := fd.Imports()
		for i := range imports.Len() {
			visit(imports.Get(i).FileDescriptor)
		}

		fdp := protoutil.ProtoFromFileDescriptor(fd)
		fdp.SourceCodeInfo = nil
		out = append(out, fdp)
	}

	for _, f := range linked {
		visit(f)
	}

	return out, nil
}

func runPlugin(plugin string, files []string, param string, protoFiles []*descriptorpb.FileDescriptorProto, outDir string) error {
	req := &pluginpb.CodeGeneratorRequest{
		FileToGenerate: files,
		Parameter:      &param,
		ProtoFile:      protoFiles,
	}

	in, err := proto.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}

	cmd := exec.Command(plugin) // #nosec G204 -- plugin is a path under our own temp bin dir
	cmd.Stdin = bytes.NewReader(in)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: %s", err, stderr.String())
	}

	var resp pluginpb.CodeGeneratorResponse
	if err := proto.Unmarshal(stdout.Bytes(), &resp); err != nil {
		return fmt.Errorf("unmarshal response: %w", err)
	}

	if msg := resp.GetError(); msg != "" {
		return fmt.Errorf("plugin reported error: %s", msg)
	}

	for _, f := range resp.GetFile() {
		dst := filepath.Join(outDir, filepath.FromSlash(f.GetName()))
		if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
			return fmt.Errorf("mkdir all: %w", err)
		}

		if err := os.WriteFile(dst, []byte(f.GetContent()), 0o600); err != nil {
			return fmt.Errorf("writeFile: %w", err)
		}
	}

	return nil
}

func pluginForMode(mode, binDir, baseOpt string) (plugin, param string, err error) {
	castPlugin := filepath.Join(binDir, "protoc-gen-go-cast")
	switch mode {
	case modeCast:
		return castPlugin, baseOpt, nil
	case modeCastGRPC:
		return castPlugin, baseOpt + ",plugins=grpc", nil
	case modeStock:
		return filepath.Join(binDir, "protoc-gen-go"), baseOpt, nil
	default:
		return "", "", fmt.Errorf("unknown proto plugin mode %q", mode)
	}
}

// stageProtos copies the proto tree under stage, applying the configuration's
// SSZ substitutions (longest key first, so a key that is a prefix
// of another one cannot clobber it).
func stageProtos(stage string, dict map[string]string) error {
	keys := make([]string, 0, len(dict))
	for k := range dict {
		keys = append(keys, k)
	}

	slices.SortFunc(keys, func(a, b string) int {
		if len(a) != len(b) {
			return len(b) - len(a)
		}

		return strings.Compare(a, b)
	})

	return filepath.WalkDir("proto", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".proto") {
			return err
		}

		data, err := os.ReadFile(path) // #nosec G304 -- path comes from WalkDir over the repo proto tree
		if err != nil {
			return fmt.Errorf("readFile: %w", err)
		}

		s := string(data)
		for _, k := range keys {
			s = strings.ReplaceAll(s, k, dict[k])
		}

		dst := filepath.Join(stage, path)
		if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
			return fmt.Errorf("mkdir all: %w", err)
		}

		return os.WriteFile(dst, []byte(s), 0o600)
	})
}

func buildMMAP(stage string) (string, error) {
	var b strings.Builder
	err := filepath.WalkDir(filepath.Join(stage, "proto"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".proto") {
			return err
		}

		rel, err := filepath.Rel(stage, path)
		if err != nil {
			return fmt.Errorf("filepath.Rel: %w", err)
		}

		rel = filepath.ToSlash(rel)
		fmt.Fprintf(&b, ",M%s=%s/%s", rel, modulePath, filepath.ToSlash(filepath.Dir(rel)))

		return nil
	})
	if err != nil {
		return "", err
	}

	b.WriteString(",Mgoogle/api/annotations.proto=google.golang.org/genproto/googleapis/api/annotations")
	b.WriteString(",Mgoogle/api/http.proto=google.golang.org/genproto/googleapis/api/annotations")

	return b.String(), nil
}

func pkgProtos(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("readDir %s: %w", dir, err)
	}

	var out []string
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".proto") {
			continue
		}

		out = append(out, filepath.ToSlash(filepath.Join(dir, e.Name())))
	}

	sort.Strings(out)

	return out, nil
}
