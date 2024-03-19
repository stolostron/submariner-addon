#!/bin/bash

source "$(dirname "${BASH_SOURCE}")/lib/init.sh"

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${SCRIPT_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../../../k8s.io/code-generator)}

verify="${VERIFY:-}"

GOFLAGS="" bash ${CODEGEN_PKG}/kube_codegen.sh "deepcopy" \
  github.com/stolostron/submariner-addon/generated \
  github.com/stolostron/submariner-addon/pkg/apis \
  "submarinerconfig:v1alpha1" \
  --go-header-file ${SCRIPT_ROOT}/hack/empty.txt \
  ${verify}

GOFLAGS="" bash ${CODEGEN_PKG}/kube_codegen.sh "deepcopy" \
  github.com/stolostron/submariner-addon/generated \
  github.com/stolostron/submariner-addon/pkg/apis \
  "submarinerdiagnoseconfig:v1alpha1" \
  --go-header-file ${SCRIPT_ROOT}/hack/empty.txt \
  ${verify}
