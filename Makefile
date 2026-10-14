SHELL := /bin/bash
.SHELLFLAGS := -o pipefail -c

GO   ?= go
DIST ?= dist

# ---------------------------------------------------------------------------
# Binaries: name -> package, plus the build tags each one always needs
# ---------------------------------------------------------------------------
BINARIES := beacon-chain validator qrysmctl client-stats deposit

PKG_beacon-chain := ./cmd/beacon-chain
PKG_validator    := ./cmd/validator
PKG_qrysmctl     := ./cmd/qrysmctl
PKG_client-stats := ./cmd/client-stats
PKG_deposit      := ./cmd/staking-deposit-cli/deposit

# qrysmctl does not embed the mainnet genesis state.
TAGS_qrysmctl := noMainnetGenesis

GEN_KINDS  := proto ssz mocks
TEST_KINDS := mainnet minimal race
LINT_KINDS := fix
E2E_KINDS  := minimal slashing slasher
IMAGES     := beacon-chain validator alltools
COMMANDS   := run build gen test lint e2e dist image image-push clean help
POSITIONAL := $(sort $(GEN_KINDS) $(TEST_KINDS) $(LINT_KINDS) $(E2E_KINDS) $(IMAGES) $(BINARIES))

TAGS  ?=
flags ?=

ALLOWED_VARS := GO DIST TAGS flags mode pkgs platform cgo tag DOCKER repo image_tag arch
BAD_VARS := $(strip $(foreach v,$(.VARIABLES),$(if $(filter command line,$(origin $(v))),$(filter-out $(ALLOWED_VARS),$(v)))))
ifneq ($(BAD_VARS),)
$(error unknown variable(s): $(BAD_VARS)  (allowed: $(ALLOWED_VARS)))
endif

comma := ,
empty :=
space := $(empty) $(empty)

# bin_tagflag(bin): -tags= with TAGS plus the binary's own tags, if any.
bin_tags    = $(subst $(space),$(comma),$(strip $(subst $(comma),$(space),$(TAGS)) $(TAGS_$(1))))
bin_tagflag = $(if $(call bin_tags,$(1)),-tags=$(call bin_tags,$(1)))

GEN_MODE     := $(or $(mode),no-force)
GEN_MODE_BAD := $(filter-out force no-force,$(GEN_MODE))

TEST_MODE     := $(or $(mode),no-race)
TEST_MODE_BAD := $(filter-out no-race race,$(TEST_MODE))
TEST_ARGS     := $(filter-out $(COMMANDS),$(MAKECMDGOALS))
TEST_BAD      := $(filter-out $(TEST_KINDS),$(TEST_ARGS))

LINT_ARGS := $(filter-out $(COMMANDS),$(MAKECMDGOALS))
LINT_BAD  := $(filter-out $(LINT_KINDS),$(LINT_ARGS))

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
	@$(if $(GEN_MODE_BAD),echo "gen: invalid mode '$(GEN_MODE)' (one of: force no-force)" >&2; exit 1;) \
	$(if $(GEN_BAD),echo "gen: unknown kind(s): $(GEN_BAD) (one of: $(GEN_KINDS))" >&2; exit 1;) \
	$(GO) run ./build/gen --mode=$(GEN_MODE) $(filter $(GEN_KINDS),$(MAKECMDGOALS))

.PHONY: clean
clean:
	rm -rf .gen-cache.json .cache/qrysm-vet $(DIST)

# ---------------------------------------------------------------------------
# Run
# ---------------------------------------------------------------------------
.PHONY: run
run:
	@$(MAKE) --no-print-directory gen
	@bin="$(strip $(RUN_BIN))"; \
	case "$$bin" in \
	  "")    echo "run: specify a binary (one of: $(BINARIES))" >&2; exit 1;; \
	  *" "*) echo "run: only one binary at a time (got: $$bin)" >&2; exit 1;; \
	esac
	$(strip $(GO) run $(call bin_tagflag,$(RUN_BIN)) $(flags) $(PKG_$(RUN_BIN)) $(RUN_ARGS))

# ---------------------------------------------------------------------------
# Build
# ---------------------------------------------------------------------------
BUILD_BINS := $(or $(filter $(BINARIES),$(MAKECMDGOALS)),$(BINARIES))
BUILD_BAD  := $(filter-out $(COMMANDS) $(BINARIES),$(MAKECMDGOALS))

.PHONY: build
build:
	@$(if $(BUILD_BAD),echo "build: not a binary: $(BUILD_BAD) (one of: $(BINARIES))" >&2; exit 1;) true
	@$(MAKE) --no-print-directory gen
	@set -e; mkdir -p $(DIST); \
	$(foreach b,$(BUILD_BINS),cmd='$(strip $(GO) build $(call bin_tagflag,$(b)) $(flags) -o $(DIST)/$(b) $(PKG_$(b)))'; echo "$$cmd"; eval "$$cmd";) \
	echo "build ==> $(DIST)/"

