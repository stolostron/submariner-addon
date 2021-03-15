#!/bin/bash

set -o pipefail
set -o errexit
set -o nounset

source scripts/clusters.sh

subctl_verion="v0.8.0"
os=$(uname | awk '{print tolower($0)}')

hub="${clusters[0]}"
managedcluster1="${clusters[0]}"
managedcluster2="${clusters[1]}"

work_dir="$(pwd)/_output"
kubeconfigs_dir="${work_dir}/kubeconfigs"

# create a clusterset on hub
echo "Switch to hub cluster ${hub}"
export KUBECONFIG="${kubeconfigs_dir}/kind-config-${hub}/kubeconfig"

echo "Label the managed clusters with cluster.open-cluster-management.io/submariner-agent"
kubectl label managedclusters "${managedcluster1}" "cluster.open-cluster-management.io/submariner-agent=true" --overwrite
kubectl label managedclusters "${managedcluster2}" "cluster.open-cluster-management.io/submariner-agent=true" --overwrite

echo "Label the managed clusters with cluster.open-cluster-management.io/clusterset"
kubectl label managedclusters "${managedcluster1}" "cluster.open-cluster-management.io/clusterset=clusterset1" --overwrite
kubectl label managedclusters "${managedcluster2}" "cluster.open-cluster-management.io/clusterset=clusterset1" --overwrite

kubectl get managedclusters --show-labels

echo "Apply a clusterset that contains managed cluster cluster1 and cluster2 ..."
cat << EOF | kubectl apply -f -
apiVersion: cluster.open-cluster-management.io/v1alpha1
kind: ManagedClusterSet
metadata:
  name: clusterset1
EOF

echo "Wait the submariner manifestworks to be created on hub ..."
while :
do
    mcounts=$(kubectl get manifestworks --all-namespaces | wc -l)
    if [ ${mcounts} -ge 3 ]; then
        kubectl get manifestworks --all-namespaces
        break
    fi
    sleep 2
done

# show submariner status
echo "Download subctl ${subctl_verion} to ${work_dir}"
cd "${work_dir}"
rm -rf subctl-${subctl_verion}
curl -LO "https://github.com/submariner-io/submariner-operator/releases/download/${subctl_verion}/subctl-${subctl_verion}-${os}-amd64.tar.xz"
tar -xf subctl-${subctl_verion}-${os}-amd64.tar.xz
subctl="subctl-${subctl_verion}/subctl-${subctl_verion}-${os}-amd64"

export KUBECONFIG="${kubeconfigs_dir}/kind-config-${hub}/kubeconfig"
echo "Wait the submariner agent to deploy in five minutes ..."
sleep 300
${subctl} show all
