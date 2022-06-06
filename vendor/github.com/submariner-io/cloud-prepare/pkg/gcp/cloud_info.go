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
	"github.com/pkg/errors"
	"github.com/submariner-io/admiral/pkg/reporter"
	gcpclient "github.com/submariner-io/cloud-prepare/pkg/gcp/client"
	"google.golang.org/api/compute/v1"
)

type CloudInfo struct {
	InfraID   string
	Region    string
	ProjectID string
	Client    gcpclient.Interface
}

// Open expected ports by creating related firewall rule.
// - if the firewall rule is not found, we will create it.
// - if the firewall rule is found and changed, we will update it.
func (c *CloudInfo) openPorts(rules ...*compute.Firewall) error {
	for _, rule := range rules {
		_, err := c.Client.GetFirewallRule(c.ProjectID, rule.Name)
		if gcpclient.IsGCPNotFoundError(err) {
			if err := c.Client.InsertFirewallRule(c.ProjectID, rule); err != nil {
				return errors.Wrapf(err, "error inserting firewall rule %#v", rule)
			}

			continue
		}

		if err != nil {
			return errors.Wrapf(err, "error retrieving firewall rule %q", rule.Name)
		}

		if err := c.Client.UpdateFirewallRule(c.ProjectID, rule.Name, rule); err != nil {
			return errors.Wrapf(err, "error updating firewall rule %#v", rule)
		}
	}

	return nil
}

func (c *CloudInfo) deleteFirewallRule(name string, status reporter.Interface) error {
	status.Start("Deleting firewall rule %q on GCP", name)

	if err := c.Client.DeleteFirewallRule(c.ProjectID, name); err != nil {
		if !gcpclient.IsGCPNotFoundError(err) {
			return status.Error(err, "unable to delete firewall rule %q", name)
		}
	}

	status.Success("Deleted firewall rule %q on GCP", name)

	return nil
}
