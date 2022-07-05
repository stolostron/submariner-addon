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
	"github.com/gophercloud/gophercloud/openstack/compute/v2/extensions/secgroups"
	"github.com/gophercloud/gophercloud/openstack/compute/v2/servers"
	"github.com/gophercloud/gophercloud/openstack/networking/v2/extensions/security/rules"
	"github.com/gophercloud/gophercloud/pagination"
	"github.com/pkg/errors"
	"github.com/submariner-io/cloud-prepare/pkg/api"
	"github.com/submariner-io/cloud-prepare/pkg/k8s"
)

type CloudInfo struct {
	Client    *gophercloud.ProviderClient
	InfraID   string
	Region    string
	K8sClient k8s.Interface
}

func (c *CloudInfo) openInternalPorts(infraID string, ports []api.PortSpec,
	computeClient, networkClient *gophercloud.ServiceClient,
) error {
	var group *secgroups.SecurityGroup
	groupName := infraID + internalSecurityGroupSuffix
	opts := secgroups.CreateOpts{
		Name:        groupName,
		Description: "Submariner Internal",
	}

	isFound, err := checkIfSecurityGroupPresent(groupName, computeClient)
	if err != nil {
		return err
	}

	if !isFound {
		group, err = secgroups.Create(computeClient, opts).Extract()
		if err != nil {
			return errors.WithMessagef(err, "creating security group failed")
		}

		for _, port := range ports {
			err = c.createSGRule(group.ID, group.ID, "", port.Port, port.Protocol, networkClient)
			if err != nil {
				return errors.WithMessage(err, "creating security group rule failed")
			}
		}
	}

	pager := servers.List(computeClient, servers.ListOpts{Name: c.InfraID})
	err = pager.EachPage(func(page pagination.Page) (bool, error) {
		serverList, err := servers.ExtractServers(page)
		if err != nil {
			return false, errors.WithMessage(err, "getting the server List failed")
		}
		for i := range serverList {
			found := false
			securityGroups := serverList[i].SecurityGroups
			for j := range securityGroups {
				existingGroupName, ok := securityGroups[j]["name"]
				if ok && existingGroupName == groupName {
					found = true
				}
			}
			if !found {
				err := secgroups.AddServer(computeClient, serverList[i].ID, groupName).ExtractErr()
				if err != nil {
					return false, errors.WithMessage(err, "failed to add the security group to the server")
				}
			}
		}

		return true, nil
	})

	return errors.WithMessagef(err, "failed to open ports")
}

func (c *CloudInfo) removeInternalFirewallRules(infraID string,
	computeClient *gophercloud.ServiceClient,
) error {
	groupName := infraID + internalSecurityGroupSuffix

	pager := servers.List(computeClient, servers.ListOpts{Name: c.InfraID})

	err := pager.EachPage(func(page pagination.Page) (bool, error) {
		serverList, err := servers.ExtractServers(page)
		if err != nil {
			return false, errors.WithMessage(err, "getting the server List failed")
		}
		for i := range serverList {
			err = secgroups.RemoveServer(computeClient, serverList[i].ID, groupName).ExtractErr()
			if err != nil {
				notFoundError := &gophercloud.ErrDefault404{}
				if errors.As(err, notFoundError) {
					continue
				}

				return false, errors.WithMessagef(err, "failed to remove the internal firewall for "+
					"the server: %q ", serverList[i].Name)
			}
		}

		return true, nil
	})

	return errors.WithMessage(err, "failed to remove security group from servers")
}

func (c *CloudInfo) createGWSecurityGroup(ports []api.PortSpec, groupName string, computeClient *gophercloud.ServiceClient,
	networkClient *gophercloud.ServiceClient,
) error {
	isFound, err := checkIfSecurityGroupPresent(groupName, computeClient)
	if err != nil {
		return err
	}

	if isFound {
		return nil
	}

	opts := secgroups.CreateOpts{
		Name:        groupName,
		Description: "Submariner Gateway",
	}

	group, err := secgroups.Create(computeClient, opts).Extract()
	if err != nil {
		return errors.WithMessage(err, "failed to create g/w security group")
	}

	for _, port := range ports {
		err = c.createSGRule(group.ID, "", allNetworkCIDR, port.Port, port.Protocol, networkClient)
		if err != nil {
			return errors.WithMessagef(err, "creating security group rule failed")
		}
	}

	return nil
}

