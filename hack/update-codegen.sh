#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${SCRIPT_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../../../k8s.io/code-generator)}

verify="${VERIFY:-}"

set -x

for group in submarinerconfig submarinerdiagnoseconfig; do
  bash ${CODEGEN_PKG}/kube_codegen.sh "client,lister,informer" \
    github.com/stolostron/submariner-addon/pkg/client/${group} \
    github.com/stolostron/submariner-addon/pkg/apis \
    "${group}:v1alpha1" \
    --go-header-file ${SCRIPT_ROOT}/hack/boilerplate.txt \
    ${verify}
done

