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

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/pkg/errors"
	"github.com/submariner-io/cloud-prepare/pkg/api"
	"github.com/submariner-io/cloud-prepare/pkg/k8s"
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
	SubscriptionID  string
	InfraID         string
	Region          string
	BaseGroupName   string
	TokenCredential azcore.TokenCredential
	K8sClient       k8s.Interface
}

//nolint:wrapcheck // Let the caller wrap it.
func (c *CloudInfo) getNsgClient() (*armnetwork.SecurityGroupsClient, error) {
	return armnetwork.NewSecurityGroupsClient(c.SubscriptionID, c.TokenCredential, nil)
}

//nolint:wrapcheck // Let the caller wrap it.
func (c *CloudInfo) getInterfacesClient() (*armnetwork.InterfacesClient, error) {
	return armnetwork.NewInterfacesClient(c.SubscriptionID, c.TokenCredential, nil)
}

//nolint:wrapcheck // Let the caller wrap it.
func (c *CloudInfo) getPublicIPClient() (*armnetwork.PublicIPAddressesClient, error) {
	return armnetwork.NewPublicIPAddressesClient(c.SubscriptionID, c.TokenCredential, nil)
}

//nolint:wrapcheck // Let the caller wrap it.
func (c *CloudInfo) getResourceSKUClient() (*armcompute.ResourceSKUsClient, error) {
	return armcompute.NewResourceSKUsClient(c.SubscriptionID, c.TokenCredential, nil)
}

func (c *CloudInfo) openInternalPorts(infraID string, ports []api.PortSpec, nsgClient *armnetwork.SecurityGroupsClient) error {
	groupName := infraID + internalSecurityGroupSuffix

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	nwSecurityGroup, err := nsgClient.Get(ctx, c.BaseGroupName, groupName, nil)
	if err != nil {
		return errors.Wrapf(err, "error getting the security group %q", groupName)
	}

	if nwSecurityGroup.Properties == nil {
		nwSecurityGroup.Properties = &armnetwork.SecurityGroupPropertiesFormat{}
	}

	isFound := checkIfSecurityRulesPresent(nwSecurityGroup.Properties.SecurityRules)
	if isFound {
		return nil
	}

	for i, port := range ports {
		nwSecurityGroup.Properties.SecurityRules = append(nwSecurityGroup.Properties.SecurityRules,
			c.createSecurityRule(internalSecurityRulePrefix, armnetwork.SecurityRuleProtocol(port.Protocol), port.Port,
				int32(basePriorityInternal+i), armnetwork.SecurityRuleDirectionInbound),
			c.createSecurityRule(internalSecurityRulePrefix, armnetwork.SecurityRuleProtocol(port.Protocol), port.Port,
				int32(basePriorityInternal+i), armnetwork.SecurityRuleDirectionOutbound))
	}

	poller, err := nsgClient.BeginCreateOrUpdate(ctx, c.BaseGroupName, groupName, nwSecurityGroup.SecurityGroup, nil)
	if err != nil {
		return errors.Wrapf(err, "updating security group %q with submariner rules failed", groupName)
	}

	_, err = poller.PollUntilDone(ctx, nil)

	return errors.Wrapf(err, "error updating  security group %q with submariner rules", groupName)
}

