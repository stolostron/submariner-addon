# submariner-addon
An integration between ACM and [Submariner](https://submariner.io/). Submariner enables direct networking between Pods and Services in different Kubernetes clusters.

## Community, discussion, contribution, and support
Check the [CONTRIBUTING Doc](CONTRIBUTING.md) for how to contribute to the repo.

## Test Locally with kind
The steps below can be used for testing on a local environment:

> Note: [`kind`](https://kind.sigs.k8s.io/), [`kubectl`](https://kubernetes.io/docs/tasks/tools/install-kubectl/), and [`imagebuilder`](https://github.com/openshift/imagebuilder) are required.

1. Clone this repository by using `git clone`.

2. Build the `submariner-addon` image locally by running `make images`.

3. Prepare clusters by running `make clusters`. This will:
    - Create three clusters: `cluster1`, `cluster2` and `cluster3`. `cluster1` is going to be used as the Hub, and the other two as the managed clusters.
    - Load the local Docker images to the kind cluster `cluster1`.
    - Deploy the `ClusterManager` on `cluster1`. This includes the required Hub cluster components.
    - Deploy the `Klusterlet` on `cluster2` and `cluster3`. This includes the required managed cluster agents.
    - Join `cluster2` and `cluster3` to the Hub cluster `cluster1`.

4. Run the demo by issuing `make demo`. This will:
    - Label the managed clusters with `cluster.open-cluster-management.io/submariner-agent`.
    - Label the managed clusters with `cluster.open-cluster-management.io/clusterset: clusterset1`.
    - Create a `ClusterSet`.
    - Deploy the Submariner Broker on the Hub cluster and the required Submariner components on the managed clusters.
    - Interconnect `cluster2` and `cluster3` using Submariner.
    - Create a Kubernetes Service `nginx` of type ClusterIP on managed cluster `cluster3` and export it. Submariner will import this Service to the managed clusters.
    - Access the exported Service from managed cluster `cluster2`.

To delete the kind environment, use `make clean`.

## Test with OCP
The steps below can be used to test with OpenShift Container Platform (OCP) clusters on AWS:

1. Prepare AWS clusters for Submariner. Please refer to [this section](https://submariner.io/quickstart/openshift/#prepare-aws-clusters-for-submariner) for detailed instructions.

2. Apply the deploy:
    ```
    kubectl apply -k deploy/config/manifests
    ```
