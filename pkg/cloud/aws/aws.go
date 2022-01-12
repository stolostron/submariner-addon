package aws

import (
	"context"
	"fmt"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/stolostron/submariner-addon/pkg/constants"

	"github.com/stolostron/submariner-addon/pkg/cloud/manifestwork"
	"github.com/stolostron/submariner-addon/pkg/cloud/reporter"
	workclient "open-cluster-management.io/api/client/work/clientset/versioned"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	cpapi "github.com/submariner-io/cloud-prepare/pkg/api"
	cpaws "github.com/submariner-io/cloud-prepare/pkg/aws"
	cpclient "github.com/submariner-io/cloud-prepare/pkg/aws/client"
)

const (
	defaultInstanceType  = "m5n.large"
	accessKeyIDSecretKey = "aws_access_key_id"
	accessKeySecretKey   = "aws_secret_access_key"
	internalELBLabel     = "kubernetes.io/role/internal-elb"
	internalGatewayLabel = "submariner.io/gateway"
	workName             = "aws-submariner-gateway-machineset"
)

type awsProvider struct {
	reporter          cpapi.Reporter
	nattPort          int64
	nattDiscoveryPort int64
	instanceType      string
	gateways          int
	cloudPrepare      cpapi.Cloud
	gatewayDeployer   cpapi.GatewayDeployer
}

func NewAWSProvider(
	kubeClient kubernetes.Interface, workClient workclient.Interface,
	eventRecorder events.Recorder,
	region, infraID, clusterName, credentialsSecretName string,
	instanceType string,
	nattPort, nattDiscoveryPort int,
	gateways int) (*awsProvider, error) {
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
	machineSetDeployer := manifestwork.NewMachineSetDeployer(workClient, workName, clusterName, eventRecorder)
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
	// See prepareAws() in https://github.com/submariner-io/submariner-operator/blob/devel/pkg/subctl/cmd/cloud/prepare/aws.go
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
	if err := a.cloudPrepare.PrepareForSubmariner(cpapi.PrepareForSubmarinerInput{
		InternalPorts: []cpapi.PortSpec{
			{Port: constants.SubmarinerRoutePort, Protocol: "udp"},
			{Port: constants.SubmarinerMetricsPort, Protocol: "tcp"},
		},
	}, a.reporter); err != nil {
		return err
	}

	a.reporter.Succeeded("The Submariner cluster environment has been set up on AWS")

	return nil
}

// CleanUpSubmarinerClusterEnv clean up submariner cluster environment on AWS after the SubmarinerConfig was deleted
func (a *awsProvider) CleanUpSubmarinerClusterEnv() error {
	if err := a.gatewayDeployer.Cleanup(a.reporter); err != nil {
		return err
	}

	if err := a.cloudPrepare.CleanupAfterSubmariner(a.reporter); err != nil {
		return err
	}

	a.reporter.Succeeded("The Submariner cluster environment has been cleaned up on AWS")

	return nil
}
