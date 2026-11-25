SHELL := /bin/bash
.SHELLFLAGS := -o pipefail -c

GO   ?= go
DIST ?= dist

empty :=
space := $(empty) $(empty)
comma := ,

BINARIES := $(notdir $(patsubst %/,%,$(dir $(wildcard cmd/*/main.go))))
GEN_KINDS := proto ssz mocks
TEST_KINDS := mainnet mainnet-spectest minimal minimal-spectest

E2E_SCENARIOS := minimal slashing slasher builder validator-rest scenario mainnet mainnet-release scenario-mainnet
E2E_SUITES    := presubmit postsubmit scenario_tests
E2E_KINDS     := $(E2E_SCENARIOS) $(E2E_SUITES)

# Suite -> scenario(s) mapping (keep in sync with the `suites` map in build/e2e/main.go).
E2E_SUITE_presubmit      := minimal slashing slasher
E2E_SUITE_postsubmit     := builder mainnet mainnet-release
E2E_SUITE_scenario_tests := scenario-mainnet scenario validator-rest

# Build tags of specific binaries (the gotags of their Bazel go_binary rule).
BIN_TAGS_qrysmctl := noMainnetGenesis

CROSS_BINARIES := beacon-chain validator client-stats qrysmctl deposit

POSITIONAL := $(sort $(GEN_KINDS) $(TEST_KINDS) $(E2E_KINDS) $(BINARIES) $(CROSS_BINARIES))
COMMANDS := run build gen clean help test e2e dist

TAGS ?=

flags ?=

ALLOWED_VARS := GO DIST TAGS flags mode platform SOURCE_DATE_EPOCH
BAD_VARS := $(strip $(foreach v,$(.VARIABLES),$(if $(filter command line,$(origin $(v))),$(filter-out $(ALLOWED_VARS),$(v)))))
ifneq ($(BAD_VARS),)
$(error unknown variable(s): $(BAD_VARS)  (allowed: $(ALLOWED_VARS)))
endif

# tagflag <bin>: the -tags flag of a binary (TAGS plus the binary's own tags).
tagflag = $(if $(strip $(TAGS) $(BIN_TAGS_$(1))),-tags=$(subst $(space),$(comma),$(strip $(subst $(comma),$(space),$(TAGS)) $(BIN_TAGS_$(1)))),)

GEN_MODE     := $(or $(mode),no-force)
GEN_MODE_BAD := $(filter-out force no-force,$(GEN_MODE))

TEST_MODE     := $(or $(mode),no-race)
TEST_MODE_BAD := $(filter-out no-race race,$(TEST_MODE))
TEST_ARGS     := $(filter-out $(COMMANDS),$(MAKECMDGOALS))
TEST_BAD      := $(filter-out $(TEST_KINDS),$(TEST_ARGS))

E2E_ARGS := $(filter-out $(COMMANDS),$(MAKECMDGOALS))
E2E_BAD  := $(filter-out $(E2E_KINDS),$(E2E_ARGS))

# Goals left over after `run` and the binary name are the program's arguments.
# A leading `--` ends make's option parsing so `--flag value` reaches us as goals
# (caught as no-ops by the catch-all) rather than being treated as make options.
RUN_BIN  := $(filter $(BINARIES),$(MAKECMDGOALS))
RUN_ARGS := $(filter-out run $(COMMANDS) $(RUN_BIN),$(MAKECMDGOALS))

# ---------------------------------------------------------------------------
# Code generation
# ---------------------------------------------------------------------------
GEN_GOALS := $(filter-out gen,$(MAKECMDGOALS))
GEN_BAD   := $(filter-out $(GEN_KINDS),$(GEN_GOALS))

.PHONY: gen
gen:
	@$(if $(GEN_MODE_BAD),echo "❌ gen: invalid mode '$(GEN_MODE)'  (one of: force no-force)" >&2; exit 1;) \
	$(if $(GEN_BAD),echo "❌ gen: unknown kind(s): $(GEN_BAD)  (one of: $(GEN_KINDS))" >&2; exit 1;) \
	$(GO) run ./build/gen --mode=$(GEN_MODE) $(filter $(GEN_KINDS),$(MAKECMDGOALS))

.PHONY: clean
clean:
	rm -f .gen-cache.json
	$(GO) clean -cache -testcache -fuzzcache

# ---------------------------------------------------------------------------
# Run
# ---------------------------------------------------------------------------
.PHONY: run
run:
	@$(MAKE) --no-print-directory gen

	@bin="$(strip $(RUN_BIN))"; \
	case "$$bin" in \
	  "")    echo "❌ run: specify a binary (one of: $(BINARIES))" >&2; exit 1;; \
	  *" "*) echo "❌ run: only one binary at a time (got: $$bin)" >&2; exit 1;; \
	esac; \
	tagflag="$(call tagflag,$(strip $(RUN_BIN)))"; \
	cmd="$(strip $(GO) run $$tagflag $(flags) ./cmd/$$bin $(RUN_ARGS))"; \
	echo "-> $$cmd"; \
	eval "$$cmd"

# ---------------------------------------------------------------------------
# C flags (shared by `build` and `dist`)
# ---------------------------------------------------------------------------
# Baseline C flags for the cgo deps. `-O2` has to be spelled out: setting CGO_CFLAGS
# replaces the toolchain default ("-O2 -g") rather than adding to it.
CGO_CFLAGS_COMMON := -O2

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------
.PHONY: build
build:
	@$(MAKE) --no-print-directory gen

	@bins="$(or $(strip $(filter $(BINARIES),$(MAKECMDGOALS))),$(BINARIES))"; \
	bad="$(strip $(filter-out $(COMMANDS) $(BINARIES),$(MAKECMDGOALS)))"; \
	[ -z "$$bad" ] || { echo "❌ build: not a binary: $$bad (available: $(BINARIES))" >&2; exit 1; }; \
	$(foreach b,$(BINARIES),tagflag_$(subst -,_,$(b))="$(call tagflag,$(b))";) \
	mkdir -p $(DIST); \
	for b in $$bins; do \
	  eval "tagflag=\$$tagflag_$${b//-/_}"; \
	  cmd="CGO_CFLAGS=\"$(CGO_CFLAGS_COMMON)\" $(GO) build $$tagflag $(flags) -o \"$(DIST)/$$b\" ./cmd/$$b"; \
	  echo "-> $$cmd"; \
	  eval "$$cmd" || exit 1; \
	done; \
	echo "✅ build ==> $(DIST)/"

# ---------------------------------------------------------------------------
# Test
# ---------------------------------------------------------------------------
.PHONY: test
test:
	@$(if $(TEST_MODE_BAD),echo "❌ test: invalid mode '$(TEST_MODE)'  (one of: no-race race)" >&2; exit 1;) \
	$(if $(TEST_BAD),echo "❌ test: unknown kind(s): $(TEST_BAD)  (one of: $(TEST_KINDS))" >&2; exit 1;) :

	@$(MAKE) --no-print-directory gen

	@GO="$(GO)" $(GO) run ./build/test $(if $(filter race,$(TEST_MODE)),-race,) $(TEST_ARGS)

# ---------------------------------------------------------------------------
# End-to-end tests
# ---------------------------------------------------------------------------
.PHONY: e2e
e2e:
	@$(if $(E2E_BAD),echo "❌ e2e: unknown target(s): $(E2E_BAD)  (scenarios: $(E2E_SCENARIOS) - suites: $(E2E_SUITES))" >&2; exit 1;) :
	@GO="$(GO)" DIST="$(DIST)" $(GO) run ./build/e2e $(filter $(E2E_KINDS),$(MAKECMDGOALS))

# ---------------------------------------------------------------------------
# Distribution build (official, all-platforms, in Docker)
# ---------------------------------------------------------------------------
CROSS_TARGETS := \
	linux/amd64/x86_64-linux-gnu.2.31 \
	linux/arm64/aarch64-linux-gnu.2.31 \
	darwin/amd64/x86_64-macos \
	darwin/arm64/aarch64-macos \
	windows/amd64/x86_64-windows-gnu

CROSS_PLATFORMS := $(foreach t,$(CROSS_TARGETS),$(word 1,$(subst /,$(space),$(t)))/$(word 2,$(subst /,$(space),$(t))))

ALL_PLATFORMS   := $(CROSS_PLATFORMS)

# linux/arm64 C optimization flags (the copts of the linux_arm64 Bazel config).
CGO_CFLAGS_LINUX_ARM64 := -ftree-vectorize -funsafe-math-optimizations -fomit-frame-pointer -march=armv8-a

# Version stamps: only the dist binaries are stamped (host builds stay cache-friendly).
VERSION_PKG := github.com/theQRL/qrysm/runtime/version
GIT_COMMIT  := $(shell git rev-parse HEAD 2>/dev/null)
GIT_TAG     := $(shell git describe --tags --abbrev=0 2>/dev/null || echo Unknown)

# Build timestamp stamped into the dist binaries. Defaults to "now". Override it to make the
# build reproducible (identical bytes across machines/times):
#   make dist SOURCE_DATE_EPOCH=1781515081
# Flatten to a simply-expanded value so both stamps below share one instant, then derive the
# RFC3339 form from it (GNU `date -d @`, with a BSD/macOS `date -r` fallback).
SOURCE_DATE_EPOCH ?= $(shell date -u +%s)
SOURCE_DATE_EPOCH := $(SOURCE_DATE_EPOCH)
BUILD_DATE_UNIX   := $(SOURCE_DATE_EPOCH)
BUILD_DATE        := $(shell date -u -d @$(SOURCE_DATE_EPOCH) +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || date -u -r $(SOURCE_DATE_EPOCH) +%Y-%m-%dT%H:%M:%SZ)

LDFLAGS_STAMPED := -X $(VERSION_PKG).gitCommit=$(GIT_COMMIT) \
           -X $(VERSION_PKG).gitTag=$(GIT_TAG) \
           -X $(VERSION_PKG).buildDate=$(BUILD_DATE) \
           -X $(VERSION_PKG).buildDateUnix=$(BUILD_DATE_UNIX)

# Positional goals for dist: the distributed binaries named on the command line, else all.
DIST_ARGS := $(filter-out $(COMMANDS),$(MAKECMDGOALS))
DIST_BAD  := $(filter-out $(CROSS_BINARIES),$(DIST_ARGS))
DIST_BINS := $(or $(strip $(filter $(CROSS_BINARIES),$(DIST_ARGS))),$(CROSS_BINARIES))

# Which run-targets `make dist` builds: empty platform= means all, otherwise platform= is one
# or a space/comma list of exact <os>/<arch> values from CROSS_PLATFORMS (each maps to its
# CROSS_TARGETS entry). DIST_PLAT_BAD collects any unknown selector.
platform ?=
ifeq ($(strip $(platform)),)
DIST_PLAT_SEL := $(ALL_PLATFORMS)
else
DIST_PLAT_SEL := $(subst $(comma),$(space),$(platform))
endif

DIST_PLAT_BAD := $(filter-out $(ALL_PLATFORMS),$(DIST_PLAT_SEL))
DIST_BIN_PLATS := $(sort $(filter $(CROSS_PLATFORMS),$(DIST_PLAT_SEL)))
DIST_TARGETS   := $(foreach s,$(DIST_BIN_PLATS),$(filter $(s)/%,$(CROSS_TARGETS)))

# Env for the in-container dist build (build/crossdocker -> build/cross). dist is always a
# release build: stamped + stripped (-s -w).
DIST_LDFLAGS := $(LDFLAGS_STAMPED) -s -w
BUILD_CROSS_ENV = GO="$(GO)" DIST="$(DIST)" GIT_TAG="$(GIT_TAG)" \
	CGO_CFLAGS_COMMON="$(CGO_CFLAGS_COMMON)" CGO_CFLAGS_LINUX_ARM64="$(CGO_CFLAGS_LINUX_ARM64)" \
	LDFLAGS="$(DIST_LDFLAGS)" TAGFLAG="$(if $(TAGS),-tags=$(TAGS),)" \
	BIN_TAGS="$(foreach b,$(CROSS_BINARIES),$(if $(BIN_TAGS_$(b)),$(b)=$(BIN_TAGS_$(b)),))" \
	BUILD_MODE="release"

.PHONY: dist
dist:
	@$(if $(DIST_BAD),echo "❌ dist: unknown binary(ies): $(DIST_BAD)  (one of: $(CROSS_BINARIES))" >&2; exit 1;) \
	$(if $(DIST_PLAT_BAD),echo "❌ dist: unknown platform(s): $(DIST_PLAT_BAD)  (valid: $(ALL_PLATFORMS))" >&2; exit 1;) :

	@$(MAKE) --no-print-directory gen

	@$(BUILD_CROSS_ENV) CROSS_BINARIES="$(DIST_BINS)" CROSS_TARGETS="$(strip $(DIST_TARGETS))" $(GO) run ./build/crossdocker

# ---------------------------------------------------------------------------
# Help (default target)
# ---------------------------------------------------------------------------
.DEFAULT_GOAL := help
.PHONY: help
help: ## Show this help
	@echo ""
	@printf '\033[1;38;5;214m'
	@echo "Qrysm - QRL consensus client"
	@printf '\033[0m'
	@echo ""
	@printf '\033[1mCommands:\033[0m\n'
	@printf "  \033[36m%-48s\033[0m %s\n" "make run <bin> [flags=...] [-- <args>]"     "Run a binary"
	@printf "  \033[36m%-48s\033[0m %s\n" "make build [<bin>...] [flags=...]"          "Build a binary (default: all)"
	@printf "  \033[36m%-48s\033[0m %s\n" "make gen [<kind>...] [mode=force|no-force]" "Create generated code (default: all,no-force)"
	@printf "  \033[36m%-48s\033[0m %s\n" "make test [<kind>...] [mode=no-race|race]"  "Run unit tests (default: all,no-race)"
	@printf "  \033[36m%-48s\033[0m %s\n" "make e2e [<scenario>|<suite>...]"           "Run end-to-end tests (default: presubmit)"
	@printf "  \033[36m%-48s\033[0m %s\n" "make dist [<bin>...] [platform=...]"        "Build official release binaries (default: all bins, all platforms)"
	@printf "  \033[36m%-48s\033[0m %s\n" "make clean"                                 "Clean everything"
	@printf "  \033[36m%-48s\033[0m %s\n" "make help"                                  "Show this help"
	@echo ""
	@printf '\033[1mOptions:\033[0m\n'
	@printf "  \033[36m%-16s\033[0m %s\n" "<bin>:"       "$(BINARIES)"
	@printf "  \033[36m%-16s\033[0m %s\n" "gen <kind>:"  "$(GEN_KINDS)"
	@printf "  \033[36m%-16s\033[0m %s\n" "test <kind>:" "$(TEST_KINDS)"
	@printf "  \033[36m%-16s\033[0m %s\n" "e2e <scenario>:" "$(E2E_SCENARIOS)"
	@printf "  \033[36m%-16s\033[0m\n" "e2e <suite>:"
	@printf "    \033[36m%-16s\033[0m %s\n" "presubmit:"      "$(E2E_SUITE_presubmit)"
	@printf "    \033[36m%-16s\033[0m %s\n" "postsubmit:"     "$(E2E_SUITE_postsubmit)"
	@printf "    \033[36m%-16s\033[0m %s\n" "scenario_tests:" "$(E2E_SUITE_scenario_tests)"
	@printf "  \033[36m%-16s\033[0m %s\n" "dist <bin>:"     "$(CROSS_BINARIES)"
	@printf "  \033[36m%-16s\033[0m %s\n" "dist platform:"  "$(ALL_PLATFORMS) (one, or a comma-separated list)"
	@echo ""
	@printf '\033[1mNotes:\033[0m\n'
	@echo "  After '--', pass '--flag value' (not '--flag=value')"
	@echo "  'make dist SOURCE_DATE_EPOCH=<unix-seconds>' pins the build timestamp (reproducible build)"
	@echo ""

# ---------------------------------------------------------------------------
# Positional-argument catch-all (lets `make gen proto` / `make build beacon-chain`
# name kinds and binaries as goals)
# ---------------------------------------------------------------------------
.PHONY: $(POSITIONAL)
$(POSITIONAL): ; @:
%:
	@:
