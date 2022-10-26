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

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/pkg/errors"
)

var (
	tagSubmarinerGateway = ec2Tag("submariner.io/gateway", "")
	tagInternalELB       = ec2Tag("kubernetes.io/role/internal-elb", "")
)

func filterSubnets(subnets []types.Subnet, filterFunc func(subnet *types.Subnet) (bool, error)) ([]types.Subnet, error) {
	var filteredSubnets []types.Subnet

	for i := range subnets {
		subnet := &subnets[i]

		filterResult, err := filterFunc(subnet)
		if err != nil {
			return nil, err
		}

		if filterResult {
			filteredSubnets = append(filteredSubnets, *subnet)
		}
	}

	return filteredSubnets, nil
}

func subnetTagged(subnet *types.Subnet) bool {
	return hasTag(subnet.Tags, tagSubmarinerGateway)
}

func (ac *awsCloud) findPublicSubnets(vpcID string, filter types.Filter) ([]types.Subnet, error) {
	filters := []types.Filter{
		ec2Filter("vpc-id", vpcID),
		ac.filterByCurrentCluster(),
		filter,
	}

	result, err := ac.client.DescribeSubnets(context.TODO(), &ec2.DescribeSubnetsInput{Filters: filters})
	if err != nil {
		return nil, errors.Wrap(err, "error describing AWS subnets")
	}

	return result.Subnets, nil
}

func (ac *awsCloud) getSubnetsSupportingInstanceType(subnets []types.Subnet, instanceType string) ([]types.Subnet, error) {
	return filterSubnets(subnets, func(subnet *types.Subnet) (bool, error) {
		output, err := ac.client.DescribeInstanceTypeOfferings(context.TODO(), &ec2.DescribeInstanceTypeOfferingsInput{
			LocationType: types.LocationTypeAvailabilityZone,
			Filters: []types.Filter{
				ec2Filter("location", *subnet.AvailabilityZone),
				ec2Filter("instance-type", instanceType),
			},
		})
		if err != nil {
			return false, err //nolint:wrapcheck // Let the caller wrap it.
		}

		return len(output.InstanceTypeOfferings) > 0, nil
	})
}

func (ac *awsCloud) getTaggedPublicSubnets(vpcID string) ([]types.Subnet, error) {
	return ac.findPublicSubnets(vpcID, ec2FilterByTag(tagSubmarinerGateway))
}

func (ac *awsCloud) tagPublicSubnet(subnetID *string) error {
	_, err := ac.client.CreateTags(context.TODO(), &ec2.CreateTagsInput{
		Resources: []string{*subnetID},
		Tags: []types.Tag{
			tagInternalELB,
			tagSubmarinerGateway,
		},
	})

	return errors.Wrap(err, "error creating AWS tag")
}

func (ac *awsCloud) untagPublicSubnet(subnetID *string) error {
	_, err := ac.client.DeleteTags(context.TODO(), &ec2.DeleteTagsInput{
		Resources: []string{*subnetID},
		Tags: []types.Tag{
			tagInternalELB,
			tagSubmarinerGateway,
		},
	})

	return errors.Wrap(err, "error deleting AWS tag")
}
