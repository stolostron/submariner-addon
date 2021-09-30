package gcp

import (
	"context"
	"fmt"
	"strconv"

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

	"github.com/open-cluster-management/submariner-addon/pkg/helpers"

	"github.com/openshift/library-go/pkg/operator/events"

	"github.com/submariner-io/cloud-prepare/pkg/api"
	cloudpreparegcp "github.com/submariner-io/cloud-prepare/pkg/gcp"
	"k8s.io/client-go/kubernetes"
)

const (
	gcpCredentialsName = "osServiceAccount.json"
	gwInstanceType     = "n1-standard-4"
)

type gcpProvider struct {
	infraId           string
	nattPort          uint16
	routePort         string
	metricsPort       uint16
	cloudPrepare      api.Cloud
	eventRecorder     events.Recorder
	gwDeployer        api.GatewayDeployer
	gateways          int
	nattDiscoveryPort int64
}

func NewGCPProvider(
	restMapper meta.RESTMapper,
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	hubKubeClient kubernetes.Interface,
	eventRecorder events.Recorder,
	region, infraId, clusterName, credentialsSecretName, instanceType string,
	nattPort, nattDiscoveryPort, gateways int) (*gcpProvider, error) {
	if infraId == "" {
		return nil, fmt.Errorf("cluster infraId is empty")
	}

	if nattPort == 0 {
		nattPort = helpers.SubmarinerNatTPort
	}

	if instanceType != "" {
		instanceType = gwInstanceType
	}

	if gateways < 1 {
		return nil, fmt.Errorf("the count of gateways is less than 1")
	}

	if nattDiscoveryPort == 0 {
		nattDiscoveryPort = helpers.SubmarinerNatTDiscoveryPort
	}

	projectId, gcpClient, err := newClient(hubKubeClient, clusterName, credentialsSecretName)
	if err != nil {
		klog.Errorf("Unable to retrieve the gcpclient :%v", err)
		return nil, err
	}

	cloudPrepare := cloudpreparegcp.NewCloud(projectId, infraId, region, gcpClient)

	msDeployer := ocp.NewK8sMachinesetDeployer(restMapper, dynamicClient)

	k8sClient, _ := k8s.NewK8sInterface(kubeClient)

	gwDeployer, err := cloudpreparegcp.NewOcpGatewayDeployer(cloudPrepare, msDeployer, instanceType,
		"", false, k8sClient)
	if err != nil {
		return nil, err
	}

	return &gcpProvider{
		infraId:           infraId,
		nattPort:          uint16(nattPort),
		routePort:         strconv.Itoa(helpers.SubmarinerRoutePort),
		metricsPort:       helpers.SubmarinerMetricsPort,
		cloudPrepare:      cloudPrepare,
		gwDeployer:        gwDeployer,
		eventRecorder:     eventRecorder,
		nattDiscoveryPort: int64(nattDiscoveryPort),
		gateways:          gateways,
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
	}, g); err != nil {
		return err
	}

	input := api.PrepareForSubmarinerInput{
		InternalPorts: []api.PortSpec{
			{Port: helpers.SubmarinerRoutePort, Protocol: "udp"},
			{Port: g.metricsPort, Protocol: "tcp"},
		},
	}
	err := g.cloudPrepare.PrepareForSubmariner(input, g)
	if err != nil {
		return err
	}

	g.eventRecorder.Eventf("SubmarinerClusterEnvBuild", "the submariner cluster env is build on gcp")
	return nil
}

// CleanUpSubmarinerClusterEnv clean up submariner cluster environment on GCP after the SubmarinerConfig was deleted
// 1. delete the inbound and outbound firewall rules to close submariner ports
// 2. delete the inbound and outbound firewall rules to close submariner metrics port
func (g *gcpProvider) CleanUpSubmarinerClusterEnv() error {
	err := g.gwDeployer.Cleanup(g)
	if err != nil {
		return err
	}

	return g.cloudPrepare.CleanupAfterSubmariner(g)
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

// Reporter functions

// Started will report that an operation started on the cloud
func (g *gcpProvider) Started(message string, args ...interface{}) {
	g.eventRecorder.Eventf("SubmarinerClusterEnvBuild", fmt.Sprintf(message, args...))
}

// Succeeded will report that the last operation on the cloud has succeeded
func (g *gcpProvider) Succeeded(message string, args ...interface{}) {
	g.eventRecorder.Eventf("SubmarinerClusterEnvBuild", message, args...)
}

// Failed will report that the last operation on the cloud has failed
func (g *gcpProvider) Failed(errs ...error) {
	message := "Failed"
	errMessages := []string{}
	for i := range errs {
		message += "\n%s"
		errMessages = append(errMessages, errs[i].Error())
	}
	g.eventRecorder.Warningf("SubmarinerClusterEnvBuild", message, errMessages)
}
