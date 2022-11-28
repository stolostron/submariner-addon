#!/bin/bash

set -o nounset

source scripts/clusters.sh
source scripts/vars.sh

hub="${clusters[0]}"
managedcluster1="${clusters[0]}"
managedcluster2="${clusters[1]}"

work_dir="$(pwd)/_output"
kubeconfigs_dir="${work_dir}/kubeconfigs"

echo "Switch to hub cluster ${hub}"
export KUBECONFIG="${kubeconfigs_dir}/kind-config-${hub}/kubeconfig"

function patch_submariner_operator() {
    local cluster=$1
    echo "Waiting for submariner manifestworks to be ready for ${cluster}"
    kubectl wait --for=condition=available manifestworks submariner-operator -n ${cluster} --timeout=60s
    kubectl wait --for=condition=available manifestworks submariner-resource -n ${cluster} --timeout=60s

    echo "Waiting to patch submariner-operator deployment on ${cluster}"
    kubectl annotate submarinerconfig submariner -n ${cluster} skipOperatorGroup=""

    clusterconfig="${kubeconfigs_dir}/kind-config-${cluster}/kubeconfig"

    # wait for submariner operator to be ready
    while :
    do
        created=$(kubectl get deployment submariner-operator -n submariner-operator --kubeconfig=${clusterconfig})
        if [ -n "$created" ]; then
            kubectl get deployment submariner-operator -n submariner-operator --kubeconfig=${clusterconfig}
            kubectl wait --for=condition=available deployment submariner-operator -n submariner-operator --kubeconfig=${clusterconfig} >/dev/null 2>&1
            break
        fi
    done
    kubectl patch deployment submariner-operator -n submariner-operator --kubeconfig=${clusterconfig} --type "json" -p '[
{"op":"replace","path":"/spec/template/spec/containers/0/image","value":"'${submrepo}'/submariner-operator:'${submver}'"}]'
    kubectl wait --for=condition=available deployment submariner-operator -n submariner-operator --kubeconfig=${clusterconfig}
}

echo "Label the managed clusters with cluster.open-cluster-management.io/clusterset"
kubectl label managedclusters "${managedcluster1}" "cluster.open-cluster-management.io/clusterset=clusterset1" --overwrite
kubectl label managedclusters "${managedcluster2}" "cluster.open-cluster-management.io/clusterset=clusterset1" --overwrite

kubectl get managedclusters --show-labels

echo "Apply a clusterset that contains managed cluster cluster1 and cluster2 ..."
cat << EOF | kubectl apply -f -
apiVersion: cluster.open-cluster-management.io/v1beta1
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

echo "Create the broker object in the clusterset1-broker namespace..."
cat << EOF | kubectl apply -f -
apiVersion: submariner.io/v1alpha1
kind: Broker
metadata:
  name: submariner-broker
  namespace: clusterset1-broker
spec:
  globalnetEnabled: false
EOF

echo "Wait the submariner manifestworks to be created on hub ..."
while :
do
    mcounts=$(kubectl get manifestworks --all-namespaces | wc -l)
    if [ ${mcounts} -ge 7 ]; then
        kubectl get manifestworks --all-namespaces
        break
    fi
    sleep 2
done

# Patch submariner-operator version
for ((i=0;i<${#clusters[*]};i++));
do
    (patch_submariner_operator ${clusters[$i]}) &
done
wait

# check submariner-addon status
for((i=1;i<=48;i++));
do
  echo "$i. Checking clusters connections ..."
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
