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
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/services/network/mgmt/2021-03-01/network"
	"github.com/Azure/go-autorest/autorest"
	"github.com/pkg/errors"
	"github.com/submariner-io/cloud-prepare/pkg/api"
	"github.com/submariner-io/cloud-prepare/pkg/k8s"
	"k8s.io/utils/pointer"
)

const (
	internalSecurityGroupSuffix = "-nsg"
	externalSecurityGroupSuffix = "-submariner-external-sg"
	internalSecurityRulePrefix  = "Submariner-Internal-"
	externalSecurityRulePrefix  = "Submariner-External-"
	allNetworkCIDR              = "0.0.0.0/0"
	basePriorityInternal        = 2500
	baseExternalInternal        = 3500
)

type CloudInfo struct {
	SubscriptionID string
	InfraID        string
	Region         string
	BaseGroupName  string
	Authorizer     autorest.Authorizer
	K8sClient      k8s.Interface
}

func (c *CloudInfo) openInternalPorts(infraID string, ports []api.PortSpec, sgClient *network.SecurityGroupsClient) error {
	groupName := infraID + internalSecurityGroupSuffix

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	nwSecurityGroup, err := sgClient.Get(ctx, c.BaseGroupName, groupName, "")
	if err != nil {
		return errors.Wrapf(err, "error getting the security group %q", groupName)
	}

	isFound := checkIfSecurityRulesPresent(nwSecurityGroup)
	if isFound {
		return nil
	}

	securityRules := *nwSecurityGroup.SecurityRules
	for i, port := range ports {
		securityRules = append(securityRules, c.createSecurityRule(internalSecurityRulePrefix,
			port.Protocol, port.Port, int32(basePriorityInternal+i), network.SecurityRuleDirectionInbound),
			c.createSecurityRule(internalSecurityRulePrefix, port.Protocol, port.Port,
				int32(basePriorityInternal+i), network.SecurityRuleDirectionOutbound))
	}

	nwSecurityGroup.SecurityRules = &securityRules

	future, err := sgClient.CreateOrUpdate(ctx, c.BaseGroupName, groupName, nwSecurityGroup)
	if err != nil {
		return errors.Wrapf(err, "updating security group %q with submariner rules failed", groupName)
	}

	err = future.WaitForCompletionRef(ctx, sgClient.Client)

	return errors.Wrapf(err, "error updating  security group %q with submariner rules", groupName)
}

func (c *CloudInfo) removeInternalFirewallRules(infraID string, sgClient *network.SecurityGroupsClient) error {
	groupName := infraID + internalSecurityGroupSuffix

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	nwSecurityGroup, err := sgClient.Get(ctx, c.BaseGroupName, groupName, "")
	if err != nil {
		return errors.Wrapf(err, "error getting the security group %q", groupName)
	}

	securityRules := []network.SecurityRule{}

	for _, existingSGRule := range *nwSecurityGroup.SecurityRules {
		if existingSGRule.Name != nil && !strings.Contains(*existingSGRule.Name, internalSecurityRulePrefix) {
			securityRules = append(securityRules, existingSGRule)
		}
	}

	nwSecurityGroup.SecurityRules = &securityRules

	future, err := sgClient.CreateOrUpdate(ctx, c.BaseGroupName, groupName, nwSecurityGroup)
	if err != nil {
		return errors.Wrapf(err, "removing submariner rules from  security group %q failed", groupName)
	}

	err = future.WaitForCompletionRef(ctx, sgClient.Client)

	return errors.Wrapf(err, "removing submariner rules from security group %q failed", groupName)
}

func checkIfSecurityRulesPresent(securityGroup network.SecurityGroup) bool {
	for _, existingSGRule := range *securityGroup.SecurityRules {
		if existingSGRule.Name != nil && strings.Contains(*existingSGRule.Name, internalSecurityRulePrefix) {
			return true
		}
	}

	return false
}

func (c *CloudInfo) createSecurityRule(securityRulePrfix, protocol string, port uint16, priority int32,
	ruleDirection network.SecurityRuleDirection,
) network.SecurityRule {
	return network.SecurityRule{
		Name: pointer.String(securityRulePrfix + protocol + "-" + strconv.Itoa(int(port)) + "-" + string(ruleDirection)),
		SecurityRulePropertiesFormat: &network.SecurityRulePropertiesFormat{
			Protocol:                 network.SecurityRuleProtocol(protocol),
			DestinationPortRange:     pointer.String(strconv.Itoa(int(port)) + "-" + strconv.Itoa(int(port))),
			SourceAddressPrefix:      pointer.String(allNetworkCIDR),
			DestinationAddressPrefix: pointer.String(allNetworkCIDR),
			SourcePortRange:          pointer.String("*"),
			Access:                   network.SecurityRuleAccessAllow,
			Direction:                ruleDirection,
			Priority:                 pointer.Int32(priority),
		},
	}
}

