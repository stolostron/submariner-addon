package aws

import (
	"context"
	"fmt"

	"github.com/openshift/library-go/pkg/operator/events"

	"github.com/open-cluster-management/submariner-addon/pkg/cloud/manifestwork"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"
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
	eventRecorder     events.Recorder
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
	region, infraId, clusterName, credentialsSecretName string,
	instanceType string,
	nattPort, nattDiscoveryPort int,
	gateways int) (*awsProvider, error) {
	if region == "" {
		return nil, fmt.Errorf("cluster region is empty")
	}

	if infraId == "" {
		return nil, fmt.Errorf("cluster infraId is empty")
	}

	if gateways < 1 {
		return nil, fmt.Errorf("the count of gateways is less than 1")
	}

	if nattPort == 0 {
		nattPort = helpers.SubmarinerNatTPort
	}

	if nattDiscoveryPort == 0 {
		nattDiscoveryPort = helpers.SubmarinerNatTDiscoveryPort
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

	cloudPrepare := cpaws.NewCloud(awsClient, infraId, region)
	machineSetDeployer := manifestwork.NewMachineSetDeployer(workClient, workName, clusterName, eventRecorder)
	gwDeployer, err := cpaws.NewOcpGatewayDeployer(cloudPrepare, machineSetDeployer, instanceType)
	if err != nil {
		return nil, err
	}

	return &awsProvider{
		eventRecorder:     eventRecorder,
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
	}, a); err != nil {
		return err
	}
	if err := a.cloudPrepare.PrepareForSubmariner(cpapi.PrepareForSubmarinerInput{
		InternalPorts: []cpapi.PortSpec{
			{Port: helpers.SubmarinerRoutePort, Protocol: "udp"},
			{Port: helpers.SubmarinerMetricsPort, Protocol: "tcp"},
		},
	}, a); err != nil {
		return err
	}

	a.eventRecorder.Eventf("SubmarinerClusterEnvBuild", "the submariner cluster env is build on aws")

	return nil
}

// CleanUpSubmarinerClusterEnv clean up submariner cluster environment on AWS after the SubmarinerConfig was deleted
func (a *awsProvider) CleanUpSubmarinerClusterEnv() error {
	if err := a.gatewayDeployer.Cleanup(a); err != nil {
		return err
	}

	if err := a.cloudPrepare.CleanupAfterSubmariner(a); err != nil {
		return err
	}

	a.eventRecorder.Eventf("SubmarinerClusterEnvCleanedUp", "the submariner cluster env is cleaned up on aws")

	return nil
}

// Reporter functions

// Started will report that an operation started on the cloud
func (a *awsProvider) Started(message string, args ...interface{}) {
	a.eventRecorder.Eventf("SubmarinerClusterEnvBuild", message, args...)
}

// Succeeded will report that the last operation on the cloud has succeeded
func (a *awsProvider) Succeeded(message string, args ...interface{}) {
	a.eventRecorder.Eventf("SubmarinerClusterEnvBuild", message, args...)
}

// Failed will report that the last operation on the cloud has failed
func (a *awsProvider) Failed(errs ...error) {
	message := "Failed"
	errMessages := []string{}
	for i := range errs {
		message += "\n%s"
		errMessages = append(errMessages, errs[i].Error())
	}
	a.eventRecorder.Warningf("SubmarinerClusterEnvBuild", message, errMessages)
}
