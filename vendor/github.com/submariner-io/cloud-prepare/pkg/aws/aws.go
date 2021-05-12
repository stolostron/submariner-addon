/*
© 2021 Red Hat, Inc. and others.

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
package aws

import (
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/submariner-io/cloud-prepare/pkg/api"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
)

const (
	messageRetrieveVPCID          = "Retrieving VPC ID"
	messageRetrievedVPCID         = "Retrieved VPC ID %s"
	messageValidatePrerequisites  = "Validating pre-requisites"
	messageValidatedPrerequisites = "Validated pre-requisites"
)

// MachineSetDeployer can deploy and delete machinesets from OCP
type MachineSetDeployer interface {
	// Deploy makes sure to deploy the given machine set (creating or updating it)
	Deploy(machineSet *unstructured.Unstructured) error

	// Delete will remove the given machineset
	Delete(machineSet *unstructured.Unstructured) error
}

type k8sMachineSetDeployer struct {
	k8sConfig *rest.Config
}

// NewK8sMachinesetDeployer returns a MachineSetDeployer capable deploying directly to Kubernetes
func NewK8sMachinesetDeployer(k8sConfig *rest.Config) MachineSetDeployer {
	return &k8sMachineSetDeployer{k8sConfig: k8sConfig}
}

type awsCloud struct {
	client         ec2iface.EC2API
	gwDeployer     MachineSetDeployer
	gwInstanceType string
	infraID        string
	region         string
}

// NewCloud creates a new api.Cloud instance which can prepare AWS for Submariner to be deployed on it
func NewCloud(gwDeployer MachineSetDeployer, client ec2iface.EC2API, infraID, region, gwInstanceType string) api.Cloud {
	return &awsCloud{
		client:         client,
		gwDeployer:     gwDeployer,
		gwInstanceType: gwInstanceType,
		infraID:        infraID,
		region:         region,
	}
}

func (ac *awsCloud) PrepareForSubmariner(input api.PrepareForSubmarinerInput, reporter api.Reporter) error {
	reporter.Started(messageRetrieveVPCID)

	vpcID, err := ac.getVpcID()
	if err != nil {
		reporter.Failed(err)
		return err
	}

	reporter.Succeeded(messageRetrievedVPCID, vpcID)

	reporter.Started(messageValidatePrerequisites)

	err = ac.validatePreparePrerequisites(vpcID, input)
	if err != nil {
		reporter.Failed(err)
		return err
	}

	reporter.Succeeded(messageValidatedPrerequisites)

	for _, port := range input.InternalPorts {
		reporter.Started("Opening port %v protocol %s for intra-cluster communications", port.Port, port.Protocol)
		err = ac.allowPortInCluster(vpcID, port.Port, port.Protocol)
		if err != nil {
			reporter.Failed(err)
			return err
		}

		reporter.Succeeded("Opened port %v protocol %s for intra-cluster communications", port.Port, port.Protocol)
	}

	reporter.Started("Creating Submariner gateway security group")

	gatewaySG, err := ac.createGatewaySG(vpcID, input.PublicPorts)
	if err != nil {
		reporter.Failed(err)
		return err
	}

	reporter.Succeeded("Created Submariner gateway security group %s", gatewaySG)

	subnets, err := ac.getPublicSubnets(vpcID)
	if err != nil {
		return err
	}

	taggedSubnets, _ := filterSubnets(subnets, func(subnet *ec2.Subnet) (bool, error) {
		return subnetTagged(subnet), nil
	})
	untaggedSubnets, _ := filterSubnets(subnets, func(subnet *ec2.Subnet) (bool, error) {
		return !subnetTagged(subnet), nil
	})

	for _, subnet := range untaggedSubnets {
		if input.Gateways > 0 && len(taggedSubnets) == input.Gateways {
			break
		}

		subnetName := extractName(subnet.Tags)

		reporter.Started("Adjusting public subnet %s to support Submariner", subnetName)

		err = ac.tagPublicSubnet(subnet.SubnetId)
		if err != nil {
			reporter.Failed(err)
			return err
		}

		taggedSubnets = append(taggedSubnets, subnet)

		reporter.Succeeded("Adjusted public subnet %s to support Submariner", subnetName)
	}

	for _, subnet := range taggedSubnets {
		subnetName := extractName(subnet.Tags)

		reporter.Started("Deploying gateway node for public subnet %s", subnetName)

		err = ac.deployGateway(vpcID, gatewaySG, subnet)
		if err != nil {
			reporter.Failed(err)
			return err
		}

		reporter.Succeeded("Deployed gateway node for public subnet %s", subnetName)
	}

	return nil
}

func (ac *awsCloud) validatePreparePrerequisites(vpcID string, input api.PrepareForSubmarinerInput) error {
	var errs []error
	errs = appendIfError(errs, ac.validateCreateSecGroup(vpcID))
	errs = appendIfError(errs, ac.validateCreateSecGroupRule(vpcID))
	err := ac.validateDescribeInstanceTypeOfferings()
	errs = appendIfError(errs, err)

	if err != nil {
		return newCompositeError(errs...)
	}

	subnets, err := ac.getPublicSubnets(vpcID)
	if err != nil {
		return err
	}

	subnetsCount := len(subnets)
	if subnetsCount == 0 {
		errs = append(errs, errors.New("found no public subnets to deploy Submariner gateway(s)"))
	}

	if input.Gateways > 0 && len(subnets) < input.Gateways {
		errs = append(errs, fmt.Errorf("not enough public subnets to deploy %v Submariner gateway(s)", input.Gateways))
	}

	if len(subnets) > 0 {
		errs = appendIfError(errs, ac.validateCreateTag(subnets[0].SubnetId))
	}

	if len(errs) > 0 {
		return newCompositeError(errs...)
	}

	return nil
}

func (ac *awsCloud) CleanupAfterSubmariner(reporter api.Reporter) error {
	reporter.Started(messageRetrieveVPCID)

	vpcID, err := ac.getVpcID()
	if err != nil {
		reporter.Failed(err)
		return err
	}

	reporter.Succeeded(messageRetrievedVPCID, vpcID)

	reporter.Started(messageValidatePrerequisites)

	err = ac.validateCleanupPrerequisites(vpcID)
	if err != nil {
		reporter.Failed(err)
		return err
	}

	reporter.Succeeded(messageValidatedPrerequisites)

	subnets, err := ac.getTaggedPublicSubnets(vpcID)
	if err != nil {
		return err
	}

	for _, subnet := range subnets {
		subnetName := extractName(subnet.Tags)

		reporter.Started("Removing gateway node for public subnet %s", subnetName)

		err = ac.deleteGateway(subnet)
		if err != nil {
			reporter.Failed(err)
			return err
		}

		reporter.Succeeded("Removed gateway node for public subnet %s", subnetName)

		reporter.Started("Untagging public subnet %s from supporting Submariner", subnetName)

		err = ac.untagPublicSubnet(subnet.SubnetId)
		if err != nil {
			reporter.Failed(err)
			return err
		}

		reporter.Succeeded("Untagged public subnet %s from supporting Submariner", subnetName)
	}

	reporter.Started("Revoking intra-cluster communication permissions")

	err = ac.revokePortsInCluster(vpcID)
	if err != nil {
		reporter.Failed(err)
		return err
	}

	reporter.Succeeded("Revoked intra-cluster communication permissions")

	reporter.Started("Deleting Submariner gateway security group")

	err = ac.deleteGatewaySG(vpcID)
	if err != nil {
		reporter.Failed(err)
		return err
	}

	reporter.Succeeded("Deleted Submariner gateway security group")

	return nil
}

func (ac *awsCloud) validateCleanupPrerequisites(vpcID string) error {
	var errs []error

	errs = appendIfError(errs, ac.validateDeleteSecGroup(vpcID))
	errs = appendIfError(errs, ac.validateDeleteSecGroupRule(vpcID))

	subnets, err := ac.getTaggedPublicSubnets(vpcID)
	if err != nil {
		return err
	}

	if len(subnets) > 0 {
		errs = appendIfError(errs, ac.validateRemoveTag(subnets[0].SubnetId))
	}

	if len(errs) > 0 {
		return newCompositeError(errs...)
	}

	return nil
}
