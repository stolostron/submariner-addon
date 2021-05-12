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
	"bytes"
	"context"
	"text/template"

	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/submariner-io/admiral/pkg/resource"
	"github.com/submariner-io/admiral/pkg/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
)

type machineSetConfig struct {
	AZ            string
	AMIId         string
	InfraID       string
	InstanceType  string
	Region        string
	SecurityGroup string
	PublicSubnet  string
}

func (ac *awsCloud) findAMIID(vpcID string) (string, error) {
	result, err := ac.client.DescribeInstances(&ec2.DescribeInstancesInput{
		Filters: []*ec2.Filter{
			ec2Filter("vpc-id", vpcID),
			ac.filterByName("{infraID}-worker*"),
			ac.filterByCurrentCluster(),
		},
	})

	if err != nil {
		return "", err
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

func (ac *awsCloud) loadGatewayYAML(gatewaySecurityGroup, amiID string, publicSubnet *ec2.Subnet) ([]byte, error) {
	var buf bytes.Buffer

	// TODO: Not working properly, but we should revisit this as it makes more sense
	// tpl, err := template.ParseFiles("pkg/aws/gw-machineset.yaml")
	tpl, err := template.New("").Parse(machineSetYAML)
	if err != nil {
		return nil, err
	}

	tplVars := machineSetConfig{
		AZ:            *publicSubnet.AvailabilityZone,
		AMIId:         amiID,
		InfraID:       ac.infraID,
		InstanceType:  ac.gwInstanceType,
		Region:        ac.region,
		SecurityGroup: gatewaySecurityGroup,
		PublicSubnet:  extractName(publicSubnet.Tags),
	}

	err = tpl.Execute(&buf, tplVars)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (ac *awsCloud) initMachineSet(gatewaySecurityGroup, amiID string, publicSubnet *ec2.Subnet) (*unstructured.Unstructured, error) {
	gatewayYAML, err := ac.loadGatewayYAML(gatewaySecurityGroup, amiID, publicSubnet)
	if err != nil {
		return nil, err
	}

	unstructDecoder := yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme)
	machineSet := &unstructured.Unstructured{}
	_, _, err = unstructDecoder.Decode(gatewayYAML, nil, machineSet)
	if err != nil {
		return nil, err
	}

	return machineSet, nil
}

func (ac *awsCloud) deployGateway(vpcID, gatewaySecurityGroup string, publicSubnet *ec2.Subnet) error {
	amiID, err := ac.findAMIID(vpcID)
	if err != nil {
		return err
	}

	machineSet, err := ac.initMachineSet(gatewaySecurityGroup, amiID, publicSubnet)
	if err != nil {
		return err
	}

	return ac.gwDeployer.Deploy(machineSet)
}

func (ac *awsCloud) deleteGateway(publicSubnet *ec2.Subnet) error {
	machineSet, err := ac.initMachineSet("", "", publicSubnet)
	if err != nil {
		return err
	}

	return ac.gwDeployer.Delete(machineSet)
}

func (msd *k8sMachineSetDeployer) clientFor(obj runtime.Object) (resource.Interface, error) {
	k8sClient, err := dynamic.NewForConfig(msd.k8sConfig)
	if err != nil {
		return nil, err
	}

	restMapper, err := util.BuildRestMapper(msd.k8sConfig)
	if err != nil {
		return nil, err
	}

	machineSet, gvr, err := util.ToUnstructuredResource(obj, restMapper)
	if err != nil {
		return nil, err
	}

	dynamicClient := k8sClient.Resource(*gvr).Namespace(machineSet.GetNamespace())

	return resource.ForDynamic(dynamicClient), nil
}

func (msd *k8sMachineSetDeployer) Deploy(machineSet *unstructured.Unstructured) error {
	machineSetClient, err := msd.clientFor(machineSet)
	if err != nil {
		return err
	}

	_, err = util.CreateOrUpdate(context.TODO(), machineSetClient, machineSet, util.Replace(machineSet))

	return err
}

func (msd *k8sMachineSetDeployer) Delete(machineSet *unstructured.Unstructured) error {
	machineSetClient, err := msd.clientFor(machineSet)
	if err != nil {
		return err
	}

	err = machineSetClient.Delete(context.TODO(), machineSet.GetName(), metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}

	return err
}
