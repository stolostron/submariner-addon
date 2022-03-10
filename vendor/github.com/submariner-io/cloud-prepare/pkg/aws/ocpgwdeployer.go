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
	"bytes"
	"context"
	"fmt"
	"text/template"

	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/pkg/errors"
	"github.com/submariner-io/cloud-prepare/pkg/api"
	"github.com/submariner-io/cloud-prepare/pkg/ocp"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type ocpGatewayDeployer struct {
	aws          *awsCloud
	msDeployer   ocp.MachineSetDeployer
	instanceType string
}

var preferredInstances = []string{"c5d.large", "m5n.large"}

// NewOcpGatewayDeployer returns a GatewayDeployer capable deploying gateways using OCP.
// If the supplied cloud is not an awsCloud, an error is returned.
func NewOcpGatewayDeployer(cloud api.Cloud, msDeployer ocp.MachineSetDeployer, instanceType string) (api.GatewayDeployer, error) {
	aws, ok := cloud.(*awsCloud)
	if !ok {
		return nil, errors.New("the cloud must be AWS")
	}

	return &ocpGatewayDeployer{
		aws:          aws,
		msDeployer:   msDeployer,
		instanceType: instanceType,
	}, nil
}

func (d *ocpGatewayDeployer) Deploy(input api.GatewayDeployInput, reporter api.Reporter) error {
	reporter.Started(messageRetrieveVPCID)

	vpcID, err := d.aws.getVpcID()
	if err != nil {
		reporter.Failed(err)
		return err
	}

	reporter.Succeeded(messageRetrievedVPCID, vpcID)

	reporter.Started(messageValidatePrerequisites)

	publicSubnets, err := d.aws.findPublicSubnets(vpcID, d.aws.filterByName("{infraID}-public-{region}*"))
	if err != nil {
		reporter.Failed(err)
		return err
	}

	err = d.validateDeployPrerequisites(vpcID, input, publicSubnets)
	if err != nil {
		reporter.Failed(err)
		return err
	}

	reporter.Succeeded(messageValidatedPrerequisites)

	reporter.Started("Creating Submariner gateway security group")

	gatewaySG, err := d.aws.createGatewaySG(vpcID, input.PublicPorts)
	if err != nil {
		reporter.Failed(err)
		return err
	}

	reporter.Succeeded("Created Submariner gateway security group %s", gatewaySG)

	subnets, err := d.aws.getSubnetsSupportingInstanceType(publicSubnets, d.instanceType)
	if err != nil {
		return err
	}

	taggedSubnets, _ := filterSubnets(subnets, func(subnet *types.Subnet) (bool, error) {
		return subnetTagged(subnet), nil
	})
	untaggedSubnets, _ := filterSubnets(subnets, func(subnet *types.Subnet) (bool, error) {
		return !subnetTagged(subnet), nil
	})

	for i := range untaggedSubnets {
		subnet := &untaggedSubnets[i]

		if input.Gateways > 0 && len(taggedSubnets) == input.Gateways {
			break
		}

		subnetName := extractName(subnet.Tags)

		reporter.Started("Adjusting public subnet %s to support Submariner", subnetName)

		err = d.aws.tagPublicSubnet(subnet.SubnetId)
		if err != nil {
			reporter.Failed(err)
			return err
		}

		taggedSubnets = append(taggedSubnets, *subnet)

		reporter.Succeeded("Adjusted public subnet %s to support Submariner", subnetName)
	}

	for i := range taggedSubnets {
		subnet := &taggedSubnets[i]
		subnetName := extractName(subnet.Tags)

		reporter.Started("Deploying gateway node for public subnet %s", subnetName)

		err = d.deployGateway(vpcID, gatewaySG, subnet)
		if err != nil {
			reporter.Failed(err)
			return err
		}

		reporter.Succeeded("Deployed gateway node for public subnet %s", subnetName)
	}

	return nil
}

func (d *ocpGatewayDeployer) validateDeployPrerequisites(vpcID string, input api.GatewayDeployInput,
	publicSubnets []types.Subnet) error {
	var errs []error
	var subnets []types.Subnet

	errs = appendIfError(errs, d.aws.validateCreateSecGroup(vpcID))
	errs = appendIfError(errs, d.aws.validateCreateSecGroupRule(vpcID))
	err := d.aws.validateDescribeInstanceTypeOfferings()
	errs = appendIfError(errs, err)

	if err != nil {
		return utilerrors.NewAggregate(errs)
	}

	// If instanceType is not specified, auto-select the most suitable one.
	if d.instanceType == "" {
		for _, instanceType := range preferredInstances {
			subnets, err = d.aws.getSubnetsSupportingInstanceType(publicSubnets, instanceType)
			if err != nil {
				return err
			}

			if len(subnets) != 0 {
				d.instanceType = instanceType
				break
			}
		}
	} else {
		subnets, err = d.aws.getSubnetsSupportingInstanceType(publicSubnets, d.instanceType)
		if err != nil {
			return err
		}
	}

	subnetsCount := len(subnets)
	if subnetsCount == 0 {
		errs = append(errs, errors.New("found no public subnets to deploy Submariner gateway(s)"))
	}

	if input.Gateways > 0 && len(subnets) < input.Gateways {
		errs = append(errs, fmt.Errorf("not enough public subnets to deploy %v Submariner gateway(s)", input.Gateways))
	}

	if len(subnets) > 0 {
		errs = appendIfError(errs, d.aws.validateCreateTag(*subnets[0].SubnetId))
	}

	return utilerrors.NewAggregate(errs)
}

