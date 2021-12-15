SHELL=/bin/sh
BINARY ?= cloud-provider-equinix-metal
BUILD_IMAGE?=equinix/cloud-provider-equinix-metal
BUILDER_IMAGE?=equinix/go-build
PACKAGE_NAME?=github.com/equinix/cloud-provider-equinix-metal
GIT_VERSION?=$(shell git log -1 --format="%h")
VERSION?=$(GIT_VERSION)
RELEASE_TAG ?= $(shell git tag --points-at HEAD)
MOST_RECENT_RELEASE_TAG ?= $(shell git describe --abbrev=0  2>/dev/null || true)
ifeq (,$(MOST_RECENT_RELEASE_TAG))
MOST_RECENT_RELEASE_TAG = v0.0.0
endif
ifneq (,$(RELEASE_TAG))
VERSION := $(RELEASE_TAG)
else
VERSION := $(MOST_RECENT_RELEASE_TAG)-$(VERSION)
endif
GO_FILES := $(shell find . -type f -not -path './vendor/*' -name '*.go')
BUILD_TAG ?= latest
TAGGED_IMAGE ?= $(BUILD_IMAGE):$(BUILD_TAG)
TAGGED_ARCH_IMAGE ?= $(TAGGED_IMAGE)-$(ARCH)
LDFLAGS ?= -ldflags '-extldflags "-static" -X "k8s.io/component-base/version.gitVersion=$(VERSION)"'

# which arches can we support
ARCHES=arm64 amd64

QEMU_VERSION?=4.2.0-7
QEMU_IMAGE?=multiarch/qemu-user-static:$(QEMU_VERSION)

# BUILDARCH is the host architecture
# ARCH is the target architecture
# we need to keep track of them separately
BUILDARCH ?= $(shell uname -m)
BUILDOS ?= $(shell uname -s | tr A-Z a-z)

# canonicalized names for host architecture
ifeq ($(BUILDARCH),aarch64)
BUILDARCH=arm64
endif
ifeq ($(BUILDARCH),x86_64)
BUILDARCH=amd64
endif

# unless otherwise set, I am building for my own architecture, i.e. not cross-compiling
# and for my OS
ARCH ?= $(BUILDARCH)
OS ?= $(BUILDOS)

# canonicalized names for target architecture
ifeq ($(ARCH),aarch64)
        override ARCH=arm64
endif
ifeq ($(ARCH),x86_64)
    override ARCH=amd64
endif

# these macros create a list of valid architectures for pushing manifests
space :=
space +=
comma := ,
prefix_linux = $(addprefix linux/,$(strip $1))
join_platforms = $(subst $(space),$(comma),$(call prefix_linux,$(strip $1)))

export GO111MODULE=on
DIST_DIR=./dist/bin
DIST_BINARY = $(DIST_DIR)/$(BINARY)-$(OS)-$(ARCH)
BUILD_CMD = CGO_ENABLED=0 GOOS=$(OS) GOARCH=$(ARCH)
ifdef DOCKERBUILD
BUILD_CMD = docker run --rm \
                -e GOARCH=$(ARCH) \
                -e GOOS=linux \
                -e CGO_ENABLED=0 \
                -v $(CURDIR):/go/src/$(PACKAGE_NAME) \
                -w /go/src/$(PACKAGE_NAME) \
		$(BUILDER_IMAGE)
endif

GOBIN ?= $(shell go env GOPATH)/bin
LINTER ?= $(GOBIN)/golangci-lint

pkgs:
ifndef PKG_LIST
	$(eval PKG_LIST := $(shell $(BUILD_CMD) go list ./... | grep -v vendor))
endif

.PHONY: fmt fmt-check lint test vet golint tag version

$(DIST_DIR):
	mkdir -p $@

tag: ## Report the git tag that would be used for the images
	@echo $(GIT_VERSION)

version: ## Report the version that would be put in the binary
	@echo $(VERSION)


fmt-check: ## Check all source code formatting
	@if [ -n "$(shell $(BUILD_CMD) gofmt -l ${GO_FILES})" ]; then \
	  $(BUILD_CMD) gofmt -s -e -d ${GO_FILES}; \
	  exit 1; \
	fi

fmt:   ## Format all source code files
	$(BUILD_CMD) gofmt -w -s ${GO_FILES}

golangci-lint: $(LINTER)
$(LINTER):
	mkdir -p hacks && cd hacks && (go mod init hacks || true) && go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.27.0