func (c *CloudInfo) removeInternalFirewallRules(infraID string, nsgClient *armnetwork.SecurityGroupsClient) error {
	groupName := infraID + internalSecurityGroupSuffix

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	nwSecurityGroup, err := nsgClient.Get(ctx, c.BaseGroupName, groupName, nil)
	if err != nil {
		return errors.Wrapf(err, "error getting the security group %q", groupName)
	}

	if nwSecurityGroup.Properties == nil {
		return nil
	}

	securityRules := []*armnetwork.SecurityRule{}

	for _, existingSGRule := range nwSecurityGroup.Properties.SecurityRules {
		if existingSGRule.Name != nil && !strings.Contains(*existingSGRule.Name, internalSecurityRulePrefix) {
			securityRules = append(securityRules, existingSGRule)
		}
	}

	nwSecurityGroup.Properties.SecurityRules = securityRules

	poller, err := nsgClient.BeginCreateOrUpdate(ctx, c.BaseGroupName, groupName, nwSecurityGroup.SecurityGroup, nil)
	if err != nil {
		return errors.Wrapf(err, "removing submariner rules from  security group %q failed", groupName)
	}

	_, err = poller.PollUntilDone(ctx, nil)

	return errors.Wrapf(err, "removing submariner rules from security group %q failed", groupName)
}

func checkIfSecurityRulesPresent(securityRules []*armnetwork.SecurityRule) bool {
	for _, existingSGRule := range securityRules {
		if existingSGRule.Name != nil && strings.Contains(*existingSGRule.Name, internalSecurityRulePrefix) {
			return true
		}
	}

	return false
}

func (c *CloudInfo) createSecurityRule(securityRulePrfix string, protocol armnetwork.SecurityRuleProtocol, port uint16, priority int32,
	ruleDirection armnetwork.SecurityRuleDirection,
) *armnetwork.SecurityRule {
	access := armnetwork.SecurityRuleAccessAllow

	return &armnetwork.SecurityRule{
		Name: to.StringPtr(securityRulePrfix + string(protocol) + "-" + strconv.Itoa(int(port)) + "-" + string(ruleDirection)),
		Properties: &armnetwork.SecurityRulePropertiesFormat{
			Protocol:                 &protocol,
			DestinationPortRange:     to.StringPtr(strconv.Itoa(int(port)) + "-" + strconv.Itoa(int(port))),
			SourceAddressPrefix:      to.StringPtr(allNetworkCIDR),
			DestinationAddressPrefix: to.StringPtr(allNetworkCIDR),
			SourcePortRange:          to.StringPtr("*"),
			Access:                   &access,
			Direction:                &ruleDirection,
			Priority:                 to.Int32Ptr(priority),
		},
	}
}

func (c *CloudInfo) createGWSecurityGroup(groupName string, ports []api.PortSpec, nsgClient *armnetwork.SecurityGroupsClient) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	isFound := c.checkIfSecurityGroupPresent(ctx, groupName, nsgClient)
	if isFound {
		return nil
	}

	securityRules := []*armnetwork.SecurityRule{}
	for i, port := range ports {
		securityRules = append(securityRules,
			c.createSecurityRule(externalSecurityRulePrefix, armnetwork.SecurityRuleProtocol(port.Protocol), port.Port,
				int32(baseExternalInternal+i), armnetwork.SecurityRuleDirectionInbound),
			c.createSecurityRule(externalSecurityRulePrefix, armnetwork.SecurityRuleProtocol(port.Protocol), port.Port,
				int32(baseExternalInternal+i), armnetwork.SecurityRuleDirectionOutbound))
	}

	nwSecurityGroup := armnetwork.SecurityGroup{
		Name:     &groupName,
		Location: to.StringPtr(c.Region),
		Properties: &armnetwork.SecurityGroupPropertiesFormat{
			SecurityRules: securityRules,
		},
	}

	poller, err := nsgClient.BeginCreateOrUpdate(ctx, c.BaseGroupName, groupName, nwSecurityGroup, nil)
	if err != nil {
		return errors.Wrapf(err, "creating security group %q failed", groupName)
	}

	_, err = poller.PollUntilDone(ctx, nil)

	return errors.Wrapf(err, "Error creating  security group %v ", groupName)
}

