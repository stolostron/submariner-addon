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
package client

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/route53"
)

//go:generate mockgen -source=./client.go -destination=./fake/client.go -package=fake

// Interface wraps an actual AWS SDK ec2 client to allow for easier testing.
type Interface interface {
	AuthorizeSecurityGroupIngress(input *ec2.AuthorizeSecurityGroupIngressInput) (*ec2.AuthorizeSecurityGroupIngressOutput, error)

	CreateSecurityGroup(input *ec2.CreateSecurityGroupInput) (*ec2.CreateSecurityGroupOutput, error)
	CreateTags(input *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error)

	DescribeInstances(input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error)
	DescribeVpcs(input *ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error)
	DescribeSecurityGroups(input *ec2.DescribeSecurityGroupsInput) (*ec2.DescribeSecurityGroupsOutput, error)
	DescribeSubnets(input *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error)
	DescribeInstanceTypeOfferings(input *ec2.DescribeInstanceTypeOfferingsInput) (*ec2.DescribeInstanceTypeOfferingsOutput, error)

	DeleteSecurityGroup(input *ec2.DeleteSecurityGroupInput) (*ec2.DeleteSecurityGroupOutput, error)
	DeleteTags(input *ec2.DeleteTagsInput) (*ec2.DeleteTagsOutput, error)

	RevokeSecurityGroupIngress(input *ec2.RevokeSecurityGroupIngressInput) (*ec2.RevokeSecurityGroupIngressOutput, error)
}

type awsClient struct {
	ec2Client ec2iface.EC2API
}

func (ac *awsClient) AuthorizeSecurityGroupIngress(
	input *ec2.AuthorizeSecurityGroupIngressInput) (*ec2.AuthorizeSecurityGroupIngressOutput, error) {
	return ac.ec2Client.AuthorizeSecurityGroupIngress(input)
}

func (ac *awsClient) CreateSecurityGroup(input *ec2.CreateSecurityGroupInput) (*ec2.CreateSecurityGroupOutput, error) {
	return ac.ec2Client.CreateSecurityGroup(input)
}

func (ac *awsClient) CreateTags(input *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error) {
	return ac.ec2Client.CreateTags(input)
}

func (ac *awsClient) DescribeInstances(input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	return ac.ec2Client.DescribeInstances(input)
}
func (ac *awsClient) DescribeVpcs(input *ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error) {
	return ac.ec2Client.DescribeVpcs(input)
}

func (ac *awsClient) DescribeSecurityGroups(input *ec2.DescribeSecurityGroupsInput) (*ec2.DescribeSecurityGroupsOutput, error) {
	return ac.ec2Client.DescribeSecurityGroups(input)
}

func (ac *awsClient) DescribeSubnets(input *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error) {
	return ac.ec2Client.DescribeSubnets(input)
}

func (ac *awsClient) DeleteSecurityGroup(input *ec2.DeleteSecurityGroupInput) (*ec2.DeleteSecurityGroupOutput, error) {
	return ac.ec2Client.DeleteSecurityGroup(input)
}

func (ac *awsClient) DeleteTags(input *ec2.DeleteTagsInput) (*ec2.DeleteTagsOutput, error) {
	return ac.ec2Client.DeleteTags(input)
}

func (ac *awsClient) RevokeSecurityGroupIngress(input *ec2.RevokeSecurityGroupIngressInput) (*ec2.RevokeSecurityGroupIngressOutput, error) {
	return ac.ec2Client.RevokeSecurityGroupIngress(input)
}

func (ac *awsClient) DescribeInstanceTypeOfferings(
	input *ec2.DescribeInstanceTypeOfferingsInput) (*ec2.DescribeInstanceTypeOfferingsOutput, error) {
	return ac.ec2Client.DescribeInstanceTypeOfferings(input)
}

func New(accessKeyID, secretAccessKey, region string) (Interface, error) {
	sess, err := session.NewSession(&aws.Config{
		Credentials:      credentials.NewStaticCredentials(accessKeyID, secretAccessKey, ""),
		Region:           aws.String(region),
		EndpointResolver: endpoints.ResolverFunc(awsChinaEndpointResolver),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create aws session: %v", err)
	}

	return &awsClient{
		ec2Client: ec2.New(sess),
	}, nil
}

func awsChinaEndpointResolver(service, region string, optFns ...func(*endpoints.Options)) (endpoints.ResolvedEndpoint, error) {
	if service != route53.EndpointsID || region != "cn-northwest-1" {
		return endpoints.DefaultResolver().EndpointFor(service, region, optFns...)
	}

	return endpoints.ResolvedEndpoint{
		URL:         "https://route53.amazonaws.com.cn",
		PartitionID: "aws-cn",
	}, nil
}
