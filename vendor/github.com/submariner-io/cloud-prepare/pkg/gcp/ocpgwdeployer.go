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

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"

	"github.com/pkg/errors"
	"github.com/submariner-io/admiral/pkg/stringset"
	"github.com/submariner-io/cloud-prepare/pkg/api"
	"github.com/submariner-io/cloud-prepare/pkg/k8s"
	"github.com/submariner-io/cloud-prepare/pkg/ocp"
	"google.golang.org/api/compute/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
)

type ocpGatewayDeployer struct {
	gcp             *gcpCloud
	msDeployer      ocp.MachineSetDeployer
	instanceType    string
	image           string
	dedicatedGWNode bool
	k8sClient       k8s.K8sInterface
}

// NewOcpGatewayDeployer returns a GatewayDeployer capable deploying gateways using OCP
// If the supplied cloud is not a gcpCloud, an error is returned
func NewOcpGatewayDeployer(cloud api.Cloud, msDeployer ocp.MachineSetDeployer, instanceType, image string,
	dedicatedGWNode bool, k8sClient k8s.K8sInterface) (api.GatewayDeployer, error) {
	gcp, ok := cloud.(*gcpCloud)
	if !ok {
		return nil, errors.New("the cloud must be GCP")
	}

	return &ocpGatewayDeployer{
		gcp:             gcp,
		msDeployer:      msDeployer,
		instanceType:    instanceType,
		image:           image,
		dedicatedGWNode: dedicatedGWNode,
		k8sClient:       k8sClient,
	}, nil
}

func (d *ocpGatewayDeployer) Deploy(input api.GatewayDeployInput, reporter api.Reporter) error {
	reporter.Started("Configuring the required firewall rules for inter-cluster traffic")

	externalIngress := newExternalFirewallRules(d.gcp.projectID, d.gcp.infraID, input.PublicPorts)
	if err := d.gcp.openPorts(externalIngress); err != nil {
		reporter.Failed(err)
		return err
	}

	reporter.Succeeded("Opened External ports %q with firewall rule %q on GCP",
		formatPorts(input.PublicPorts), externalIngress.Name)

	numGatewayNodes, eligibleZonesForGW, err := d.parseCurrentGatewayInstances(reporter)
	if err != nil {
		reporter.Failed(err)
		return err
	}

	if numGatewayNodes == input.Gateways {
		reporter.Succeeded("Current gateways match the required number of gateways")
		return nil
	}

	// Currently, we only support increasing the number of Gateway nodes which could be a valid use-case
	// to convert a non-HA deployment to an HA deployment. We are not supporting decreasing the Gateway
	// nodes (for now) as it might impact the datapath if we accidentally delete the active GW node.
	if numGatewayNodes < input.Gateways {
		gatewayNodesToDeploy := input.Gateways - numGatewayNodes

		if d.dedicatedGWNode {
			for _, zone := range eligibleZonesForGW.Elements() {
				reporter.Started(fmt.Sprintf("Deploying dedicated gateway node in zone %q", zone))
				err = d.deployGateway(zone)
				if err != nil {
					reporter.Failed(err)
					return err
				}

				gatewayNodesToDeploy--
				if gatewayNodesToDeploy <= 0 {
					reporter.Succeeded("Successfully deployed gateway node")
					return nil
				}
			}
		} else {
			// Query the list of instances in the eligibleZones of the current region and if it's a worker node,
			// configure the instance as Submariner Gateway node.
			for _, zone := range eligibleZonesForGW.Elements() {
				workerNodes, err := d.k8sClient.ListWorkerNodes("topology.kubernetes.io/zone=" + zone + ",node-role.kubernetes.io/worker")
				if err != nil {
					return reportFailure(reporter, err, "failed to list k8s nodes in zone %q of project %q", zone, d.gcp.projectID)
				}

				for _, node := range workerNodes.Items {
					machineSetInfo := node.GetAnnotations()["machine.openshift.io/machine"]
					if machineSetInfo != "" {
						gcpInstanceInfo := strings.Split(machineSetInfo, "/")
						if len(gcpInstanceInfo) > 1 {
							reporter.Started(fmt.Sprintf("Configuring worker node %q in zone %q as gateway node", node.Name, zone))
							if err := d.configureExistingNodeAsGW(zone, gcpInstanceInfo[1], node.Name); err != nil {
								reporter.Failed(err)
								return err
							}
							gatewayNodesToDeploy--
							break
						}
					}
				}

				if gatewayNodesToDeploy <= 0 {
					reporter.Succeeded("Successfully deployed gateway node")
					return nil
				}
			}
		}

		// We try to deploy a single Gateway node per zone (in the selected region). If the numGateways
		// is more than the number of Zones, its treated as an error.
		if gatewayNodesToDeploy > 0 {
			reporter.Failed(fmt.Errorf("there are insufficient zones to deploy the required number of gateways"))
			return nil
		}
	}

	return nil
}

