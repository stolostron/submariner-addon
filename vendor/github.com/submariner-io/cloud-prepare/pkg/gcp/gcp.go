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
	"errors"
	"fmt"
	"strings"

	"github.com/submariner-io/cloud-prepare/pkg/api"
	gcpclient "github.com/submariner-io/cloud-prepare/pkg/gcp/client"

	"google.golang.org/api/compute/v1"
)

type gcpCloud struct {
	infraID   string
	region    string
	projectID string
	client    gcpclient.Interface
}

// NewCloud creates a new api.Cloud instance which can prepare GCP for Submariner to be deployed on it
func NewCloud(projectID, infraID, region string, client gcpclient.Interface) api.Cloud {
	return &gcpCloud{
		infraID:   infraID,
		projectID: projectID,
		region:    region,
		client:    client,
	}
}

// PrepareForSubmariner prepares submariner cluster environment on GCP
func (gc *gcpCloud) PrepareForSubmariner(input api.PrepareForSubmarinerInput, reporter api.Reporter) error {
	// create the inbound firewall rule for submariner internal ports
	reporter.Started("Opening internal ports %q for intra-cluster communications on GCP", formatPorts(input.InternalPorts))
	internalIngress := newInternalFirewallRule(gc.projectID, gc.infraID, input.InternalPorts)
	if err := gc.openPorts(internalIngress); err != nil {
		reporter.Failed(err)
		return err
	}

	reporter.Succeeded("Opened internal ports %q with firewall rule %q on GCP",
		formatPorts(input.InternalPorts), internalIngress.Name)

	return nil
}

// CleanupAfterSubmariner clean up submariner cluster environment on GCP
func (gc *gcpCloud) CleanupAfterSubmariner(reporter api.Reporter) error {
	// delete the inbound and outbound firewall rules to close submariner internal ports
	internalIngressName := generateRuleName(gc.infraID, internalPortsRuleName)

	return gc.deleteFirewallRule(internalIngressName, reporter)
}

// open expected ports by creating related firewall rule
// - if the firewall rule is not found, we will create it
// - if the firewall rule is found and changed, we will update it
func (gc *gcpCloud) openPorts(rules ...*compute.Firewall) error {
	for _, rule := range rules {
		_, err := gc.client.GetFirewallRule(gc.projectID, rule.Name)
		if gcpclient.IsGCPNotFoundError(err) {
			if err := gc.client.InsertFirewallRule(gc.projectID, rule); err != nil {
				return err
			}

			continue
		}

		if err != nil {
			return err
		}

		if err := gc.client.UpdateFirewallRule(gc.projectID, rule.Name, rule); err != nil {
			return err
		}
	}

	return nil
}

func (gc *gcpCloud) deleteFirewallRule(name string, reporter api.Reporter) error {
	reporter.Started("Deleting firewall rule %q on GCP", name)

	if err := gc.client.DeleteFirewallRule(gc.projectID, name); err != nil {
		if !gcpclient.IsGCPNotFoundError(err) {
			reporter.Failed(err)
			return err
		}
	}

	reporter.Succeeded("Deleted firewall rule %q on GCP", name)

	return nil
}

func formatPorts(ports []api.PortSpec) string {
	portStrs := []string{}
	for _, port := range ports {
		portStrs = append(portStrs, fmt.Sprintf("%d/%s", port.Port, port.Protocol))
	}

	return strings.Join(portStrs, ", ")
}

type gcpGatewayDeployer struct {
	gcp *gcpCloud
}

// NewGCPGatewayDeployer created a GatewayDeployer capable of deploying gateways to GCP
func NewGCPGatewayDeployer(cloud api.Cloud) (api.GatewayDeployer, error) {
	gcp, ok := cloud.(*gcpCloud)
	if !ok {
		return nil, errors.New("the cloud must be GCP")
	}

	return &gcpGatewayDeployer{gcp: gcp}, nil
}

func (d *gcpGatewayDeployer) Deploy(input api.GatewayDeployInput, reporter api.Reporter) error {
	// create the inbound and outbound firewall rules for submariner public ports
	reporter.Started("Opening public ports %q for cluster communications on GCP", formatPorts(input.PublicPorts))
	ingress := newExternalFirewallRules(d.gcp.projectID, d.gcp.infraID, input.PublicPorts)
	if err := d.gcp.openPorts(ingress); err != nil {
		reporter.Failed(err)
		return err
	}

	reporter.Succeeded("Opened public ports %q with firewall rules %q on GCP",
		formatPorts(input.PublicPorts), ingress.Name)

	return nil
}

func (d *gcpGatewayDeployer) Cleanup(reporter api.Reporter) error {
	// delete the inbound and outbound firewall rules to close submariner public ports
	ingressName := generateRuleName(d.gcp.infraID, publicPortsRuleName)

	if err := d.gcp.deleteFirewallRule(ingressName, reporter); err != nil {
		return err
	}

	return nil
}
