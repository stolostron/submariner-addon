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

import (
	"bytes"
	"context"
	"strings"
	"text/template"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"github.com/submariner-io/admiral/pkg/reporter"
	"github.com/submariner-io/admiral/pkg/stringset"
	"github.com/submariner-io/cloud-prepare/pkg/api"
	"github.com/submariner-io/cloud-prepare/pkg/ocp"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
)

const (
	submarinerGatewayGW      = "-subgw-"
	azureVirtualMachines     = "virtualMachines"
	topologyLabel            = "topology.kubernetes.io/zone"
	submarinerGatewayNodeTag = "submariner-io-gateway-node"
)

type ocpGatewayDeployer struct {
	CloudInfo
	azure           *azureCloud
	msDeployer      ocp.MachineSetDeployer
	instanceType    string
	dedicatedGWNode bool
}

// NewOcpGatewayDeployer returns a GatewayDeployer capable deploying gateways using OCP.
// If the supplied cloud is not an azureCloud, an error is returned.
func NewOcpGatewayDeployer(info *CloudInfo, cloud api.Cloud, msDeployer ocp.MachineSetDeployer, instanceType string,
	dedicatedGWNode bool,
) (api.GatewayDeployer, error) {
	azure, ok := cloud.(*azureCloud)
	if !ok {
		return nil, errors.New("the cloud must be Azure")
	}

	return &ocpGatewayDeployer{
		CloudInfo:       *info,
		azure:           azure,
		msDeployer:      msDeployer,
		instanceType:    instanceType,
		dedicatedGWNode: dedicatedGWNode,
	}, nil
}

func (d *ocpGatewayDeployer) Deploy(input api.GatewayDeployInput, status reporter.Interface) error {
	if input.Gateways == 0 {
		return nil
	}

	status.Start("Deploying gateway node")

	nsgClient, err := d.getNsgClient()
	if err != nil {
		return status.Error(err, "Failed to get network security groups client")
	}

	nwClient, err := d.getInterfacesClient()
	if err != nil {
		return status.Error(err, "Failed to get network interfaces client")
	}

	pubIPClient, err := d.getPublicIPClient()
	if err != nil {
		return status.Error(err, "Failed to get network public IP addresses client")
	}

	groupName := d.InfraID + externalSecurityGroupSuffix

	gwNodes, err := d.azure.K8sClient.ListGatewayNodes()
	if err != nil {
		return errors.Wrap(err, "error getting the gateway node")
	}

	// Open the g/w ports and assign public-ip if not already done for manually tagged nodes if any
	gwNodeItems := gwNodes.Items
	gatewayNodesToDeploy := input.Gateways - len(gwNodeItems)

	if len(gwNodeItems) != 0 || gatewayNodesToDeploy != 0 {
		if err := d.createGWSecurityGroup(groupName, input.PublicPorts, nsgClient); err != nil {
			return status.Error(err, "creating gateway security group failed")
		}
	}

	for i := range gwNodeItems {
		if err = d.prepareGWInterface(gwNodeItems[i].Name, groupName, nsgClient, nwClient, pubIPClient); err != nil {
			return status.Error(err, "failed to open the Submariner gateway port for already existing nodes")
		}
	}

	if gatewayNodesToDeploy == 0 {
		status.Success("Current gateways match the required number of gateways")
		return nil
	}

	// Currently, we only support increasing the number of Gateway nodes which could be a valid use-case
	// to convert a non-HA deployment to an HA deployment. We are not supporting decreasing the Gateway
	// nodes (for now) as it might impact the datapath if we accidentally delete the active GW node.
	if gatewayNodesToDeploy < 0 {
		status.Failure("Decreasing the number of Gateway nodes is not currently supported")
		return nil
	}

	if d.dedicatedGWNode {
		err = d.deployDedicatedGWNode(gwNodeItems, gatewayNodesToDeploy, status)
	} else {
		err = d.tagExistingNode(nsgClient, nwClient, pubIPClient, gatewayNodesToDeploy, status)
	}

	if err != nil {
		status.Success("Deployed gateway node")
	}

	return err
}

