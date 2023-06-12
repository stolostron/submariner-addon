package azure

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/pkg/errors"
	"github.com/stolostron/submariner-addon/pkg/cloud/provider"
	"github.com/stolostron/submariner-addon/pkg/cloud/reporter"
	"github.com/stolostron/submariner-addon/pkg/constants"
	submreporter "github.com/submariner-io/admiral/pkg/reporter"
	"github.com/submariner-io/cloud-prepare/pkg/api"
	"github.com/submariner-io/cloud-prepare/pkg/azure"
	"github.com/submariner-io/cloud-prepare/pkg/k8s"
	"github.com/submariner-io/cloud-prepare/pkg/ocp"
	"github.com/submariner-io/submariner/pkg/cni"
	corev1 "k8s.io/api/core/v1"
)

const (
	servicePrincipalJSON = "osServicePrincipal.json"
	gwInstanceType       = "Standard_F4s_v2"
)

type azureProvider struct {
	infraID           string
	cniType           string
	cloudPrepare      api.Cloud
	reporter          submreporter.Interface
	gwDeployer        api.GatewayDeployer
	nattDiscoveryPort int64
	gateways          int
	nattPort          uint16
	airGapped         bool
}

//nolint:revive // Ignore unexported-return - we can't reference the Provider interface here.
func NewProvider(info *provider.Info) (*azureProvider, error) {
	if info.InfraID == "" {
		return nil, fmt.Errorf("cluster infraID is empty")
	}

	instanceType := info.GatewayConfig.Azure.InstanceType
	if instanceType == "" {
		instanceType = gwInstanceType
	}

	if info.Gateways < 1 {
		return nil, fmt.Errorf("the count of gateways is less than 1")
	}

	subscriptionID, err := initializeFromAuthFile(info.CredentialsSecret)
	if err != nil {
		return nil, errors.Wrap(err, "unable to initialize from auth file")
	}

	credentials, err := azidentity.NewEnvironmentCredential(nil)
	if err != nil {
		return nil, errors.Wrap(err, "unable to create the Azure credentials")
	}

	k8sClient := k8s.NewInterface(info.KubeClient)

	msDeployer := ocp.NewK8sMachinesetDeployer(info.RestMapper, info.DynamicClient)

	cloudInfo := azure.CloudInfo{
		SubscriptionID:  subscriptionID,
		InfraID:         info.InfraID,
		Region:          info.Region,
		BaseGroupName:   info.InfraID + "-rg",
		TokenCredential: credentials,
		K8sClient:       k8sClient,
	}

	azureCloud := azure.NewCloud(&cloudInfo)

	gwDeployer, err := azure.NewOcpGatewayDeployer(&cloudInfo, azureCloud, msDeployer, instanceType, true)
	if err != nil {
		return nil, errors.Wrap(err, "failed to initialise a GatewayDeployer config")
	}

	cloudPrepare := azure.NewCloud(&cloudInfo)

	return &azureProvider{
		infraID:           info.InfraID,
		nattPort:          uint16(info.IPSecNATTPort),
		cniType:           info.NetworkType,
		cloudPrepare:      cloudPrepare,
		gwDeployer:        gwDeployer,
		reporter:          reporter.NewEventRecorderWrapper("AzureCloudProvider", info.EventRecorder),
		nattDiscoveryPort: int64(info.NATTDiscoveryPort),
		gateways:          info.Gateways,
		airGapped:         info.SubmarinerConfigSpec.AirGappedDeployment,
	}, nil
}

// PrepareSubmarinerClusterEnv prepares submariner cluster environment on Azure
// The below tasks will be executed
// 1. create the inbound and outbound firewall rules for submariner, below ports will be opened
//   - NAT traversal port (by default 4500/UDP)
//   - 4800/UDP port to encapsulate Pod traffic from worker and master nodes to the Submariner Gateway nodes
//   - ESP & AH protocols for private-ip to private-ip gateway communications
func (r *azureProvider) PrepareSubmarinerClusterEnv() error {
	// TODO For ovn the port 4800 need not be opened.
	if err := r.gwDeployer.Deploy(api.GatewayDeployInput{
		PublicPorts: []api.PortSpec{
			{Port: r.nattPort, Protocol: "udp"},
			{Port: uint16(r.nattDiscoveryPort), Protocol: "udp"},
			{Port: 0, Protocol: "esp"},
			{Port: 0, Protocol: "ah"},
		},
		Gateways:  r.gateways,
		AirGapped: r.airGapped,
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

	r.reporter.Success("The Submariner cluster environment has been set up on Azure")

	return nil
}

// CleanUpSubmarinerClusterEnv clean up submariner cluster environment on Azure after the SubmarinerConfig was deleted
// 1. delete any dedicated gateways that were previously deployed.
// 2. delete the inbound and outbound firewall rules to close submariner ports.
func (r *azureProvider) CleanUpSubmarinerClusterEnv() error {
	err := r.gwDeployer.Cleanup(r.reporter)
	if err != nil {
		return err
	}

	err = r.cloudPrepare.ClosePorts(r.reporter)
	if err != nil {
		return err
	}

	r.reporter.Success("The Submariner cluster environment has been cleaned up on Azure")

	return nil
}

func initializeFromAuthFile(credentialsSecret *corev1.Secret) (string, error) {
	servicePrincipalJSON, ok := credentialsSecret.Data[servicePrincipalJSON]

	//nolint:revive,stylecheck // Ignore var-naming: struct field ClientId should be ClientID et al.
	var authInfo struct {
		ClientId       string
		ClientSecret   string
		SubscriptionId string
		TenantId       string
	}

	if !ok {
		return "", errors.New("servicePrincipalJSON is not found in the credentials")
	}

	err := json.Unmarshal(servicePrincipalJSON, &authInfo)
	if err != nil {
		return "", errors.Wrap(err, "error unmarshalling servicePrincipalJSON")
	}

	if err := os.Setenv("AZURE_CLIENT_ID", authInfo.ClientId); err != nil {
		return "", err
	}

	if err := os.Setenv("AZURE_CLIENT_SECRET", authInfo.ClientSecret); err != nil {
		return "", err
	}

	if err := os.Setenv("AZURE_TENANT_ID", authInfo.TenantId); err != nil {
		return "", err
	}

	return authInfo.SubscriptionId, nil
}
