package rhos

import (
	"context"
	"errors"
	"fmt"
	"github.com/gophercloud/gophercloud"
	"gopkg.in/yaml.v2"
	"strconv"

	"github.com/gophercloud/utils/openstack/clientconfig"
	"github.com/stolostron/submariner-addon/pkg/constants"
	"github.com/submariner-io/cloud-prepare/pkg/k8s"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/stolostron/submariner-addon/pkg/cloud/reporter"

	"github.com/submariner-io/cloud-prepare/pkg/api"
	cloudpreparerhos "github.com/submariner-io/cloud-prepare/pkg/rhos"
	"k8s.io/client-go/kubernetes"
)

const (
	cloudsYAMLName = "clouds.yaml"
	cloudName      = "cloud"
	gwInstanceType     = "n1-standard-4"
)

type rhosProvider struct {
	infraID           string
	nattPort          uint16
	routePort         string
	metricsPort       uint16
	cloudPrepare      api.Cloud
	reporter          api.Reporter
	gwDeployer        api.GatewayDeployer
	gateways          int
	nattDiscoveryPort int64
}

func NewRHOSProvider(
	restMapper meta.RESTMapper,
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	hubKubeClient kubernetes.Interface,
	eventRecorder events.Recorder,
	region, infraID, clusterName, credentialsSecretName, instanceType string,
	nattPort, nattDiscoveryPort, gateways int) (*rhosProvider, error) {
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

	projectId, providerClient, err := newClient(hubKubeClient, clusterName, credentialsSecretName)
	if err != nil {
		klog.Errorf("Unable to retrieve the gcpclient :%v", err)
		return nil, err
	}
	k8sClient := k8s.NewInterface(kubeClient)

	cloudInfo := cloudpreparerhos.CloudInfo{
		InfraID:   infraID,
		Region:    region,
		K8sClient: k8sClient,
		Client:    providerClient,
	}

	cloudPrepare := cloudpreparerhos.NewCloud(cloudInfo)

	gwDeployer := cloudpreparerhos.NewOcpGatewayDeployer(cloudInfo, projectId)

	return &rhosProvider{
		infraID:           infraID,
		nattPort:          uint16(nattPort),
		routePort:         strconv.Itoa(constants.SubmarinerRoutePort),
		metricsPort:       constants.SubmarinerMetricsPort,
		cloudPrepare:      cloudPrepare,
		gwDeployer:        gwDeployer,
		reporter:          reporter.NewEventRecorderWrapper("GCPCloudProvider", eventRecorder),
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
func (r *rhosProvider) PrepareSubmarinerClusterEnv() error {
	if err := r.gwDeployer.Deploy(api.GatewayDeployInput{
		PublicPorts: []api.PortSpec{
			{Port: r.nattPort, Protocol: "udp"},
			{Port: uint16(r.nattDiscoveryPort), Protocol: "udp"},
			// ESP & AH protocols are used for private-ip to private-ip gateway communications
			{Port: 0, Protocol: "esp"},
			{Port: 0, Protocol: "ah"},
		},
		Gateways: r.gateways,
	}, r.reporter); err != nil {
		return err
	}

	input := api.PrepareForSubmarinerInput{
		InternalPorts: []api.PortSpec{
			{Port: constants.SubmarinerRoutePort, Protocol: "udp"},
			{Port: r.metricsPort, Protocol: "tcp"},
		},
	}
	err := r.cloudPrepare.PrepareForSubmariner(input, r.reporter)
	if err != nil {
		return err
	}

	r.reporter.Succeeded("The Submariner cluster environment has been set up on GCP")
	return nil
}

// CleanUpSubmarinerClusterEnv clean up submariner cluster environment on GCP after the SubmarinerConfig was deleted
// 1. delete the inbound and outbound firewall rules to close submariner ports
// 2. delete the inbound and outbound firewall rules to close submariner metrics port
func (r rhosProvider) CleanUpSubmarinerClusterEnv() error {
	err := r.gwDeployer.Cleanup(r.reporter)
	if err != nil {
		return err
	}

	err = r.cloudPrepare.CleanupAfterSubmariner(r.reporter)
	if err != nil {
		return err
	}

	r.reporter.Succeeded("The Submariner cluster environment has been cleaned up on GCP")
	return nil
}

func newClient(kubeClient kubernetes.Interface, secretNamespace, secretName string) (string, *gophercloud.ProviderClient, error) {
	credentialsSecret, err := kubeClient.CoreV1().Secrets(secretNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return "", nil, err
	}

	cloudsYAML, ok := credentialsSecret.Data[cloudsYAMLName]
	if !ok  {
		return "", nil, errors.New("cloud yaml is not found in the credentials")
	}

	cloudName, ok := credentialsSecret.Data[cloudName]
	if !ok  {
		return "", nil, errors.New("cloud name is not found in the credentials")
	}

	var cloudsAll clientconfig.Clouds
	if err := yaml.Unmarshal(cloudsYAML, &cloudsAll); err != nil {
		return "", nil, err
	}
	cloud := cloudsAll.Clouds[string(cloudName)]

	opts := &clientconfig.ClientOpts{
		Cloud:        string(cloudName),
		AuthInfo:     cloud.AuthInfo,
	}

	projectID := cloud.AuthInfo.ProjectID

	providerClient, err := clientconfig.AuthenticatedClient(opts)
	if err != nil {
		return "", nil, err
	}

	return projectID, providerClient, nil
}
