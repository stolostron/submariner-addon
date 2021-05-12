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
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/submariner-io/cloud-prepare/pkg/api"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"
)

const internalTraffic = "Internal Submariner traffic"

func (ac *awsCloud) getSecurityGroupID(vpcID, name string) (*string, error) {
	group, err := ac.getSecurityGroup(vpcID, name)
	if err != nil {
		return nil, err
	}

	return group.GroupId, nil
}

func (ac *awsCloud) getSecurityGroup(vpcID, name string) (*ec2.SecurityGroup, error) {
	filters := []*ec2.Filter{
		ec2Filter("vpc-id", vpcID),
		ac.filterByName(name),
		ac.filterByCurrentCluster(),
	}

	result, err := ac.client.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		Filters: filters,
	})

	if err != nil {
		return nil, err
	}

	if len(result.SecurityGroups) == 0 {
		return nil, newNotFoundError("security group %s", name)
	}

	return result.SecurityGroups[0], nil
}

func (ac *awsCloud) authorizeSecurityGroupIngress(groupID *string, ipPermissions []*ec2.IpPermission) error {
	input := &ec2.AuthorizeSecurityGroupIngressInput{
		GroupId:       groupID,
		IpPermissions: ipPermissions,
	}

	_, err := ac.client.AuthorizeSecurityGroupIngress(input)
	if isAWSError(err, "InvalidPermission.Duplicate") {
		return nil
	}

	return err
}

func (ac *awsCloud) createClusterSGRule(srcGroup, destGroup *string, port uint16, protocol, description string) error {
	ipPermissions := []*ec2.IpPermission{
		{
			FromPort:   aws.Int64(int64(port)),
			ToPort:     aws.Int64(int64(port)),
			IpProtocol: aws.String(protocol),
			UserIdGroupPairs: []*ec2.UserIdGroupPair{
				{
					Description: aws.String(description),
					GroupId:     srcGroup,
				},
			},
		},
	}

	return ac.authorizeSecurityGroupIngress(destGroup, ipPermissions)
}

func (ac *awsCloud) allowPortInCluster(vpcID string, port uint16, protocol string) error {
	workerGroupID, err := ac.getSecurityGroupID(vpcID, "{infraID}-worker-sg")
	if err != nil {
		return err
	}

	masterGroupID, err := ac.getSecurityGroupID(vpcID, "{infraID}-master-sg")
	if err != nil {
		return err
	}

	err = ac.createClusterSGRule(workerGroupID, workerGroupID, port, protocol, fmt.Sprintf("%s between the workers", internalTraffic))
	if err != nil {
		return err
	}

	err = ac.createClusterSGRule(workerGroupID, masterGroupID, port, protocol, fmt.Sprintf("%s from worker to master nodes", internalTraffic))
	if err != nil {
		return err
	}

	return ac.createClusterSGRule(masterGroupID, workerGroupID, port, protocol, fmt.Sprintf("%s from master to worker nodes", internalTraffic))
}

func (ac *awsCloud) createPublicSGRule(groupID *string, port uint16, protocol, description string) error {
	ipPermissions := []*ec2.IpPermission{
		{
			FromPort:   aws.Int64(int64(port)),
			ToPort:     aws.Int64(int64(port)),
			IpProtocol: aws.String(protocol),
			IpRanges: []*ec2.IpRange{
				{
					CidrIp:      aws.String("0.0.0.0/0"),
					Description: aws.String(description),
				},
			},
		},
	}

	return ac.authorizeSecurityGroupIngress(groupID, ipPermissions)
}

func (ac *awsCloud) createGatewaySG(vpcID string, ports []api.PortSpec) (string, error) {
	groupName := ac.withAWSInfo("{infraID}-submariner-gw-sg")
	gatewayGroupID, err := ac.getSecurityGroupID(vpcID, groupName)
	if err != nil {
		if !isNotFoundError(err) {
			return "", err
		}

		input := &ec2.CreateSecurityGroupInput{
			GroupName:   &groupName,
			Description: aws.String("Submariner Gateway"),
			VpcId:       &vpcID,
			TagSpecifications: []*ec2.TagSpecification{
				{
					ResourceType: aws.String("security-group"),
					Tags: []*ec2.Tag{
						ec2Tag("Name", groupName),
						ec2Tag(ac.withAWSInfo("kubernetes.io/cluster/{infraID}"), "owned"),
					},
				},
			},
		}

		result, err := ac.client.CreateSecurityGroup(input)
		if err != nil {
			return "", err
		}

		gatewayGroupID = result.GroupId
	}

	for _, port := range ports {
		err = ac.createPublicSGRule(gatewayGroupID, port.Port, port.Protocol, "Public Submariner traffic")
		if err != nil {
			return "", err
		}
	}

	return groupName, nil
}

func gatewayDeletionRetriable(err error) bool {
	return isAWSError(err, "DependencyViolation")
}

func (ac *awsCloud) deleteGatewaySG(vpcID string) error {
	groupName := ac.withAWSInfo("{infraID}-submariner-gw-sg")
	gatewayGroupID, err := ac.getSecurityGroupID(vpcID, groupName)
	if err != nil {
		if isNotFoundError(err) {
			return nil
		}

		return err
	}

	backoff := wait.Backoff{
		Steps:    30,
		Duration: 500 * time.Millisecond,
		Factor:   1.2,
		Cap:      10 * time.Minute,
	}

	err = retry.OnError(backoff, gatewayDeletionRetriable, func() error {
		_, err = ac.client.DeleteSecurityGroup(&ec2.DeleteSecurityGroupInput{
			GroupId: gatewayGroupID,
		})

		return err
	})

	if isAWSError(err, "InvalidPermission.NotFound") {
		return nil
	}

	return err
}

func (ac *awsCloud) revokePortsInCluster(vpcID string) error {
	workerGroup, err := ac.getSecurityGroup(vpcID, "{infraID}-worker-sg")
	if err != nil {
		return err
	}

	masterGroup, err := ac.getSecurityGroup(vpcID, "{infraID}-master-sg")
	if err != nil {
		return err
	}

	err = ac.revokePortsFromGroup(workerGroup)
	if err != nil {
		return err
	}

	return ac.revokePortsFromGroup(masterGroup)
}

func (ac *awsCloud) revokePortsFromGroup(group *ec2.SecurityGroup) error {
	var permissionsToRevoke []*ec2.IpPermission

	for _, permission := range group.IpPermissions {
		for _, groupPair := range permission.UserIdGroupPairs {
			if groupPair.Description != nil && strings.Contains(*groupPair.Description, internalTraffic) {
				permissionsToRevoke = append(permissionsToRevoke, permission)
				break
			}
		}
	}

	if len(permissionsToRevoke) == 0 {
		return nil
	}

	input := &ec2.RevokeSecurityGroupIngressInput{
		GroupId:       group.GroupId,
		IpPermissions: permissionsToRevoke,
	}

	_, err := ac.client.RevokeSecurityGroupIngress(input)

	return err
}
