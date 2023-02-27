/*
SPDX-License-Identifier: Apache-2.0

Copyright Contributors to the Submariner project.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package azure

var machineSetYAML = `apiVersion: machine.openshift.io/v1beta1
kind: MachineSet
metadata:
  labels:
    machine.openshift.io/cluster-api-cluster: {{.InfraID}}
    machine.openshift.io/cluster-api-machine-role: worker
    machine.openshift.io/cluster-api-machine-type: worker
  name: {{.Name}}
  namespace: openshift-machine-api
spec:
  replicas: 1
  selector:
    matchLabels:
      machine.openshift.io/cluster-api-cluster: {{.InfraID}}
      machine.openshift.io/cluster-api-machineset: {{.Name}}
  template:
    metadata:
      labels:
        machine.openshift.io/cluster-api-cluster: {{.InfraID}}
        machine.openshift.io/cluster-api-machine-role: worker
        machine.openshift.io/cluster-api-machine-type: worker
        machine.openshift.io/cluster-api-machineset: {{.Name}}
    spec:
      metadata:
        labels:
          submariner.io/gateway: "true"
      taints:
        - effect: NoSchedule
          key: node-role.submariner.io/gateway
      providerSpec:
        value:
          apiVersion: azureproviderconfig.openshift.io/v1beta1
          credentialsSecret:
            name: azure-cloud-credentials
            namespace: openshift-machine-api
          image:
            offer: ""
            publisher: ""
            resourceID: {{.Image}} 
            sku: ""
            version: ""
          internalLoadBalancer: ""
          kind: AzureMachineProviderSpec
          location: {{.Region}}
          managedIdentity: {{.InfraID}}-identity
          natRule: null
          networkResourceGroup: {{.InfraID}}-rg
          osDisk:
            diskSizeGB: 128
            managedDisk:
              storageAccountType: Premium_LRS
            osType: Linux
          publicIP: true
          publicLoadBalancer: ""
          resourceGroup: {{.InfraID}}-rg
          sshPrivateKey: ""
          sshPublicKey: ""
          subnet: {{.InfraID}}-worker-subnet
          securityGroup: {{.InfraID}}-submariner-external-sg
          userDataSecret:
            name: worker-user-data
          vmSize: {{.InstanceType}}
          vnet: {{.InfraID}}-vnet
          zone: {{.AZ}}`
