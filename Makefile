all: build
.PHONY: all

GO ?= go
export GOPATH ?= $(shell go env GOPATH)
GOHOSTOS ?=$(shell $(GO) env GOHOSTOS)
GOHOSTARCH ?=$(shell $(GO) env GOHOSTARCH)
GO_FILES ?=$(shell find . -name '*.go' -not -path '*/vendor/*' -not -path '*/_output/*' -print)
go_files_count :=$(words $(GO_FILES))
GO_BUILD_FLAGS ?=-trimpath

SOURCE_GIT_TAG ?=$(shell git describe --long --tags --abbrev=7 --match 'v[0-9]*' || echo 'v0.0.0-unknown-$(SOURCE_GIT_COMMIT)')
SOURCE_GIT_COMMIT ?=$(shell git rev-parse --short "HEAD^{commit}" 2>/dev/null)
SOURCE_GIT_TREE_STATE ?=$(shell ( ( [ ! -d ".git/" ] || git diff --quiet ) && echo 'clean' ) || echo 'dirty')

PERMANENT_TMP :=_output
PERMANENT_TMP_GOPATH :=$(PERMANENT_TMP)/tools

# Image URL to use all building/pushing image targets;
IMAGE ?= submariner-addon
IMAGE_REGISTRY ?= quay.io/stolostron

GIT_HOST ?= github.com/stolostron
BASE_DIR := $(shell basename $(PWD))
DEST := $(GOPATH)/src/$(GIT_HOST)/$(BASE_DIR)

# CSV_VERSION is used to generate new CSV manifests
CSV_VERSION?=0.4.0

OPERATOR_SDK?=$(PERMANENT_TMP_GOPATH)/bin/operator-sdk
OPERATOR_SDK_VERSION?=v1.1.0
OPERATOR_SDK_ARCHOS:=x86_64-linux-gnu
ifeq ($(GOHOSTOS),darwin)
	ifeq ($(GOHOSTARCH),amd64)
		OPERATOR_SDK_ARCHOS:=x86_64-apple-darwin
	endif
endif
operatorsdk_gen_dir:=$(dir $(OPERATOR_SDK))

# Add packages to do unit test
GO_TEST_PACKAGES :=./pkg/...

IMAGE_BUILD_DEFAULT_FLAGS ?=--allow-pull
IMAGE_BUILD_EXTRA_FLAGS ?=

IMAGEBUILDER_VERSION ?=1.2.3

IMAGEBUILDER ?= $(shell which imagebuilder 2>/dev/null)
ifneq "" "$(IMAGEBUILDER)"
_imagebuilder_installed_version = $(shell $(IMAGEBUILDER) --version)
endif

images: ensure-imagebuilder
	$(strip imagebuilder $(IMAGE_BUILD_DEFAULT_FLAGS) $(IMAGE_BUILD_EXTRA_FLAGS) \
		-t $(IMAGE_REGISTRY)/$(IMAGE) -f ./Dockerfile . \
	)

ensure-imagebuilder:
ifeq "" "$(IMAGEBUILDER)"
	$(error imagebuilder not found! Get it with: `go get github.com/openshift/imagebuilder/cmd/imagebuilder@v$(IMAGEBUILDER_VERSION)`)
else
	$(info Using existing imagebuilder from $(IMAGEBUILDER))
	@[[ "$(_imagebuilder_installed_version)" == $(IMAGEBUILDER_VERSION) ]] || \
	echo "Warning: Installed imagebuilder version $(_imagebuilder_installed_version) does not match expected version $(IMAGEBUILDER_VERSION)."
endif

# $1 - target name
# $2 - apis
# $3 - manifests
# $4 - output
$(call add-crd-gen,submarinerconfigv1alpha1,./pkg/apis/submarinerconfig/v1alpha1,./pkg/apis/submarinerconfig/v1alpha1,./pkg/apis/submarinerconfig/v1alpha1)
$(call add-crd-gen,submarinerdiagnoseconfigv1alpha1,./pkg/apis/submarinerdiagonseconfig/v1alpha1,./pkg/apis/submarinerdiagnoseconfig/v1alpha1,./pkg/apis/submarinerdiagnoseconfig/v1alpha1)

clean:
	scripts/deploy.sh cleanup
.PHONY: clean

clusters:
	scripts/deploy.sh

demo:
	scripts/demo.sh