# ---------------------------------------------------------------------------
# Test
# ---------------------------------------------------------------------------
.PHONY: test
test:
	@$(if $(TEST_MODE_BAD),echo "test: invalid mode '$(TEST_MODE)' (one of: no-race race)" >&2; exit 1;) \
	$(if $(TEST_BAD),echo "test: unknown kind(s): $(TEST_BAD) (one of: $(TEST_KINDS))" >&2; exit 1;) true
	@$(MAKE) --no-print-directory gen mode=no-force
	@GO="$(GO)" $(GO) run ./build/test $(if $(filter race,$(TEST_MODE)),-race) $(if $(pkgs),-pkgs="$(pkgs)") $(TEST_ARGS)

# ---------------------------------------------------------------------------
# Static analysis
# ---------------------------------------------------------------------------
# qrysm-vet runs the analyzers of Bazel's nogo with its nogo_config.json, in
# the configurations Bazel builds the packages in: the default one for every
# package, `develop` for the packages Bazel tests with gotags = ["develop"],
# `minimal` for those it tests with qrl_network = "minimal". -deps also reports
# on the dependencies of the latter within the module, as nogo analyzes every
# package a target builds.
VET         := $(CURDIR)/.cache/qrysm-vet
VET_DEVELOP := ./beacon-chain/blockchain ./config/params
VET_MINIMAL := ./beacon-chain/core/helpers ./beacon-chain/rpc/qrysm/v1alpha1/beacon \
	./beacon-chain/rpc/qrysm/v1alpha1/validator ./beacon-chain/state/state-native \
	./config/fieldparams ./config/params ./testing/endtoend \
	./testing/spectest/minimal/zond/epoch_processing ./testing/spectest/minimal/zond/finality \
	./testing/spectest/minimal/zond/forkchoice ./testing/spectest/minimal/zond/operations \
	./testing/spectest/minimal/zond/random ./testing/spectest/minimal/zond/rewards \
	./testing/spectest/minimal/zond/sanity ./testing/spectest/minimal/zond/ssz_static

.PHONY: lint
lint:
	@$(if $(LINT_BAD),echo "lint: unknown argument(s): $(LINT_BAD) (one of: $(LINT_KINDS))" >&2; exit 1;) true
	@$(MAKE) --no-print-directory gen mode=no-force
	$(GO) build -o $(VET) ./tools/qrysm-vet
ifneq ($(filter fix,$(LINT_ARGS)),)
	$(GO) fix -fixtool=$(VET) ./...
	$(GO) fix -fixtool=$(VET) -tags=develop $(VET_DEVELOP)
	$(GO) fix -fixtool=$(VET) -tags=minimal $(VET_MINIMAL)
endif
	$(GO) vet -vettool=$(VET) ./...
	$(GO) vet -vettool=$(VET) -tags=develop -deps $(VET_DEVELOP)
	$(GO) vet -vettool=$(VET) -tags=minimal -deps $(VET_MINIMAL)

# ---------------------------------------------------------------------------
# End-to-end tests
# ---------------------------------------------------------------------------
# Builds beacon-chain, validator, bootnode and gqrl into $(DIST)/ and runs the
# scenarios one at a time (default: minimal). The Bazel e2e targets that need
# web3signer or a builder (builder, mainnet, the scenario runs) are not
# ported: Bazel never defined the @web3signer they depend on.
.PHONY: e2e
e2e:
	@$(if $(E2E_BAD),echo "e2e: unknown scenario(s): $(E2E_BAD) (one of: $(E2E_KINDS))" >&2; exit 1;) true
	@$(MAKE) --no-print-directory gen mode=no-force
	@GO="$(GO)" DIST="$(DIST)" $(GO) run ./build/e2e $(E2E_ARGS)

# ---------------------------------------------------------------------------
# Release binaries
# ---------------------------------------------------------------------------
# `make dist [<bin>...] [platform=<os>/<arch>,...] [cgo=1] [tag=...]` builds
# $(DIST)/<bin>-<tag>-<os>-<arch>[.exe] and a .sha256 for each, stamped with
# the version and reproducible (see build/dist). By default: the binaries a
# release publishes, for linux/amd64, linux/arm64, darwin/amd64, darwin/arm64
# and windows/amd64, with CGO_ENABLED=0. cgo=1 builds with cgo, for the
# native platform only.
DIST_RELEASE_BINS := beacon-chain validator deposit
DIST_ARGS := $(filter-out $(COMMANDS),$(MAKECMDGOALS))
DIST_BAD  := $(filter-out $(BINARIES),$(DIST_ARGS))
DIST_BINS := $(or $(filter $(BINARIES),$(DIST_ARGS)),$(DIST_RELEASE_BINS))
DIST_CGO_BAD := $(filter-out 0 1,$(or $(cgo),0))

