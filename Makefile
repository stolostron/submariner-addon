all: build
.PHONY: all

export GOPATH ?= $(shell go env GOPATH)

# Include the library makefile
include $(addprefix ./vendor/github.com/openshift/build-machinery-go/make/, \
	golang.mk \
	targets/openshift/deps.mk \
	targets/openshift/images.mk \
	targets/openshift/bindata.mk \
	lib/tmp.mk \
)

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

# This will call a macro called "build-image" which will generate image specific targets based on the parameters:
# $0 - macro name
# $1 - target suffix
# $2 - Dockerfile path
# $3 - context directory for image build
# It will generate target "image-$(1)" for building the image and binding it as a prerequisite to target "images".
$(call build-image,$(IMAGE),$(IMAGE_REGISTRY)/$(IMAGE),./Dockerfile,.)

# $1 - target name
# $2 - apis
# $3 - manifests
# $4 - output
$(call add-crd-gen,submarinerconfigv1alpha1,./pkg/apis/submarinerconfig/v1alpha1,./pkg/apis/submarinerconfig/v1alpha1,./pkg/apis/submarinerconfig/v1alpha1)

$(call add-bindata,submariner-broker,./manifests/broker/...,bindata,bindata,./pkg/hub/submarinerbroker/bindata/bindata.go)
$(call add-bindata,submariner-agent,./manifests/agent/...,bindata,bindata,./pkg/hub/submarineragent/bindata/bindata.go)
$(call add-bindata,submarineraddon-agent,./pkg/hub/submarineraddonagent/manifests/...,bindata,bindata,./pkg/hub/submarineraddonagent/bindata/bindata.go)
$(call add-bindata,submariner-gw-machineset,./pkg/cloud/aws/manifests/...,bindata,bindata,./pkg/cloud/aws/bindata/bindata.go)

clean:
	scripts/deploy.sh cleanup
.PHONY: clean

clusters:
	scripts/deploy.sh

demo:
	scripts/demo.sh

update-csv: ensure-operator-sdk
	cd deploy && rm -rf olm-catalog/manifests && ../$(OPERATOR_SDK) generate bundle --manifests --deploy-dir config/ --crds-dir config/crds/ --output-dir olm-catalog/ --version $(CSV_VERSION)
	rm ./deploy/olm-catalog/manifests/submariner-addon_v1_serviceaccount.yaml

update-scripts:
	hack/update-deepcopy.sh
	hack/update-swagger-docs.sh
	hack/update-codegen.sh
.PHONY: update-scripts

update-crds: ensure-controller-gen
	$(CONTROLLER_GEN) crd paths=./pkg/apis/submarinerconfig/v1alpha1 output:crd:artifacts:config=deploy/config/crds
	cp deploy/config/crds/submarineraddon.open-cluster-management.io_submarinerconfigs.yaml pkg/apis/submarinerconfig/v1alpha1/0000_00_submarineraddon.open-cluster-management.io_submarinerconfigs.crd.yaml

verify-scripts:
	bash -x hack/verify-deepcopy.sh
	bash -x hack/verify-swagger-docs.sh
	bash -x hack/verify-crds.sh
	bash -x hack/verify-codegen.sh
.PHONY: verify-scripts

verify: verify-scripts

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
