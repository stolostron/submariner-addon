TEST_TMP :=/tmp

SETUP_ENVTEST := $(PERMANENT_TMP_GOPATH)/bin/setup-envtest

K8S_VERSION ?=1.29.x

# Install setup-envtest tool
$(SETUP_ENVTEST):
	$(info Installing setup-envtest)
	GOBIN=$(shell pwd)/$(PERMANENT_TMP_GOPATH)/bin $(GO) install sigs.k8s.io/controller-runtime/tools/setup-envtest@latest

# Use setup-envtest to download and configure kubebuilder tools
ensure-kubebuilder-tools: $(SETUP_ENVTEST)
	$(eval KUBEBUILDER_ASSETS := $(shell $(SETUP_ENVTEST) use $(K8S_VERSION) -p path))
	@echo "Using kubebuilder assets from $(KUBEBUILDER_ASSETS)"
.PHONY: ensure-kubebuilder-tools

clean-integration-test:
	rm -rf $(TEST_TMP)/envtest-*
	$(RM) ./integration.test
.PHONY: clean-integration-test

clean: clean-integration-test

test-integration: vendor $(SETUP_ENVTEST)
	@KUBEBUILDER_ASSETS=$$($(SETUP_ENVTEST) use $(K8S_VERSION) -p path) && \
	export KUBEBUILDER_ASSETS && \
	echo "Using KUBEBUILDER_ASSETS=$$KUBEBUILDER_ASSETS" && \
	go test -c ./test/integration && \
	./integration.test -ginkgo.v -ginkgo.fail-fast
.PHONY: test-integration
