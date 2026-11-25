SHELL := /bin/bash
.SHELLFLAGS := -o pipefail -c

GO   ?= go

GEN_KINDS := proto ssz mocks

POSITIONAL := $(GEN_KINDS)
COMMANDS := gen clean help

ALLOWED_VARS := GO mode
BAD_VARS := $(strip $(foreach v,$(.VARIABLES),$(if $(filter command line,$(origin $(v))),$(filter-out $(ALLOWED_VARS),$(v)))))
ifneq ($(BAD_VARS),)
$(error unknown variable(s): $(BAD_VARS)  (allowed: $(ALLOWED_VARS)))
endif

GEN_MODE     := $(or $(mode),no-force)
GEN_MODE_BAD := $(filter-out force no-force,$(GEN_MODE))

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
	@printf "  \033[36m%-48s\033[0m %s\n" "make gen [<kind>...] [mode=force|no-force]" "Create generated code (default: all,no-force)"
	@printf "  \033[36m%-48s\033[0m %s\n" "make clean"                                 "Clean everything"
	@printf "  \033[36m%-48s\033[0m %s\n" "make help"                                  "Show this help"
	@echo ""
	@printf '\033[1mOptions:\033[0m\n'
	@printf "  \033[36m%-16s\033[0m %s\n" "gen <kind>:"  "$(GEN_KINDS)"
	@echo ""

# ---------------------------------------------------------------------------
# Positional-argument catch-all (lets `make gen proto` name kinds as goals)
# ---------------------------------------------------------------------------
.PHONY: $(POSITIONAL)
$(POSITIONAL): ; @:
%:
	@:
