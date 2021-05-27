#!/bin/bash

set -o nounset

source scripts/clusters.sh

hub="${clusters[0]}"
managedcluster1="${clusters[0]}"
managedcluster2="${clusters[1]}"

work_dir="$(pwd)/_output"
kubeconfigs_dir="${work_dir}/kubeconfigs"

echo "Switch to hub cluster ${hub}"
export KUBECONFIG="${kubeconfigs_dir}/kind-config-${hub}/kubeconfig"

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

echo "Apply managedclusteraddon submariner-addon on managed cluster cluster1 namespace ..."
cat << EOF | kubectl apply -f -
apiVersion: addon.open-cluster-management.io/v1alpha1
kind: ManagedClusterAddOn
metadata:
  name: submariner
  namespace: cluster1
spec:
  installNamespace: submariner-operator
EOF

echo "Apply managedclusteraddon submariner-addon on managed cluster cluster2 namespace ..."
cat << EOF | kubectl apply -f -
apiVersion: addon.open-cluster-management.io/v1alpha1
kind: ManagedClusterAddOn
metadata:
  name: submariner
  namespace: cluster2
spec:
  installNamespace: submariner-operator
EOF

echo "Wait the submariner manifestworks to be created on hub ..."
while :
do
    mcounts=$(kubectl get manifestworks --all-namespaces | wc -l)
    if [ ${mcounts} -ge 5 ]; then
        kubectl get manifestworks --all-namespaces
        break
    fi
    sleep 2
done

# check submariner-addon status
for((i=1;i<=24;i++));
do
  echo "Checking clusters connections ..."
  connected=$(kubectl -n cluster1 get managedclusteraddons submariner -o=jsonpath='{range .status.conditions[*]}{.type}{"\t"}{.status}{"\n"}{end}' | grep SubmarinerConnectionDegraded | grep False)
  if [ -n "$connected" ]; then
    echo "Clusters are connected"
    echo "Use following command to check submariner-addon status"
    echo "kubectl --kubeconfig $KUBECONFIG -n cluster1 get managedclusteraddons submariner -o=yaml"
    echo "kubectl --kubeconfig $KUBECONFIG -n cluster2 get managedclusteraddons submariner -o=yaml"
    exit 0
  fi
  sleep 5
done

echo "Clusters are not connected"
echo "Use following command to check submariner-addon status"
echo "kubectl --kubeconfig $KUBECONFIG -n cluster1 get managedclusteraddons submariner -o=yaml"
echo "kubectl --kubeconfig $KUBECONFIG -n cluster2 get managedclusteraddons submariner -o=yaml"
