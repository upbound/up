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

# Connect agent version (overrides the version referenced in internal/version).
UP_CONNECT_AGENT_VERSION ?=

# Release target version
RELEASE_TARGET ?= debug

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

GO_STATIC_PACKAGES = $(GO_PROJECT)/cmd/up $(GO_PROJECT)/cmd/docker-credential-up
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.version=$(VERSION)
ifneq ($(UP_CONNECT_AGENT_VERSION),)
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.agentVersion=$(UP_CONNECT_AGENT_VERSION)
endif
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.target=$(RELEASE_TARGET)
GO_LDFLAGS += -X $(GO_PROJECT)/internal/version.gitCommit=$(shell git rev-parse --short HEAD 2> /dev/null || true)
GO_SUBDIRS += cmd internal
GO111MODULE = on
GO_REQUIRED_VERSION = 1.22
GOLANGCILINT_VERSION = 1.56.2

-include build/makelib/golang.mk

# ====================================================================================
# Setup binaries
ifneq ($(shell type shasum 2>/dev/null),)
SHA256SUM := shasum -a 256
else ifneq ($(shell type sha256sum 2>/dev/null),)
SHA256SUM := sha256sum
else
$(error Please install sha256sum)
endif

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
	@mkdir -p $(abspath $(OUTPUT_DIR)/bundle/up)
	@mkdir -p $(abspath $(OUTPUT_DIR)/bundle/docker-credential-up)

ifeq ($(OS), linux)
ifneq ($(HOSTOS), darwin)
build.artifacts.platform: build.artifacts.bundle.platform build.artifacts.pkg.platform
endif
else
build.artifacts.platform: build.artifacts.bundle.platform
endif

build.artifacts.bundle.platform:
	@$(SHA256SUM) $(GO_OUT_DIR)/up$(GO_OUT_EXT) | head -c 64 >  $(GO_OUT_DIR)/up.sha256
	@tar -czvf $(abspath $(OUTPUT_DIR)/bundle/up/$(PLATFORM)).tar.gz -C $(GO_BIN_DIR) $(PLATFORM)/up$(GO_OUT_EXT) $(PLATFORM)/up.sha256
	@$(SHA256SUM) $(GO_OUT_DIR)/docker-credential-up$(GO_OUT_EXT) | head -c 64 >  $(GO_OUT_DIR)/docker-credential-up.sha256
	@tar -czvf $(abspath $(OUTPUT_DIR)/bundle/docker-credential-up/$(PLATFORM)).tar.gz -C $(GO_BIN_DIR) $(PLATFORM)/docker-credential-up$(GO_OUT_EXT) $(PLATFORM)/docker-credential-up.sha256

build.artifacts.pkg.platform:
	@mkdir -p $(CACHE_DIR)
	@mkdir -p $(OUTPUT_DIR)/deb/$(PLATFORM)
	@mkdir -p $(OUTPUT_DIR)/rpm/$(PLATFORM)
	@cat $(ROOT_DIR)/nfpm_up.yaml | GO_BIN_DIR=$(GO_BIN_DIR) envsubst > $(CACHE_DIR)/nfpm_up.yaml
	@cat $(ROOT_DIR)/nfpm_docker-credential-up.yaml | GO_BIN_DIR=$(GO_BIN_DIR) envsubst > $(CACHE_DIR)/nfpm_docker-credential-up.yaml
	@CACHE_DIR=$(CACHE_DIR) OUTPUT_DIR=$(OUTPUT_DIR) PLATFORM=$(PLATFORM) PACKAGER=deb $(GO) generate -tags packaging ./...
	@CACHE_DIR=$(CACHE_DIR) OUTPUT_DIR=$(OUTPUT_DIR) PLATFORM=$(PLATFORM) PACKAGER=rpm $(GO) generate -tags packaging ./...

# Update the submodules, such as the common build scripts.
submodules:
	@git submodule sync
	@git submodule update --init --recursive

.PHONY: submodules fallthrough

install:
	@$(MAKE) go.install
	@echo "New 'up' binary located at $(GOPATH)/bin/up, please ensure $(GOPATH)/bin is prepended to your 'PATH'"

# NOTE(epk): the build submodule currently overrides XDG_CACHE_HOME in
# order to force the Helm 3 to use the .work/helm directory. This causes Go on
# Linux machines to use that directory as the build cache as well. We should
# adjust this behavior in the build submodule because it is also causing Linux
# users to duplicate their build cache, but for now we just make it easier to
# identify its location in CI so that we cache between builds.
go.cachedir:
	@go env GOCACHE

go.mod.cachedir:
	@go env GOMODCACHE
