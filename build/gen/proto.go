package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
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
	// goModulePrefix is the import path prefix of the protos in this repository.
	goModulePrefix = "github.com/theQRL/qrysm/"

	// grpcGatewayModule provides proto/gateway/event_source.proto and the
	// protoc-gen-grpc-gateway plugin. go.mod replaces it with the theQRL fork.
	grpcGatewayModule = "github.com/grpc-ecosystem/grpc-gateway/v2"

	// googleapisPin and googleapisSHA256 match the googleapis http_archive in
	// MODULE.bazel.
	googleapisPin    = "64926d52febbf298cb82a8f472ade4a3969ba922"
	googleapisSHA256 = "9d1a930e767c93c825398b8f8692eca3fe353b9aaadedfbcf1fca2282c85df88"
)

var googleapisProtos = []string{
	"google/api/annotations.proto",
	"google/api/http.proto",
}

// The protoc plugins, built from the versions go.mod selects.
var protoPlugins = map[string]string{
	"protoc-gen-go":           "google.golang.org/protobuf/cmd/protoc-gen-go",
	"protoc-gen-go-cast":      "github.com/prysmaticlabs/protoc-gen-go-cast",
	"protoc-gen-grpc-gateway": grpcGatewayModule + "/protoc-gen-grpc-gateway",
}

const (
	modeCast     = "cast"
	modeCastGRPC = "cast_grpc"
	modeStock    = "stock"
)

// protoPkg is a proto package directory and how its Go code is generated.
type protoPkg struct {
	dir  string
	mode string
	// gateway holds the protoc-gen-grpc-gateway options, nil when the package
	// has no .pb.gw.go files.
	gateway []string
}

// protoToolchain is what generating Go code from the protos needs besides the
// repository: the plugins and the directories of the imported protos.
type protoToolchain struct {
	binDir   string
	includes []string
}

func newProtoToolchain(tmpRoot string) (*protoToolchain, error) {
	googleapis, err := fetchGoogleapis(filepath.Join(tmpRoot, "googleapis"))
	if err != nil {
		return nil, fmt.Errorf("fetch googleapis: %w", err)
	}

	gateway, err := moduleDir(grpcGatewayModule)
	if err != nil {
		return nil, fmt.Errorf("locate %s: %w", grpcGatewayModule, err)
	}

	binDir, err := buildProtoPlugins(tmpRoot)
	if err != nil {
		return nil, fmt.Errorf("build proto plugins: %w", err)
	}

	return &protoToolchain{binDir: binDir, includes: []string{googleapis, gateway}}, nil
}

func genProto() error {
	mainnet, minimal, pkgs := sszSizes["mainnet"], sszSizes["minimal"], protoPkgs

	tmpRoot, err := os.MkdirTemp("", "gen-proto-")
	if err != nil {
		return fmt.Errorf("mkdirTemp: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpRoot) }()

	tc, err := newProtoToolchain(tmpRoot)
	if err != nil {
		return err
	}

	outMain := filepath.Join(tmpRoot, "mainnet")
	if err := generateNetwork(mainnet, outMain, tc, pkgs); err != nil {
		return fmt.Errorf("generate mainnet: %w", err)
	}

	outMin := filepath.Join(tmpRoot, "minimal")
	if err := generateNetwork(minimal, outMin, tc, pkgs); err != nil {
		return fmt.Errorf("generate minimal: %w", err)
	}

	written, err := writeProtoOutputs(pkgs, outMain, outMin)
	if err != nil {
		return fmt.Errorf("write outputs: %w", err)
	}

	// The checked-in files have always been run through goimports and
	// gofmt -s (https://github.com/gogo/protobuf/issues/554).
	if err := goimports(written...); err != nil {
		return fmt.Errorf("goimports: %w", err)
	}

	if err := gofmtSimplify(written...); err != nil {
		return fmt.Errorf("gofmtSimplify: %w", err)
	}

	return applyGenModes(written)
}

// writeProtoOutputs replaces the generated files of every package with the
// generated mainnet ones. A file whose minimal version differs gets the
// "!minimal" build constraint and a <name>.minimal.pb[.gw].go twin with the
// "minimal" one. It returns the repo-relative paths it wrote.
func writeProtoOutputs(pkgs []protoPkg, outMain, outMin string) ([]string, error) {
	for _, p := range pkgs {
		if err := removeGeneratedProtos(p.dir); err != nil {
			return nil, fmt.Errorf("remove generated files: %w", err)
		}
	}

	mainFiles, err := generatedFiles(outMain)
	if err != nil {
		return nil, err
	}

	minFiles, err := generatedFiles(outMin)
	if err != nil {
		return nil, err
	}

	if len(mainFiles) != len(minFiles) {
		return nil, fmt.Errorf("mainnet generated %d files, minimal %d", len(mainFiles), len(minFiles))
	}

	var written []string
	for _, rel := range mainFiles {
		mainData, err := os.ReadFile(filepath.Join(outMain, rel)) // #nosec G304 -- under our own temp out dir
		if err != nil {
			return nil, fmt.Errorf("readFile: %w", err)
		}

		minData, err := os.ReadFile(filepath.Join(outMin, rel)) // #nosec G304 -- under our own temp out dir
		if err != nil {
			return nil, fmt.Errorf("minimal counterpart of %s: %w", rel, err)
		}

		if bytes.Equal(mainData, minData) {
			if err := writeGenerated(rel, mainData); err != nil {
				return nil, err
			}

			written = append(written, rel)
			continue
		}

		twin := minimalTwin(rel)
		if err := writeGenerated(rel, withBuildTag("!minimal", mainData)); err != nil {
			return nil, err
		}

		if err := writeGenerated(twin, withBuildTag("minimal", minData)); err != nil {
			return nil, err
		}

		written = append(written, rel, twin)
	}

	return written, nil
}

// generatedFiles lists the repo-relative paths of the .go files under dir.
func generatedFiles(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") {
			return err
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return fmt.Errorf("filepath.Rel: %w", err)
		}

		out = append(out, filepath.ToSlash(rel))

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk %s: %w", dir, err)
	}

	sort.Strings(out)

	return out, nil
}

