#!/bin/bash

set -o errexit
set -o nounset
set -o pipefail

SCRIPT_ROOT=$(dirname ${BASH_SOURCE})/..
CODEGEN_PKG=${CODEGEN_PKG:-$(cd ${SCRIPT_ROOT}; ls -d -1 ./vendor/k8s.io/code-generator 2>/dev/null || echo ../../../k8s.io/code-generator)}

if [ "${VERIFY:-}" = "--verify-only" ]; then
  outprefix=$(mktemp -d -p .)/
  trap "rm -rf ${outprefix}" EXIT
else
  outprefix=
fi

set -x

. "${CODEGEN_PKG}/kube_codegen.sh"

for group in submarinerconfig submarinerdiagnoseconfig; do
   kube::codegen::gen_client \
     --output-dir "${outprefix}pkg/client/${group}" \
     --output-pkg "github.com/stolostron/submariner-addon/pkg/client/${group}" \
     --boilerplate "${SCRIPT_ROOT}/hack/boilerplate.txt" \
     --with-watch \
     --one-input-api "${group}" \
     "pkg/apis"
done

if [ "${VERIFY:-}" = "--verify-only" ]; then
  if ! diff -urN "${outprefix}/pkg/client" pkg/client; then
      echo "Regenerating the client code resulted in changes."
      echo "Commit the updates along with the changes that cause them."
      exit 1
  fi
fi