func (c *CloudInfo) prepareGWInterface(nodeName, groupName string, nsgClient *armnetwork.SecurityGroupsClient,
	nwClient *armnetwork.InterfacesClient, pubIPClient *armnetwork.PublicIPAddressesClient,
) error {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	nwSecurityGroup, err := nsgClient.Get(ctx, c.BaseGroupName, groupName, nil)
	if err != nil {
		return errors.Wrapf(err, "error getting the submariner gateway security group %q", groupName)
	}

	publicIPName := nodeName + "-pub"

	var pubIP armnetwork.PublicIPAddress

	pubIP, err = c.getPublicIP(ctx, publicIPName, pubIPClient)

	if err != nil {
		var err error

		pubIP, err = c.createPublicIP(ctx, publicIPName, pubIPClient)
		if err != nil {
			return errors.Wrapf(err, "failed to create public IP %q", publicIPName)
		}
	}

	interfaceName := nodeName + "-nic"

	nwInterface, err := nwClient.Get(ctx, c.BaseGroupName, interfaceName, nil)
	if err != nil {
		return errors.Wrapf(err, "error getting the interfaces %q from resource group %q", interfaceName, c.BaseGroupName)
	}

	if nwInterface.Properties == nil {
		nwInterface.Properties = &armnetwork.InterfacePropertiesFormat{}
	}

	nwInterface.Properties.NetworkSecurityGroup = &nwSecurityGroup.SecurityGroup

	for i := range nwInterface.Properties.IPConfigurations {
		props := nwInterface.Properties.IPConfigurations[i].Properties
		if props != nil && props.Primary != nil && *props.Primary {
			nwInterface.Properties.IPConfigurations[i].Properties.PublicIPAddress = &pubIP
			break
		}
	}

	poller, err := nwClient.BeginCreateOrUpdate(ctx, c.BaseGroupName, *nwInterface.Name, nwInterface.Interface, nil)
	if err != nil {
		return errors.Wrapf(err, "adding security group %q and public IP %q to interface %q failed", *nwSecurityGroup.Name,
			*pubIP.Name, *nwInterface.ID)
	}

	_, err = poller.PollUntilDone(ctx, nil)

	return errors.Wrapf(err, "updating interface %q failed", *nwInterface.Name)
}

func (c *CloudInfo) cleanupGWInterface(infraID string, nsgClient *armnetwork.SecurityGroupsClient,
	nwClient *armnetwork.InterfacesClient,
) error {
	groupName := infraID + externalSecurityGroupSuffix

	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Second)
	defer cancel()

	isFound := c.checkIfSecurityGroupPresent(ctx, groupName, nsgClient)

	if !isFound {
		return nil
	}

	nwSecurityGroup, err := nsgClient.Get(ctx, c.BaseGroupName, groupName, nil)
	if err != nil {
		return errors.Wrapf(err, "error getting the submariner gateway security group %q", groupName)
	}

	interfacesInRGMap := map[string]*armnetwork.Interface{}

	interfacesInRGPager := nwClient.NewListPager(c.BaseGroupName, nil)
	for interfacesInRGPager.More() {
		nextResult, err := interfacesInRGPager.NextPage(ctx)
		if err != nil {
			return errors.Wrapf(err, "error paging the resource group interfaces")
		}

		for _, interfacesInRG := range nextResult.Value {
			interfacesInRGMap[*interfacesInRG.ID] = interfacesInRG
		}
	}

	if nwSecurityGroup.Properties == nil {
		nwSecurityGroup.Properties = &armnetwork.SecurityGroupPropertiesFormat{}
	}

	for _, interfaceWithID := range nwSecurityGroup.Properties.NetworkInterfaces {
		interfaceWithSG := interfacesInRGMap[*interfaceWithID.ID]
		if interfaceWithSG == nil {
			continue
		}

		if interfaceWithSG.Properties != nil {
			interfaceWithSG.Properties.NetworkSecurityGroup = nil
			if interfaceWithSG.Properties.IPConfigurations != nil {
				removePublicIP(interfaceWithSG.Properties.IPConfigurations)
			}
		}

		poller, err := nwClient.BeginCreateOrUpdate(ctx, c.BaseGroupName, *interfaceWithSG.Name, *interfaceWithSG, nil)
		if err != nil {
			return errors.Wrapf(err, "removing security group %q from interface %q failed", groupName,
				*interfaceWithSG.ID)
		}

		_, err = poller.PollUntilDone(ctx, nil)
		if err != nil {
			return errors.Wrapf(err, "updating interface %q failed", *interfaceWithSG.Name)
		}
	}

	if err != nil {
		return errors.Wrapf(err, "waiting for the submariner gateway security group  %q to be updated failed", groupName)
	}

	poller, err := nsgClient.BeginDelete(ctx, c.BaseGroupName, groupName, nil)
	if err != nil {
		return errors.Wrapf(err, "deleting security group %q failed", groupName)
	}

	_, err = poller.PollUntilDone(ctx, nil)

	return errors.WithMessage(err, "failed to remove the submariner gateway security group from servers")
}

