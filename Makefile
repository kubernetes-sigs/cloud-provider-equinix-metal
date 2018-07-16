SHELL=/bin/bash
DOCKERREGISTRY ?= "packethost"
CCM_IMAGE_TAG ?= "latest"
PKG_LIST := $(shell go list ./... | grep -v vendor)
GO_FILES := $(shell find . -type f -not -path './vendor/*' -name '*.go')

.PHONY: fmt-check lint test vet

all-tests: fmt-check lint test vet race

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

undeploy-ccm: ## Delete the ccm
	kubectl delete --now -f releases/v0.0.0.yaml

deploy-ccm: ## Deploy the ccm
	kubectl apply -f releases/v0.0.0.yaml

build-ccm: ## Build the coordinator in Docker
	docker image build -t $(DOCKERREGISTRY)/packet-cloud-controller-manager:$(CCM_IMAGE_TAG) .

ccm: build-ccm deploy-ccm ## Build and deploy the ccm
