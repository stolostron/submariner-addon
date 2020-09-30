#!/bin/bash

set -o pipefail
set -o errexit
set -o nounset

source scripts/clusters.sh

hub="${clusters[0]}"
managedcluster1="${clusters[1]}"
managedcluster2="${clusters[2]}"

work_dir="$(pwd)/_output"
kubeconfigs_dir="${work_dir}/kubeconfigs"

# create a clusterset on hub
echo "Switch to hub cluster ${hub}"
export KUBECONFIG="${kubeconfigs_dir}/kind-config-${hub}/kubeconfig"

echo "Label the managed clusters with cluster.open-cluster-management.io/submariner-agent"
kubectl label managedclusters "${managedcluster1}" "cluster.open-cluster-management.io/submariner-agent=true" --overwrite
kubectl label managedclusters "${managedcluster2}" "cluster.open-cluster-management.io/submariner-agent=true" --overwrite

kubectl label managedclusters "${managedcluster1}" "cluster.open-cluster-management.io/clusterset=clusterset1" --overwrite
kubectl label managedclusters "${managedcluster2}" "cluster.open-cluster-management.io/clusterset=clusterset1" --overwrite

kubectl get managedclusters --show-labels

echo "Apply a clusterset that contains managed cluster cluster2 and cluster3 ..."
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
    if [ ${mcounts} -ge 7 ]; then
        kubectl get manifestworks --all-namespaces
        break
    fi
    sleep 2
done

echo "The submariner broker namespace on hub"
kubectl get ns submariner-clusterset-clusterset1-broker


# export a service from a managed cluster
echo "Switch to managed cluster ${managedcluster2}"
export KUBECONFIG="${kubeconfigs_dir}/kind-config-${managedcluster2}/kubeconfig"

echo "Wait the submariner agents to be deployed by manifestworks on managedcluster ${managedcluster2} ..."
while :
do
  mcounts=$(kubectl -n submariner-operator get pods | wc -l)
  if [ ${mcounts} -ge 7 ]; then
    kubectl -n submariner-operator get pods
    break
  fi
  sleep 2
done
kubectl wait --for=condition=Ready pods -l name=submariner-operator -n submariner-operator --timeout="5m"
kubectl wait --for=condition=Ready pods -l component=submariner-lighthouse -n submariner-operator --timeout="5m"
kubectl wait --for=condition=Ready pods -l app=submariner-engine -n submariner-operator --timeout="5m"
kubectl wait --for=condition=Ready pods -l app=submariner-routeagent -n submariner-operator --timeout="5m"

echo "Create a service nginx.defualt.svc on managedcluster ${managedcluster2}"
kubectl -n default create deployment nginx --image=nginx
kubectl -n default expose deployment nginx --port=80
kubectl -n default get svc nginx

echo "Export the service nginx.defualt.svc on managedcluster ${managedcluster2}"
cat << EOF | kubectl -n default apply -f -
apiVersion: lighthouse.submariner.io/v2alpha1
kind: ServiceExport
metadata:
  name: nginx
  namespace: default
EOF

# resovle the exported service from another managed cluster
echo "Switch to managed cluster ${managedcluster1}"
export KUBECONFIG="${kubeconfigs_dir}/kind-config-${managedcluster1}/kubeconfig"

echo "Wait the submariner agents to be deployed by manifestworks on managedcluster ${managedcluster1} ..."
while :
do
  mcounts=$(kubectl -n submariner-operator get pods | wc -l)
  if [ ${mcounts} -ge 7 ]; then
    kubectl -n submariner-operator get pods
    break
  fi
  sleep 2
done
kubectl wait --for=condition=Ready pods -l name=submariner-operator -n submariner-operator --timeout="5m"
kubectl wait --for=condition=Ready pods -l component=submariner-lighthouse -n submariner-operator --timeout="5m"
kubectl wait --for=condition=Ready pods -l app=submariner-engine -n submariner-operator --timeout="5m"
kubectl wait --for=condition=Ready pods -l app=submariner-routeagent -n submariner-operator --timeout="5m"

echo "Wait the managedcluster ${managedcluster2} service nginx.defualt.svc is imported on managedcluster ${managedcluster1} by submariner ..."
while :
do
  icounts=$(kubectl get serviceimports.lighthouse.submariner.io --all-namespaces | wc -l)
  if [ ${icounts} -ge 2 ]; then
    kubectl get serviceimports.lighthouse.submariner.io --all-namespaces
    break
  fi
  sleep 2
done

echo "Install a dnstools to test the imported service ..."
cat <<EOF | kubectl apply -f -
apiVersion: v1
kind: Pod
metadata:
  name: dnstools
  labels:
    app: dnstools
spec:
  containers:
  - name: dnstools
    image: infoblox/dnstools:latest
    command: ["/bin/sleep", "3650d"]
EOF
sleep 2
kubectl wait --for=condition=Ready pods -l app=dnstools -n default --timeout="5m"

echo "Test the service nginx.default.svc.clusterset.local..."
echo "kubectl --kubeconfig ${KUBECONFIG} -n default exec -it dnstools -- curl -v nginx.default.svc.clusterset.local"
sleep 15
kubectl exec -it dnstools -- curl -v nginx.default.svc.clusterset.local
