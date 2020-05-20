SHELL=/bin/sh
BINARY ?= packet-cloud-controller-manager
BUILD_IMAGE?=packethost/packet-ccm
BUILDER_IMAGE?=packethost/go-build
PACKAGE_NAME?=github.com/packethost/packet-ccm
GIT_VERSION?=$(shell git log -1 --format="%h")
VERSION?=$(GIT_VERSION)
RELEASE_TAG ?= $(shell git tag --points-at HEAD)
ifneq (,$(RELEASE_TAG))
VERSION=$(RELEASE_TAG)-$(VERSION)
endif
GO_FILES := $(shell find . -type f -not -path './vendor/*' -name '*.go')
FROMTAG ?= latest
LDFLAGS ?= -ldflags '-extldflags "-static" -X "$(PACKAGE_NAME)/pkg/version.VERSION=$(VERSION)"'

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

IMAGENAME ?= $(BUILD_IMAGE):$(IMAGETAG)-$(ARCH)

# Manifest tool, until `docker manifest` is fully ready. As of this writing, it remains experimental
MANIFEST_VERSION ?= 1.0.0
MANIFEST_URL = https://github.com/estesp/manifest-tool/releases/download/v$(MANIFEST_VERSION)/manifest-tool-$(BUILDOS)-$(BUILDARCH)

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
MANIFEST_TOOL ?= $(GOBIN)/manifest-tool

pkgs:
ifndef PKG_LIST
	$(eval PKG_LIST := $(shell $(BUILD_CMD) go list ./... | grep -v vendor))
endif

.PHONY: fmt-check lint test vet golint tag version

$(DIST_DIR):
	mkdir -p $@

## report the git tag that would be used for the images
tag:
	@echo $(GIT_VERSION)

## report the version that would be put in the binary
version:
	@echo $(VERSION)


## Check the file format
fmt-check:
	@if [ -n "$(shell $(BUILD_CMD) gofmt -l ${GO_FILES})" ]; then \
	  $(BUILD_CMD) gofmt -s -e -d ${GO_FILES}; \
	  exit 1; \
	fi

golangci-lint: $(LINTER)
$(LINTER):
	mkdir -p hacks && cd hacks && (go mod init hacks || true) && go get github.com/golangci/golangci-lint/cmd/golangci-lint@v1.27.0

golint:
ifeq (, $(shell which golint))
	go get -u golang.org/x/lint/golint
endif

## Lint the files
lint: pkgs golangci-lint
	@$(BUILD_CMD) $(LINTER) run --disable-all --enable=golint ./ ./packet

## Run unittests
test: pkgs
	@$(BUILD_CMD) go test -short ${PKG_LIST}

## Vet the files
vet: pkgs
	@$(BUILD_CMD) go vet ${PKG_LIST}

## Read about data race https://golang.org/doc/articles/race_detector.html
## to not test file for race use `// +build !race` at top
## Run data race detector
race: pkgs
	@$(BUILD_CMD) go test -race -short ${PKG_LIST}

## Display this help screen
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

## Delete the ccm
undeploy:
	kubectl delete --now -f releases/v0.0.0.yaml

## Deploy the controller to kubernetes
deploy:
	kubectl apply -f releases/v0.0.0.yaml



.PHONY: build build-all image push deploy ci cd dep manifest-tool

## Build the binaries for all supported ARCH
build-all: $(addprefix sub-build-, $(ARCHES))
sub-build-%:
	@$(MAKE) ARCH=$* build

## Build the binary for a single ARCH
build: $(DIST_BINARY)
$(DIST_BINARY): $(DIST_DIR)
	$(BUILD_CMD) go build -v -o $@ $(LDFLAGS) ./

## copy a binary to an install destination
install:
ifneq (,$(DESTDIR))
	mkdir -p $(DESTDIR)
	cp $(DIST_BINARY) $(DESTDIR)/$(shell basename $(DIST_BINARY))
endif

manifest-tool: $(MANIFEST_TOOL)
$(MANIFEST_TOOL):
	curl -L -o $@ $(MANIFEST_URL)
	chmod +x $@

## make the images for all supported ARCH
image-all: $(addprefix sub-image-, $(ARCHES))
sub-image-%:
	@$(MAKE) ARCH=$* image

