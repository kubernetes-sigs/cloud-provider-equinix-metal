SHELL=/bin/bash
BINARY ?= packet-cloud-controller-manager
BUILD_IMAGE?=packethost/packet-cloud-controller-manager
BUILDER_IMAGE?=packethost/go-build
PACKAGE_NAME?=github.com/packethost/packet-ccm
GIT_VERSION?=$(shell git describe --tags --dirty --always --long) 
PKG_LIST := $(shell go list ./... | grep -v vendor)
GO_FILES := $(shell find . -type f -not -path './vendor/*' -name '*.go')

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
BUILD_CMD = docker run --rm \
                -e GOARCH=$(ARCH) \
                -e GOOS=linux \
                -e CGO_ENABLED=0 \
                -v $(CURDIR):/go/src/$(PACKAGE_NAME) \
                -w /go/src/$(PACKAGE_NAME) \
		$(BUILDER_IMAGE)

.PHONY: fmt-check lint test vet

$(DIST_DIR):
	mkdir -p $@

fmt-check: ## Check the file format
	@gofmt -s -e -d ${GO_FILES}

lint: ## Lint the files
	@golint -set_exit_status ${PKG_LIST}

test: ## Run unittests
	@go test -short ${PKG_LIST}

vet: ## Vet the files
	@go vet ${VET_LIST}

## Read about data race https://golang.org/doc/articles/race_detector.html
## to not test file for race use `// +build !race` at top
race: ## Run data race detector
	@go test -race -short ${PKG_LIST}

help: ## Display this help screen
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

undeploy: ## Delete the ccm
	kubectl delete --now -f releases/v0.0.0.yaml

## Deploy the controller to kubernetes
deploy: 
	kubectl apply -f releases/v0.0.0.yaml




## Build the binary in docker
build: $(DIST_BINARY)
$(DIST_BINARY): $(DIST_BIN) builder vendor
	$(BUILD_CMD) go build -v -o $@ $(LDFLAGS) ./

## ensure vendor dependencies are installed
vendor: builder
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
	docker tag $(BUILD_IMAGE):latest $(BUILD_IMAGE):$(IMAGETAG)

###############################################################################
# CI/CD
###############################################################################
.PHONY: ci cd build deploy push
## Run what CI runs
ci: build fmt-check lint test vet race

cd:
ifndef CONFIRM
	$(error CONFIRM is undefined - run using make <target> CONFIRM=true)
endif
ifndef BRANCH_NAME
	$(error BRANCH_NAME is undefined - run using make <target> BRANCH_NAME=var or set an environment variable)
endif
	$(MAKE) tag-images push IMAGETAG=${BRANCH_NAME}
	$(MAKE) tag-images push IMAGETAG=${GIT_VERSION}

ccm: build deploy ## Build and deploy the ccm