func (d *ocpGatewayDeployer) parseCurrentGatewayInstances(reporter api.Reporter) (int, stringset.Interface, error) {
	zones, err := d.retrieveZones(reporter)
	if err != nil {
		return 0, nil, err
	}

	reporter.Started("Verifying if current gateways match the required number of gateways")

	zonesWithSubmarinerGW := stringset.New()
	eligibleZonesForGW := stringset.New()

	for _, zone := range zones.Items {
		// The list of zones include zones from all the regions, so filter out the zones that do
		// not belong to the current region.
		if d.ignoreZone(zone) {
			continue
		}

		instanceList, err := d.gcp.client.ListInstances(zone.Name)
		if err != nil {
			return 0, nil, reportFailure(reporter, err, "failed to list instances in zone %q of project %q", zone.Name, d.gcp.projectID)
		}

		for _, instance := range instanceList.Items {
			// Check if the instance belongs to the cluster (identified via infraID) we are operating on.
			if !strings.HasPrefix(instance.Name, d.gcp.infraID) {
				continue
			}

			// A GatewayNode will always be tagged with submarinerGatewayNodeTag when deployed with OCPMachineSet
			// as well as when an existing worker node is updated as a Gateway node.
			if d.isInstanceGatewayNode(instance) {
				zonesWithSubmarinerGW.Add(zone.Name)
				break
			}
		}

		if !zonesWithSubmarinerGW.Contains(zone.Name) {
			eligibleZonesForGW.Add(zone.Name)
		}
	}

	return zonesWithSubmarinerGW.Size(), eligibleZonesForGW, nil
}

type machineSetConfig struct {
	AZ                  string
	InfraID             string
	ProjectID           string
	InstanceType        string
	Region              string
	Image               string
	SubmarinerGWNodeTag string
}

