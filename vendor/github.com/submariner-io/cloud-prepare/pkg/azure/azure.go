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
	"fmt"
	"strings"

	reporterInterface "github.com/submariner-io/admiral/pkg/reporter"
	"github.com/submariner-io/cloud-prepare/pkg/api"
)

type azureCloud struct {
	CloudInfo
}

// NewCloud creates a new api.Cloud instance which can prepare Azure for Submariner to be deployed on it.
func NewCloud(info *CloudInfo) api.Cloud {
	return &azureCloud{
		CloudInfo: *info,
	}
}

func (az *azureCloud) PrepareForSubmariner(input api.PrepareForSubmarinerInput, reporter reporterInterface.Interface) error {
	reporter.Start("Opening internal ports for intra-cluster communications on Azure")

	nsgClient, err := az.getNsgClient()
	if err != nil {
		return reporter.Error(err, "Failed to get network security groups client")
	}

	if err := az.openInternalPorts(az.InfraID, input.InternalPorts, nsgClient); err != nil {
		return reporter.Error(err, "Failed to open internal ports")
	}

	reporter.Success("Opened internal ports %q for intra-cluster communications on Azure",
		formatPorts(input.InternalPorts))

	return nil
}

func (az *azureCloud) CleanupAfterSubmariner(reporter reporterInterface.Interface) error {
	reporter.Start("Revoking intra-cluster communication permissions")

	nsgClient, err := az.getNsgClient()
	if err != nil {
		return reporter.Error(err, "Failed to get network security groups client")
	}

	if err := az.removeInternalFirewallRules(az.InfraID, nsgClient); err != nil {
		return reporter.Error(err, "Failed to revoke intra-cluster communication permissions")
	}

	reporter.Success("Revoked intra-cluster communication permissions")

	return nil
}

func formatPorts(ports []api.PortSpec) string {
	portStrs := []string{}
	for _, port := range ports {
		portStrs = append(portStrs, fmt.Sprintf("%d/%s", port.Port, port.Protocol))
	}

	return strings.Join(portStrs, ", ")
}
