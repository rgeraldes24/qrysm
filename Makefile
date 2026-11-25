SHELL := /bin/bash
.SHELLFLAGS := -o pipefail -c

GO   ?= go
DIST ?= dist

empty :=
space := $(empty) $(empty)
comma := ,

BINARIES := $(notdir $(patsubst %/,%,$(dir $(wildcard cmd/*/main.go))))
GEN_KINDS := proto ssz mocks

# Build tags of specific binaries (the gotags of their Bazel go_binary rule).
BIN_TAGS_qrysmctl := noMainnetGenesis

POSITIONAL := $(sort $(GEN_KINDS) $(BINARIES))
COMMANDS := run build gen clean help

TAGS ?=

flags ?=

ALLOWED_VARS := GO DIST TAGS flags mode
BAD_VARS := $(strip $(foreach v,$(.VARIABLES),$(if $(filter command line,$(origin $(v))),$(filter-out $(ALLOWED_VARS),$(v)))))
ifneq ($(BAD_VARS),)
$(error unknown variable(s): $(BAD_VARS)  (allowed: $(ALLOWED_VARS)))
endif

# tagflag <bin>: the -tags flag of a binary (TAGS plus the binary's own tags).
tagflag = $(if $(strip $(TAGS) $(BIN_TAGS_$(1))),-tags=$(subst $(space),$(comma),$(strip $(subst $(comma),$(space),$(TAGS)) $(BIN_TAGS_$(1)))),)

GEN_MODE     := $(or $(mode),no-force)
GEN_MODE_BAD := $(filter-out force no-force,$(GEN_MODE))

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
# C flags
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
	@printf "  \033[36m%-48s\033[0m %s\n" "make clean"                                 "Clean everything"
	@printf "  \033[36m%-48s\033[0m %s\n" "make help"                                  "Show this help"
	@echo ""
	@printf '\033[1mOptions:\033[0m\n'
	@printf "  \033[36m%-16s\033[0m %s\n" "<bin>:"       "$(BINARIES)"
	@printf "  \033[36m%-16s\033[0m %s\n" "gen <kind>:"  "$(GEN_KINDS)"
	@echo ""
	@printf '\033[1mNotes:\033[0m\n'
	@echo "  After '--', pass '--flag value' (not '--flag=value')"
	@echo ""

# ---------------------------------------------------------------------------
# Positional-argument catch-all (lets `make gen proto` / `make build beacon-chain`
# name kinds and binaries as goals)
# ---------------------------------------------------------------------------
.PHONY: $(POSITIONAL)
$(POSITIONAL): ; @:
%:
	@:
