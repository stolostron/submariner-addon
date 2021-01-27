# Submariner With ACM

This is a Tech Preview feature.

`Submariner` is a tool built to connect overlay networks of different Kubernetes clusters. 
Submariner enables direct networking between Pods and Services in different Kubernetes clusters, either on-premises or in the cloud. 
See [Submariner](https://submariner.io/) for more details.

We integrate `Submariner` into ACM with the concepts of `ManagedCluster` and `ManagedClusterSet`. 
The Pods and Services in the manged clusters of a `ManagedClusterSet` can connect with each other according the `Submariner`.

## Apply Submariner on the manged clusters 

### Prerequisites

There are some prerequisites for the managed clusters which are going to run `Submariner`. See [Prerequisites](prerequisites.md) for more details.

### Setup Submariner on the Hub cluster

1. Create a `ManagedClusterSet`.

   ```yaml
   apiVersion: cluster.open-cluster-management.io/v1alpha1
   kind: ManagedClusterSet
   metadata:
     name: <mangedClusterSet-name>
   ```

2. Enable the `Submariner` for the `ManagedClusters`.

   ```
   $ oc label managedclusters <managedcluster-name> "cluster.open-cluster-management.io/submariner-agent=true" --overwrite
   ```

3. Join the `ManagedClusters` into the `ManagedClusterSet`.
    
   ```
   $ oc label managedclusters <managedcluster-name> "cluster.open-cluster-management.io/clusterset=<mangedClusterSet-name>" --overwrite
   ```

### Verify the Submariner with Service Discovery 

We use `nginx` service as example to verify the Submariner with service discovery.
See [Install Submariner with Service Discovery](https://submariner.io/getting-started/quickstart/openshift/aws/#install-submariner-with-service-discovery) for more examples.

1. Apply a `nginx` service in one of manged Cluster. 

   ```bash
   $ oc -n default create deployment nginx --image=nginxinc/nginx-unprivileged:stable-alpine
   $ oc -n default expose deployment nginx --port=8080
   ```
   
2. Export the `nginx` service.
   
   ```yaml
    apiVersion: multicluster.x-k8s.io/v1alpha1
    kind: ServiceExport
    metadata:
      name: nginx
      namespace: default
   ```

3. Run `nettest` from another managed cluster to access the `nginx` service.

   ```bash
   $ oc -n default  run --generator=run-pod/v1 tmp-shell --rm -i --tty --image quay.io/submariner/nettest -- /bin/bash
    curl nginx.default.svc.clusterset.local:8080
   ```
