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
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/pkg/errors"
	"github.com/submariner-io/cloud-prepare/pkg/api"
	awsClient "github.com/submariner-io/cloud-prepare/pkg/aws/client"
)

const (
	messageRetrieveVPCID          = "Retrieving VPC ID"
	messageRetrievedVPCID         = "Retrieved VPC ID %s"
	messageValidatePrerequisites  = "Validating pre-requisites"
	messageValidatedPrerequisites = "Validated pre-requisites"
)

type awsCloud struct {
	client  awsClient.Interface
	infraID string
	region  string
}

// NewCloud creates a new api.Cloud instance which can prepare AWS for Submariner to be deployed on it.
func NewCloud(client awsClient.Interface, infraID, region string) api.Cloud {
	return &awsCloud{
		client:  client,
		infraID: infraID,
		region:  region,
	}
}

// NewCloudFromConfig creates a new api.Cloud instance based on an AWS configuration
// which can prepare AWS for Submariner to be deployed on it.
func NewCloudFromConfig(cfg *aws.Config, infraID, region string) api.Cloud {
	return &awsCloud{
		client:  ec2.NewFromConfig(*cfg),
		infraID: infraID,
		region:  region,
	}
}

// NewCloudFromSettings creates a new api.Cloud instance using the given credentials file and profile
// which can prepare AWS for Submariner to be deployed on it.
func NewCloudFromSettings(credentialsFile, profile, infraID, region string) (api.Cloud, error) {
	options := []func(*config.LoadOptions) error{config.WithRegion(region), config.WithSharedConfigProfile(profile)}
	if credentialsFile != DefaultCredentialsFile() {
		options = append(options, config.WithSharedCredentialsFiles([]string{credentialsFile}))
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(), options...)
	if err != nil {
		return nil, errors.Wrap(err, "error loading default config")
	}

	return NewCloudFromConfig(&cfg, infraID, region), nil
}

// DefaultCredentialsFile returns the default credentials file name.
func DefaultCredentialsFile() string {
	return config.DefaultSharedCredentialsFilename()
}

// DefaultProfile returns the default profile name.
func DefaultProfile() string {
	return "default"
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