## make the image for a single ARCH
image:
	docker buildx build -t $(BUILD_IMAGE):latest-$(ARCH) -f Dockerfile --build-arg ARCH=$(ARCH) --platform $(OS)/$(ARCH) .
	echo "Done. image is at $(BUILD_IMAGE):latest-$(ARCH)"

# Targets used when cross building.
.PHONY: register
# Enable binfmt adding support for miscellaneous binary formats.
# This is only needed when running non-native binaries.
register:
	docker pull $(QEMU_IMAGE)
	docker run --rm --privileged $(QEMU_IMAGE) --reset -p yes || true

## push the multi-arch manifest
push-manifest: manifest-tool imagetag
	# path to credentials based on manifest-tool's requirements here https://github.com/estesp/manifest-tool#sample-usage
	$(GOBIN)/manifest-tool push from-args --platforms $(call join_platforms,$(ARCHES)) --template $(BUILD_IMAGE):$(IMAGETAG)-ARCH --target $(BUILD_IMAGE):$(IMAGETAG)

## push the images for all supported ARCH
push-all: imagetag $(addprefix sub-push-, $(ARCHES))
sub-push-%:
	@$(MAKE) ARCH=$* push IMAGETAG=$(IMAGETAG)

push: imagetag
	docker push $(IMAGENAME)

# ensure we have a real imagetag
imagetag:
ifndef IMAGETAG
	$(error IMAGETAG is undefined - run using make <target> IMAGETAG=X.Y.Z)
endif

## tag the images for all supported ARCH
tag-images-all: $(addprefix sub-tag-image-, $(ARCHES))
sub-tag-image-%:
	@$(MAKE) ARCH=$* IMAGETAG=$(IMAGETAG) tag-images

tag-images: imagetag
	docker tag $(BUILD_IMAGE):$(FROMTAG)-$(ARCH) $(IMAGENAME)

## ensure that a particular tagged image exists across all support archs
pull-images-all: $(addprefix sub-pull-image-, $(ARCHES))
sub-pull-image-%:
	@$(MAKE) ARCH=$* IMAGETAG=$(IMAGETAG) pull-images

## ensure that a particular tagged image exists locally; if not, pull it
pull-images: imagetag
	@if [ "$$(docker image ls -q $(IMAGENAME))" = "" ]; then \
	docker pull $(IMAGENAME); \
	fi

## clean up all artifacts
clean:
	$(eval IMAGE_TAGS := $(shell docker image ls | awk "/^$(subst /,\/,$(BUILD_IMAGE))\s/"' {print $$2}' ))
	docker image rm $(addprefix $(BUILD_IMAGE):,$(IMAGE_TAGS))
	rm -rf dist/

###############################################################################
# CI/CD
###############################################################################
.PHONY: ci cd build deploy push release confirm pull-images
## Run what CI runs
# race has an issue with alpine, see https://github.com/golang/go/issues/14481
# image-all removed so can run ci locally
ci: build-all fmt-check lint test vet # image-all race

confirm:
ifndef CONFIRM
	$(error CONFIRM is undefined - run using make <target> CONFIRM=true)
endif

cd: confirm
ifndef BRANCH_NAME
	$(error BRANCH_NAME is undefined - run using make <target> BRANCH_NAME=var or set an environment variable)
endif
	$(MAKE) tag-images-all push-all push-manifest IMAGETAG=${BRANCH_NAME}
	$(MAKE) tag-images-all push-all push-manifest IMAGETAG=${GIT_VERSION}

## cut a release by using the latest git tag should only be run for an image that already exists and was pushed out
release: confirm
ifeq (,$(RELEASE_TAG))
	$(error RELEASE_TAG is undefined - this means we are trying to do a release at a commit which does not have a release tag)
endif
	$(MAKE) pull-images-all IMAGETAG=${GIT_VERSION} # ensure we have the image with the tag ${GIT_VERSION} or pull it
	$(MAKE) tag-images-all FROMTAG=${GIT_VERSION} IMAGETAG=${RELEASE_TAG}  # tag the pulled image
	$(MAKE) push-all push-manifest IMAGETAG=${RELEASE_TAG}        # push it


ccm: build deploy ## Build and deploy the ccm
