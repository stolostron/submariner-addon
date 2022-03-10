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
	CloudInfo
	msDeployer      ocp.MachineSetDeployer
	instanceType    string
	image           string
	dedicatedGWNode bool
	k8sClient       k8s.Interface
}

// NewOcpGatewayDeployer returns a GatewayDeployer capable of deploying gateways using OCP.
func NewOcpGatewayDeployer(info CloudInfo, msDeployer ocp.MachineSetDeployer, instanceType, image string,
	dedicatedGWNode bool, k8sClient k8s.Interface) api.GatewayDeployer {
	return &ocpGatewayDeployer{
		CloudInfo:       info,
		msDeployer:      msDeployer,
		instanceType:    instanceType,
		image:           image,
		dedicatedGWNode: dedicatedGWNode,
		k8sClient:       k8sClient,
	}
}

func (d *ocpGatewayDeployer) Deploy(input api.GatewayDeployInput, reporter api.Reporter) error {
	reporter.Started("Configuring the required firewall rules for inter-cluster traffic")

	externalIngress := newExternalFirewallRules(d.ProjectID, d.InfraID, input.PublicPorts)
	if err := d.openPorts(externalIngress); err != nil {
		return reportFailure(reporter, err, "error creating firewall rule %q", externalIngress.Name)
	}

	reporter.Succeeded("Opened External ports %q with firewall rule %q on GCP",
		formatPorts(input.PublicPorts), externalIngress.Name)

	numGatewayNodes, eligibleZonesForGW, err := d.parseCurrentGatewayInstances(reporter)
	if err != nil {
		return reportFailure(reporter, err, "error parsing current gateway instances")
	}

	gatewayNodesToDeploy := input.Gateways - numGatewayNodes

	if gatewayNodesToDeploy == 0 {
		reporter.Succeeded("Current gateways match the required number of gateways")
		return nil
	}

	// Currently, we only support increasing the number of Gateway nodes which could be a valid use-case
	// to convert a non-HA deployment to an HA deployment. We are not supporting decreasing the Gateway
	// nodes (for now) as it might impact the datapath if we accidentally delete the active GW node.
	if gatewayNodesToDeploy < 0 {
		reporter.Failed(fmt.Errorf("decreasing the number of Gateway nodes is not currently supported"))
		return nil
	}

	if d.dedicatedGWNode {
		for _, zone := range eligibleZonesForGW.Elements() {
			reporter.Started(fmt.Sprintf("Deploying dedicated gateway node in zone %q", zone))

			err = d.deployGateway(zone)
			if err != nil {
				return reportFailure(reporter, err, "error deploying gateway for zone %q", zone)
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
			workerNodes, err := d.k8sClient.ListNodesWithLabel("topology.kubernetes.io/zone=" + zone + ",node-role.kubernetes.io/worker")
			if err != nil {
				return reportFailure(reporter, err, "failed to list k8s nodes in zone %q of project %q", zone, d.ProjectID)
			}

			for i := range workerNodes.Items {
				node := &workerNodes.Items[i]
				machineSetInfo := node.GetAnnotations()["machine.openshift.io/machine"]
				gcpInstanceInfo := strings.Split(machineSetInfo, "/")
				if len(gcpInstanceInfo) <= 1 {
					continue
				}

				reporter.Started(fmt.Sprintf("Configuring worker node %q in zone %q as gateway node", node.Name, zone))
				if err := d.configureExistingNodeAsGW(zone, gcpInstanceInfo[1], node.Name); err != nil {
					return reportFailure(reporter, err, "error configuring gateway node %q", node.Name)
				}

				gatewayNodesToDeploy--
				break
			}

			if gatewayNodesToDeploy <= 0 {
				reporter.Succeeded("Successfully deployed gateway node")
				return nil
			}
		}
	}

	// We try to deploy a single Gateway node per zone (in the selected region). If the numGateways
	// is more than the number of Zones, its treated as an error.
	err = fmt.Errorf("there are an insufficient number of zones (%d) to deploy the desired number of gateways (%d)",
		eligibleZonesForGW.Size(), input.Gateways)
	reporter.Failed(err)

	return err
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

		instanceList, err := d.Client.ListInstances(zone.Name)
		if err != nil {
			return 0, nil, errors.Wrapf(err, "failed to list instances in zone %q of project %q", zone.Name, d.ProjectID)
		}

		for _, instance := range instanceList.Items {
			// Check if the instance belongs to the cluster (identified via infraID) we are operating on.
			if !strings.HasPrefix(instance.Name, d.InfraID) {
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
		return nil, errors.Wrap(err, "error parsing machine set YAML")
	}

	tplVars := machineSetConfig{
		AZ:                  zone,
		InfraID:             d.InfraID,
		ProjectID:           d.ProjectID,
		InstanceType:        d.instanceType,
		Region:              d.Region,
		Image:               image,
		SubmarinerGWNodeTag: submarinerGatewayNodeTag,
	}

	err = tpl.Execute(&buf, tplVars)
	if err != nil {
		return nil, errors.Wrap(err, "error executing the template")
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
		return nil, errors.Wrap(err, "error converting YAML to machine set")
	}

	return machineSet, nil
}

func (d *ocpGatewayDeployer) deployGateway(zone string) error {
	machineSet, err := d.initMachineSet(zone)
	if err != nil {
		return err
	}

	if d.image == "" {
		// TODO: use machineSetClient.List() instead of hard coding.
		workerNodeList := []string{d.InfraID + "-worker-b", d.InfraID + "-worker-c", d.InfraID + "-worker-d"}

		d.image, err = d.msDeployer.GetWorkerNodeImage(workerNodeList, machineSet, d.InfraID)
		if err != nil {
			return errors.Wrap(err, "error retrieving worker node image")
		}

		machineSet, err = d.initMachineSet(zone)
		if err != nil {
			return err
		}
	}

	return errors.Wrapf(d.msDeployer.Deploy(machineSet), "error deploying machine set %q", machineSet.GetName())
}

func (d *ocpGatewayDeployer) configureExistingNodeAsGW(zone, gcpInstanceInfo, nodeName string) error {
	instance, err := d.Client.GetInstance(zone, gcpInstanceInfo)
	if err != nil {
		return errors.Wrapf(err, "error retrieving GCP instance %q in zode %q", gcpInstanceInfo, zone)
	}

	tags := &compute.Tags{
		Items:       instance.Tags.Items,
		Fingerprint: instance.Tags.Fingerprint,
	}

	tags.Items = append(tags.Items, submarinerGatewayNodeTag)

	err = d.Client.UpdateInstanceNetworkTags(d.ProjectID, zone, instance.Name, tags)
	if err != nil {
		return errors.Wrapf(err, "error updating network tags for GCP instance %q in zode %q", instance.Name, zone)
	}

	err = d.Client.ConfigurePublicIPOnInstance(instance)
	if err != nil {
		return errors.Wrapf(err, "error configuring public IP for GCP instance %q in zode %q", instance.Name, zone)
	}

	err = d.k8sClient.AddGWLabelOnNode(nodeName)
	if err != nil {
		return errors.Wrapf(err, "error labeling node %q", nodeName)
	}

	return nil
}

func (d *ocpGatewayDeployer) Cleanup(reporter api.Reporter) error {
	reporter.Started("Retrieving the Submariner gateway firewall rules")

	err := d.deleteExternalFWRules(reporter)
	if err != nil {
		return reportFailure(reporter, err, "failed to delete the gateway firewall rules in the project %q", d.ProjectID)
	}

	reporter.Succeeded("Successfully deleted the firewall rules")

	zones, err := d.retrieveZones(reporter)
	if err != nil {
		return reportFailure(reporter, err, "error retrieving zones")
	}

	for _, zone := range zones.Items {
		if d.ignoreZone(zone) {
			continue
		}

		instanceList, err := d.Client.ListInstances(zone.Name)
		if err != nil {
			return reportFailure(reporter, err, "failed to list instances in zone %q of project %q", zone.Name, d.ProjectID)
		}

		for _, instance := range instanceList.Items {
			// Check if the instance belongs to the cluster (identified via infraID) we are operating on.
			if !strings.HasPrefix(instance.Name, d.InfraID) {
				continue
			}

			if !d.isInstanceGatewayNode(instance) {
				continue
			}

			// If the instance name matches with d.InfraID + "-submariner-gw-" + zone.Name, it implies that
			// the gateway node was deployed using the OCPMachineSet API otherwise it's an existing worker node.
			prefix := d.InfraID + "-submariner-gw-" + zone.Name
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

	reporter.Started("Removing the Submariner gateway label from worker nodes")

	err = d.k8sClient.RemoveGWLabelFromWorkerNodes()
	if err != nil {
		return reportFailure(reporter, err, "error removing the gateway label from worker nodes")
	}

	reporter.Succeeded("Successfully removed the label from the worker nodes")

	return nil
}

func (d *ocpGatewayDeployer) deleteGateway(zone string) error {
	machineSet, err := d.initMachineSet(zone)
	if err != nil {
		return err
	}

	return errors.Wrapf(d.msDeployer.Delete(machineSet), "error deleting machine set %q", machineSet.GetName())
}

func (d *ocpGatewayDeployer) deleteExternalFWRules(reporter api.Reporter) error {
	ingressName := generateRuleName(d.InfraID, publicPortsRuleName)

	if err := d.deleteFirewallRule(ingressName, reporter); err != nil {
		return errors.Wrapf(err, "error deleting firewall rule %q", ingressName)
	}

	return nil
}

func reportFailure(reporter api.Reporter, failure error, format string, args ...interface{}) error {
	err := errors.WithMessagef(failure, format, args...)
	reporter.Failed(err)

	return err
}

func (d *ocpGatewayDeployer) ignoreZone(zone *compute.Zone) bool {
	region := zone.Region[strings.LastIndex(zone.Region, "/")+1:]

	return region != d.Region
}

func (d *ocpGatewayDeployer) isInstanceGatewayNode(instance *compute.Instance) bool {
	if instance.Tags == nil {
		return false
	}

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

	err := d.Client.UpdateInstanceNetworkTags(d.ProjectID, zone, instance.Name, tags)
	if err != nil {
		return errors.Wrapf(err, "error updating network tags for GCP instance %q in zode %q", instance.Name, zone)
	}

	err = d.Client.DeletePublicIPOnInstance(instance)
	if err != nil {
		return errors.Wrapf(err, "error deleting public IP for GCP instance %q in zode %q", instance.Name, zone)
	}

	return nil
}

func (d *ocpGatewayDeployer) retrieveZones(reporter api.Reporter) (*compute.ZoneList, error) {
	reporter.Started("Retrieving the current zones in the project")

	zones, err := d.Client.ListZones()
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list the zones in the project %q", d.ProjectID)
	}

	reporter.Succeeded("Retrieved the zones")

	return zones, nil
}