func removePublicIP(nwInterfaceIPConfiguration []*armnetwork.InterfaceIPConfiguration) {
	for i := range nwInterfaceIPConfiguration {
		if nwInterfaceIPConfiguration[i].Properties != nil && nwInterfaceIPConfiguration[i].Properties.Primary != nil &&
			*nwInterfaceIPConfiguration[i].Properties.Primary {
			nwInterfaceIPConfiguration[i].Properties.PublicIPAddress = nil
			break
		}
	}
}

func (c *CloudInfo) checkIfSecurityGroupPresent(ctx context.Context, groupName string, nsgClient *armnetwork.SecurityGroupsClient) bool {
	_, err := nsgClient.Get(ctx, c.BaseGroupName, groupName, nil)

	return err == nil
}

func (c *CloudInfo) getPublicIP(ctx context.Context, publicIPName string, pubIPClient *armnetwork.PublicIPAddressesClient,
) (armnetwork.PublicIPAddress, error) {
	resp, err := pubIPClient.Get(ctx, c.BaseGroupName, publicIPName, nil)

	return resp.PublicIPAddress, errors.Wrapf(err, "error getting public ip: %q", publicIPName)
}

func (c *CloudInfo) createPublicIP(ctx context.Context, ipName string, ipClient *armnetwork.PublicIPAddressesClient,
) (ip armnetwork.PublicIPAddress, err error) {
	ipVersion := armnetwork.IPVersionIPv4
	ipAllocMethod := armnetwork.IPAllocationMethodStatic
	skuName := armnetwork.PublicIPAddressSKUNameStandard

	poller, err := ipClient.BeginCreateOrUpdate(
		ctx,
		c.BaseGroupName,
		ipName,
		armnetwork.PublicIPAddress{
			Name: to.StringPtr(ipName),
			Properties: &armnetwork.PublicIPAddressPropertiesFormat{
				PublicIPAddressVersion:   &ipVersion,
				PublicIPAllocationMethod: &ipAllocMethod,
			},
			Location: &c.Region,
			SKU: &armnetwork.PublicIPAddressSKU{
				Name: &skuName,
			},
		}, nil)
	if err != nil {
		return ip, errors.Wrapf(err, "cannot create public ip address: %q", ipName)
	}

	resp, err := poller.PollUntilDone(ctx, nil)
	if err != nil {
		return ip, errors.Wrapf(err, "cannot get public ip address create or update response: %q", ipName)
	}

	return resp.PublicIPAddress, nil
}

func (c *CloudInfo) deletePublicIP(ctx context.Context, ipClient *armnetwork.PublicIPAddressesClient, ipName string) (err error) {
	poller, err := ipClient.BeginDelete(ctx, c.BaseGroupName, ipName, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to delete public ip : %q", ipName)
	}

	_, err = poller.PollUntilDone(ctx, nil)

	return errors.Wrapf(err, "failed to delete public ip : %q", ipName)
}