func (c *CloudInfo) createGWSecurityGroup(groupName string, ports []api.PortSpec, sgClient *network.SecurityGroupsClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	isFound := checkIfSecurityGroupPresent(ctx, groupName, sgClient, c.BaseGroupName)
	if isFound {
		return nil
	}

	securityRules := []network.SecurityRule{}
	for i, port := range ports {
		securityRules = append(securityRules, c.createSecurityRule(externalSecurityRulePrefix, port.Protocol,
			port.Port, int32(baseExternalInternal+i), network.SecurityRuleDirectionInbound),
			c.createSecurityRule(externalSecurityRulePrefix, port.Protocol, port.Port,
				int32(baseExternalInternal+i), network.SecurityRuleDirectionOutbound))
	}

	nwSecurityGroup := network.SecurityGroup{
		Name:     &groupName,
		Location: pointer.String(c.Region),
		SecurityGroupPropertiesFormat: &network.SecurityGroupPropertiesFormat{
			SecurityRules: &securityRules,
		},
	}

	future, err := sgClient.CreateOrUpdate(ctx, c.BaseGroupName, groupName, nwSecurityGroup)
	if err != nil {
		return errors.Wrapf(err, "creating security group %q failed", groupName)
	}

	err = future.WaitForCompletionRef(ctx, sgClient.Client)

	return errors.Wrapf(err, "Error creating  security group %v ", groupName)
}

func (c *CloudInfo) prepareGWInterface(nodeName, groupName string, nsgClient *network.SecurityGroupsClient,
	nwClient *network.InterfacesClient, pubIPClient *network.PublicIPAddressesClient,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	nwSecurityGroup, err := nsgClient.Get(ctx, c.BaseGroupName, groupName, "")
	if err != nil {
		return errors.Wrapf(err, "error getting the submariner gateway security group %q", groupName)
	}

	publicIPName := nodeName + "-pub"

	var pubIP network.PublicIPAddress

	pubIP, err = getPublicIP(ctx, publicIPName, pubIPClient, c.BaseGroupName)

	if err != nil {
		var err error

		pubIP, err = c.CreatePublicIP(ctx, publicIPName, pubIPClient)
		if err != nil {
			return errors.Wrapf(err, "failed to create public IP %q", publicIPName)
		}
	}

	interfaceName := nodeName + "-nic"

	nwInterface, err := nwClient.Get(ctx, c.BaseGroupName, interfaceName, "")
	if err != nil {
		return errors.Wrapf(err, "error getting the interfaces %q from resource group %q", interfaceName, c.BaseGroupName)
	}

	nwInterface.InterfacePropertiesFormat.NetworkSecurityGroup = &nwSecurityGroup

	nwInterfaceIPConfiguration := *nwInterface.InterfacePropertiesFormat.IPConfigurations
	for i := range nwInterfaceIPConfiguration {
		if nwInterfaceIPConfiguration[i].Primary != nil && *nwInterfaceIPConfiguration[i].Primary {
			nwInterfaceIPConfiguration[i].PublicIPAddress = &pubIP
			break
		}
	}

	future, err := nwClient.CreateOrUpdate(ctx, c.BaseGroupName, *nwInterface.Name, nwInterface)
	if err != nil {
		return errors.Wrapf(err, "adding security group %q and public IP %q to interface %q failed", *nwSecurityGroup.Name,
			*pubIP.Name, *nwInterface.ID)
	}

	err = future.WaitForCompletionRef(ctx, nwClient.Client)
	if err != nil {
		return errors.Wrapf(err, "updating interface %q failed", *nwInterface.Name)
	}

	return errors.Wrapf(err, "waiting for the g/w interface %q to be updated failed", interfaceName)
}

