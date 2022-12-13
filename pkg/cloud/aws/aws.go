package aws

import (
	"context"
	"fmt"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/stolostron/submariner-addon/pkg/cloud/reporter"
	"github.com/stolostron/submariner-addon/pkg/constants"
	submreporter "github.com/submariner-io/admiral/pkg/reporter"
	cpapi "github.com/submariner-io/cloud-prepare/pkg/api"
	cpaws "github.com/submariner-io/cloud-prepare/pkg/aws"
	cpclient "github.com/submariner-io/cloud-prepare/pkg/aws/client"
	"github.com/submariner-io/cloud-prepare/pkg/ocp"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const (
	defaultInstanceType  = "m5n.large"
	accessKeyIDSecretKey = "aws_access_key_id"
	//#nosec G101 -- This is the name of a key that will store a secret, but not a default secret
	accessKeySecretKey = "aws_secret_access_key"
	workName           = "aws-submariner-gateway-machineset"
)

type awsProvider struct {
	reporter          submreporter.Interface
	nattPort          int64
	nattDiscoveryPort int64
	instanceType      string
	gateways          int
	cloudPrepare      cpapi.Cloud
	gatewayDeployer   cpapi.GatewayDeployer
}

func NewAWSProvider(
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	restMapper meta.RESTMapper,
	eventRecorder events.Recorder,
	region, infraID, clusterName, credentialsSecretName string,
	instanceType string,
	nattPort, nattDiscoveryPort int,
	gateways int,
) (*awsProvider, error) {
	if region == "" {
		return nil, fmt.Errorf("cluster region is empty")
	}

	if infraID == "" {
		return nil, fmt.Errorf("cluster infraID is empty")
	}

	if gateways < 1 {
		return nil, fmt.Errorf("the count of gateways is less than 1")
	}

	if nattPort == 0 {
		nattPort = constants.SubmarinerNatTPort
	}

	if nattDiscoveryPort == 0 {
		nattDiscoveryPort = constants.SubmarinerNatTDiscoveryPort
	}

	if instanceType == "" {
		instanceType = defaultInstanceType
	}

	credentialsSecret, err := kubeClient.CoreV1().Secrets(clusterName).Get(context.TODO(), credentialsSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	accessKeyID, ok := credentialsSecret.Data[accessKeyIDSecretKey]
	if !ok {
		return nil, fmt.Errorf("the aws credentials key %s is not in secret %s/%s", accessKeyIDSecretKey, clusterName, credentialsSecretName)
	}
	secretAccessKey, ok := credentialsSecret.Data[accessKeySecretKey]
	if !ok {
		return nil, fmt.Errorf("the aws credentials key %s is not in secret %s/%s", accessKeySecretKey, clusterName, credentialsSecretName)
	}

	awsClient, err := cpclient.New(string(accessKeyID), string(secretAccessKey), region)
	if err != nil {
		return nil, err
	}

	cloudPrepare := cpaws.NewCloud(awsClient, infraID, region)

	machineSetDeployer := ocp.NewK8sMachinesetDeployer(restMapper, dynamicClient)
	gwDeployer, err := cpaws.NewOcpGatewayDeployer(cloudPrepare, machineSetDeployer, instanceType)
	if err != nil {
		return nil, err
	}

	return &awsProvider{
		reporter:          reporter.NewEventRecorderWrapper("AWSCloudProvider", eventRecorder),
		nattPort:          int64(nattPort),
		nattDiscoveryPort: int64(nattDiscoveryPort),
		instanceType:      instanceType,
		gateways:          gateways,
		cloudPrepare:      cloudPrepare,
		gatewayDeployer:   gwDeployer,
	}, nil
}

// PrepareSubmarinerClusterEnv prepares submariner cluster environment on AWS
func (a *awsProvider) PrepareSubmarinerClusterEnv() error {
	// See AWS() in https://github.com/submariner-io/subctl/blob/devel/pkg/cloud/prepare/aws.go
	// For now we only support at least one gateway (no load-balancer)
	if err := a.gatewayDeployer.Deploy(cpapi.GatewayDeployInput{
		PublicPorts: []cpapi.PortSpec{
			{Port: uint16(a.nattPort), Protocol: "udp"},
			{Port: uint16(a.nattDiscoveryPort), Protocol: "udp"},
			// ESP & AH protocols are used for private-ip to private-ip gateway communications
			{Port: 0, Protocol: "50"},
			{Port: 0, Protocol: "51"},
		},
		Gateways: a.gateways,
	}, a.reporter); err != nil {
		return err
	}
	if err := a.cloudPrepare.OpenPorts([]cpapi.PortSpec{
		{Port: constants.SubmarinerRoutePort, Protocol: "udp"},
	}, a.reporter); err != nil {
		return err
	}

	a.reporter.Success("The Submariner cluster environment has been set up on AWS")

	return nil
}

// CleanUpSubmarinerClusterEnv clean up submariner cluster environment on AWS after the SubmarinerConfig was deleted
func (a *awsProvider) CleanUpSubmarinerClusterEnv() error {
	if err := a.gatewayDeployer.Cleanup(a.reporter); err != nil {
		return err
	}

	if err := a.cloudPrepare.ClosePorts(a.reporter); err != nil {
		return err
	}

	a.reporter.Success("The Submariner cluster environment has been cleaned up on AWS")

	return nil
}
