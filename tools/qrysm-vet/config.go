package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// The config is embedded rather than read at run time: `go vet` / `go fix` cache results keyed on
// the tool binary, so a config read from disk could be changed without invalidating stale results.
//
//go:embed nogo_config.json
var rawConfig []byte

// baseConfigName is the entry applying to every analyzer, overridden field by field by the
// analyzer's own entry (same semantics as rules_go's nogo).
const baseConfigName = "_base"

// jsonConfig is one entry of nogo_config.json.
type jsonConfig struct {
	OnlyFiles     map[string]string `json:"only_files"`
	ExcludeFiles  map[string]string `json:"exclude_files"`
	AnalyzerFlags map[string]string `json:"analyzer_flags"`
}

// config is a compiled nogo_config.json entry.
type config struct {
	onlyFiles     []*regexp.Regexp
	excludeFiles  []*regexp.Regexp
	analyzerFlags map[string]string
}

// globalExcludes mirrors the exclude_files of the nogo_config_exclude rule in
// the root BUILD.bazel: the generated and third-party code nothing reports on.
var globalExcludes = []string{
	"bazel-out/.*",
	"external/.*",
	".*/mock/.*",
	".*/mock\\.go$",
	".*_mock\\.go$",
	".*\\.pb(\\.gw)?\\.go$",
	".*ssz_generated_.*\\.go$",
	// The checked-in name of what Bazel generates as ssz_generated_files.go.
	".*generated(\\.minimal)?\\.ssz\\.go$",
}

// excludeChecks mirrors NOGO_CONFIG_EXCLUDE_CHECKS in the root BUILD.bazel,
// without the staticcheck names: the analyzers that are not in
// nogo_config.json but get globalExcludes too. Like the nogo_config tool,
// parseConfigs adds globalExcludes to every entry of nogo_config.json and to
// these.
var excludeChecks = []string{
	"appends", "asmdecl", "atomic", "atomicalign", "bools", "buildssa", "ctrlflow",
	"deepequalerrors", "defers", "directive", "errorsas", "findcall", "framepointer",
	"httpmux", "httpresponse", "httpwriter", "ifaceassert", "inspect", "nilfunc",
	"printf", "slog", "sortslice", "testinggoroutine", "tests", "timeformat",
	"unmarshal", "unusedresult", "usesgenerics",
}

// parseConfigs compiles every entry of a nogo_config.json content, after adding
// excludes to the exclude_files of each entry and of each analyzer of checks.
func parseConfigs(raw []byte, excludes, checks []string) (map[string]*config, error) {
	var entries map[string]jsonConfig
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if len(excludes) > 0 {
		names := make([]string, 0, len(entries)+len(checks))
		for name := range entries {
			names = append(names, name)
		}

		for _, name := range append(names, checks...) {
			entry := entries[name]
			if entry.ExcludeFiles == nil {
				entry.ExcludeFiles = make(map[string]string, len(excludes))
			}

			for _, e := range excludes {
				if _, ok := entry.ExcludeFiles[e]; !ok {
					entry.ExcludeFiles[e] = "Excluded by qrysm-vet's globalExcludes"
				}
			}

			entries[name] = entry
		}
	}

	configs := make(map[string]*config, len(entries))
	for name, entry := range entries {
		only, err := compileAll(entry.OnlyFiles)
		if err != nil {
			return nil, fmt.Errorf("%s: only_files: %w", name, err)
		}

		exclude, err := compileAll(entry.ExcludeFiles)
		if err != nil {
			return nil, fmt.Errorf("%s: exclude_files: %w", name, err)
		}

		configs[name] = &config{onlyFiles: only, excludeFiles: exclude, analyzerFlags: entry.AnalyzerFlags}
	}

	return configs, nil
}

// compileAll compiles the keys of a nogo `pattern -> reason` map. A nil map stays nil, so that
// an absent field falls back to the base config.
func compileAll(patterns map[string]string) ([]*regexp.Regexp, error) {
	if patterns == nil {
		return nil, nil
	}

	res := make([]*regexp.Regexp, 0, len(patterns))
	for pattern := range patterns {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}

		res = append(res, re)
	}

	return res, nil
}

// configFor returns the effective config of an analyzer, or nil if none applies.
func configFor(configs map[string]*config, name string) *config {
	base, own := configs[baseConfigName], configs[name]
	if base == nil && own == nil {
		return nil
	}

	var effective config
	if base != nil {
		effective = *base
	}

	if own != nil {
		if own.onlyFiles != nil {
			effective.onlyFiles = own.onlyFiles
		}

		if own.excludeFiles != nil {
			effective.excludeFiles = own.excludeFiles
		}

		if own.analyzerFlags != nil {
			effective.analyzerFlags = own.analyzerFlags
		}
	}

	return &effective
}

// keep reports whether a diagnostic in filename (relative to the repository root) is reported.
func (c *config) keep(filename string) bool {
	if len(c.onlyFiles) > 0 && !matchAny(c.onlyFiles, filename) {
		return false
	}

	return !matchAny(c.excludeFiles, filename)
}

func matchAny(res []*regexp.Regexp, s string) bool {
	for _, re := range res {
		if re.MatchString(s) {
			return true
		}
	}

	return false
}

// moduleRoot returns the closest directory at or above dir holding a go.mod file.
// `go vet` runs the tool from the analyzed package's directory.
func moduleRoot(dir string) (string, error) {
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod found")
		}

		dir = parent
	}
}
