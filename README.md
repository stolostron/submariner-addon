# acm-submariner
An integration between acm and submariner

## Locally Testing With KIND

Below steps can be used to run this repo at a local environment

> Note: [`kind`](https://kind.sigs.k8s.io/) and [`kubectl`](https://kubernetes.io/docs/tasks/tools/install-kubectl/) are required

1. Build the `acm-submariner` image on local by `make images`
2. Prepare clusters by `make clusters`, this will
    - Create three clusters: `cluster1`, `clustr2` and `cluster3`. `cluster1` is the hub cluster, and the others are the managed clusters.
    - Load the local docker images to the kind cluster `cluster1`.
    - Deploy the `ClusterManager` on hub cluster `cluster1` to deploy hub cluster components.
    - Deploy the `Klusterlet` on `cluster2` and `cluster3` to deploy managed cluster agents.
    - Make the `cluster2` and `cluster3` join to the hub cluster `cluster1`.
3. Run the demo by `make demo`, this will
    - Label the managed cluster with `cluster.open-cluster-management.io/submariner-agent`.
    - Label the managed cluster with `cluster.open-cluster-management.io/clusterset: clusterset1`.
    - Create a `ClusterSet`, and the `acm-submariner` controller will deploy the submariner broker on the
      hub cluster and deploy the submariner agents on the managed clusters by `ManifestWorks`.
    - Create a service on managed cluster `cluster3` and export it. After the service is created, the submariner will import this service to the managed clusters.
    - Access the exported service on managed cluster `cluster2`.
