/*
© 2021 Red Hat, Inc. and others.

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
package aws

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
      creationTimestamp: null
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
          ami:
            id: {{.AMIId}}
          apiVersion: awsproviderconfig.openshift.io/v1beta1
          credentialsSecret:
            name: aws-cloud-credentials
          deviceIndex: 0
          iamInstanceProfile:
            id: {{.InfraID}}-worker-profile
          instanceType: {{.InstanceType}}
          kind: AWSMachineProviderConfig
          placement:
            availabilityZone: {{.AZ}}
            region: {{.Region}}
          securityGroups:
            - filters:
                - name: tag:Name
                  values:
                    - {{.InfraID}}-worker-sg
                    - {{.SecurityGroup}}
          subnet:
            filters:
              - name: tag:Name
                values:
                  - {{.PublicSubnet}}
          tags:
            - name: kubernetes.io/cluster/{{.InfraID}}
              value: owned
            - name: submariner.io
              value: gateway
          userDataSecret:
            name: worker-user-data
          publicIp: true`
