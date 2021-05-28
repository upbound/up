# ====================================================================================
# Setup Project

PROJECT_NAME := up
PROJECT_REPO := github.com/upbound/$(PROJECT_NAME)

PLATFORMS ?= linux_amd64 linux_arm64 linux_arm darwin_amd64 darwin_arm64 windows_amd64
# -include will silently skip missing files, which allows us
# to load those files with a target in the Makefile. If only
# "include" was used, the make command would fail and refuse
# to run a target until the include commands succeeded.
-include build/makelib/common.mk

# ====================================================================================
# Setup Output

S3_BUCKET ?= public-cli.releases
-include build/makelib/output.mk

# ====================================================================================
# Setup Go

# Set a sane default so that the nprocs calculation below is less noisy on the initial
# loading of this file
NPROCS ?= 1

# each of our test suites starts a kube-apiserver and running many test suites in
# parallel can lead to high CPU utilization. by default we reduce the parallelism
# to half the number of CPU cores.
GO_TEST_PARALLEL := $(shell echo $$(( $(NPROCS) / 2 )))

GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/up
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.version=$(VERSION)
GO_SUBDIRS += cmd internal
GO111MODULE = on
-include build/makelib/golang.mk

# ====================================================================================
# Targets

# run `make help` to see the targets and options

# We want submodules to be set up the first time `make` is run.
# We manage the build/ folder and its Makefiles as a submodule.
# The first time `make` is run, the includes of build/*.mk files will
# all fail, and this target will be run. The next time, the default as defined
# by the includes will be run instead.
fallthrough: submodules
	@echo Initial setup complete. Running make again . . .
	@make

# TODO(hasheddan): consider adding the following build targets as native
# functionality in the build submodule.
build.init: build.bundle.init

build.bundle.init:
	@mkdir -p $(abspath $(OUTPUT_DIR)/bundle)

ifeq ($(OS),linux)
build.artifacts.platform: build.artifacts.bundle.platform build.artifacts.pkg.platform
else
build.artifacts.platform: build.artifacts.bundle.platform
endif

build.artifacts.bundle.platform:
	@sha256sum $(GO_OUT_DIR)/up$(GO_OUT_EXT) | head -c 64 >  $(GO_OUT_DIR)/up.sha256
	@tar -czvf $(abspath $(OUTPUT_DIR)/bundle/$(PLATFORM)).tar.gz -C $(GO_BIN_DIR) $(PLATFORM)

build.artifacts.pkg.platform:
	@cat nfpm.yaml | GO_BIN_DIR=$(GO_BIN_DIR) envsubst > $(CACHE_DIR)/nfpm.yaml
	@mkdir -p $(OUTPUT_DIR)/deb/$(PLATFORM)
	@CACHE_DIR=$(CACHE_DIR) OUTPUT_DIR=$(OUTPUT_DIR) PLATFORM=$(PLATFORM) PACKAGER=deb $(GO) generate -tags packaging ./...
	@mkdir -p $(OUTPUT_DIR)/rpm/$(PLATFORM)
	@CACHE_DIR=$(CACHE_DIR) OUTPUT_DIR=$(OUTPUT_DIR) PLATFORM=$(PLATFORM) PACKAGER=rpm $(GO) generate -tags packaging ./...

# Update the submodules, such as the common build scripts.
submodules:
	@git submodule sync
	@git submodule update --init --recursive

.PHONY: submodules fallthrough