func (d *ocpGatewayDeployer) deployDedicatedGWNode(gwNodes []v1.Node, gatewayNodesToDeploy int, status reporter.Interface,
) error {
	az, err := d.getAvailabilityZones(gwNodes)
	if err != nil || az.Size() == 0 {
		return status.Error(err, "error getting the availability zones for region %q", d.Region)
	}

	for _, zone := range az.Elements() {
		status.Start("Deploying dedicated gateway node")

		err := d.deployGateway(zone)
		if err != nil {
			return status.Error(err, "error deploying gateway for zone %q", zone)
		}

		gatewayNodesToDeploy--
		if gatewayNodesToDeploy <= 0 {
			status.Success("Successfully deployed gateway node")
			return nil
		}
	}

	if gatewayNodesToDeploy != 0 {
		return status.Error(err, "not enough zones available in the region %q to deploy required number of gateway nodes", d.Region)
	}

	return nil
}

func (d *ocpGatewayDeployer) tagExistingNode(nsgClient *armnetwork.SecurityGroupsClient, nwClient *armnetwork.InterfacesClient,
	pubIPClient *armnetwork.PublicIPAddressesClient, gatewayNodesToDeploy int, status reporter.Interface,
) error {
	groupName := d.InfraID + externalSecurityGroupSuffix

	workerNodes, err := d.K8sClient.ListNodesWithLabel("node-role.kubernetes.io/worker")
	if err != nil {
		return status.Error(err, "failed to list k8s nodes in ResorceGroup %q", d.BaseGroupName)
	}

	nodes := workerNodes.Items
	for i := range nodes {
		alreadyTagged := nodes[i].GetLabels()[submarinerGatewayNodeTag]
		if alreadyTagged == "true" {
			continue
		}

		status.Start("Configuring worker node %q as Submariner gateway node", nodes[i].Name)

		err := d.K8sClient.AddGWLabelOnNode(nodes[i].Name)
		if err != nil {
			return status.Error(err, "failed to label the node %q as Submariner gateway node", nodes[i].Name)
		}

		if err = d.prepareGWInterface(nodes[i].Name, groupName, nsgClient, nwClient, pubIPClient); err != nil {
			return status.Error(err, "failed to open the Submariner gateway port")
		}

		gatewayNodesToDeploy--
		if gatewayNodesToDeploy <= 0 {
			status.Success("Successfully deployed Submariner gateway node")
			status.End()

			return nil
		}
	}

	if gatewayNodesToDeploy > 0 {
		return status.Error(err, "there are insufficient nodes to deploy the required number of gateways")
	}

	return nil
}

type machineSetConfig struct {
	Name         string
	AZ           string
	InfraID      string
	InstanceType string
	Region       string
}

func (d *ocpGatewayDeployer) loadGatewayYAML(name, zone string) ([]byte, error) {
	var buf bytes.Buffer

	tpl, err := template.New("").Parse(machineSetYAML)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing machine set YAML")
	}

	tplVars := machineSetConfig{
		Name:         name,
		InfraID:      d.azure.InfraID,
		InstanceType: d.instanceType,
		Region:       d.azure.Region,
		AZ:           zone,
	}

	err = tpl.Execute(&buf, tplVars)
	if err != nil {
		return nil, errors.Wrap(err, "error executing the template")
	}

	return buf.Bytes(), nil
}

func (d *ocpGatewayDeployer) initMachineSet(name, zone string) (*unstructured.Unstructured, error) {
	gatewayYAML, err := d.loadGatewayYAML(name, zone)
	if err != nil {
		return nil, err
	}

	unStructDecoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

	machineSet := &unstructured.Unstructured{}

	_, _, err = unStructDecoder.Decode(gatewayYAML, nil, machineSet)
	if err != nil {
		return nil, errors.Wrap(err, "error converting YAML to machine set")
	}

	return machineSet, nil
}

func (d *ocpGatewayDeployer) deployGateway(zone string) error {
	machineSet, err := d.initMachineSet(MachineName(d.azure.InfraID, d.azure.Region, zone), zone)
	if err != nil {
		return err
	}

	return errors.Wrapf(d.msDeployer.Deploy(machineSet), "error deploying machine set %q", machineSet.GetName())
}

