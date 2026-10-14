GO ?= go

GEN_KINDS := proto ssz mocks

# ---------------------------------------------------------------------------
# Code generation
# ---------------------------------------------------------------------------
GEN_GOALS := $(filter-out gen,$(MAKECMDGOALS))
GEN_BAD   := $(filter-out $(GEN_KINDS),$(GEN_GOALS))

.PHONY: gen
gen:
	@$(if $(GEN_BAD),echo "gen: unknown kind(s): $(GEN_BAD) (one of: $(GEN_KINDS))" >&2; exit 1;) \
	$(GO) run ./build/gen $(filter $(GEN_KINDS),$(MAKECMDGOALS))

# ---------------------------------------------------------------------------
# Help (default target)
# ---------------------------------------------------------------------------
.DEFAULT_GOAL := help
.PHONY: help
help: ## Show this help
	@echo ""
	@echo "Qrysm - QRL consensus client"
	@echo ""
	@echo "Commands:"
	@printf "  %-28s %s\n" "make gen [$(GEN_KINDS)]" "Regenerate the checked-in generated code (default: all)"
	@printf "  %-28s %s\n" "make help"               "Show this help"
	@echo ""

# ---------------------------------------------------------------------------
# Positional-argument catch-all (lets `make gen proto` name kinds as goals)
# ---------------------------------------------------------------------------
.PHONY: $(GEN_KINDS)
$(GEN_KINDS): ; @:
%:
	@:
