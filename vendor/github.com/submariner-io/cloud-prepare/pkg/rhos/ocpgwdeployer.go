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

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/pkg/errors"
	"github.com/submariner-io/cloud-prepare/pkg/api"
	"github.com/submariner-io/cloud-prepare/pkg/ocp"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
)

type ocpGatewayDeployer struct {
	CloudInfo
	projectID       string
	instanceType    string
	image           string
	cloudName       string
	dedicatedGWNode bool
	msDeployer      ocp.MachineSetDeployer
}

// NewOcpGatewayDeployer returns a GatewayDeployer capable of deploying gateways using OCP.
func NewOcpGatewayDeployer(info CloudInfo, msDeployer ocp.MachineSetDeployer, projectID, instanceType, image, cloudName string,
	dedicatedGWNode bool) api.GatewayDeployer {
	return &ocpGatewayDeployer{
		CloudInfo:       info,
		projectID:       projectID,
		instanceType:    instanceType,
		image:           image,
		cloudName:       cloudName,
		dedicatedGWNode: dedicatedGWNode,
		msDeployer:      msDeployer,
	}
}

type machineSetConfig struct {
	Index               string
	InfraID             string
	ProjectID           string
	InstanceType        string
	Region              string
	Image               string
	SubmarinerGWNodeTag string
	CloudName           string
}

func (d *ocpGatewayDeployer) loadGatewayYAML(index, image string) ([]byte, error) {
	var buf bytes.Buffer

	// TODO: Not working properly, but we should revisit this as it makes more sense
	// tpl, err := template.ParseFiles("pkg/aws/gw-machineset.yaml.template")
	tpl, err := template.New("").Parse(machineSetYAML)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create machine set template")
	}

	tplVars := machineSetConfig{
		Index:               index,
		InfraID:             d.InfraID,
		ProjectID:           d.projectID,
		InstanceType:        d.instanceType,
		Region:              d.Region,
		CloudName:           d.cloudName,
		Image:               image,
		SubmarinerGWNodeTag: submarinerGatewayNodeTag,
	}

	err = tpl.Execute(&buf, tplVars)
	if err != nil {
		return nil, errors.Wrap(err, "failed to execute machine set template")
	}

	return buf.Bytes(), nil
}

func (d *ocpGatewayDeployer) initMachineSet(index string) (*unstructured.Unstructured, error) {
	gatewayYAML, err := d.loadGatewayYAML(index, d.image)
	if err != nil {
		return nil, err
	}

	unstructDecoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	machineSet := &unstructured.Unstructured{}
	_, _, err = unstructDecoder.Decode(gatewayYAML, nil, machineSet)

	return machineSet, errors.Wrap(err, "error decoding message gateway yaml")
}

func (d *ocpGatewayDeployer) deployGateway(index string) error {
	machineSet, err := d.initMachineSet(index)
	if err != nil {
		return err
	}

	if d.image == "" {
		// TODO: use machineSetClient.List() instead of hard coding.
		workerNodeList := []string{d.InfraID + "-worker-0", d.InfraID + "-worker-1", d.InfraID + "-worker-2"}

		d.image, err = d.msDeployer.GetWorkerNodeImage(workerNodeList, machineSet, d.InfraID)
		if err != nil {
			return errors.Wrap(err, "error getting the worker image")
		}

		machineSet, err = d.initMachineSet(index)
		if err != nil {
			return err
		}
	}

	return errors.Wrap(d.msDeployer.Deploy(machineSet), "failed to deploy submariner gateway node")
}

func (d *ocpGatewayDeployer) Deploy(input api.GatewayDeployInput, reporter api.Reporter) error {
	reporter.Started("Configuring the required firewall rules for inter-cluster traffic")

	computeClient, err := openstack.NewComputeV2(d.Client, gophercloud.EndpointOpts{Region: d.Region})
	if err != nil {
		return errors.Wrap(err, "error creating the compute client")
	}

	networkClient, err := openstack.NewNetworkV2(d.Client, gophercloud.EndpointOpts{Region: d.Region})
	if err != nil {
		return errors.Wrap(err, "error creating the network client")
	}

	groupName := d.InfraID + gwSecurityGroupSuffix
	if err := d.createGWSecurityGroup(input.PublicPorts, groupName, computeClient, networkClient); err != nil {
		return errors.Wrap(err, "creating gateway security group failed")
	}

	reporter.Succeeded("Opened External ports %q in security group %q on RHOS",
		formatPorts(input.PublicPorts), groupName)

	reporter.Started("Configuring the required number of Submariner gateway pods")

	gwNodes, err := d.K8sClient.ListGatewayNodes()
	if err != nil {
		return errors.Wrap(err, "listing the existing gatway nodes failed")
	}

	return d.deployGWNode(gwNodes, input.Gateways, groupName, computeClient, reporter)
}