golint:
ifeq (, $(shell which golint))
	go get -u golang.org/x/lint/golint
endif

lint: pkgs golangci-lint ## Lint the files
	@$(BUILD_CMD) $(LINTER) run --disable-all --enable=golint ./ ./metal

test: pkgs ## Run unit tests
	@$(BUILD_CMD) go test -short ${PKG_LIST}

vet: pkgs ## Vet the files
	@$(BUILD_CMD) go vet ${PKG_LIST}

## Read about data race https://golang.org/doc/articles/race_detector.html
## to not test file for race use `// +build !race` at top
## Run data race detector
race: pkgs
	@$(BUILD_CMD) go test -race -short ${PKG_LIST}

help: ## Display this help screen
	@printf "\033[36m%s\n" "For all commands that can be used with one or more OS architecture, set the target architecture with ARCH= and the OS with OS="
	@printf "\033[36m%s\n" "Supported OS and ARCH are those for GOOS and GOARCH"
	@echo
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

## Delete the ccm
undeploy:
	kubectl delete --now -f releases/v0.0.0.yaml

## Deploy the controller to kubernetes
deploy:
	kubectl apply -f releases/v0.0.0.yaml



.PHONY: build build-all image deploy ci

build-all: $(addprefix sub-build-, $(ARCHES)) ## Build the binaries for all supported ARCH
sub-build-%:
	@$(MAKE) ARCH=$* build

build: $(DIST_BINARY) ## Build the binary for a single ARCH
$(DIST_BINARY): $(DIST_DIR)
	$(BUILD_CMD) go build -v -o $@ $(LDFLAGS) ./

## copy a binary to an install destination
install:
ifneq (,$(DESTDIR))
	mkdir -p $(DESTDIR)
	cp $(DIST_BINARY) $(DESTDIR)/$(shell basename $(DIST_BINARY))
endif

$(GOBIN):
	mkdir -p $(GOBIN)

image-all: $(addprefix sub-image-, $(ARCHES)) ## make the images for all supported ARCH
sub-image-%:
	@$(MAKE) ARCH=$* image

image: ## make the image for a single ARCH
	docker buildx build --load -t $(TAGGED_ARCH_IMAGE) -f Dockerfile --platform $(OS)/$(ARCH) .
	echo "Done. image is at $(TAGGED_ARCH_IMAGE)"

push-all: $(addprefix push-arch-, $(ARCHES)) ## Push all built images.
push-arch-%:
	@$(MAKE) ARCH=$* push

push: image ## Push image to registry for a single ARCH.
	docker push $(TAGGED_ARCH_IMAGE)

manifest-push: manifest-all ## Make single image manifest for all supported ARCH and push it to registry.
	docker manifest push $(TAGGED_IMAGE)

manifest-all: manifest-create $(addprefix manifest-annotate-arch-, $(ARCHES)) ## Annotate docker manifest with all supported ARCH.
manifest-annotate-arch-%:
	@$(MAKE) ARCH=$* manifest-annotate

manifest-annotate:
	docker manifest annotate $(TAGGED_IMAGE) $(TAGGED_ARCH_IMAGE) --arch=$(ARCH) --os=$(OS)

manifest-create: push-all ## Creates Docker manifest for all created images.
	docker manifest create $(TAGGED_IMAGE) $(addprefix --amend $(TAGGED_IMAGE)-, $(ARCHES))

# Targets used when cross building.
.PHONY: register
# Enable binfmt adding support for miscellaneous binary formats.
# This is only needed when running non-native binaries.
register:
	docker pull $(QEMU_IMAGE)
	docker run --rm --privileged $(QEMU_IMAGE) --reset -p yes || true

clean: ## clean up all artifacts
	$(eval IMAGE_TAGS := $(shell docker image ls | awk "/^$(subst /,\/,$(BUILD_IMAGE))\s/"' {print $$2}' ))
	docker image rm $(addprefix $(BUILD_IMAGE):,$(IMAGE_TAGS))
	rm -rf dist/

###############################################################################
# CI
###############################################################################
.PHONY: ci build deploy
## Run what CI runs
# race has an issue with alpine, see https://github.com/golang/go/issues/14481
# image-all removed so can run ci locally
ci: build-all fmt-check lint test vet # image-all race

ccm: build deploy ## Build and deploy the ccm
