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

//nolint:wrapcheck // The functions are simple wrappers so let the caller wrap errors.
package client

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

//go:generate mockgen -source=./client.go -destination=./fake/client.go -package=fake

// Interface wraps an actual AWS SDK ec2 client to allow for easier testing.
type Interface interface {
	AuthorizeSecurityGroupIngress(ctx context.Context, params *ec2.AuthorizeSecurityGroupIngressInput,
		optFns ...func(*ec2.Options)) (*ec2.AuthorizeSecurityGroupIngressOutput, error)
	CreateSecurityGroup(ctx context.Context, params *ec2.CreateSecurityGroupInput,
		optFns ...func(*ec2.Options)) (*ec2.CreateSecurityGroupOutput, error)
	CreateTags(ctx context.Context, params *ec2.CreateTagsInput,
		optFns ...func(*ec2.Options)) (*ec2.CreateTagsOutput, error)
	DescribeInstances(ctx context.Context, params *ec2.DescribeInstancesInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeInstancesOutput, error)
	DescribeVpcs(ctx context.Context, params *ec2.DescribeVpcsInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeVpcsOutput, error)
	DescribeSecurityGroups(ctx context.Context, params *ec2.DescribeSecurityGroupsInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeSecurityGroupsOutput, error)
	DescribeSubnets(ctx context.Context, params *ec2.DescribeSubnetsInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeSubnetsOutput, error)
	DescribeInstanceTypeOfferings(ctx context.Context, params *ec2.DescribeInstanceTypeOfferingsInput,
		optFns ...func(*ec2.Options)) (*ec2.DescribeInstanceTypeOfferingsOutput, error)
	DeleteSecurityGroup(ctx context.Context, params *ec2.DeleteSecurityGroupInput,
		optFns ...func(*ec2.Options)) (*ec2.DeleteSecurityGroupOutput, error)
	DeleteTags(ctx context.Context, params *ec2.DeleteTagsInput, optFns ...func(*ec2.Options)) (*ec2.DeleteTagsOutput, error)
	RevokeSecurityGroupIngress(ctx context.Context, params *ec2.RevokeSecurityGroupIngressInput,
		optFns ...func(*ec2.Options)) (*ec2.RevokeSecurityGroupIngressOutput, error)
}

type awsClient struct {
	ec2Client ec2.Client
}

func (ac *awsClient) AuthorizeSecurityGroupIngress(ctx context.Context, input *ec2.AuthorizeSecurityGroupIngressInput,
	optFns ...func(*ec2.Options),
) (*ec2.AuthorizeSecurityGroupIngressOutput, error) {
	return ac.ec2Client.AuthorizeSecurityGroupIngress(ctx, input, optFns...)
}

func (ac *awsClient) CreateSecurityGroup(ctx context.Context, input *ec2.CreateSecurityGroupInput,
	optFns ...func(*ec2.Options),
) (*ec2.CreateSecurityGroupOutput, error) {
	return ac.ec2Client.CreateSecurityGroup(ctx, input, optFns...)
}

func (ac *awsClient) CreateTags(ctx context.Context, input *ec2.CreateTagsInput,
	optFns ...func(*ec2.Options),
) (*ec2.CreateTagsOutput, error) {
	return ac.ec2Client.CreateTags(ctx, input, optFns...)
}

func (ac *awsClient) DescribeInstances(ctx context.Context, input *ec2.DescribeInstancesInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeInstancesOutput, error) {
	return ac.ec2Client.DescribeInstances(ctx, input, optFns...)
}

func (ac *awsClient) DescribeVpcs(ctx context.Context, input *ec2.DescribeVpcsInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeVpcsOutput, error) {
	return ac.ec2Client.DescribeVpcs(ctx, input, optFns...)
}

func (ac *awsClient) DescribeSecurityGroups(ctx context.Context, input *ec2.DescribeSecurityGroupsInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeSecurityGroupsOutput, error) {
	return ac.ec2Client.DescribeSecurityGroups(ctx, input, optFns...)
}

func (ac *awsClient) DescribeSubnets(ctx context.Context, input *ec2.DescribeSubnetsInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeSubnetsOutput, error) {
	return ac.ec2Client.DescribeSubnets(ctx, input, optFns...)
}

func (ac *awsClient) DeleteSecurityGroup(ctx context.Context, input *ec2.DeleteSecurityGroupInput,
	optFns ...func(*ec2.Options),
) (*ec2.DeleteSecurityGroupOutput, error) {
	return ac.ec2Client.DeleteSecurityGroup(ctx, input, optFns...)
}

func (ac *awsClient) DeleteTags(ctx context.Context, input *ec2.DeleteTagsInput,
	optFns ...func(*ec2.Options),
) (*ec2.DeleteTagsOutput, error) {
	return ac.ec2Client.DeleteTags(ctx, input, optFns...)
}

func (ac *awsClient) RevokeSecurityGroupIngress(ctx context.Context, input *ec2.RevokeSecurityGroupIngressInput,
	optFns ...func(*ec2.Options),
) (*ec2.RevokeSecurityGroupIngressOutput, error) {
	return ac.ec2Client.RevokeSecurityGroupIngress(ctx, input, optFns...)
}

func (ac *awsClient) DescribeInstanceTypeOfferings(ctx context.Context, input *ec2.DescribeInstanceTypeOfferingsInput,
	optFns ...func(*ec2.Options),
) (*ec2.DescribeInstanceTypeOfferingsOutput, error) {
	return ac.ec2Client.DescribeInstanceTypeOfferings(ctx, input, optFns...)
}

func New(accessKeyID, secretAccessKey, region string) (Interface, error) {
	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
		config.WithEndpointResolverWithOptions(aws.EndpointResolverWithOptionsFunc(
			func(service, region string, options ...interface{}) (aws.Endpoint, error) {
				if service != "route53" || region != "cn-northwest-1" {
					return aws.Endpoint{}, &aws.EndpointNotFoundError{}
				}

				return aws.Endpoint{
					URL:         "https://route53.amazonaws.com.cn",
					PartitionID: "aws-cn",
				}, nil
			})))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS configuration: %w", err)
	}

	return &awsClient{
		ec2Client: *ec2.NewFromConfig(cfg),
	}, nil
}
