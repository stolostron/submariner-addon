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

> Note: minimum supported version is OpenShift 4.4/Kuberenets 1.17

The steps below can be used to test with OpenShift Container Platform (OCP) clusters on AWS:

### Setup of Cluster Manager and Klusterlet

1. Prepare 3 OCP clusters (1 Hub cluster and 2 managed clusters) on AWS for Submariner. Please refer to [this section](https://submariner.io/getting_started/quickstart/openshift/aws/#prepare-aws-clusters-for-submariner) for detailed instructions.

2. On the Hub cluster, install `Cluster Manager` Operator and instance (version >= 0.2.0) from OperatorHub.

3. On the managed clusters, install `Klusterlet` Operator and instance (version >= 0.2.0) from OperatorHub.

4. Approve the `ManagedClusters` on the hub cluster.

    ```
    $ oc get managedclusters
    $ oc get csr | grep <managedcluster name> | grep Pending
    $ oc certificate approve <managedcluster csr>
    ```

5. Accept the `ManagedClusters` on the Hub cluster.

   ```
   $ oc patch managedclusters <managedcluster name> --type merge --patch '{"spec":{"hubAcceptsClient":true}}'
   ```

### Setup the Add-on on the Hub cluster

1. Apply the manifests of submariner-addon.

    ```
    $ oc apply -k deploy/config/manifests
    ```

### Setup Submariner on the Hub cluster

1. Create a `ManagedClusterSet`.

   ```
   kind: ManagedClusterSet
   metadata:
     name: pro
   ```

2. Enable the `Submariner` for the `ManagedClusters`.

   ```
   $ oc label managedclusters <managedcluster name> "cluster.open-cluster-management.io/submariner-agent=true" --overwrite
   ```

3. Join the `ManagedClusters` into the `ManagedClusterSet`.

   ```
   $ oc label managedclusters <managedcluster name> "cluster.open-cluster-management.io/clusterset=pro" --overwrite
   ```

## Test with ACM

The add-on has been integrated into ACM 2.2 as a default component:

1. Install ACM following the [`deploy`](https://github.com/open-cluster-management/deploy) repo.

2. Import or create OCP clusters as managed cluster through the ACM console UI.
    >Note: The manged clusters must meet the [`prerequisites`](https://submariner.io/getting_started/#prerequisites) for Submariner.

3. Start deploying Submariner to managed clusters following the [Setup of Submariner on the Hub cluster](#setup-of-submariner-on-the-hub-cluster) above.
