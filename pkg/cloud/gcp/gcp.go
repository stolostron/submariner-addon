package gcp

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"github.com/stolostron/submariner-addon/pkg/cloud/provider"
	"github.com/stolostron/submariner-addon/pkg/cloud/reporter"
	"github.com/stolostron/submariner-addon/pkg/cloud/tls"
	"github.com/stolostron/submariner-addon/pkg/constants"
	submreporter "github.com/submariner-io/admiral/pkg/reporter"
	"github.com/submariner-io/cloud-prepare/pkg/api"
	cloudpreparegcp "github.com/submariner-io/cloud-prepare/pkg/gcp"
	gcpclient "github.com/submariner-io/cloud-prepare/pkg/gcp/client"
	"github.com/submariner-io/cloud-prepare/pkg/k8s"
	"github.com/submariner-io/cloud-prepare/pkg/ocp"
	"github.com/submariner-io/submariner/pkg/cni"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/dns/v1"
	"google.golang.org/api/option"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/dynamic"
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

func NewProvider(ctx context.Context, info *provider.Info) (*gcpProvider, error) {
	if info.InfraID == "" {
		return nil, errors.New("cluster infraID is empty")
	}

	instanceType := info.GatewayConfig.GCP.InstanceType
	if instanceType != "" {
		instanceType = gwInstanceType
	}

	if info.Gateways < 1 {
		return nil, errors.New("the count of gateways is less than 1")
	}

	rep := reporter.NewEventRecorderWrapper("GCPCloudProvider", info.EventRecorder)

	projectID, gcpClient, err := newClient(ctx, info.DynamicClient, rep, info.CredentialsSecret)
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
		nattPort:          uint16(info.IPSecNATTPort), //nolint:gosec // Usable port numbers fit
		cniType:           info.NetworkType,
		cloudPrepare:      cloudPrepare,
		gwDeployer:        gwDeployer,
		reporter:          rep,
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
func (g *gcpProvider) PrepareSubmarinerClusterEnv(ctx context.Context) error {
	if err := g.gwDeployer.Deploy(ctx, api.GatewayDeployInput{
		PublicPorts: []api.PortSpec{
			{Port: g.nattPort, Protocol: "udp"},
			{Port: uint16(g.nattDiscoveryPort), Protocol: "udp"}, //nolint:gosec // Usable port numbers fit
			// ESP & AH protocols are used for private-ip to private-ip gateway communications
			{Port: 0, Protocol: "esp"},
			{Port: 0, Protocol: "ah"},
		},
		Gateways: g.gateways,
	}, g.reporter); err != nil {
		return errors.Wrap(err, "error deploying gateway")
	}

	if !strings.EqualFold(g.cniType, cni.OVNKubernetes) {
		if err := g.cloudPrepare.OpenPorts(ctx, []api.PortSpec{
			{Port: constants.SubmarinerRoutePort, Protocol: "udp"},
		}, g.reporter); err != nil {
			return errors.Wrap(err, "error opening ports")
		}
	}

	g.reporter.Success("The Submariner cluster environment has been set up on GCP")

	return nil
}

// CleanUpSubmarinerClusterEnv clean up submariner cluster environment on GCP after the SubmarinerConfig was deleted
// 1. delete the inbound and outbound firewall rules to close submariner ports.
func (g *gcpProvider) CleanUpSubmarinerClusterEnv(ctx context.Context) error {
	err := g.gwDeployer.Cleanup(ctx, g.reporter)
	if err != nil {
		return errors.Wrap(err, "error cleaning up gateway")
	}

	err = g.cloudPrepare.ClosePorts(ctx, g.reporter)
	if err != nil {
		return errors.Wrap(err, "error closing ports")
	}

	g.reporter.Success("The Submariner cluster environment has been cleaned up on GCP")

	return nil
}

func newClient(ctx context.Context, dynamicClient dynamic.Interface, rep submreporter.Interface, credentialsSecret *corev1.Secret,
) (string, gcpclient.Interface, error) {
	authJSON, ok := credentialsSecret.Data[gcpCredentialsName]
	if !ok {
		return "", nil, fmt.Errorf("the gcp credentials %s is not in secret %s/%s", gcpCredentialsName,
			credentialsSecret.Namespace, credentialsSecret.Name)
	}

	// Create HTTP client with TLS configuration BEFORE setting up credentials
	// This ensures the OAuth2 token acquisition also uses the cluster TLS profile
	baseHTTPClient, err := tls.GetConfiguredHTTPClient(ctx, dynamicClient, rep, false)
	if err != nil {
		return "", nil, errors.Wrap(err, "unable to create HTTP client")
	}

	// since we're using a single creds var, we should specify all the required scopes when initializing
	// Use CredentialsFromJSONWithType to explicitly validate that we're loading a service account credential
	creds, err := google.CredentialsFromJSONWithType(ctx, authJSON, google.ServiceAccount, dns.CloudPlatformScope)
	if err != nil {
		return "", nil, errors.Wrap(err, "error retrieving credentials")
	}

	// Wrap the TLS-configured transport with OAuth2 transport to apply service account authentication
	// while preserving the cluster TLS profile settings
	httpClient := &http.Client{
		Transport: &oauth2.Transport{
			Base:   baseHTTPClient.Transport,
			Source: creds.TokenSource,
		},
	}

	// Create a GCP client with the custom HTTP client (which includes both TLS config and auth)
	computeClient, err := gcpclient.NewClient(ctx, creds.ProjectID, []option.ClientOption{
		option.WithHTTPClient(httpClient),
	})
	if err != nil {
		return "", nil, errors.Wrap(err, "error creating GCP client")
	}

	return creds.ProjectID, computeClient, nil
}
