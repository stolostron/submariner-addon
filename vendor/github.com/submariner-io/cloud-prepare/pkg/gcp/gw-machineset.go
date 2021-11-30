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
package gcp

var machineSetYAML = `apiVersion: machine.openshift.io/v1beta1
kind: MachineSet
metadata:
  labels:
    machine.openshift.io/cluster-api-cluster: {{.InfraID}}
  name: {{.InfraID}}-submariner-gw-{{.AZ}}
  namespace: openshift-machine-api
spec:
  replicas: 1
  selector:
    matchLabels:
      machine.openshift.io/cluster-api-cluster: {{.InfraID}}
      machine.openshift.io/cluster-api-machineset: {{.InfraID}}-submariner-gw-{{.AZ}}
  template:
    metadata:
      labels:
        machine.openshift.io/cluster-api-cluster: {{.InfraID}}
        machine.openshift.io/cluster-api-machine-role: worker
        machine.openshift.io/cluster-api-machine-type: worker
        machine.openshift.io/cluster-api-machineset: {{.InfraID}}-submariner-gw-{{.AZ}}
    spec:
      metadata:
        labels:
          submariner.io/gateway: "true"
      taints:
        - effect: NoSchedule
          key: node-role.submariner.io/gateway
      providerSpec:
        value:
          apiVersion: gcpprovider.openshift.io/v1beta1
          canIPForward: true
          credentialsSecret:
            name: gcp-cloud-credentials
          deletionProtection: false
          disks:
          - autoDelete: true
            boot: true
            image: {{.Image}} 
            labels: null
            sizeGb: 128
            type: pd-ssd
          kind: GCPMachineProviderSpec
          machineType: {{.InstanceType}}
          metadata:
          networkInterfaces:
          - network: {{.InfraID}}-network
            subnetwork: {{.InfraID}}-worker-subnet
            publicIP: true
          projectID: {{.ProjectID}}
          region: {{.Region}}
          serviceAccounts:
          - email: {{.InfraID}}-w@{{.ProjectID}}.iam.gserviceaccount.com
            scopes:
            - https://www.googleapis.com/auth/cloud-platform
          tags:
          - {{.InfraID}}-worker
          - {{.SubmarinerGWNodeTag}}
          userDataSecret:
            name: worker-user-data
          zone: {{.AZ}}`