func (d *ocpGatewayDeployer) deployGWNode(gwNodes *v1.NodeList, gatewayCount int, groupName string,
	computeClient *gophercloud.ServiceClient, reporter api.Reporter) error {
	numGatewayNodes := len(gwNodes.Items)

	if numGatewayNodes == gatewayCount {
		reporter.Succeeded("Current Submariner gateways match the required number of Submariner gateways")
		return nil
	}

	// Currently, we only support increasing the number of Gateway nodes which could be a valid use-case
	// to convert a non-HA deployment to an HA deployment. We are not supporting decreasing the Gateway
	// nodes (for now) as it might impact the datapath if we accidentally delete the active GW node.
	if numGatewayNodes < gatewayCount {
		gatewayNodesToDeploy := gatewayCount - numGatewayNodes

		workerNodes, err := d.K8sClient.ListNodesWithLabel("node-role.kubernetes.io/worker")
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("failed to list k8s nodes in project %q", d.projectID))
		}

		nodes := workerNodes.Items

		for i := range nodes {
			if d.dedicatedGWNode {
				reporter.Started(fmt.Sprintf("Deploying dedicated gateway node %s",
					d.InfraID+"-submariner-gw"+strconv.Itoa(i)))

				err = d.deployGateway(strconv.Itoa(i))

				if err != nil {
					reporter.Failed(err)
					return err
				}
			} else {
				alreadyTagged := nodes[i].GetLabels()[submarinerGatewayNodeTag]
				if alreadyTagged == "true" {
					continue
				}

				reporter.Started(fmt.Sprintf("Configuring worker node %q as Submariner gateway node", nodes[i].Name))

				err := d.K8sClient.AddGWLabelOnNode(nodes[i].Name)
				if err != nil {
					return errors.Wrapf(err, "failed to label the node %q as Submariner gateway node", nodes[i].Name)
				}
			}

			if err := d.openGatewayPort(groupName, nodes[i].Name, computeClient); err != nil {
				return errors.Wrap(err, "failed to open the Submariner gateway port")
			}

			gatewayNodesToDeploy--
			if gatewayNodesToDeploy <= 0 {
				reporter.Succeeded("Successfully deployed Submariner gateway node")
				return nil
			}

			if gatewayNodesToDeploy > 0 {
				reporter.Failed(errors.Wrap(err, "there are insufficient nodes to deploy the required number of gateways"))
				return nil
			}
		}
	}

	return nil
}

func (d *ocpGatewayDeployer) Cleanup(reporter api.Reporter) error {
	reporter.Started("Removing the Submariner gateway configuration from nodes ")

	computeClient, err := openstack.NewComputeV2(d.Client, gophercloud.EndpointOpts{Region: d.Region})
	if err != nil {
		return errors.Wrapf(err, "error creating the compute client for the region: %q", d.Region)
	}

	gwNodesList, err := d.K8sClient.ListGatewayNodes()
	if err != nil {
		return errors.Wrap(err, "error listing the Submariner gateway nodes")
	}

	groupName := d.InfraID + gwSecurityGroupSuffix
	gwNodes := gwNodesList.Items

	for i := range gwNodes {
		// Check if the instance belongs to the cluster (identified via infraID) we are operating on.
		if !strings.HasPrefix(gwNodes[i].Name, d.InfraID) {
			continue
		}

		// If the instance name matches with d.InfraID + "-submariner-gw-", it implies that
		// the gateway node was deployed using the OCPMachineSet API otherwise it's an existing worker node.
		prefix := d.InfraID + "-submariner-gw-"
		if strings.HasPrefix(gwNodes[i].Name, prefix) {
			reporter.Started(fmt.Sprintf("Deleting the gateway instance %q", gwNodes[i].Name))

			err = d.deleteGateway(strconv.Itoa(i))
			if err != nil {
				return errors.Wrapf(err, "error deleting the Submariner gateway security group rules from node: %q",
					gwNodes[i].Name)
			}

			reporter.Succeeded("Successfully deleted the instance")
		} else {
			err = d.removeGWFirewallRules(groupName, gwNodes[i].Name, computeClient)
			if err != nil {
				return errors.Wrapf(err, "error deleting the Submariner gateway security group rules from node: %q",
					gwNodes[i].Name)
			}

			reporter.Started(fmt.Sprintf("Removing the gateway configuration from instance %q", gwNodes[i].Name))
			err = d.K8sClient.RemoveGWLabelFromWorkerNode(&gwNodes[i])
			if err != nil {
				return errors.Wrap(err, "failed to remove labels from worker node")
			}

			reporter.Succeeded("Successfully reconfigured the instance")
		}
	}

	reporter.Succeeded("Successfully removed the Submariner gateway configuration from the nodes")

	reporter.Started("Deleting the Submariner gateway security group")

	err = d.deleteSG(groupName, computeClient)
	if err != nil {
		return errors.Wrap(err, "error deleting the Submariner gateway security group")
	}

	reporter.Succeeded("Successfully deleted the Submariner Submariner gateway firewall rules")

	return nil
}

func formatPorts(ports []api.PortSpec) string {
	portStrs := []string{}
	for _, port := range ports {
		portStrs = append(portStrs, fmt.Sprintf("%d/%s", port.Port, port.Protocol))
	}

	return strings.Join(portStrs, ", ")
}

func (d *ocpGatewayDeployer) deleteGateway(index string) error {
	machineSet, err := d.initMachineSet(index)
	if err != nil {
		return err
	}

	return errors.Wrap(d.msDeployer.Delete(machineSet), "error deleting the submariner gateway node")
}
