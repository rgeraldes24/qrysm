package main

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// The config is embedded rather than read at run time: `go vet` / `go fix` cache results keyed on
// the tool binary, so a config read from disk could be changed without invalidating stale results.
//
//go:embed config.json
var rawConfig []byte

// baseConfigName is the entry applying to every analyzer, overridden field by field by the
// analyzer's own entry.
const baseConfigName = "_base"

// jsonConfig is one entry of config.json: per analyzer, the files it only reports on, the files it
// does not report on (regular expressions matched against the repository-relative path, mapped to
// the reason) and the values of its flags.
type jsonConfig struct {
	OnlyFiles     map[string]string `json:"only_files"`
	ExcludeFiles  map[string]string `json:"exclude_files"`
	AnalyzerFlags map[string]string `json:"analyzer_flags"`
}

// config is a compiled config.json entry.
type config struct {
	onlyFiles     []*regexp.Regexp
	excludeFiles  []*regexp.Regexp
	analyzerFlags map[string]string
}

// The generated files (excludedFiles) are excluded for every analyzer of config.json, for
// excludedChecks, and for every staticcheck check.
var (
	excludedFiles = []string{
		".*/mock/.*",
		".*/mock\\.go$",
		".*_mock\\.go$",
		".*\\.pb(\\.gw)?\\.go$",
		".*\\.ssz\\.go$",
	}

	excludedChecks = []string{
		"appends", "asmdecl", "atomic", "atomicalign", "bools", "buildssa", "ctrlflow",
		"deepequalerrors", "defers", "directive", "errorsas", "findcall", "framepointer", "httpmux",
		"httpresponse", "httpwriter", "ifaceassert", "inspect", "nilfunc", "printf", "slog",
		"sortslice", "testinggoroutine", "tests", "timeformat", "unmarshal", "unusedresult",
		"usesgenerics",
	}
)

const exclusionMessage = "Generated file"

// loadConfigs compiles a config.json content, with the generated files excluded.
func loadConfigs(raw []byte) (map[string]*config, error) {
	entries, err := decodeConfigs(raw)
	if err != nil {
		return nil, err
	}

	checks := make([]string, 0, len(entries)+len(excludedChecks)+len(staticcheckChecks))
	for name := range entries {
		checks = append(checks, name)
	}

	checks = append(checks, excludedChecks...)
	for _, sa := range staticcheckChecks {
		checks = append(checks, strings.ToUpper(sa))
	}

	files := excludedFiles
	for _, check := range checks {
		entry := entries[check]
		if entry.ExcludeFiles == nil {
			entry.ExcludeFiles = make(map[string]string, len(files))
		}

		for _, f := range files {
			if _, ok := entry.ExcludeFiles[f]; !ok {
				entry.ExcludeFiles[f] = exclusionMessage
			}
		}

		entries[check] = entry
	}

	return compileConfigs(entries)
}

// parseConfigs compiles every entry of a config.json content.
func parseConfigs(raw []byte) (map[string]*config, error) {
	entries, err := decodeConfigs(raw)
	if err != nil {
		return nil, err
	}

	return compileConfigs(entries)
}

func decodeConfigs(raw []byte) (map[string]jsonConfig, error) {
	var entries map[string]jsonConfig
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return entries, nil
}

func compileConfigs(entries map[string]jsonConfig) (map[string]*config, error) {
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

// compileAll compiles the keys of a `pattern -> reason` map. A nil map stays nil, so that
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