// isGeneratedProto reports whether name is a file generate writes for a proto
// package.
func isGeneratedProto(name string) bool {
	return strings.HasSuffix(name, ".pb.go") || strings.HasSuffix(name, ".pb.gw.go")
}

// removeGeneratedProtos deletes the generated files of a package, so a file
// whose proto went away, or that no longer needs a minimal twin, does not
// linger.
func removeGeneratedProtos(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("readDir: %w", err)
	}

	for _, e := range entries {
		if e.IsDir() || !isGeneratedProto(e.Name()) {
			continue
		}

		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			return fmt.Errorf("remove: %w", err)
		}
	}

	return nil
}

// minimalTwin returns the name of the minimal twin of a generated file:
// x.pb.go -> x.minimal.pb.go, x.pb.gw.go -> x.minimal.pb.gw.go,
// generated.ssz.go -> generated.minimal.ssz.go.
func minimalTwin(name string) string {
	for _, ext := range []string{".pb.gw.go", ".pb.go", ".ssz.go"} {
		if strings.HasSuffix(name, ext) {
			return strings.TrimSuffix(name, ext) + ".minimal" + ext
		}
	}

	return strings.TrimSuffix(name, ".go") + ".minimal.go"
}

func withBuildTag(tag string, data []byte) []byte {
	return append([]byte("//go:build "+tag+"\n\n"), data...)
}

func writeGenerated(path string, data []byte) error {
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("writeFile: %w", err)
	}

	return nil
}

// applyGenModes reproduces the modes of the Bazel outputs the generated files
// used to be copied from: 0755 for .pb.go and .pb.gw.go (rules_go marks them
// executable, and changing that would touch every file's mode in git), 0644
// for the minimal twins, which Bazel never wrote.
func applyGenModes(paths []string) error {
	for _, path := range paths {
		mode := os.FileMode(0o755)
		if strings.Contains(filepath.Base(path), ".minimal.") {
			mode = 0o644
		}

		if err := os.Chmod(path, mode); err != nil {
			return fmt.Errorf("chmod %s: %w", path, err)
		}
	}

	return nil
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
	resp, err := http.Get(url) // #nosec G107 -- url is built from the pinned googleapis commit constant
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

	if sum := fmt.Sprintf("%x", sha256.Sum256(data)); sum != wantSHA256 {
		return nil, fmt.Errorf("sha256 mismatch for %s: got %s, want %s", url, sum, wantSHA256)
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

	data, err := io.ReadAll(rc) // #nosec G110 -- archive is SHA-256 verified above
	if err != nil {
		return fmt.Errorf("readAll: %w", err)
	}

	return os.WriteFile(dst, data, 0o600)
}

func generateNetwork(dict map[string]string, outDir string, tc *protoToolchain, pkgs []protoPkg) error {
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
	for _, p := range pkgs {
		protos, err := pkgProtos(p.dir)
		if err != nil {
			return fmt.Errorf("pkgProtos: %w", err)
		}

		pkgFiles[p.dir] = protos
		allProtos = append(allProtos, protos...)
	}

	protoFiles, err := compileDescriptors(append([]string{stage}, tc.includes...), allProtos)
	if err != nil {
		return fmt.Errorf("compileDescriptors: %w", err)
	}

	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return fmt.Errorf("mkdir all: %w", err)
	}

	for _, p := range pkgs {
		protos := pkgFiles[p.dir]
		fmt.Printf("  %s (%s): %d files\n", p.dir, p.mode, len(protos))

		opts := []string{"paths=source_relative" + mmap}
		plugin, param, err := pluginForMode(p.mode, tc.binDir, opts)
		if err != nil {
			return fmt.Errorf("pluginForMode: %w", err)
		}

		if err := runPlugin(plugin, request(protos, param, protoFiles), outDir); err != nil {
			return fmt.Errorf("%s: %s: %w", p.dir, filepath.Base(plugin), err)
		}

		if p.gateway == nil {
			continue
		}

		gwPlugin := filepath.Join(tc.binDir, "protoc-gen-grpc-gateway")
		gwParam := strings.Join(append(append([]string{}, p.gateway...), opts...), ",")
		if err := runPlugin(gwPlugin, request(protos, gwParam, protoFiles), outDir); err != nil {
			return fmt.Errorf("%s: %s: %w", p.dir, filepath.Base(gwPlugin), err)
		}
	}

	return nil
}

