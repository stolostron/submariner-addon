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
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

var (
	tagSubmarinerGatgeway = ec2Tag("submariner.io/gateway", "")
	tagInternalELB        = ec2Tag("kubernetes.io/role/internal-elb", "")
)

func filterSubnets(subnets []*ec2.Subnet, filterFunc func(subnet *ec2.Subnet) (bool, error)) ([]*ec2.Subnet, error) {
	var filteredSubnets []*ec2.Subnet

	for _, subnet := range subnets {
		filterResult, err := filterFunc(subnet)
		if err != nil {
			return nil, err
		}

		if filterResult {
			filteredSubnets = append(filteredSubnets, subnet)
		}
	}

	return filteredSubnets, nil
}

func subnetTagged(subnet *ec2.Subnet) bool {
	return hasTag(subnet.Tags, tagSubmarinerGatgeway)
}

func (ac *awsCloud) findPublicSubnets(vpcID string, filter *ec2.Filter) ([]*ec2.Subnet, error) {
	filters := []*ec2.Filter{
		ec2Filter("vpc-id", vpcID),
		ac.filterByCurrentCluster(),
		filter,
	}

	result, err := ac.client.DescribeSubnets(&ec2.DescribeSubnetsInput{Filters: filters})
	if err != nil {
		return nil, err
	}

	return result.Subnets, nil
}

func (ac *awsCloud) subnetSupportsInstanceType(subnet *ec2.Subnet) (bool, error) {
	output, err := ac.client.DescribeInstanceTypeOfferings(&ec2.DescribeInstanceTypeOfferingsInput{
		LocationType: aws.String("availability-zone"),
		Filters: []*ec2.Filter{
			ec2Filter("location", *subnet.AvailabilityZone),
			ec2Filter("instance-type", ac.gwInstanceType),
		},
	})
	if err != nil {
		return false, err
	}

	return len(output.InstanceTypeOfferings) > 0, nil
}

func (ac *awsCloud) getPublicSubnets(vpcID string) ([]*ec2.Subnet, error) {
	subnets, err := ac.findPublicSubnets(vpcID, ac.filterByName("{infraID}-public-{region}*"))
	if err != nil {
		return nil, err
	}

	return filterSubnets(subnets, ac.subnetSupportsInstanceType)
}

func (ac *awsCloud) getTaggedPublicSubnets(vpcID string) ([]*ec2.Subnet, error) {
	return ac.findPublicSubnets(vpcID, ec2FilterByTag(tagSubmarinerGatgeway))
}

func (ac *awsCloud) tagPublicSubnet(subnetID *string) error {
	_, err := ac.client.CreateTags(&ec2.CreateTagsInput{
		Resources: []*string{subnetID},
		Tags: []*ec2.Tag{
			tagInternalELB,
			tagSubmarinerGatgeway,
		},
	})

	return err
}

func (ac *awsCloud) untagPublicSubnet(subnetID *string) error {
	_, err := ac.client.DeleteTags(&ec2.DeleteTagsInput{
		Resources: []*string{subnetID},
		Tags: []*ec2.Tag{
			tagInternalELB,
			tagSubmarinerGatgeway,
		},
	})

	return err
}
