#!/bin/bash

source scripts/clusters.sh
k8s_version="v1.20.2"

if [ "$1"x = "cleanup"x ]; then
    for cluster in "${clusters[@]}"; do
        kind delete cluster --name=$cluster
    done
    exit 0
fi

### functions ###
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

    kubectl taint nodes --all node-role.kubernetes.io/master-
}

function deploy_olm() {
    local cluster=$1
    local kubeconfig_dir="${work_dir}/kubeconfigs/kind-config-${cluster}"

    export KUBECONFIG=${kubeconfig_dir}/kubeconfig

    kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/v0.17.0/crds.yaml
    kubectl apply -f https://github.com/operator-framework/operator-lifecycle-manager/releases/download/v0.17.0/olm.yaml
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
    # deploy operator
    kubectl apply -k ${deploy_dir}/config/manifests

    # deploy cluster managers
    kubectl apply -k ${deploy_dir}/config/samples

    # deploy the acm_submariner
    echo "Deploy ACM submariner-addon on cluster ${cluster} ..."
    local submariner_deploy_dir="${work_dir}/submariner/deploy"
    local submariner_deployment="${submariner_deploy_dir}/config/operator/operator.yaml"
    local master_ip=$(docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' ${hub}-control-plane | head -n 1)

    mkdir -p ${submariner_deploy_dir}
    cp -r deploy/* ${submariner_deploy_dir}

    # Set submariner-addon imagePullPolicy to IfNotPresent
    sed -i -- '/image: quay.io*/a\        imagePullPolicy: IfNotPresent' ${submariner_deploy_dir}/config/operator/operator.yaml


    # add master apiserver to submariner-addon deployment
    cat <<EOF >> ${submariner_deployment}
          - name: BROKER_API_SERVER
            value: "${master_ip}:6443"
EOF
    kubectl apply -k ${submariner_deploy_dir}/config/manifests
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
    # deploy operator
    kubectl apply -k ${deploy_dir}/config/manifests

    # deploy klusterlet
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
    for i in {0..120};
    do
        kubectl get managedclusters $cluster >/dev/null 2>&1
        if [ 0 -eq $? ]; then
            break
        fi
        sleep 2
        times=$(($times+1))
    done
    if [ $times -ge 120 ]; then
        echo "Unable to find managed cluster $cluster within 4 min"
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

    # apply SubmarinerConfig to customize subscription
    echo "apply SubmarinerConfig to cluster ${cluster}"
    cat << EOF | kubectl apply -f -
apiVersion: submarineraddon.open-cluster-management.io/v1alpha1
kind: SubmarinerConfig
metadata:
  name: submariner
  namespace: ${cluster}
spec:
  subscriptionConfig:
    source: operatorhubio-catalog
    sourceNamespace: olm
EOF

    submrepo="quay.io/submariner"
    submver=0.13.4
    kubectl patch submarinerconfigs submariner -n ${cluster} --type "json" -p '[
{"op":"add","path":"/spec/imagePullSpecs/submarinerImagePullSpec","value":"'${submrepo}'/submariner-gateway:'${submver}'"},
{"op":"add","path":"/spec/imagePullSpecs/submarinerRouteAgentImagePullSpec","value":"'${submrepo}'/submariner-route-agent:'${submver}'"},
{"op":"add","path":"/spec/imagePullSpecs/lighthouseAgentImagePullSpec","value":"'${submrepo}'/lighthouse-agent:'${submver}'"},
{"op":"add","path":"/spec/imagePullSpecs/lighthouseCoreDNSImagePullSpec","value":"'${submrepo}'/lighthouse-coredns:'${submver}'"},
{"op":"add","path":"/spec/imagePullSpecs/submarinerGlobalnetImagePullSpec","value":"'${submrepo}'/submariner-globalnet:'${submver}'"},
{"op":"add","path":"/spec/imagePullSpecs/submarinerNetworkPluginSyncerImagePullSpec","value":"'${submrepo}'/submariner-networkplugin-syncer:'${submver}'"},
{"op":"add","path":"/spec/imagePullSpecs/metricsProxyImagePullSpec","value":"'${submrepo}'/nettest:'${submver}'"},
{"op":"add","path":"/spec/imagePullSpecs/nettestImagePullSpec","value":"'${submrepo}'/nettest:'${submver}'"},
{"op":"add","path":"/spec/NATTEnable","value":false}]'

}

function validate_kubectl_version() {
    majorVersion=$(kubectl version --short --client | cut -d: -f2 | cut -d. -f1)
    minorVersion=$(kubectl version --short --client | cut -d: -f2 | cut -d. -f2)
    if [ $majorVersion != "v1" ] || [ $minorVersion -lt 19 ]
    then
	    echo "Please update your kubectl version to v1.19 (or above)"
	    exit 1
    fi
}

validate_kubectl_version

### main ###
work_dir="$(pwd)/_output"
rm -rf ${work_dir}
mkdir -p ${work_dir}


# prepare clusters
i=1
for cluster in "${clusters[@]}";
do
    create_kind_cluster "$i" "$cluster"
    i=$(($i+1))
done

# deploy the olm
for cluster in "${clusters[@]}";
do
    deploy_olm "$cluster"
done

# download registration-operator
registration_operator_dir="${work_dir}/registration-operator"
git clone --depth 1 https://github.com/stolostron/registration-operator.git ${registration_operator_dir}

# the first cluster is hub cluster
hub="${clusters[0]}"

# load submariner-addon image from local
kind load --name="${hub}" docker-image quay.io/stolostron/submariner-addon:latest

# deploy hub
deploy_hub ${hub}

# deploy klusterlet on managed cluters
for ((i=0;i<${#clusters[*]};i++));
do
    (deploy_klusterlet ${clusters[$i]}) &
done
wait

# accept managed clusters
for ((i=0;i<${#clusters[*]};i++));
do
    (accept_managed_cluster ${clusters[$i]}) &
done
wait
