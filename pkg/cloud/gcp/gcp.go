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
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/klog"

	"github.com/open-cluster-management/submariner-addon/pkg/helpers"

	"github.com/openshift/library-go/pkg/operator/events"
	operatorhelpers "github.com/openshift/library-go/pkg/operator/v1helpers"

	"github.com/submariner-io/cloud-prepare/pkg/api"
	cpapi "github.com/submariner-io/cloud-prepare/pkg/api"
	cloudpreparegcp "github.com/submariner-io/cloud-prepare/pkg/gcp"
	"k8s.io/client-go/kubernetes"
)

const (
	gcpCredentialsName = "osServiceAccount.json"
	gwInstanceType     = "n1-standard-4"
)

type gcpProvider struct {
	infraId           string
	projectId         string
	instanceType      string
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
	restConfig *rest.Config	,
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

	projectId, gcpClient, err := NewClient(hubKubeClient, clusterName, credentialsSecretName)
	if err != nil {
		klog.Errorf("Unable to retrieve the gcpclient :%v", err)
		return nil, err
	}

	cloudPrepare := cloudpreparegcp.NewCloud(projectId, infraId, region, gcpClient)

	restMapper, err := buildRestMapper(restConfig)
	if err != nil {
		return nil, err
	}

	msDeployer := ocp.NewK8sMachinesetDeployer(restMapper, dynamicClient)

	k8sClient, _ := k8s.NewK8sInterface(kubeClient)

	gwDeployer, err := cloudpreparegcp.NewOcpGatewayDeployer(cloudPrepare, msDeployer, instanceType,
		"", false, k8sClient)

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
	if err := g.gwDeployer.Deploy(cpapi.GatewayDeployInput{
		PublicPorts: []cpapi.PortSpec{
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
	var errs []error

	err := g.gwDeployer.Cleanup(g)
	if err != nil {
		return err
	}

	err = g.cloudPrepare.CleanupAfterSubmariner(g)
	if err != nil {
		return err
	}
	return operatorhelpers.NewMultiLineAggregate(errs)
}

func NewClient(kubeClient kubernetes.Interface, secretNamespace, secretName string) (string, gcpclient.Interface, error) {
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

func buildRestMapper(restConfig *rest.Config) (meta.RESTMapper, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating discovery client: %v", err)
	}

	groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		return nil, fmt.Errorf("error retrieving API group resources: %v", err)
	}

	return restmapper.NewDiscoveryRESTMapper(groupResources), nil
}

func (g *gcpProvider) Started(message string, args ...interface{}) {
	klog.Infof(fmt.Sprintf(message, args...))
}

func (g *gcpProvider) Succeeded(message string, args ...interface{}) {
	if message != "" {
		klog.Info(fmt.Sprintf(message, args...))
	}
}

func (g *gcpProvider) Failed(err ...error) {
	if len(err) > 0 {
		klog.Error(err[0].Error())
	}
}
