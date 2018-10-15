SHELL=/bin/sh
BINARY ?= packet-cloud-controller-manager
BUILD_IMAGE?=packethost/packet-ccm
BUILDER_IMAGE?=packethost/go-build
PACKAGE_NAME?=github.com/packethost/packet-ccm
GIT_VERSION?=$(shell git describe --tags --dirty --always --long) 
RELEASE_TAG ?= $(shell git tag --points-at HEAD)
GO_FILES := $(shell find . -type f -not -path './vendor/*' -name '*.go')
FROMTAG ?= latest

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
ARCH ?= $(BUILDARCH)

# canonicalized names for target architecture
ifeq ($(ARCH),aarch64)
        override ARCH=arm64
endif
ifeq ($(ARCH),x86_64)
    override ARCH=amd64
endif

DIST_DIR=./dist/bin
DIST_BINARY = $(DIST_DIR)/$(BINARY)-$(ARCH)
BUILD_CMD =
ifdef DOCKERBUILD
BUILD_CMD = docker run --rm \
                -e GOARCH=$(ARCH) \
                -e GOOS=linux \
                -e CGO_ENABLED=0 \
                -v $(CURDIR):/go/src/$(PACKAGE_NAME) \
                -w /go/src/$(PACKAGE_NAME) \
		$(BUILDER_IMAGE)
endif

pkgs:
ifndef PKG_LIST
	$(eval PKG_LIST := $(shell $(BUILD_CMD) go list ./... | grep -v vendor))
endif

.PHONY: fmt-check lint test vet golint

$(DIST_DIR):
	mkdir -p $@

## Check the file format
fmt-check: 
	@if [ -n "$(shell $(BUILD_CMD) gofmt -l ${GO_FILES})" ]; then \
	  $(BUILD_CMD) gofmt -s -e -d ${GO_FILES}; \
	  exit 1; \
	fi

golint:
ifeq (, $(shell which golint))
	go get -u golang.org/x/lint/golint
endif

## Lint the files
lint: pkgs golint
	@$(BUILD_CMD) golint -set_exit_status ${PKG_LIST}

## Run unittests
test: pkgs
	@$(BUILD_CMD) go test -short ${PKG_LIST}

## Vet the files
vet: 
	@$(BUILD_CMD) go vet ${VET_LIST}

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



.PHONY: build builder image vendor push deploy ci cd dep

## Build the binary in docker
build: $(DIST_BINARY)
$(DIST_BINARY): $(DIST_DIR) builder vendor
	$(BUILD_CMD) go build -v -o $@ $(LDFLAGS) ./

## ensure we have dep installed
dep: 
ifeq (, $(shell which dep))
	mkdir -p $$GOPATH/bin
	curl https://raw.githubusercontent.com/golang/dep/master/install.sh | sh
endif


## ensure vendor dependencies are installed
vendor: builder dep
	$(BUILD_CMD) dep ensure -vendor-only

builder:
	docker build -t $(BUILDER_IMAGE) -f Dockerfile_builder .

## make the image
image:
	docker image build -t $(BUILD_IMAGE):latest -f Dockerfile.$(ARCH) dist/

push: imagetag	
	docker push $(BUILD_IMAGE):$(IMAGETAG)

# ensure we have a real imagetag
imagetag:
ifndef IMAGETAG
	$(error IMAGETAG is undefined - run using make <target> IMAGETAG=X.Y.Z)
endif

tag-images: imagetag 
	docker tag $(BUILD_IMAGE):$(FROMTAG) $(BUILD_IMAGE):$(IMAGETAG)

## ensure that a particular tagged image exists locally; if not, pull it
pull-images: imagetag
ifeq (,$(shell docker image ls -q $(BUILD_IMAGE):$(IMAGETAG)))
	docker pull $(BUILD_IMAGE):$(IMAGETAG)
endif

## clean up all artifacts
	#rm -rf $(DIST_DIR)
clean:
	$(eval IMAGE_TAGS := $(shell docker image ls | awk "/^$(subst /,\/,$(BUILD_IMAGE))\s/"' {print $$2}' ))
	docker image rm $(addprefix $(BUILD_IMAGE):,$(IMAGE_TAGS))

###############################################################################
# CI/CD
###############################################################################
.PHONY: ci cd build deploy push release confirm pull-images
## Run what CI runs
# race has an issue with alpine, see https://github.com/golang/go/issues/14481
ci: build fmt-check lint test vet image # race

confirm:
ifndef CONFIRM
	$(error CONFIRM is undefined - run using make <target> CONFIRM=true)
endif

cd: confirm
ifndef BRANCH_NAME
	$(error BRANCH_NAME is undefined - run using make <target> BRANCH_NAME=var or set an environment variable)
endif
	$(MAKE) tag-images push IMAGETAG=${BRANCH_NAME}
	$(MAKE) tag-images push IMAGETAG=${GIT_VERSION}

## cut a release by using the latest git tag should only be run for an image that already exists and was pushed out
release: confirm
ifeq (,$(RELEASE_TAG))
	$(error RELEASE_TAG is undefined - this means we are trying to do a release at a commit which does not have a release tag)
endif
	$(MAKE) pull-images IMAGETAG=${GIT_VERSION} # ensure we have the image with the tag ${GIT_VERSION} or pull it
	$(MAKE) tag-images FROMTAG=${GIT_VERSION} IMAGETAG=${RELEASE_TAG}  # tag the pulled image
	$(MAKE) push IMAGETAG=${RELEASE_TAG}        # push it


ccm: build deploy ## Build and deploy the ccm