func checkIfSecurityGroupPresent(groupName string, computeClient *gophercloud.ServiceClient) (bool, error) {
	pager := secgroups.List(computeClient)
	var isFound bool

	err := pager.EachPage(func(page pagination.Page) (bool, error) {
		serverList, err := secgroups.ExtractSecurityGroups(page)

		for _, s := range serverList {
			if s.Name == groupName {
				isFound = true
			}
		}
		return true, errors.WithMessagef(err, "failed to extract the security group %q from results", groupName)
	})

	return isFound, errors.WithMessagef(err, "error getting the security group : %q", groupName)
}

func (c *CloudInfo) openGatewayPort(groupName, nodeName string, computeClient *gophercloud.ServiceClient) error {
	opts := servers.ListOpts{Name: nodeName}
	pager := servers.List(computeClient, opts)

	err := pager.EachPage(func(page pagination.Page) (bool, error) {
		serverList, err := servers.ExtractServers(page)
		if err != nil {
			return false, errors.WithMessagef(err, "getting the server List failed for node %q", nodeName)
		}
		for i := range serverList {
			securityGroups := serverList[i].SecurityGroups
			for j := range securityGroups {
				existingGroupName, ok := securityGroups[j]["name"]
				if ok && existingGroupName == groupName {
					return true, nil
				}
			}
			err = secgroups.AddServer(computeClient, serverList[i].ID, groupName).ExtractErr()
			if err != nil {
				return false, errors.WithMessagef(err, "adding security group %q to the server %q failed",
					groupName, serverList[i].Name)
			}
		}

		return true, nil
	})

	return errors.WithMessagef(err, "open gateway ports failed")
}

func (c *CloudInfo) removeFirewallRulesFromGW(groupName, nodeName string, computeClient *gophercloud.ServiceClient) error {
	opts := servers.ListOpts{Name: nodeName}
	pager := servers.List(computeClient, opts)

	err := pager.EachPage(func(page pagination.Page) (bool, error) {
		serverList, err := servers.ExtractServers(page)
		if err != nil {
			return false, errors.WithMessagef(err, "getting the server list failed")
		}
		for i := range serverList {
			err = secgroups.RemoveServer(computeClient, serverList[i].ID, groupName).ExtractErr()
			if err != nil {
				notFoundError := &gophercloud.ErrDefault404{}
				if errors.As(err, notFoundError) {
					continue
				}

				return false, errors.WithMessagef(err, "failed to remove the firewall for"+
					" the server: %q", serverList[i].Name)
			}
		}

		return true, nil
	})

	return errors.WithMessagef(err, "removing firewall rules failed for security group %q", groupName)
}

func (c *CloudInfo) deleteSG(groupName string, computeClient *gophercloud.ServiceClient) error {
	pager := secgroups.List(computeClient)
	var isFound bool
	var securityGroupID string

	err := pager.EachPage(func(page pagination.Page) (bool, error) {
		serverList, err := secgroups.ExtractSecurityGroups(page)
		if err != nil {
			return false, errors.WithMessagef(err, "failed to list the security group %q", groupName)
		}
		for _, s := range serverList {
			if s.Name == groupName {
				isFound = true
				securityGroupID = s.ID
				return true, nil
			}
		}
		return true, errors.WithMessagef(err, "error finding the uuid for the security group: %q", groupName)
	})

	if err == nil && isFound {
		err = secgroups.Delete(computeClient, securityGroupID).ExtractErr()
	}

	return errors.WithMessagef(err, "error deleting the security group %q", groupName)
}

func (c *CloudInfo) createSGRule(group, remoteGroupID, remoteIPPrefix string, port uint16,
	protocol string, networkClient *gophercloud.ServiceClient,
) error {
	opts := rules.CreateOpts{
		Direction:      "ingress",
		EtherType:      rules.EtherType4,
		SecGroupID:     group,
		PortRangeMax:   int(port),
		PortRangeMin:   int(port),
		Protocol:       rules.RuleProtocol(protocol),
		RemoteGroupID:  remoteGroupID,
		RemoteIPPrefix: remoteIPPrefix,
	}

	_, err := rules.Create(networkClient, opts).Extract()

	return errors.WithMessagef(err, "failed creating security group rule with port %d , protocol %q,"+
		"remotegroupID %q, remoteIPprefix %q , in security group %q", port, protocol, remoteGroupID, remoteIPPrefix, group)
}
