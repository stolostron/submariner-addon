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
    - Create two clusters: `cluster1` and `cluster2`. `cluster1` is going to be used as the Hub.
    - Load the local Docker images to the kind cluster `cluster1`.
    - Deploy the [operator-lifecycle-manager](https://github.com/operator-framework/operator-lifecycle-manager)
    - Deploy the `ClusterManager` and `submariner-addon` on `cluster1`. This includes the required Hub cluster components.
    - Deploy the `Klusterlet` on `cluster1` and `cluster2`. This includes the required the managed cluster agents.
    - Join `cluster1` and `cluster2` to the Hub cluster `cluster1`, the `cluster1` and `cluster2` are the managed clusters.

4. Run the demo by issuing `make demo`. This will:
    - Label the managed clusters with `cluster.open-cluster-management.io/clusterset: clusterset1`.
    - Create a `ClusterSet`.
    - Create `ManagedClusterAddon` on each managed cluster namespaces.
    - Deploy the Submariner Broker on the Hub cluster and the required Submariner components on the managed clusters.
    - Interconnect `cluster1` and `cluster2` using Submariner.

To delete the kind environment, use `make clean`.

## Test with OCP

> Note: minimum supported version is OpenShift 4.4/Kubernetes 1.17

The steps below can be used to test with OpenShift Container Platform (OCP) clusters on AWS:

### Setup of Cluster Manager and Klusterlet

1. Prepare 3 OCP clusters (1 Hub cluster and 2 managed clusters) on AWS for Submariner. Please refer to [this section](https://submariner.io/getting-started/quickstart/openshift/aws/) for detailed instructions.

2. On the Hub cluster, install `Cluster Manager` Operator and instance (version >= 0.2.0) from OperatorHub.

3. On the managed clusters, install `Klusterlet` Operator and instance (version >= 0.2.0) from OperatorHub.

4. Approve the `ManagedClusters` on the hub cluster.

    ```
    $ oc get managedclusters
    $ oc get csr | grep <managedcluster name> | grep Pending
    $ oc adm certificate approve <managedcluster csr>
    ```

5. Accept the `ManagedClusters` on the Hub cluster.

   ```
   $ oc patch managedclusters <managedcluster name> --type merge --patch '{"spec":{"hubAcceptsClient":true}}'
   ```

### Install the Submariner-addon on the Hub cluster

1. Apply the manifests of submariner-addon.

    ```
    $ oc apply -k deploy/config/manifests
    ```

### Setup Submariner on the Hub cluster

1. Create a `ManagedClusterSet`.

   ```
   apiVersion: cluster.open-cluster-management.io/v1beta1
   kind: ManagedClusterSet
   metadata:
     name: pro
   ```

2. Join the `ManagedClusters` into the `ManagedClusterSet`.

   ```
   $ oc label managedclusters <managedcluster name> "cluster.open-cluster-management.io/clusterset=pro" --overwrite
   ```

3. Create a `ManagedClusterAddon` in the managed cluster namespace to deploy the Submariner on the managed cluster.

   ```
   apiVersion: addon.open-cluster-management.io/v1alpha1
   kind: ManagedClusterAddOn
   metadata:
     name: submariner
     namespace: <managedcluster name>
   spec:
     installNamespace: submariner-operator
   ```

   > Note: the name of `ManagedClusterAddOn` must be `submariner`

## Test with ACM

The add-on has been integrated into ACM 2.2 as a default component:

1. Install ACM following the [`deploy`](https://github.com/stolostron/deploy) repo.

2. Import or create OCP clusters as managed cluster through the ACM console UI.
   
   > Note: The manged clusters must meet the [`Prerequisites`](/doc/prerequisites.md) for Submariner.

3. To test an in-development version of the addon, build an image and push it to one of your repositories
   on Quay, then edit the `ClusterServiceVersion` resource:

   ```
   kubectl edit ClusterServiceVersion -n open-cluster-management
   ```

   to replace all instances of the addon with your image tag.

   You can find the appropriate digest on Quay by clicking on the “download” button and choosing
   “Docker Pull (by digest)”.

4. Start deploying Submariner to managed clusters following the [Setup of Submariner on the Hub cluster](#setup-of-submariner-on-the-hub-cluster) above.

To use a different version of Submariner itself, edit `submariner.io-submariners-cr.yaml` and rebuild your image.
