
# Image URL to use all building/pushing image targets
IMG ?= mosheshi/go-k8s-http-api-interface:latest  # TODO(moshe): implement versioning

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

# CONTAINER_TOOL defines the container tool to be used for building images.
# Be aware that the target commands are only tested with Docker which is
# scaffolded by default. However, you might want to replace it to use other
# tools. (i.e. podman)
CONTAINER_TOOL ?= docker

# Setting SHELL to bash allows bash commands to be executed by recipes.
# Options are set to exit when a recipe line exits non-zero or a piped command fails.
SHELL = /usr/bin/env bash -o pipefail
.SHELLFLAGS = -ec

.PHONY: all
all: build

##@ General

# The help target prints out all targets with their descriptions organized
# beneath their categories. The categories are represented by '##@' and the
# target descriptions by '##'. The awk command is responsible for reading the
# entire set of makefiles included in this invocation, looking for lines of the
# file as xyz: ## something, and then pretty-format the target and help. Then,
# if there's a line with ##@ something, that gets pretty-printed as a category.
# More info on the usage of ANSI control characters for terminal formatting:
# https://en.wikipedia.org/wiki/ANSI_escape_code#SGR_parameters
# More info on the awk command:
# http://linuxcommand.org/lc3_adv_awk.php

.PHONY: help
help: ## Display this help.
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} /^[a-zA-Z_0-9-]+:.*?##/ { printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2 } /^##@/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 5) } ' $(MAKEFILE_LIST)

##@ Development

.PHONY: fmt
fmt: ## Run go fmt against code.
	go fmt ./...

.PHONY: vet
vet: ## Run go vet against code.
	go vet ./...

.PHONY: test
test: fmt vet ## Run tests.
	go test ./... -coverprofile cover.out -mod=vendor

GOLANGCI_LINT = $(shell pwd)/bin/golangci-lint
GOLANGCI_LINT_VERSION ?= v1.63.4
golangci-lint:
	@[ -f $(GOLANGCI_LINT) ] || { \
	set -e ;\
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(shell dirname $(GOLANGCI_LINT)) $(GOLANGCI_LINT_VERSION) ;\
	}

.PHONY: lint
lint: golangci-lint ## Run golangci-lint linter & yamllint
	$(GOLANGCI_LINT) run

.PHONY: lint-fix
lint-fix: golangci-lint ## Run golangci-lint linter and perform fixes
	$(GOLANGCI_LINT) run --fix

.PHONY: helm-lint
helm-lint: ## Run helm lint
	helm lint helm

.PHONY: ci 
ci: lint test helm-lint ## Run all CI tests

##@ Build

.PHONY: generate-certs
generate-certs:   ## Generate certs for the api. Will generate certs in ./certs directory if they don't exist (to recreate- delete the certs directory).
	./hack/generate-certs.sh


.PHONY: build
build: fmt vet ## Build api binary.
	go build -o bin/api cmd/main.go

.PHONY: run
run: fmt vet generate-certs ## Run the api locally on your host. Will load certs from ./certs directory and generate if they don't exist. Will load kubeconfig from ~/.kube/config. Will listen on port 8443 (https).
	go run ./cmd/main.go --server-cert certs/server.crt --cert-key certs/server.key --ca-cert ./certs/ca.crt

# If you wish to build the api image targeting other platforms you can use the --platform flag.
# (i.e. docker build --platform linux/arm64). However, you must enable docker buildKit for it.
# More info: https://docs.docker.com/develop/develop-images/build_enhancements/
.PHONY: docker-build
docker-build: ## Build docker image for the api.
	$(CONTAINER_TOOL) build -t ${IMG} .

.PHONY: docker-push
docker-push: ## Push docker image for the api.
	$(CONTAINER_TOOL) push ${IMG}

.PHONY: docker-build-push
docker-build-push: docker-build docker-push ## Build and push docker image for the api.

##@ Deployment

.PHONY: deploy
deploy: ## Deploy api to the K8s cluster specified in ~/.kube/config.
	$(HELM) install k8s-api-proxy --namespace k8s-api-proxy --create-namespace --wait --timeout 5m0s ./helm

.PHONY: undeploy
undeploy: ## Undeploy api from the K8s cluster specified in ~/.kube/config
	$(HELM) uninstall k8s-api-proxy --namespace k8s-api-proxy --timeout 5m0s
	$(KUBECTL) delete namespace k8s-api-proxy --timeout 5m0s

##@ Build Dependencies

## Tool Binaries
KUBECTL ?= kubectl
HELM ?= helm
