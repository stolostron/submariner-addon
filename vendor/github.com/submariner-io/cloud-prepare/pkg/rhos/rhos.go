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
	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/pkg/errors"
	"github.com/submariner-io/cloud-prepare/pkg/api"
)

const (
	gwSecurityGroupSuffix       = "-submariner-gw-sg"
	internalSecurityGroupSuffix = "-submariner-internal-sg"
	submarinerGatewayNodeTag    = "submariner-io-gateway-node"
	allNetworkCIDR              = "0.0.0.0/0"
)

type rhosCloud struct {
	CloudInfo
}

// NewCloud creates a new api.Cloud instance which can prepare RHOS for Submariner to be deployed on it.
func NewCloud(info CloudInfo) api.Cloud {
	return &rhosCloud{
		CloudInfo: info,
	}
}

func (rc *rhosCloud) PrepareForSubmariner(input api.PrepareForSubmarinerInput, reporter api.Reporter) error {
	reporter.Started("Opening internal ports for intra-cluster communications on RHOS")

	computeClient, err := openstack.NewComputeV2(rc.Client, gophercloud.EndpointOpts{Region: rc.Region})
	if err != nil {
		return errors.WithMessage(err, "Error creating the compute client")
	}

	networkClient, err := openstack.NewNetworkV2(rc.Client, gophercloud.EndpointOpts{Region: rc.Region})
	if err != nil {
		return errors.WithMessage(err, "Error creating the network client")
	}

	if err := rc.openInternalPorts(rc.InfraID, input.InternalPorts, computeClient, networkClient); err != nil {
		reporter.Failed(err)
		return err
	}

	reporter.Succeeded("Opened internal ports %q for intra-cluster communications on RHOS",
		formatPorts(input.InternalPorts))

	return nil
}

func (rc *rhosCloud) CleanupAfterSubmariner(reporter api.Reporter) error {
	reporter.Started("Revoking intra-cluster communication permissions")

	computeClient, err := openstack.NewComputeV2(rc.Client, gophercloud.EndpointOpts{Region: rc.Region})
	if err != nil {
		return errors.WithMessagef(err, "creating compute client failed for region %q", rc.Region)
	}

	if err := rc.removeInternalFirewallRules(rc.InfraID, computeClient); err != nil {
		reporter.Failed(err)
		return err
	}

	if err := rc.deleteSG(rc.InfraID+internalSecurityGroupSuffix, computeClient); err != nil {
		return err
	}

	reporter.Succeeded("Revoked intra-cluster communication permissions")

	return nil
}