func (d *ocpGatewayDeployer) loadGatewayYAML(zone, image string) ([]byte, error) {
	var buf bytes.Buffer

	// TODO: Not working properly, but we should revisit this as it makes more sense
	// tpl, err := template.ParseFiles("pkg/aws/gw-machineset.yaml.template")
	tpl, err := template.New("").Parse(machineSetYAML)
	if err != nil {
		return nil, err
	}

	tplVars := machineSetConfig{
		AZ:                  zone,
		InfraID:             d.gcp.infraID,
		ProjectID:           d.gcp.projectID,
		InstanceType:        d.instanceType,
		Region:              d.gcp.region,
		Image:               image,
		SubmarinerGWNodeTag: submarinerGatewayNodeTag,
	}

	err = tpl.Execute(&buf, tplVars)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (d *ocpGatewayDeployer) initMachineSet(zone string) (*unstructured.Unstructured, error) {
	gatewayYAML, err := d.loadGatewayYAML(zone, d.image)
	if err != nil {
		return nil, err
	}

	unstructDecoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	machineSet := &unstructured.Unstructured{}
	_, _, err = unstructDecoder.Decode(gatewayYAML, nil, machineSet)
	if err != nil {
		return nil, err
	}

	return machineSet, nil
}

func (d *ocpGatewayDeployer) deployGateway(zone string) error {
	machineSet, err := d.initMachineSet(zone)
	if err != nil {
		return err
	}

	if d.image == "" {
		d.image, err = d.msDeployer.GetWorkerNodeImage(machineSet, d.gcp.infraID)
		if err != nil {
			return err
		}

		machineSet, err = d.initMachineSet(zone)
		if err != nil {
			return err
		}
	}

	return d.msDeployer.Deploy(machineSet)
}

func (d *ocpGatewayDeployer) configureExistingNodeAsGW(zone, gcpInstanceInfo, nodeName string) error {
	instance, err := d.gcp.client.GetInstance(zone, gcpInstanceInfo)
	if err != nil {
		return err
	}

	tags := &compute.Tags{
		Items:       instance.Tags.Items,
		Fingerprint: instance.Tags.Fingerprint,
	}

	tags.Items = append(tags.Items, submarinerGatewayNodeTag)

	err = d.gcp.client.UpdateInstanceNetworkTags(d.gcp.projectID, zone, instance.Name, tags)
	if err != nil {
		return err
	}

	err = d.gcp.client.ConfigurePublicIPOnInstance(instance)
	if err != nil {
		return err
	}

	err = d.k8sClient.AddGWLabelOnNode(nodeName)
	if err != nil {
		return err
	}

	return nil
}

func (d *ocpGatewayDeployer) Cleanup(reporter api.Reporter) error {
	reporter.Started("Retrieving the Submariner gateway firewall rules")
	err := d.deleteExternalFWRules(reporter)
	if err != nil {
		return reportFailure(reporter, err, "failed to delete the gateway firewall rules in the project %q", d.gcp.projectID)
	}

	reporter.Succeeded("Successfully deleted the firewall rules")
	zones, err := d.retrieveZones(reporter)
	if err != nil {
		return err
	}

	for _, zone := range zones.Items {
		if d.ignoreZone(zone) {
			continue
		}

		instanceList, err := d.gcp.client.ListInstances(zone.Name)
		if err != nil {
			return reportFailure(reporter, err, "failed to list instances in zone %q of project %q", zone.Name, d.gcp.projectID)
		}

		for _, instance := range instanceList.Items {
			// Check if the instance belongs to the cluster (identified via infraID) we are operating on.
			if !strings.HasPrefix(instance.Name, d.gcp.infraID) {
				continue
			}

			if !d.isInstanceGatewayNode(instance) {
				continue
			}

			// If the instance name matches with d.gcp.infraID + "-submariner-gw-" + zone.Name, it implies that
			// the gateway node was deployed using the OCPMachineSet API otherwise it's an existing worker node.
			prefix := d.gcp.infraID + "-submariner-gw-" + zone.Name
			if strings.HasPrefix(instance.Name, prefix) {
				reporter.Started(fmt.Sprintf("Deleting the gateway instance %q", instance.Name))
				err := d.deleteGateway(zone.Name)
				if err != nil {
					return reportFailure(reporter, err, "failed to delete dedicated gateway instance %q", instance.Name)
				}

				reporter.Succeeded("Successfully deleted the instance")
			} else {
				reporter.Started(fmt.Sprintf("Removing the gateway configuration from instance %q", instance.Name))
				err = d.resetExistingGWNode(zone.Name, instance)
				if err != nil {
					return reportFailure(reporter, err, "failed to delete gateway instance %q", instance.Name)
				}

				reporter.Succeeded("Successfully reconfigured the instance")
			}
		}
	}

	reporter.Started("Removing the Submariner gateway labels from K8s nodes")

	err = d.k8sClient.RemoveGWLabelFromWorkerNodes()
	if err != nil {
		return err
	}

	reporter.Succeeded("Successfully removed the labels from the nodes")

	return nil
}

func (d *ocpGatewayDeployer) deleteGateway(zone string) error {
	machineSet, err := d.initMachineSet(zone)
	if err != nil {
		return err
	}

	return d.msDeployer.Delete(machineSet)
}

func (d *ocpGatewayDeployer) deleteExternalFWRules(reporter api.Reporter) error {
	ingressName := generateRuleName(d.gcp.infraID, publicPortsRuleName)

	if err := d.gcp.deleteFirewallRule(ingressName, reporter); err != nil {
		reporter.Failed(err)
		return err
	}

	return nil
}

func reportFailure(reporter api.Reporter, failure error, format string, args ...string) error {
	err := errors.WithMessagef(failure, format, args)
	reporter.Failed(err)

	return err
}

func (d *ocpGatewayDeployer) ignoreZone(zone *compute.Zone) bool {
	region := zone.Region[strings.LastIndex(zone.Region, "/")+1:]

	return region != d.gcp.region
}

func (d *ocpGatewayDeployer) isInstanceGatewayNode(instance *compute.Instance) bool {
	for _, tag := range instance.Tags.Items {
		if tag == submarinerGatewayNodeTag {
			return true
		}
	}

	return false
}

func (d *ocpGatewayDeployer) resetExistingGWNode(zone string, instance *compute.Instance) error {
	for i := range instance.Tags.Items {
		if instance.Tags.Items[i] == submarinerGatewayNodeTag {
			instance.Tags.Items = append(instance.Tags.Items[:i], instance.Tags.Items[i+1:]...)
		}
	}

	tags := &compute.Tags{
		Items:       instance.Tags.Items,
		Fingerprint: instance.Tags.Fingerprint,
	}

	err := d.gcp.client.UpdateInstanceNetworkTags(d.gcp.projectID, zone, instance.Name, tags)
	if err != nil {
		return err
	}

	err = d.gcp.client.DeletePublicIPOnInstance(instance)
	if err != nil {
		return err
	}

	return nil
}

func (d *ocpGatewayDeployer) retrieveZones(reporter api.Reporter) (*compute.ZoneList, error) {
	reporter.Started("Retrieving the current zones in the project")

	zones, err := d.gcp.client.ListZones()
	if err != nil {
		return nil, reportFailure(reporter, err, "failed to list the zones in the project %q. %v", d.gcp.projectID)
	}

	reporter.Succeeded("Retrieved the zones")

	return zones, nil
}