func request(files []string, param string, protoFiles []*descriptorpb.FileDescriptorProto) *pluginpb.CodeGeneratorRequest {
	return &pluginpb.CodeGeneratorRequest{
		FileToGenerate: files,
		Parameter:      &param,
		ProtoFile:      protoFiles,
	}
}

// compileDescriptors compiles files and returns them with all their imports
// in dependency order, as protoc hands them to a plugin. The source info is
// dropped: rules_go runs the plugins on descriptor sets without it, so the
// checked-in code carries no comments from the protos.
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

func runPlugin(plugin string, req *pluginpb.CodeGeneratorRequest, outDir string) error {
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
		if !strings.HasPrefix(dst, filepath.Clean(outDir)+string(os.PathSeparator)) {
			return fmt.Errorf("plugin output %q escapes the output directory", f.GetName())
		}

		if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
			return fmt.Errorf("mkdir all: %w", err)
		}

		if err := os.WriteFile(dst, []byte(f.GetContent()), 0o600); err != nil {
			return fmt.Errorf("writeFile: %w", err)
		}
	}

	return nil
}

// pluginForMode returns the plugin binary and parameter for a package's .pb.go
// files, with the options of the go_proto_compiler the BUILD file names.
func pluginForMode(mode, binDir string, opts []string) (plugin, param string, err error) {
	castPlugin := filepath.Join(binDir, "protoc-gen-go-cast")
	switch mode {
	case modeCast:
		return castPlugin, strings.Join(opts, ","), nil
	case modeCastGRPC:
		return castPlugin, strings.Join(append([]string{"plugins=grpc", "silent=true"}, opts...), ","), nil
	case modeStock:
		return filepath.Join(binDir, "protoc-gen-go"), strings.Join(opts, ","), nil
	default:
		return "", "", fmt.Errorf("unknown proto plugin mode %q", mode)
	}
}

// buildProtoPlugins builds the protoc plugins at the versions go.mod selects
// (the grpc-gateway one from the theQRL fork, through go.mod's replace).
func buildProtoPlugins(tmpRoot string) (string, error) {
	binDir := filepath.Join(tmpRoot, "bin")
	if err := os.MkdirAll(binDir, 0o750); err != nil {
		return "", fmt.Errorf("mkdir all: %w", err)
	}

	names := make([]string, 0, len(protoPlugins))
	for name := range protoPlugins {
		names = append(names, name)
	}

	sort.Strings(names)

	for _, name := range names {
		fmt.Printf("building %s\n", name)
		if err := sh("go", "build", "-o", filepath.Join(binDir, name), protoPlugins[name]); err != nil {
			return "", fmt.Errorf("build %s: %w", name, err)
		}
	}

	return binDir, nil
}

// stageProtos copies every .proto under proto/ into stage with the SSZ size
// placeholders of dict substituted, as ssz_proto_files does in Bazel.
func stageProtos(stage string, dict map[string]string) error {
	keys := make([]string, 0, len(dict))
	for k := range dict {
		keys = append(keys, k)
	}

	// Longest key first, so a key that is a substring of another cannot
	// replace part of it.
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}

		return keys[i] < keys[j]
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

		if err := os.WriteFile(dst, []byte(s), 0o600); err != nil {
			return fmt.Errorf("writeFile: %w", err)
		}

		return nil
	})
}

// buildMMAP maps every proto of the repository to the Go import path of its
// directory, as rules_go does with the importpath of each go_proto_library.
// The protos in this repository set go_package as well, except for
// proto/testing.
func buildMMAP(stage string) (string, error) {
	var b strings.Builder
	stageProto := filepath.Join(stage, "proto")
	err := filepath.WalkDir(stageProto, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".proto") {
			return err
		}

		rel, err := filepath.Rel(stage, path)
		if err != nil {
			return fmt.Errorf("filepath.Rel: %w", err)
		}

		rel = filepath.ToSlash(rel)
		fmt.Fprintf(&b, ",M%s=%s%s", rel, goModulePrefix, filepath.ToSlash(filepath.Dir(rel)))

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
