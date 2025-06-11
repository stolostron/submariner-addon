package rhos

import (
	"crypto/tls"
	"errors"
	"net/http"
	"strings"

	"github.com/gophercloud/gophercloud"
	"github.com/gophercloud/gophercloud/openstack"
	"github.com/gophercloud/utils/openstack/clientconfig"
	"github.com/stolostron/submariner-addon/pkg/cloud/provider"
	"github.com/stolostron/submariner-addon/pkg/cloud/reporter"
	"github.com/stolostron/submariner-addon/pkg/constants"
	submreporter "github.com/submariner-io/admiral/pkg/reporter"
	"github.com/submariner-io/cloud-prepare/pkg/api"
	"github.com/submariner-io/cloud-prepare/pkg/k8s"
	"github.com/submariner-io/cloud-prepare/pkg/ocp"
	cloudpreparerhos "github.com/submariner-io/cloud-prepare/pkg/rhos"
	"github.com/submariner-io/submariner/pkg/cni"
	"gopkg.in/yaml.v2"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
)

const (
	cloudsYAMLName = "clouds.yaml"
	cloudName      = "cloud"
	gwInstanceType = "PnTAE.CPU_4_Memory_8192_Disk_50"
)

type rhosProvider struct {
	infraID           string
	nattPort          uint16
	cniType           string
	cloudPrepare      api.Cloud
	reporter          submreporter.Interface
	gwDeployer        api.GatewayDeployer
	gateways          int
	nattDiscoveryPort int64
}

func NewProvider(info *provider.Info) (*rhosProvider, error) {
	if info.InfraID == "" {
		return nil, errors.New("cluster infraID is empty")
	}

	instanceType := info.GatewayConfig.RHOS.InstanceType
	if instanceType == "" {
		instanceType = gwInstanceType
	}

	if info.Gateways < 1 {
		return nil, errors.New("the count of gateways is less than 1")
	}

	projectID, cloudEntry, providerClient, err := newClient(info.CredentialsSecret)
	if err != nil {
		klog.Errorf("Unable to retrieve the rhosclient :%v", err)
		return nil, err
	}

	k8sClient := k8s.NewInterface(info.KubeClient)

	cloudInfo := cloudpreparerhos.CloudInfo{
		InfraID:   info.InfraID,
		Region:    info.Region,
		K8sClient: k8sClient,
		Client:    providerClient,
	}

	cloudPrepare := cloudpreparerhos.NewCloud(cloudInfo)
	msDeployer := ocp.NewK8sMachinesetDeployer(info.RestMapper, info.DynamicClient)

	gwDeployer := cloudpreparerhos.NewOcpGatewayDeployer(cloudInfo, msDeployer, projectID, instanceType, "", cloudEntry)

	return &rhosProvider{
		infraID:           info.InfraID,
		nattPort:          uint16(info.IPSecNATTPort), //nolint:gosec // Valid port numbers fit in 16 bits
		cniType:           info.NetworkType,
		cloudPrepare:      cloudPrepare,
		gwDeployer:        gwDeployer,
		reporter:          reporter.NewEventRecorderWrapper("RHOSCloudProvider", info.EventRecorder),
		nattDiscoveryPort: int64(info.NATTDiscoveryPort),
		gateways:          info.Gateways,
	}, nil
}

// PrepareSubmarinerClusterEnv prepares submariner cluster environment on RHOS
// The below tasks will be executed
// 1. create the inbound and outbound firewall rules for submariner, below ports will be opened
//   - NAT traversal port (by default 4500/UDP)
//   - 4800/UDP port to encapsulate Pod traffic from worker and master nodes to the Submariner Gateway nodes
//   - ESP & AH protocols for private-ip to private-ip gateway communications
func (r *rhosProvider) PrepareSubmarinerClusterEnv() error {
	if err := r.gwDeployer.Deploy(api.GatewayDeployInput{
		PublicPorts: []api.PortSpec{
			{Port: r.nattPort, Protocol: "udp"},
			{Port: uint16(r.nattDiscoveryPort), Protocol: "udp"}, //nolint:gosec // Valid port numbers fit in 16 bits
			{Port: 0, Protocol: "esp"},
			{Port: 0, Protocol: "ah"},
		},
		Gateways: r.gateways,
	}, r.reporter); err != nil {
		return err
	}

	if !strings.EqualFold(r.cniType, cni.OVNKubernetes) {
		if err := r.cloudPrepare.OpenPorts([]api.PortSpec{
			{Port: constants.SubmarinerRoutePort, Protocol: "udp"},
		}, r.reporter); err != nil {
			return err
		}
	}

	r.reporter.Success("The Submariner cluster environment has been set up on RHOS")

	return nil
}

// CleanUpSubmarinerClusterEnv clean up submariner cluster environment on RHOS after the SubmarinerConfig was deleted
// 1. delete any dedicated gateways that were previously deployed.
// 2. delete the inbound and outbound firewall rules to close submariner ports.
func (r *rhosProvider) CleanUpSubmarinerClusterEnv() error {
	err := r.gwDeployer.Cleanup(r.reporter)
	if err != nil {
		return err
	}

	err = r.cloudPrepare.ClosePorts(r.reporter)
	if err != nil {
		return err
	}

	r.reporter.Success("The Submariner cluster environment has been cleaned up on RHOS")

	return nil
}

func newClient(credentialsSecret *corev1.Secret) (string, string, *gophercloud.ProviderClient, error) {
	cloudsYAML, ok := credentialsSecret.Data[cloudsYAMLName]
	if !ok {
		return "", "", nil, errors.New("cloud yaml is not found in the credentials")
	}

	cloudName, ok := credentialsSecret.Data[cloudName]
	if !ok {
		return "", "", nil, errors.New("cloud name is not found in the credentials")
	}

	var cloudsAll clientconfig.Clouds
	if err := yaml.Unmarshal(cloudsYAML, &cloudsAll); err != nil {
		return "", "", nil, err
	}

	cloudNameStr := string(cloudName)
	cloud := cloudsAll.Clouds[cloudNameStr]

	opts := gophercloud.AuthOptions{
		IdentityEndpoint:            cloud.AuthInfo.AuthURL,
		TokenID:                     cloud.AuthInfo.Token,
		ApplicationCredentialID:     cloud.AuthInfo.ApplicationCredentialID,
		ApplicationCredentialSecret: cloud.AuthInfo.ApplicationCredentialSecret,
		Username:                    cloud.AuthInfo.Username,
		Password:                    cloud.AuthInfo.Password,
		DomainName:                  cloud.AuthInfo.UserDomainName,
		TenantID:                    cloud.AuthInfo.ProjectID,
		TenantName:                  cloud.AuthInfo.ProjectName,
	}

	projectID := cloud.AuthInfo.ProjectID

	providerClient, err := openstack.AuthenticatedClient(opts)
	if err != nil {
		return "", "", nil, err
	}

	if !ptr.Deref(cloud.Verify, true) {
		config := &tls.Config{InsecureSkipVerify: true} //nolint:gosec // Disabling certificate validation is explicitly requested
		transport := http.DefaultTransport.(*http.Transport).Clone()
		transport.TLSClientConfig = config
		providerClient.HTTPClient = http.Client{Transport: transport}
	}

	return projectID, cloudNameStr, providerClient, nil
}
