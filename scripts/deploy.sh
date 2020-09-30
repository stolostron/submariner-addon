#!/bin/bash

source scripts/clusters.sh
k8s_version="v1.18.0"

### functions ###
if [ "$1"x = "cleanup"x ]; then
    for cluster in "${clusters[@]}"; do
        kind delete cluster --name=$cluster
    done
    exit 0
fi

function create_kind_cluster() {
    local idx=$1
    local cluster=$2
    local pod_cidr="10.24${idx}.0.0/16"
    local service_cidr="100.9${idx}.0.0/16"
    local dns_domain="${cluster}.local"
    local kubeconfig_dir="${work_dir}/kubeconfigs/kind-config-${cluster}"

    rm -rf ${kubeconfig_dir}
    mkdir -p ${kubeconfig_dir}

    export KUBECONFIG=${kubeconfig_dir}/kubeconfig
    # create kind cluster
    cat << EOF | kind create cluster --image=kindest/node:${k8s_version} --name=${cluster} --config=-
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
networking:
  disableDefaultCNI: true
  podSubnet: ${pod_cidr}
  serviceSubnet: ${service_cidr}
kubeadmConfigPatches:
- |
  apiVersion: kubeadm.k8s.io/v1beta2
  kind: ClusterConfiguration
  metadata:
    name: config
  networking:
    podSubnet: ${pod_cidr}
    serviceSubnet: ${service_cidr}
    dnsDomain: ${dns_domain}
nodes:
- role: control-plane
- role: worker
EOF

    # fixup kubeconfig
    local master_ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${cluster}-control-plane | head -n 1)
    sed -i -- "s/server: .*/server: https:\/\/$master_ip:6443/g" $KUBECONFIG
    sed -i -- "s/user: kind-.*/user: ${cluster}/g" $KUBECONFIG
    sed -i -- "s/name: kind-.*/name: ${cluster}/g" $KUBECONFIG
    sed -i -- "s/cluster: kind-.*/cluster: ${cluster}/g" $KUBECONFIG
    sed -i -- "s/current-context: .*/current-context: ${cluster}/g" $KUBECONFIG
    chmod a+r $KUBECONFIG
    echo "The Kubeconfig of cluster ${cluster} locates at: ${KUBECONFIG}"

    # apply weave network
    echo "Applying weave network for ${cluster} ..."
    kubectl apply -f "https://cloud.weave.works/k8s/net?k8s-version=${k8s_version}&env.IPALLOC_RANGE=${pod_cidr}"
    echo "Waiting for weave-net pods to be ready for ${cluster} ..."
    kubectl wait --for=condition=Ready pods -l name=weave-net -n kube-system --timeout="5m"
    echo "Waiting for core-dns deployment to be ready for ${cluster} ..."
    kubectl -n kube-system rollout status deploy/coredns --timeout="5m"
}

