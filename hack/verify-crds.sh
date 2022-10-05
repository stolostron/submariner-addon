#!/bin/bash

if [ ! -f ./_output/tools/bin/yq ]; then
    mkdir -p ./_output/tools/bin
    curl -s -f -L https://github.com/mikefarah/yq/releases/download/2.4.0/yq_$(go env GOHOSTOS)_$(go env GOHOSTARCH) -o ./_output/tools/bin/yq
    chmod +x ./_output/tools/bin/yq
fi

FILES=(deploy/config/crds/submarineraddon.open-cluster-management.io_submarinerconfigs.yaml)

FAILS=false
for f in $FILES
do
    if [[ $(./_output/tools/bin/yq r $f spec.versions[0].schema.openAPIV3Schema.properties.metadata.description) != "null" ]]; then
        echo "Error: cannot have a metadata description in $f"
        FAILS=true
    fi

done

if [ "$FAILS" = true ] ; then
    exit 1
fi

# Verify regenerating CRDs doesn't result in changes
if [ ! git diff --exit-code deploy/config/crds pkg/apis ] ; then
    echo "There are already changes to the CRDs, can't verify regeneration doesn't cause changes."
    exit 1
fi
make update-crds
if [ ! git diff --exit-code deploy/config/crds pkg/apis ] ; then
    echo "Regenerating CRDs (make update-crds) resulted in changes."
    echo "Commit the CRD updates along with the changes that cause them."
    exit 1
fi
