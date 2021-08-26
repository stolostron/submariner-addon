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
package aws

import (
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/submariner-io/cloud-prepare/pkg/api"
)

const (
	messageRetrieveVPCID          = "Retrieving VPC ID"
	messageRetrievedVPCID         = "Retrieved VPC ID %s"
	messageValidatePrerequisites  = "Validating pre-requisites"
	messageValidatedPrerequisites = "Validated pre-requisites"
)

type awsClient interface {
	AuthorizeSecurityGroupIngress(input *ec2.AuthorizeSecurityGroupIngressInput) (*ec2.AuthorizeSecurityGroupIngressOutput, error)

	CreateSecurityGroup(input *ec2.CreateSecurityGroupInput) (*ec2.CreateSecurityGroupOutput, error)
	CreateTags(input *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error)

	DeleteSecurityGroup(input *ec2.DeleteSecurityGroupInput) (*ec2.DeleteSecurityGroupOutput, error)
	DeleteTags(input *ec2.DeleteTagsInput) (*ec2.DeleteTagsOutput, error)

	DescribeInstances(input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error)
	DescribeInstanceTypeOfferings(input *ec2.DescribeInstanceTypeOfferingsInput) (*ec2.DescribeInstanceTypeOfferingsOutput, error)
	DescribeSecurityGroups(input *ec2.DescribeSecurityGroupsInput) (*ec2.DescribeSecurityGroupsOutput, error)
	DescribeSubnets(input *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error)
	DescribeVpcs(input *ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error)

	RevokeSecurityGroupIngress(input *ec2.RevokeSecurityGroupIngressInput) (*ec2.RevokeSecurityGroupIngressOutput, error)
}

type awsCloud struct {
	client  awsClient
	infraID string
	region  string
}

// NewCloud creates a new api.Cloud instance which can prepare AWS for Submariner to be deployed on it
func NewCloud(client awsClient, infraID, region string) api.Cloud {
	return &awsCloud{
		client:  client,
		infraID: infraID,
		region:  region,
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

	err = ac.validatePreparePrerequisites(vpcID)
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

	return nil
}

func (ac *awsCloud) validatePreparePrerequisites(vpcID string) error {
	return ac.validateCreateSecGroupRule(vpcID)
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

	reporter.Started("Revoking intra-cluster communication permissions")

	err = ac.revokePortsInCluster(vpcID)
	if err != nil {
		reporter.Failed(err)
		return err
	}

	reporter.Succeeded("Revoked intra-cluster communication permissions")

	return nil
}

func (ac *awsCloud) validateCleanupPrerequisites(vpcID string) error {
	return ac.validateDeleteSecGroupRule(vpcID)
}
