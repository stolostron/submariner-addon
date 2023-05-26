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

package rhos

var machineSetYAML = `apiVersion: machine.openshift.io/v1beta1
kind: MachineSet
metadata:
  labels:
    machine.openshift.io/cluster-api-cluster: {{.InfraID}}
    machine.openshift.io/cluster-api-machine-role: worker
    machine.openshift.io/cluster-api-machine-type: worker
  name: {{.InfraID}}-submariner-gw-{{.UUID}}
  namespace: openshift-machine-api
spec:
  replicas: 1
  selector:
    matchLabels:
      machine.openshift.io/cluster-api-cluster: {{.InfraID}}
      machine.openshift.io/cluster-api-machineset: {{.InfraID}}-submariner-gw-{{.UUID}}
  template:
    metadata:
      labels:
        machine.openshift.io/cluster-api-cluster: {{.InfraID}}
        machine.openshift.io/cluster-api-machine-role: worker
        machine.openshift.io/cluster-api-machine-type: worker
        machine.openshift.io/cluster-api-machineset: {{.InfraID}}-submariner-gw-{{.UUID}}
    spec:
      metadata:
        labels:
          submariner.io/gateway: "true"
      taints:
        - effect: NoSchedule
          key: node-role.submariner.io/gateway
      providerSpec:
        value:
          apiVersion: openstackproviderconfig.openshift.io/v1alpha1
          cloudName: {{.CloudName}}
          cloudsSecret:
            name: openstack-cloud-credentials
          flavor:  {{.InstanceType}}
          image: {{.Image}} 
          kind: OpenstackProviderSpec
          metadata:
            creationTimestamp: null
          networks:
          - filter: {}
            subnets:
            - filter:
                name: {{.InfraID}}-nodes
                tags: openshiftClusterID={{.InfraID}}
          securityGroups:
          - name: {{.InfraID}}-worker
          {{- if .UseSubmarinerInternalSG }}
          - name: {{.InfraID}}-submariner-internal-sg
          {{- end }}
          - name: {{.InfraID}}-submariner-gw-sg
          serverMetadata:
            Name: {{.InfraID}}-worker
            openshiftClusterID: {{.InfraID}}
          tags:
          - openshiftClusterID={{.InfraID}}
          - {{.SubmarinerGWNodeTag}}
          trunk: true
          userDataSecret:
            name: worker-user-data`