function deploy_hub() {
    local cluster=$1
    local kubeconfig_dir="${work_dir}/kubeconfigs/kind-config-${cluster}"
    local deploy_dir="${registration_operator_dir}/deploy/cluster-manager"

    if [ ! -d "${kubeconfig_dir}" ]; then
        echo "The cluster ${cluster} Kubeconfig dir does not exist"
        return 2
    fi

    export KUBECONFIG=${kubeconfig_dir}/kubeconfig
    echo "Deploy hub on cluster ${cluster} ..."
    # delploy operator
    kubectl create namespace open-cluster-management
    kubectl apply -f ${deploy_dir}/crds/0000_01_operator.open-cluster-management.io_clustermanagers.crd.yaml
    kubectl apply -f ${deploy_dir}/cluster_role.yaml
    kubectl apply -f ${deploy_dir}/cluster_role_binding.yaml
    kubectl apply -f ${deploy_dir}/service_account.yaml
    kubectl apply -f ${deploy_dir}/operator.yaml

    # delploy cluster managers
    kubectl apply -f ${deploy_dir}/crds/operator_open-cluster-management_clustermanagers.cr.yaml

    # deploy the acm_submariner
    echo "Deploy ACM Submariner on cluster ${cluster} ..."
    local submariner_deploy_dir="${work_dir}/submariner/deploy"
    local submariner_kustomization="${submariner_deploy_dir}/kustomization.yaml"
    local master_ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${hub}-control-plane | head -n 1)

    mkdir -p ${submariner_deploy_dir}
    cp -r deploy/* ${submariner_deploy_dir}

    sed -i -- "s/apiserver=\"10.0.118.46:42415\"/apiserver=\"${master_ip}:6443\"/g" ${submariner_kustomization}
    kubectl apply -k ${submariner_deploy_dir}
}

function deploy_klusterlet() {
    local cluster=$1
    local kubeconfig_dir="${work_dir}/kubeconfigs/kind-config-${cluster}"
    local deploy_dir="${registration_operator_dir}/deploy/klusterlet"
    local hub_kubeconfig="${work_dir}/kubeconfigs/kind-config-${hub}/kubeconfig"
    local master_ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${cluster}-control-plane | head -n 1)

    if [ ! -d "${kubeconfig_dir}" ]; then
        echo "The cluster ${cluster} Kubeconfig dir does not exist"
        return 2
    fi

    export KUBECONFIG=${kubeconfig_dir}/kubeconfig
    echo "Deploy klusterlet on cluster ${cluster} ..."
    # delploy operator
    kubectl create namespace open-cluster-management
    kubectl apply -f ${deploy_dir}/crds/0000_00_operator.open-cluster-management.io_klusterlets.crd.yaml
    kubectl apply -f ${deploy_dir}/cluster_role.yaml
    kubectl apply -f ${deploy_dir}/cluster_role_binding.yaml
    kubectl apply -f ${deploy_dir}/service_account.yaml
    kubectl apply -f ${deploy_dir}/operator.yaml

    # delploy klusterlet
    kubectl create namespace open-cluster-management-agent
    kubectl -n open-cluster-management-agent create secret generic bootstrap-hub-kubeconfig --from-file=${hub_kubeconfig}
    cat << EOF | kubectl apply -f -
apiVersion: operator.open-cluster-management.io/v1
kind: Klusterlet
metadata:
  name: klusterlet
spec:
  registrationImagePullSpec: quay.io/open-cluster-management/registration
  workImagePullSpec: quay.io/open-cluster-management/work
  clusterName: ${cluster}
  namespace: open-cluster-management-agent
  externalServerURLs:
  - url: https://${master_ip}
EOF

    # tag worker node with submariner.io/gateway=true
    kubectl label nodes "${cluster}-worker" "submariner.io/gateway=true" --overwrite
}

function accept_managed_cluster() {
    local cluster=$1
    local hub_kubeconfig="${work_dir}/kubeconfigs/kind-config-${hub}/kubeconfig"

    if [ ! -f "${hub_kubeconfig}" ]; then
        echo "The hub cluster ${hub} Kubeconfig file does not exist"
        return 2
    fi

    echo "Accept managed cluster ${cluster} and approve its csr ..."

    export KUBECONFIG=${hub_kubeconfig}
    # try to find the managed cluster
    times=0
    for i in {0..60};
    do
        kubectl get managedclusters $cluster >/dev/null 2>&1
        if [ 0 -eq $? ]; then
            break
        fi
        sleep 1
        times=$(($times+1))
    done
    if [ $times -ge 60 ]; then
        echo "Unabel to find managed cluster $cluster within 1 min"
        return 2
    fi

    # accept the managed cluster
    kubectl patch managedclusters $cluster --type merge --patch '{"spec":{"hubAcceptsClient":true}}'

    # approve the managed cluster csr
    csr_name=$(kubectl get csr |grep "${cluster}" | grep "Pending" |awk '{print $1}')
    kubectl certificate approve "${csr_name}"

    echo "Wait managed clusters are joined ..."
    while :
    do
        jcounts=$(kubectl get managedclusters ${cluster} | awk '{print $4}' | grep "True" | wc -l)
        if [ ${jcounts} -eq 1 ]; then
            kubectl get managedclusters ${cluster}
            break
        fi
        sleep 1
    done
}

### main ###
work_dir="$(pwd)/_output"
rm -rf ${work_dir}
mkdir -p ${work_dir}

# prepare clusters
i=1
for cluster in "${clusters[@]}";
do
    (create_kind_cluster "$i" "$cluster") &
    i=$(($i+1))
done
wait

# downlaod registration-operator
registration_operator_dir="${work_dir}/registration-operator"
git clone --depth 1 --branch release-2.1 https://github.com/open-cluster-management/registration-operator.git ${registration_operator_dir}

# the first cluster is hub cluster
hub="${clusters[0]}"

# load acm-submariner image from local
kind load --name="${hub}" docker-image quay.io/open-cluster-management/acm-submariner:latest

# deploy hub
deploy_hub ${hub}

# deploy klusterlet on managed cluters
for ((i=1;i<${#clusters[*]};i++));
do
    (deploy_klusterlet ${clusters[$i]}) &
done
wait

# accept managed clusters
for ((i=1;i<${#clusters[*]};i++));
do
    (accept_managed_cluster ${clusters[$i]}) &
done
wait