type machineSetConfig struct {
	AZ            string
	AMIId         string
	InfraID       string
	InstanceType  string
	Region        string
	SecurityGroup string
	PublicSubnet  string
}

func (d *ocpGatewayDeployer) findAMIID(vpcID string) (string, error) {
	result, err := d.aws.client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{
		Filters: []types.Filter{
			ec2Filter("vpc-id", vpcID),
			d.aws.filterByName("{infraID}-worker*"),
			d.aws.filterByCurrentCluster(),
		},
	})
	if err != nil {
		return "", errors.Wrap(err, "error describing AWS instances")
	}

	if len(result.Reservations) == 0 {
		return "", newNotFoundError("reservations")
	}

	if len(result.Reservations[0].Instances) == 0 {
		return "", newNotFoundError("worker instances")
	}

	if result.Reservations[0].Instances[0].ImageId == nil {
		return "", newNotFoundError("AMI ID")
	}

	return *result.Reservations[0].Instances[0].ImageId, nil
}

func (d *ocpGatewayDeployer) loadGatewayYAML(gatewaySecurityGroup, amiID string, publicSubnet *types.Subnet) ([]byte, error) {
	var buf bytes.Buffer

	// TODO: Not working properly, but we should revisit this as it makes more sense
	// tpl, err := template.ParseFiles("pkg/aws/gw-machineset.yaml.template")
	tpl, err := template.New("").Parse(machineSetYAML)
	if err != nil {
		return nil, errors.Wrap(err, "error parsing machine set YAML")
	}

	tplVars := machineSetConfig{
		AZ:            *publicSubnet.AvailabilityZone,
		AMIId:         amiID,
		InfraID:       d.aws.infraID,
		InstanceType:  d.instanceType,
		Region:        d.aws.region,
		SecurityGroup: gatewaySecurityGroup,
		PublicSubnet:  extractName(publicSubnet.Tags),
	}

	err = tpl.Execute(&buf, tplVars)
	if err != nil {
		return nil, errors.Wrap(err, "error executing the template")
	}

	return buf.Bytes(), nil
}

func (d *ocpGatewayDeployer) initMachineSet(gwSecurityGroup, amiID string, publicSubnet *types.Subnet) (*unstructured.Unstructured, error) {
	gatewayYAML, err := d.loadGatewayYAML(gwSecurityGroup, amiID, publicSubnet)
	if err != nil {
		return nil, err
	}

	unstructDecoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)

	machineSet := &unstructured.Unstructured{}

	_, _, err = unstructDecoder.Decode(gatewayYAML, nil, machineSet)
	if err != nil {
		return nil, errors.Wrap(err, "error converting YAML to machine set")
	}

	return machineSet, nil
}

func (d *ocpGatewayDeployer) deployGateway(vpcID, gatewaySecurityGroup string, publicSubnet *types.Subnet) error {
	amiID, err := d.findAMIID(vpcID)
	if err != nil {
		return err
	}

	machineSet, err := d.initMachineSet(gatewaySecurityGroup, amiID, publicSubnet)
	if err != nil {
		return err
	}

	return errors.Wrapf(d.msDeployer.Deploy(machineSet), "error deploying machine set %q", machineSet.GetName())
}

func (d *ocpGatewayDeployer) Cleanup(reporter api.Reporter) error {
	reporter.Started(messageRetrieveVPCID)

	vpcID, err := d.aws.getVpcID()
	if err != nil {
		reporter.Failed(err)
		return err
	}

	reporter.Succeeded(messageRetrievedVPCID, vpcID)

	reporter.Started(messageValidatePrerequisites)

	err = d.validateCleanupPrerequisites(vpcID)
	if err != nil {
		reporter.Failed(err)
		return err
	}

	reporter.Succeeded(messageValidatedPrerequisites)

	subnets, err := d.aws.getTaggedPublicSubnets(vpcID)
	if err != nil {
		return err
	}

	for i := range subnets {
		subnet := &subnets[i]
		subnetName := extractName(subnet.Tags)

		reporter.Started("Removing gateway node for public subnet %s", subnetName)

		err = d.deleteGateway(subnet)
		if err != nil {
			reporter.Failed(err)
			return err
		}

		reporter.Succeeded("Removed gateway node for public subnet %s", subnetName)

		reporter.Started("Untagging public subnet %s from supporting Submariner", subnetName)

		err = d.aws.untagPublicSubnet(subnet.SubnetId)
		if err != nil {
			reporter.Failed(err)
			return err
		}

		reporter.Succeeded("Untagged public subnet %s from supporting Submariner", subnetName)
	}

	reporter.Started("Deleting Submariner gateway security group")

	err = d.aws.deleteGatewaySG(vpcID)
	if err != nil {
		reporter.Failed(err)
		return err
	}

	reporter.Succeeded("Deleted Submariner gateway security group")

	return nil
}

func (d *ocpGatewayDeployer) validateCleanupPrerequisites(vpcID string) error {
	var errs []error

	errs = appendIfError(errs, d.aws.validateDeleteSecGroup(vpcID))

	subnets, err := d.aws.getTaggedPublicSubnets(vpcID)
	if err != nil {
		return err
	}

	if len(subnets) > 0 {
		errs = appendIfError(errs, d.aws.validateRemoveTag(subnets[0].SubnetId))
	}

	return utilerrors.NewAggregate(errs)
}

func (d *ocpGatewayDeployer) deleteGateway(publicSubnet *types.Subnet) error {
	machineSet, err := d.initMachineSet("", "", publicSubnet)
	if err != nil {
		return err
	}

	return errors.Wrapf(d.msDeployer.Delete(machineSet), "error deleting machine set %q", machineSet.GetName())
}
