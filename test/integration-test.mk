TEST_TMP :=/tmp

export KUBEBUILDER_ASSETS ?=$(TEST_TMP)/kubebuilder/bin

K8S_VERSION ?=1.30.0
KB_TOOLS_ARCHIVE_NAME :=envtest-v$(K8S_VERSION)-$(GOHOSTOS)-$(GOHOSTARCH).tar.gz
KB_TOOLS_ARCHIVE_PATH := $(TEST_TMP)/$(KB_TOOLS_ARCHIVE_NAME)

# download the kubebuilder-tools to get kube-apiserver binaries from it
ensure-kubebuilder-tools:
ifeq "" "$(wildcard $(KUBEBUILDER_ASSETS))"
	$(info Downloading kube-apiserver into '$(KUBEBUILDER_ASSETS)')
	mkdir -p '$(KUBEBUILDER_ASSETS)'
	curl -s -f -L https://github.com/kubernetes-sigs/controller-tools/releases/download/envtest-v$(K8S_VERSION)/$(KB_TOOLS_ARCHIVE_NAME) -o '$(KB_TOOLS_ARCHIVE_PATH)'
	tar -C '$(KUBEBUILDER_ASSETS)' --strip-components=2 -zvxf '$(KB_TOOLS_ARCHIVE_PATH)'
else
	$(info Using existing kube-apiserver from "$(KUBEBUILDER_ASSETS)")
endif
.PHONY: ensure-kubebuilder-tools

clean-integration-test:
	$(RM) '$(KB_TOOLS_ARCHIVE_PATH)'
	rm -rf $(TEST_TMP)/kubebuilder
	$(RM) ./integration.test
.PHONY: clean-integration-test

clean: clean-integration-test

test-integration: ensure-kubebuilder-tools
	go test -c ./test/integration
	./integration.test -ginkgo.v -ginkgo.fail-fast
.PHONY: test-integration
