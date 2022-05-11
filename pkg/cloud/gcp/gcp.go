package gcp

import (
	"context"
	"fmt"
	"strconv"

	"github.com/stolostron/submariner-addon/pkg/constants"
	gcpclient "github.com/submariner-io/cloud-prepare/pkg/gcp/client"
	"github.com/submariner-io/cloud-prepare/pkg/k8s"
	"github.com/submariner-io/cloud-prepare/pkg/ocp"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/dns/v1"
	"google.golang.org/api/option"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/stolostron/submariner-addon/pkg/cloud/reporter"

	"github.com/submariner-io/cloud-prepare/pkg/api"
	cloudpreparegcp "github.com/submariner-io/cloud-prepare/pkg/gcp"
	"k8s.io/client-go/kubernetes"
)

const (
	gcpCredentialsName = "osServiceAccount.json"
	gwInstanceType     = "n1-standard-4"
)

type gcpProvider struct {
	infraID              string
	nattPort             uint16
	routePort            string
	gatewayMetricsPort   uint16
	globalnetMetricsPort uint16
	cloudPrepare         api.Cloud
	reporter             api.Reporter
	gwDeployer           api.GatewayDeployer
	gateways             int
	nattDiscoveryPort    int64
}

func NewGCPProvider(
	restMapper meta.RESTMapper,
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	hubKubeClient kubernetes.Interface,
	eventRecorder events.Recorder,
	region, infraID, clusterName, credentialsSecretName, instanceType string,
	nattPort, nattDiscoveryPort, gateways int,
) (*gcpProvider, error) {
	if infraID == "" {
		return nil, fmt.Errorf("cluster infraID is empty")
	}

	if nattPort == 0 {
		nattPort = constants.SubmarinerNatTPort
	}

	if instanceType != "" {
		instanceType = gwInstanceType
	}

	if gateways < 1 {
		return nil, fmt.Errorf("the count of gateways is less than 1")
	}

	if nattDiscoveryPort == 0 {
		nattDiscoveryPort = constants.SubmarinerNatTDiscoveryPort
	}

	projectId, gcpClient, err := newClient(hubKubeClient, clusterName, credentialsSecretName)
	if err != nil {
		klog.Errorf("Unable to retrieve the gcpclient :%v", err)
		return nil, err
	}

	cloudInfo := cloudpreparegcp.CloudInfo{
		InfraID:   infraID,
		Region:    region,
		ProjectID: projectId,
		Client:    gcpClient,
	}

	cloudPrepare := cloudpreparegcp.NewCloud(cloudInfo)

	msDeployer := ocp.NewK8sMachinesetDeployer(restMapper, dynamicClient)

	k8sClient := k8s.NewInterface(kubeClient)

	gwDeployer := cloudpreparegcp.NewOcpGatewayDeployer(cloudInfo, msDeployer, instanceType,
		"", true, k8sClient)

	return &gcpProvider{
		infraID:              infraID,
		nattPort:             uint16(nattPort),
		routePort:            strconv.Itoa(constants.SubmarinerRoutePort),
		gatewayMetricsPort:   constants.SubmarinerGatewayMetricsPort,
		globalnetMetricsPort: constants.SubmarinerGlobalnetMetricsPort,
		cloudPrepare:         cloudPrepare,
		gwDeployer:           gwDeployer,
		reporter:             reporter.NewEventRecorderWrapper("GCPCloudProvider", eventRecorder),
		nattDiscoveryPort:    int64(nattDiscoveryPort),
		gateways:             gateways,
	}, nil
}

// PrepareSubmarinerClusterEnv prepares submariner cluster environment on GCP
// The below tasks will be executed
// 1. create the inbound and outbound firewall rules for submariner, below ports will be opened
//    - IPsec IKE port (by default 500/UDP)
//    - NAT traversal port (by default 4500/UDP)
//    - 4800/UDP port to encapsulate Pod traffic from worker and master nodes to the Submariner Gateway nodes
// 2. create the inbound and outbound firewall rules to open 8080/TCP port to export metrics service from the Submariner gateway
func (g *gcpProvider) PrepareSubmarinerClusterEnv() error {
	if err := g.gwDeployer.Deploy(api.GatewayDeployInput{
		PublicPorts: []api.PortSpec{
			{Port: g.nattPort, Protocol: "udp"},
			{Port: uint16(g.nattDiscoveryPort), Protocol: "udp"},
			// ESP & AH protocols are used for private-ip to private-ip gateway communications
			{Port: 0, Protocol: "esp"},
			{Port: 0, Protocol: "ah"},
		},
		Gateways: g.gateways,
	}, g.reporter); err != nil {
		return err
	}

	input := api.PrepareForSubmarinerInput{
		InternalPorts: []api.PortSpec{
			{Port: constants.SubmarinerRoutePort, Protocol: "udp"},
			{Port: g.gatewayMetricsPort, Protocol: "tcp"},
			{Port: g.globalnetMetricsPort, Protocol: "tcp"},
		},
	}
	err := g.cloudPrepare.PrepareForSubmariner(input, g.reporter)
	if err != nil {
		return err
	}

	g.reporter.Succeeded("The Submariner cluster environment has been set up on GCP")
	return nil
}

// CleanUpSubmarinerClusterEnv clean up submariner cluster environment on GCP after the SubmarinerConfig was deleted
// 1. delete the inbound and outbound firewall rules to close submariner ports
// 2. delete the inbound and outbound firewall rules to close submariner metrics port
func (g *gcpProvider) CleanUpSubmarinerClusterEnv() error {
	err := g.gwDeployer.Cleanup(g.reporter)
	if err != nil {
		return err
	}

	err = g.cloudPrepare.CleanupAfterSubmariner(g.reporter)
	if err != nil {
		return err
	}

	g.reporter.Succeeded("The Submariner cluster environment has been cleaned up on GCP")
	return nil
}

func newClient(kubeClient kubernetes.Interface, secretNamespace, secretName string) (string, gcpclient.Interface, error) {
	credentialsSecret, err := kubeClient.CoreV1().Secrets(secretNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return "", nil, err
	}

	authJSON, ok := credentialsSecret.Data[gcpCredentialsName]
	if !ok {
		return "", nil, fmt.Errorf("the gcp credentials %s is not in secret %s/%s", gcpCredentialsName, secretNamespace, secretName)
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
