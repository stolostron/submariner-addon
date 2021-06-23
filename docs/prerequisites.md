# Prerequisites

Submariner with ACM has a few requirements to get started:

1. ACM only supports Submariner running on the OCP clusters.
2. The minimum supported version is OpenShift 4.4/Kubernetes 1.17.
3. ACM only supports non-overlapping Pod and Service CIDRs between managed clusters using Submariner to connect workloads across each other at the current stage.
4. IP reachability between the gateway nodes. When connecting two clusters, at least one of the clusters should have a publicly routable IP address designated to the Gateway node.
5. Ensure that firewall configuration allows 4800/UDP across all nodes in the managed cluster in both directions.
6. Ensure that firewall configuration allows ingress 8080/TCP on the Gateway nodes so that other nodes in the cluster can access it.

Refer to [Prerequisites of Submariner](https://submariner.io/getting-started/#prerequisites) for the detailed prerequisites.

# The  before running Submariner

We have verified the Submariner with ACM on the OCP clusters host on AWS, GCP, Azure, IBM Cloud, VMware vSphere, Bare Metal and OSD.

In order to meet the prerequisites, we need to complete the following configurations for each Cloud Platform.

## AWS

Use `SubmarinerConfig` API to build the cluster environment. See [SubmarinerConfig](submarinerConfig.md) for more details.

## GCP

Use `SubmarinerConfig` API to build the cluster environment. See [SubmarinerConfig](submarinerConfig.md) for more details.

## Azure

1. Create two load balancing inbound NAT rules to forward IPsec IKE (by default 500/UDP) and NAT traversal (by default 4500/UDP) request for Submariner.

    ```bash
    # create inbound nat rule
    $ az network lb inbound-nat-rule create --lb-name <lb-name> \
    --resource-group <res-group> \
    --name <name> \
    --protocol Udp --frontend-port <ipsec-port> \
    --backend-port <ipsec-port> \
    --frontend-ip-name <frontend-ip-name>

    # add your vm network interface to the created inbound nat rule
    $ az network nic ip-config inbound-nat-rule add \
    --lb-name <lb-name> --resource-group <res-group> \
    --inbound-nat-rule <name> \
    --nic-name <nic-name> --ip-config-name pipConfig
    ```
   > Replace <lb-name> with your load balancer name.  
   > Replace <res-group> with your resource group name.  
   > Replace <name> with your load balancing inbound NAT rule name.  
   > Replace <ipsec-port> with your IPsec port.  
   > Replace <frontend-ip-name> with your cluster frontend IP configuration name.  
   > Replace <nic-name> with your network interface (NIC).

2. Create one load balancing inbound NAT rules to forward Submariner gateway metrics service request.

    ```bash
    # create inbound nat rule
    $ az network lb inbound-nat-rule create --lb-name <lb-name> \
    --resource-group <res-group> \
    --name <name> \
    --protocol Tcp --frontend-port 8080 --backend-port 8080 \
    --frontend-ip-name <frontend-ip-name>

    # add your vm network interface to the created inbound nat rule
    $ az network nic ip-config inbound-nat-rule add \
    --lb-name <lb-name> --resource-group <res-group> \
    --inbound-nat-rule <name> \
    --nic-name <nic-name> --ip-config-name pipConfig
    ```
   > Replace <lb-name> with your load balancer name.  
   > Replace <res-group> with your resource group name.  
   > Replace <name> with your load balancing inbound NAT rule name.  
   > Replace <frontend-ip-name> with your cluster frontend IP configuration name.  
   > Replace <nic-name> with your network interface (NIC).

3. Create NSG (network security groups) security rules on your Azure to open IPsec IKE (by default 500/UDP) and NAT traversal ports (by default 4500/UDP) for Submariner.

    ```bash
    $ az network nsg rule create --resource-group <res-group> \
    --nsg-name <nsg-name> --priority <priority> \
    --name <name> --direction Inbound --access Allow \
    --protocol Udp --destination-port-ranges <ipsec-port>

    $ az network nsg rule create --resource-group <res-group> \
    --nsg-name <nsg-name> --priority <priority> \
    --name <name> --direction Outbound --access Allow \
    --protocol Udp --destination-port-ranges <ipsec-port>
    ```
   > Replace <res-group> with your resource group name.  
   > Replace <nsg-name> with your NSG name.  
   > Replace <priority> with your rule priority.  
   > Replace <name> with your rule name.  
   > Replace <ipsec-port> with your IPsec port.

4. Create the NSG rules to open 4800/UDP port to encapsulate Pod traffic from the worker and master nodes to the Submariner Gateway nodes.

    ```bash
    $ az network nsg rule create --resource-group <res-group> \
    --nsg-name <nsg-name> --priority <priority> \
    --name <name> --direction Inbound --access Allow \
    --protocol Udp --destination-port-ranges 4800 \

    $ az network nsg rule create --resource-group <res-group> \
    --nsg-name <nsg-name> --priority <priority> \
    --name <name> --direction Outbound --access Allow \
    --protocol Udp --destination-port-ranges 4800
    ```
   > Replace <res-group> with your resource group name.  
   > Replace <nsg-name> with your NSG name.  
   > Replace <priority> with your rule priority.  
   > Replace <name> with your rule name.

5. Create the NSG rules to open 8080/TCP port to export metrics service from the Submariner gateway.

    ```bash
    $ az network nsg rule create --resource-group <res-group> \
    --nsg-name <nsg-name> --priority <priority> \
    --name <name> --direction Inbound --access Allow \
    --protocol Tcp --destination-port-ranges 8080 \

    $ az network nsg rule create --resource-group <res-group> \
    --nsg-name <nsg-name> --priority <priority> \
    --name <name> --direction Outbound --access Allow \
    --protocol Udp --destination-port-ranges 8080
    ```
   > Replace <res-group> with your resource group name.  
   > Replace <nsg-name> with your NSG name.  
   > Replace <priority> with your rule priority.  
   > Replace <name> with your rule name.

6. Label your worker node with the `submariner.io/gateway=true` in your cluster 
   ```
   kubectl label nodes <worker-node-name> "submariner.io/gateway=true" --overwrite
   ```
   > Replace <worker-node-name> with your worker node name.

## IBM Cloud

There are 2 kinds Red Hat OpenShift on IBM Cloud (ROKS), the Classic Cluster and the second generation of compute infrastructure in a Virtual Private Cloud (VPC).

> Note: Submariner can not run on the classic ROKS cluster since cannot configure the IPSec ports for the classic cluster.

The configurations below are for the clusters on VPC.
1. Please refer to [VPC Subnets](https://cloud.ibm.com/docs/openshift?topic=openshift-vpc-subnets#vpc_basics) to specify subnets for Pods and Services to avoid overlapping CIDRs with other clusters before creating a cluster. Make sure there are no overlapping Pods and Services CIDRs between clusters if using an existing cluster.
2. Please refer to [Public Gateway](https://cloud.ibm.com/docs/openshift?topic=openshift-vpc-subnets#vpc_basics_pgw) to attach a public gateway to subnets used in the cluster.
3. Please refer to [Security Group](https://cloud.ibm.com/docs/openshift?topic=openshift-vpc-network-policy#security_groups_ui) to create inbound rules for the default security group of the cluster. Ensure that firewall allows inbound/outbound UDP/4500 and UDP/500 ports for Gateway nodes, and allows inbound/outbound 4800/UDP for all the other nodes.
4. Label a node which has the public gateway with “submariner.io/gateway=true” in the cluster.
5. Please refer to [Calico](https://submariner.io/operations/deployment/calico/) to configure Calico CNI by creating IPPools in the cluster.


## OSD

OSD (RedHat OpenShift Dedicated) supports 2 provisioners AWS and Google Cloud Platform.

### AWS cluster:

1. The default group `dedicated-admin` has no permission to create `MachineSet`, please grant `cluster-admin` group for OSD cluster from OpenShift Hosted SRE Support by [ticket](https://issues.redhat.com/secure/CreateIssue!default.jspa).
2. Please refer to the [steps](https://docs.openshift.com/dedicated/4/administering_a_cluster/cluster-admin-role.html) to join the user into the `cluster-admin` group.
3. Please refer to the [AWS section](#aws) for the prerequisites configurations using the credentials of the user `osdCcsAdmin`.

### GCP cluster:

1. Please refer to the [GCP section](#gcp) for the prerequisites configurations using the credentials of the Service Account `osd-ccs-admin`.

## VMware vSphere

1. At least one of the clusters should have a publicly routable IP address designated to the Gateway node.
2. The default ports used by IPsec are 4500/UDP and 500/UDP. If firewalls that block the default ports, should set custom non-standard ports like 4501/UDP and 501/UDP. See [SubmarinerConfig](submarinerConfig.md) for more details.
