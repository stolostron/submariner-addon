#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${SCRIPT_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../../../k8s.io/code-generator)}

verify="${VERIFY:-}"

set -x

if [ "$verify" == --verify-only ]; then
  # Remove the file-nuking portion of the code-generation scripts
  awk -i inplace '/# Nuke existing files/ { nuke = 1 } /^$/ { nuke = 0 } !nuke' ${CODEGEN_PKG}/generate-internal-groups.sh
fi

chmod 755 ${CODEGEN_PKG}/generate-internal-groups.sh

for group in submarinerconfig submarinerdiagnoseconfig; do
  bash ${CODEGEN_PKG}/generate-groups.sh "client,lister,informer" \
    github.com/stolostron/submariner-addon/pkg/client/${group} \
    github.com/stolostron/submariner-addon/pkg/apis \
    "${group}:v1alpha1" \
    --go-header-file ${SCRIPT_ROOT}/hack/boilerplate.txt \
    ${verify}
done