// MachineName generates a machine name for the gateway.
// The name length is limited to 40 characters to ensure we don't hit the 63-character limit
// when generating the "machine public IP name".
// At most 6 characters for the zone (which is usually very short),
// at most 12 for the region and zone combined,
// at most 32 for the infra id, region and zone combined
// (the infra id is the longest significant piece of information here).
// We add "-subgw-", 7 characters, for a total of 40 with the hyphen between region and zone.
func MachineName(infraID, region, zone string) string {
	if len(infraID)+len(region)+len(zone) > 32 {
		// Limit the name length to 40 characters
		if len(zone) > 6 {
			zone = zone[0:6]
		}

		if len(region) > 12-len(zone) {
			region = region[0 : 12-len(zone)]
		}

		if len(infraID) > 32-len(zone)-len(region) {
			infraID = infraID[0 : 32-len(zone)-len(region)]
		}
	}

	return infraID + submarinerGatewayGW + region + "-" + zone
}

func (d *ocpGatewayDeployer) getAvailabilityZones(gwNodes []v1.Node) (stringset.Interface, error) {
	zonesWithSubmarinerGW := stringset.New()

	for i := range gwNodes {
		zonesWithSubmarinerGW.Add(gwNodes[i].GetLabels()[topologyLabel])
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	resourceSKUClient, err := d.getResourceSKUClient()
	if err != nil {
		return nil, errors.Wrap(err, "Failed to get resource SKU client")
	}

	pager := resourceSKUClient.NewListPager(&armcompute.ResourceSKUsClientListOptions{
		Filter: to.StringPtr(d.azure.Region),
	})

	eligibleZonesForSubmarinerGW := stringset.New()

	for pager.More() {
		nextResult, err := pager.NextPage(ctx)
		if err != nil {
			return nil, errors.Wrapf(err, "error paging the resource SKUs in the regiom %q", d.azure.Region)
		}

		for _, resourceSKU := range nextResult.Value {
			if *resourceSKU.ResourceType == azureVirtualMachines && *resourceSKU.Name == d.instanceType {
				for _, zone := range resourceSKU.LocationInfo[0].Zones {
					if !zonesWithSubmarinerGW.Contains(d.azure.Region + "-" + *zone) {
						eligibleZonesForSubmarinerGW.Add(*zone)
					}
				}
			}
		}
	}

	return eligibleZonesForSubmarinerGW, nil
}

func (d *ocpGatewayDeployer) Cleanup(status reporter.Interface) error {
	status.Start("Removing gateway node")

	nsgClient, err := d.getNsgClient()
	if err != nil {
		return status.Error(err, "Failed to get network security groups client")
	}

	nwClient, err := d.getInterfacesClient()
	if err != nil {
		return status.Error(err, "Failed to get network interfaces client")
	}

	if err := d.cleanupGWInterface(d.InfraID, nsgClient, nwClient); err != nil {
		return status.Error(err, "deleting gateway security group failed")
	}

	err = d.deleteGateway()
	if err != nil {
		return status.Error(err, "removing gateway node failed")
	}

	status.Success("Removed gateway node")

	return nil
}

func (d *ocpGatewayDeployer) deleteGateway() error {
	gwNodes, err := d.azure.K8sClient.ListGatewayNodes()
	if err != nil {
		return errors.Wrapf(err, "error getting the gw nodes")
	}

	gwNodesList := gwNodes.Items

	pubIPClient, err := d.getPublicIPClient()
	if err != nil {
		return errors.Wrapf(err, "Failed to get network public IP addresses client")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	for i := 0; i < len(gwNodesList); i++ {
		if !strings.Contains(gwNodesList[i].Name, submarinerGatewayGW) {
			err = d.K8sClient.RemoveGWLabelFromWorkerNode(&gwNodesList[i])
			if err != nil {
				return errors.Wrapf(err, "failed to remove labels from worker node")
			}

			publicIPName := gwNodesList[i].Name + "-pub"

			err = d.deletePublicIP(ctx, pubIPClient, publicIPName)
			if err != nil {
				return errors.Wrapf(err, "failed to delete public-ip")
			}
		} else {
			machineSetName := gwNodesList[i].Name[:strings.LastIndex(gwNodesList[i].Name, "-")]
			prefix := machineSetName[:strings.LastIndex(gwNodesList[i].Name, "-")]
			zone := machineSetName[strings.LastIndex(gwNodesList[i].Name, "-")-1:]

			machineSet, err := d.initMachineSet(prefix, zone)
			if err != nil {
				return err
			}

			return errors.Wrapf(d.msDeployer.Delete(machineSet), "error deleting machine set %q", machineSet.GetName())
		}
	}

	return nil
}
