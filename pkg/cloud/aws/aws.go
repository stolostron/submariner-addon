package aws

import (
	"fmt"
	"strings"

	"github.com/stolostron/submariner-addon/pkg/cloud/provider"
	"github.com/stolostron/submariner-addon/pkg/cloud/reporter"
	"github.com/stolostron/submariner-addon/pkg/constants"
	submreporter "github.com/submariner-io/admiral/pkg/reporter"
	cpapi "github.com/submariner-io/cloud-prepare/pkg/api"
	cpaws "github.com/submariner-io/cloud-prepare/pkg/aws"
	cpclient "github.com/submariner-io/cloud-prepare/pkg/aws/client"
	"github.com/submariner-io/cloud-prepare/pkg/ocp"
	"github.com/submariner-io/submariner/pkg/cni"
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
	cniType           string
	gateways          int
	cloudPrepare      cpapi.Cloud
	gatewayDeployer   cpapi.GatewayDeployer
}

//nolint:revive // Ignore unexported-return - we can't reference the Provider interface here.
func NewProvider(info *provider.Info) (*awsProvider, error) {
	if info.Region == "" {
		return nil, fmt.Errorf("cluster region is empty")
	}

	if info.InfraID == "" {
		return nil, fmt.Errorf("cluster infraID is empty")
	}

	if info.Gateways < 1 {
		return nil, fmt.Errorf("the count of gateways is less than 1")
	}

	instanceType := info.GatewayConfig.AWS.InstanceType
	if instanceType == "" {
		instanceType = defaultInstanceType
	}

	accessKeyID, ok := info.CredentialsSecret.Data[accessKeyIDSecretKey]
	if !ok {
		return nil, fmt.Errorf("the aws credentials key %s is not in secret %s/%s", accessKeyIDSecretKey, info.ClusterName,
			info.CredentialsSecret.Name)
	}

	secretAccessKey, ok := info.CredentialsSecret.Data[accessKeySecretKey]
	if !ok {
		return nil, fmt.Errorf("the aws credentials key %s is not in secret %s/%s", accessKeySecretKey, info.ClusterName,
			info.CredentialsSecret.Name)
	}

	awsClient, err := cpclient.New(string(accessKeyID), string(secretAccessKey), info.Region)
	if err != nil {
		return nil, err
	}

	var cloudOptions []cpaws.CloudOption

	if info.SubmarinerConfigAnnotations != nil {
		annotations := info.SubmarinerConfigAnnotations

		if vpcID, exists := annotations["submariner.io/vpc-id"]; exists {
			cloudOptions = append(cloudOptions, cpaws.WithVPCName(vpcID))
		}

		if subnetIDList, exists := annotations["submariner.io/subnet-id-list"]; exists {
			subnetIDs := strings.Split(subnetIDList, ",")
			for i := range subnetIDs {
				subnetIDs[i] = strings.TrimSpace(subnetIDs[i])
			}
			cloudOptions = append(cloudOptions, cpaws.WithPublicSubnetList(subnetIDs))
		}

		if controlPlaneSGID, exists := annotations["submariner.io/control-plane-sg-id"]; exists {
			cloudOptions = append(cloudOptions, cpaws.WithControlPlaneSecurityGroup(controlPlaneSGID))
		}

		if workerSGID, exists := annotations["submariner.io/worker-sg-id"]; exists {
			cloudOptions = append(cloudOptions, cpaws.WithWorkerSecurityGroup(workerSGID))
		}
	}

	cloudPrepare := cpaws.NewCloud(awsClient, info.InfraID, info.Region, cloudOptions...)

	machineSetDeployer := ocp.NewK8sMachinesetDeployer(info.RestMapper, info.DynamicClient)
	gwDeployer, err := cpaws.NewOcpGatewayDeployer(cloudPrepare, machineSetDeployer, instanceType)
	if err != nil {
		return nil, err
	}

	return &awsProvider{
		reporter:          reporter.NewEventRecorderWrapper("AWSCloudProvider", info.EventRecorder),
		nattPort:          int64(info.IPSecNATTPort),
		nattDiscoveryPort: int64(info.NATTDiscoveryPort),
		cniType:           info.NetworkType,
		instanceType:      instanceType,
		gateways:          info.Gateways,
		cloudPrepare:      cloudPrepare,
		gatewayDeployer:   gwDeployer,
	}, nil
}

// PrepareSubmarinerClusterEnv prepares submariner cluster environment on AWS.
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

	if !strings.EqualFold(a.cniType, cni.OVNKubernetes) {
		if err := a.cloudPrepare.OpenPorts([]cpapi.PortSpec{
			{Port: constants.SubmarinerRoutePort, Protocol: "udp"},
		}, a.reporter); err != nil {
			return err
		}
	}

	a.reporter.Success("The Submariner cluster environment has been set up on AWS")

	return nil
}

// CleanUpSubmarinerClusterEnv clean up submariner cluster environment on AWS after the SubmarinerConfig was deleted.
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
