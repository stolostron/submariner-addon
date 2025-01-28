package gcp

import (
	"context"
	"fmt"
	"strings"

	"github.com/stolostron/submariner-addon/pkg/cloud/provider"
	"github.com/stolostron/submariner-addon/pkg/cloud/reporter"
	"github.com/stolostron/submariner-addon/pkg/constants"
	submreporter "github.com/submariner-io/admiral/pkg/reporter"
	"github.com/submariner-io/cloud-prepare/pkg/api"
	cloudpreparegcp "github.com/submariner-io/cloud-prepare/pkg/gcp"
	gcpclient "github.com/submariner-io/cloud-prepare/pkg/gcp/client"
	"github.com/submariner-io/cloud-prepare/pkg/k8s"
	"github.com/submariner-io/cloud-prepare/pkg/ocp"
	"github.com/submariner-io/submariner/pkg/cni"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/dns/v1"
	"google.golang.org/api/option"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

const (
	gcpCredentialsName = "osServiceAccount.json"
	gwInstanceType     = "n1-standard-4"
)

type gcpProvider struct {
	infraID           string
	nattPort          uint16
	cniType           string
	cloudPrepare      api.Cloud
	reporter          submreporter.Interface
	gwDeployer        api.GatewayDeployer
	gateways          int
	nattDiscoveryPort int64
}

//nolint:revive // Ignore unexported-return - we can't reference the Provider interface here.
func NewProvider(info *provider.Info) (*gcpProvider, error) {
	if info.InfraID == "" {
		return nil, fmt.Errorf("cluster infraID is empty")
	}

	instanceType := info.GatewayConfig.GCP.InstanceType
	if instanceType != "" {
		instanceType = gwInstanceType
	}

	if info.Gateways < 1 {
		return nil, fmt.Errorf("the count of gateways is less than 1")
	}

	projectID, gcpClient, err := newClient(info.CredentialsSecret)
	if err != nil {
		klog.Errorf("Unable to retrieve the gcpclient :%v", err)
		return nil, err
	}

	cloudInfo := cloudpreparegcp.CloudInfo{
		InfraID:   info.InfraID,
		Region:    info.Region,
		ProjectID: projectID,
		Client:    gcpClient,
	}

	if info.SubmarinerConfigAnnotations != nil {
		annotations := info.SubmarinerConfigAnnotations

		if vpcName, exists := annotations["submariner.io/vpc-name"]; exists {
			cloudInfo.VpcName = vpcName
		}

		if publicSubnetName, exists := annotations["submariner.io/public-subnet-name"]; exists {
			cloudInfo.PublicSubnetName = publicSubnetName
		}
	}

	cloudPrepare := cloudpreparegcp.NewCloud(cloudInfo)

	msDeployer := ocp.NewK8sMachinesetDeployer(info.RestMapper, info.DynamicClient)

	k8sClient := k8s.NewInterface(info.KubeClient)

	gwDeployer := cloudpreparegcp.NewOcpGatewayDeployer(cloudInfo, msDeployer, instanceType, "", k8sClient)

	return &gcpProvider{
		infraID:           info.InfraID,
		nattPort:          uint16(info.IPSecNATTPort), //nolint:gosec // Valid port numbers fit in 16 bits
		cniType:           info.NetworkType,
		cloudPrepare:      cloudPrepare,
		gwDeployer:        gwDeployer,
		reporter:          reporter.NewEventRecorderWrapper("GCPCloudProvider", info.EventRecorder),
		nattDiscoveryPort: int64(info.NATTDiscoveryPort),
		gateways:          info.Gateways,
	}, nil
}

// PrepareSubmarinerClusterEnv prepares submariner cluster environment on GCP
// The below tasks will be executed
// 1. create the inbound and outbound firewall rules for submariner, below ports will be opened
//   - IPsec IKE port (by default 500/UDP)
//   - NAT traversal port (by default 4500/UDP)
//   - 4800/UDP port to encapsulate Pod traffic from worker and master nodes to the Submariner Gateway nodes
func (g *gcpProvider) PrepareSubmarinerClusterEnv() error {
	if err := g.gwDeployer.Deploy(api.GatewayDeployInput{
		PublicPorts: []api.PortSpec{
			{Port: g.nattPort, Protocol: "udp"},
			{Port: uint16(g.nattDiscoveryPort), Protocol: "udp"}, //nolint:gosec // Valid port numbers fit in 16 bits
			// ESP & AH protocols are used for private-ip to private-ip gateway communications
			{Port: 0, Protocol: "esp"},
			{Port: 0, Protocol: "ah"},
		},
		Gateways: g.gateways,
	}, g.reporter); err != nil {
		return err
	}

	if !strings.EqualFold(g.cniType, cni.OVNKubernetes) {
		if err := g.cloudPrepare.OpenPorts([]api.PortSpec{
			{Port: constants.SubmarinerRoutePort, Protocol: "udp"},
		}, g.reporter); err != nil {
			return err
		}
	}

	g.reporter.Success("The Submariner cluster environment has been set up on GCP")

	return nil
}

// CleanUpSubmarinerClusterEnv clean up submariner cluster environment on GCP after the SubmarinerConfig was deleted
// 1. delete the inbound and outbound firewall rules to close submariner ports.
func (g *gcpProvider) CleanUpSubmarinerClusterEnv() error {
	err := g.gwDeployer.Cleanup(g.reporter)
	if err != nil {
		return err
	}

	err = g.cloudPrepare.ClosePorts(g.reporter)
	if err != nil {
		return err
	}

	g.reporter.Success("The Submariner cluster environment has been cleaned up on GCP")

	return nil
}

func newClient(credentialsSecret *corev1.Secret) (string, gcpclient.Interface, error) {
	authJSON, ok := credentialsSecret.Data[gcpCredentialsName]
	if !ok {
		return "", nil, fmt.Errorf("the gcp credentials %s is not in secret %s/%s", gcpCredentialsName,
			credentialsSecret.Namespace, credentialsSecret.Name)
	}

	ctx := context.TODO()

	// since we're using a single creds var, we should specify all the required scopes when initializing
	creds, err := google.CredentialsFromJSON(ctx, authJSON, dns.CloudPlatformScope)
	if err != nil {
		return "", nil, err
	}

	// Create a GCP client with the credentials.
	computeClient, err := gcpclient.NewClient(creds.ProjectID, []option.ClientOption{option.WithCredentials(creds)})
	if err != nil {
		return "", nil, err
	}

	return creds.ProjectID, computeClient, nil
}