func (c *CloudInfo) cleanupGWInterface(infraID string, sgClient *network.SecurityGroupsClient,
	nwClient *network.InterfacesClient,
) error {
	groupName := infraID + externalSecurityGroupSuffix

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	isFound := checkIfSecurityGroupPresent(ctx, groupName, sgClient, c.BaseGroupName)

	if !isFound {
		return nil
	}

	nwSecurityGroup, err := sgClient.Get(ctx, c.BaseGroupName, groupName, "")
	if err != nil {
		return errors.Wrapf(err, "error getting the submariner gateway security group %q", groupName)
	}

	interfacesInRG, err := nwClient.List(ctx, c.BaseGroupName)
	if err != nil {
		return errors.Wrapf(err, "error getting the interfaces list in resource group %q", c.BaseGroupName)
	}

	if nwSecurityGroup.SecurityGroupPropertiesFormat != nil && nwSecurityGroup.SecurityGroupPropertiesFormat.NetworkInterfaces != nil {
		for _, interfaceWithID := range *nwSecurityGroup.SecurityGroupPropertiesFormat.NetworkInterfaces {
			interfacesInRGValues := interfacesInRG.Values()

			var interfaceWithSG network.Interface

			for _, values := range interfacesInRGValues {
				if *values.ID == *interfaceWithID.ID {
					interfaceWithSG = values
					break
				}
			}

			interfaceWithSG.InterfacePropertiesFormat.NetworkSecurityGroup = nil
			if interfaceWithSG.InterfacePropertiesFormat.IPConfigurations != nil {
				removePublicIP(*interfaceWithSG.InterfacePropertiesFormat.IPConfigurations)
			}

			future, err := nwClient.CreateOrUpdate(ctx, c.BaseGroupName, *interfaceWithSG.Name, interfaceWithSG)
			if err != nil {
				return errors.Wrapf(err, "removing  security group %q from interface %q failed", groupName,
					*interfaceWithSG.ID)
			}

			err = future.WaitForCompletionRef(ctx, sgClient.Client)
			if err != nil {
				return errors.Wrapf(err, "updating  interface  %q failed", *interfaceWithSG.Name)
			}
		}
	}

	if err != nil {
		return errors.Wrapf(err, "waiting for the submariner gateway security group  %q to be updated failed", groupName)
	}

	deleteFuture, err := sgClient.Delete(ctx, c.BaseGroupName, groupName)
	if err != nil {
		return errors.Wrapf(err, "deleting security group %q failed", groupName)
	}

	err = deleteFuture.WaitForCompletionRef(ctx, sgClient.Client)

	if err != nil {
		return errors.Wrapf(err, "waiting for the submariner gateway  ecurity group  %q to be deleted failed", groupName)
	}

	return errors.WithMessage(err, "failed to remove the submariner gateway security group from servers")
}

func removePublicIP(nwInterfaceIPConfiguration []network.InterfaceIPConfiguration) {
	for i := range nwInterfaceIPConfiguration {
		if nwInterfaceIPConfiguration[i].Primary != nil && *nwInterfaceIPConfiguration[i].Primary {
			nwInterfaceIPConfiguration[i].PublicIPAddress = nil
			break
		}
	}
}

func checkIfSecurityGroupPresent(ctx context.Context, groupName string, networkClient *network.SecurityGroupsClient,
	baseGroupName string,
) bool {
	_, err := networkClient.Get(ctx, baseGroupName, groupName, "")

	return err == nil
}

func getPublicIP(ctx context.Context, publicIPName string, pubIPClient *network.PublicIPAddressesClient, baseGroupName string,
) (network.PublicIPAddress, error) {
	publicIP, err := pubIPClient.Get(ctx, baseGroupName, publicIPName, "")

	return publicIP, errors.Wrapf(err, "error getting public ip: %q", publicIPName)
}

func (c *CloudInfo) CreatePublicIP(ctx context.Context, ipName string, ipClient *network.PublicIPAddressesClient,
) (ip network.PublicIPAddress, err error) {
	future, err := ipClient.CreateOrUpdate(
		ctx,
		c.BaseGroupName,
		ipName,
		network.PublicIPAddress{
			Name: pointer.String(ipName),
			PublicIPAddressPropertiesFormat: &network.PublicIPAddressPropertiesFormat{
				PublicIPAddressVersion:   network.IPVersionIPv4,
				PublicIPAllocationMethod: network.IPAllocationMethodStatic,
			},
			Location: &c.Region,
			Sku: &network.PublicIPAddressSku{
				Name: network.PublicIPAddressSkuNameStandard,
			},
		},
	)
	if err != nil {
		return ip, errors.Wrapf(err, "cannot create public ip address: %q", ipName)
	}

	err = future.WaitForCompletionRef(ctx, ipClient.Client)
	if err != nil {
		return ip, errors.Wrapf(err, "cannot get public ip address create or update future response: %q", ipName)
	}

	ipAddress, err := future.Result(*ipClient)

	return ipAddress, errors.Wrapf(err, "Error getting the public ip %q", ipName)
}

func (c *CloudInfo) DeletePublicIP(ctx context.Context, ipClient *network.PublicIPAddressesClient, ipName string,
) (err error) {
	future, err := ipClient.Delete(ctx, c.BaseGroupName, ipName)
	if err != nil {
		return errors.Wrapf(err, "failed to delete public ip : %q", ipName)
	}

	err = future.WaitForCompletionRef(ctx, ipClient.Client)
	if err != nil {
		return errors.Wrapf(err, "failed to remove the public ip : %q", ipName)
	}

	return nil
}