.PHONY: dist
dist:
	@$(if $(DIST_BAD),echo "dist: not a binary: $(DIST_BAD) (one of: $(BINARIES))" >&2; exit 1;) true
	@$(if $(DIST_CGO_BAD),echo "dist: cgo must be 0 or 1" >&2; exit 1;) true
	@$(MAKE) --no-print-directory gen mode=no-force
	$(GO) run ./build/dist -out=$(DIST) -go=$(GO) $(if $(platform),-platforms=$(platform)) \
	  $(if $(filter 1,$(cgo)),-cgo) $(if $(tag),-tag=$(tag)) \
	  $(foreach b,$(DIST_BINS),$(b)=$(PKG_$(b))$(if $(call bin_tags,$(b)),:$(call bin_tags,$(b))))

# ---------------------------------------------------------------------------
# Docker images
# ---------------------------------------------------------------------------
# tools/docker/Dockerfile reproduces tools/qrysm_image.bzl: the same
# distroless base and Debian packages, with the binaries `make dist` builds
# for linux. Images are tagged <repo>:<image>-<image_tag>, as
# hack/build_and_upload_docker.sh pushed them with Bazel.
#
#   make image [<image>...] [arch=amd64|arm64]  build for one arch, load into docker
#   make image-push [<image>...] image_tag=...  build for linux/amd64 and linux/arm64,
#                                              push the multi-arch images
DOCKER      ?= docker
repo        ?= index.docker.io/qrledger/qrysm
arch        ?= $(shell $(GO) env GOARCH)
IMAGE_BINS  := beacon-chain validator qrysmctl client-stats deposit
VERSION_TAG  = $(or $(tag),$(shell git describe --tags --abbrev=0 2>/dev/null || git rev-parse --short=12 HEAD))
IMAGE_TAG    = $(or $(image_tag),$(VERSION_TAG))
IMAGE_ARGS  := $(filter-out $(COMMANDS),$(MAKECMDGOALS))
IMAGE_BAD   := $(filter-out $(IMAGES),$(IMAGE_ARGS))
IMAGE_SEL   := $(or $(filter $(IMAGES),$(IMAGE_ARGS)),$(IMAGES))

# image_build(platforms, output flag)
image_build = set -e; tag="$(VERSION_TAG)"; \
	$(MAKE) --no-print-directory dist $(IMAGE_BINS) platform=$(1) tag="$$tag"; \
	for img in $(IMAGE_SEL); do \
	  ref="$(repo):$$img-$(IMAGE_TAG)"; \
	  echo "$(DOCKER) buildx build $$img for $(1) -> $$ref"; \
	  $(DOCKER) buildx build -f tools/docker/Dockerfile --platform $(1) --target "$$img" \
	    --build-arg TAG="$$tag" -t "$$ref" $(2) .; \
	done

.PHONY: image
image:
	@$(if $(IMAGE_BAD),echo "image: unknown image(s): $(IMAGE_BAD) (one of: $(IMAGES))" >&2; exit 1;) true
	@$(call image_build,linux/$(arch),--load)

.PHONY: image-push
image-push:
	@$(if $(IMAGE_BAD),echo "image-push: unknown image(s): $(IMAGE_BAD) (one of: $(IMAGES))" >&2; exit 1;) true
	@$(call image_build,linux/amd64$(comma)linux/arm64,--push)

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
	@printf "  %-54s %s\n" "make run <bin> [flags=...] [-- <args>]"         "Run a binary"
	@printf "  %-54s %s\n" "make build [<bin>...] [flags=...]"              "Build binaries into $(DIST)/ (default: all)"
	@printf "  %-54s %s\n" "make gen [$(GEN_KINDS)] [mode=force|no-force]"  "Regenerate the generated code (default: all, no-force)"
	@printf "  %-54s %s\n" "make test [$(TEST_KINDS)] [mode=no-race|race]" "Run the unit tests (default: all, no-race)"
	@printf "  %-54s %s\n" "make lint [fix]"                                "Run the static analysis (qrysm-vet); fix applies the suggested fixes"
	@printf "  %-54s %s\n" "make e2e [$(E2E_KINDS)]"                     "Run end-to-end tests (default: minimal)"
	@printf "  %-54s %s\n" "make dist [<bin>...] [platform=...] [cgo=1] [tag=...]" "Build release binaries (default: $(DIST_RELEASE_BINS), all platforms)"
	@printf "  %-54s %s\n" "make image [<image>...] [arch=...]"           "Build Docker images for one arch and load them (images: $(IMAGES))"
	@printf "  %-54s %s\n" "make image-push [<image>...] image_tag=..."   "Build multi-arch Docker images and push them to $(repo)"
	@printf "  %-54s %s\n" "make clean"                                     "Remove $(DIST)/ and the make gen cache"
	@printf "  %-54s %s\n" "make help"                                      "Show this help"
	@echo ""
	@echo "Options:"
	@printf "  %-16s %s\n" "<bin>:"   "$(BINARIES)"
	@printf "  %-16s %s\n" "TAGS=..." "extra build tags, comma-separated"
	@printf "  %-16s %s\n" "flags=..." "extra go build/run flags"
	@printf "  %-16s %s\n" "pkgs=..."  "test: package patterns to restrict to, '-' prefix excludes"
	@echo ""
	@echo "Notes:"
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