update-csv: ensure-operator-sdk
	cd deploy && rm olm-catalog/manifests/*clusterserviceversion.yaml olm-catalog/manifests/*submarinerconfigs.yaml olm-catalog/manifests/*submarinerdiagnoseconfigs.yaml && ../$(OPERATOR_SDK) generate bundle --manifests --deploy-dir config/ --crds-dir config/crds/ --output-dir olm-catalog/ --version $(CSV_VERSION)
	rm ./deploy/olm-catalog/manifests/submariner-addon_v1_serviceaccount.yaml

update-scripts:
	hack/update-deepcopy.sh
	hack/update-swagger-docs.sh
	hack/update-codegen.sh
.PHONY: update-scripts

update-crds: ensure-controller-gen
	$(CONTROLLER_GEN) crd paths=./pkg/apis/submarinerconfig/v1alpha1 output:crd:artifacts:config=deploy/config/crds
	cp deploy/config/crds/submarineraddon.open-cluster-management.io_submarinerconfigs.yaml pkg/apis/submarinerconfig/v1alpha1/0000_00_submarineraddon.open-cluster-management.io_submarinerconfigs.crd.yaml
	$(CONTROLLER_GEN) crd paths=./pkg/apis/submarinerdiagnoseconfig/v1alpha1 output:crd:artifacts:config=deploy/config/crds
	#cp deploy/config/crds/submarineraddon.open-cluster-management.io_submarinerconfigs.yaml pkg/apis/submarinerconfig/v1alpha1/0000_00_submarineraddon.open-cluster-management.io_submarinerconfigs.crd.yaml

verify-scripts:
	bash -x hack/verify-deepcopy.sh
	bash -x hack/verify-swagger-docs.sh
	bash -x hack/verify-crds.sh
	bash -x hack/verify-csv.sh
	bash -x hack/verify-codegen.sh
.PHONY: verify-scripts

deploy-addon: ensure-operator-sdk
	$(OPERATOR_SDK) run packagemanifests deploy/olm-catalog/ --namespace open-cluster-management --version $(CSV_VERSION) --install-mode OwnNamespace --timeout=10m

clean-addon: ensure-operator-sdk
	$(OPERATOR_SDK) cleanup submariner-addon --namespace open-cluster-management --timeout 10m

ensure-operator-sdk:
ifeq "" "$(wildcard $(OPERATOR_SDK))"
	$(info Installing operator-sdk into '$(OPERATOR_SDK)')
	mkdir -p '$(operatorsdk_gen_dir)'
	curl -s -f -L https://github.com/operator-framework/operator-sdk/releases/download/$(OPERATOR_SDK_VERSION)/operator-sdk-$(OPERATOR_SDK_VERSION)-$(OPERATOR_SDK_ARCHOS) -o '$(OPERATOR_SDK)'
	chmod +x '$(OPERATOR_SDK)';
else
	$(info Using existing operator-sdk from "$(OPERATOR_SDK)")
endif

include ./test/integration-test.mk

# Ensure controller-gen
ensure-controller-gen:
ifeq (, $(shell which controller-gen))
	@{ \
	set -e ;\
	CONTROLLER_GEN_TMP_DIR=$$(mktemp -d) ;\
	cd $$CONTROLLER_GEN_TMP_DIR ;\
	go mod init tmp ;\
	go get sigs.k8s.io/controller-tools/cmd/controller-gen@v0.5.0 ;\
	rm -rf $$CONTROLLER_GEN_TMP_DIR ;\
	}
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

GOLANGCI_LINT_VERSION=v1.54.2
GOLANGCI_LINT?=$(PERMANENT_TMP_GOPATH)/bin/golangci-lint

$(GOLANGCI_LINT):
	curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(PERMANENT_TMP_GOPATH)/bin $(GOLANGCI_LINT_VERSION)

# [golangci-lint] validates Go code in the project
golangci-lint: vendor | $(GOLANGCI_LINT)
	$(GOLANGCI_LINT) version
	$(GOLANGCI_LINT) linters
	$(GOLANGCI_LINT) cache clean
	$(GOLANGCI_LINT) run --timeout 10m

vendor: go.mod
	$(GO) mod download
	$(GO) mod tidy
	$(GO) mod vendor

verify: vendor verify-gofmt verify-govet verify-scripts

verify-gofmt:
	$(info Running gofmt -s -l on $(go_files_count) file(s).)
	@TMP=$$( mktemp ); \
	gofmt -s -l $(GO_FILES) | tee $${TMP}; \
	if [ -s $${TMP} ]; then \
		echo "$@ failed - please run \`make update-gofmt\`"; \
		exit 1; \
	fi;

verify-govet:
	$(GO) vet ./...

build:
	$(GO) build $(GO_BUILD_FLAGS) -ldflags "-X github.com/stolostron/submariner-addon/pkg/version.versionFromGit="$(SOURCE_GIT_TAG)" \
	 -X github.com/stolostron/submariner-addon/pkg/version.commitFromGit="$(SOURCE_GIT_COMMIT)" \
	 -X github.com/stolostron/submariner-addon/pkg/version.gitTreeState="$(SOURCE_GIT_TREE_STATE)" \
	 -X github.com/stolostron/submariner-addon/pkg/version.buildDate="$(shell date -u +'%Y-%m-%dT%H:%M:%SZ')"" \
	 github.com/stolostron/submariner-addon/cmd/submariner

test:
	$(GO) test -race ./pkg/...
.PHONY: test

test-integration: vendor
