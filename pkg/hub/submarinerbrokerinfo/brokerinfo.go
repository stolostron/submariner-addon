package submarinerbrokerinfo

import (
	addonv1alpha1 "github.com/open-cluster-management/api/addon/v1alpha1"
	clusterv1 "github.com/open-cluster-management/api/cluster/v1"
	configv1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	configclient "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	"github.com/stolostron/submariner-addon/pkg/helpers"

	"github.com/openshift/library-go/pkg/operator/events"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const (
	catalogName                   = "submariner"
	defaultCatalogSource          = "redhat-operators"
	defaultCatalogSourceNamespace = "openshift-marketplace"
	defaultCableDriver            = "libreswan"
	defaultInstallationNamespace  = "open-cluster-management-agent-addon"
)

var (
	// catalogSource represents submariner catalog source, this can be set with go build ldflags
	catalogSource string
)

type SubmarinerBrokerInfo struct {
	NATEnabled                bool
	IPSecIKEPort              int
	IPSecNATTPort             int
	InstallationNamespace     string
	BrokerAPIServer           string
	BrokerNamespace           string
	BrokerToken               string
	BrokerCA                  string
	IPSecPSK                  string
	CableDriver               string
	ClusterName               string
	ClusterCIDR               string
	ServiceCIDR               string
	CatalogChannel            string
	CatalogName               string
	CatalogSource             string
	CatalogSourceNamespace    string
	CatalogStartingCSV        string
	SubmarinerGatewayImage    string
	SubmarinerRouteAgentImage string
	LighthouseAgentImage      string
	LighthouseCoreDNSImage    string
}

// NewSubmarinerBrokerInfo creates submariner broker information with hub information
func NewSubmarinerBrokerInfo(
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	configClient configclient.Interface,
	recorder events.Recorder,
	managedCluster *clusterv1.ManagedCluster,
	brokeNamespace string,
	submarinerConfig *configv1alpha1.SubmarinerConfig,
	managedClusterAddOn *addonv1alpha1.ManagedClusterAddOn) (*SubmarinerBrokerInfo, error) {
	brokerInfo := &SubmarinerBrokerInfo{
		CableDriver:            defaultCableDriver,
		IPSecIKEPort:           helpers.SubmarinerIKEPort,
		IPSecNATTPort:          helpers.SubmarinerNatTPort,
		BrokerNamespace:        brokeNamespace,
		ClusterName:            managedCluster.Name,
		CatalogName:            catalogName,
		CatalogSource:          defaultCatalogSource,
		CatalogSourceNamespace: defaultCatalogSourceNamespace,
		InstallationNamespace:  defaultInstallationNamespace,
	}

	if len(managedClusterAddOn.Spec.InstallNamespace) != 0 {
		brokerInfo.InstallationNamespace = managedClusterAddOn.Spec.InstallNamespace
	}

	apiServer, err := helpers.GetBrokerAPIServer(dynamicClient)
	if err != nil {
		return nil, err
	}
	brokerInfo.BrokerAPIServer = apiServer

	IPSecPSK, err := helpers.GetIPSecPSK(kubeClient, brokeNamespace)
	if err != nil {
		return nil, err
	}
	brokerInfo.IPSecPSK = IPSecPSK

	token, ca, err := helpers.GetBrokerTokenAndCA(kubeClient, dynamicClient, brokeNamespace, managedCluster.Name)
	if err != nil {
		return nil, err
	}
	brokerInfo.BrokerCA = ca
	brokerInfo.BrokerToken = token

	if err := applySubmarinerConfig(configClient, recorder, brokerInfo, submarinerConfig); err != nil {
		return nil, err
	}

	if helpers.GetClusterProduct(managedCluster) == helpers.ProductOCP {
		brokerInfo.NATEnabled = true
	}

	return brokerInfo, nil
}

func applySubmarinerConfig(
	configClient configclient.Interface,
	recorder events.Recorder,
	brokerInfo *SubmarinerBrokerInfo, submarinerConfig *configv1alpha1.SubmarinerConfig) error {
	if submarinerConfig == nil {
		return nil
	}

	if submarinerConfig.Spec.CableDriver != "" {
		brokerInfo.CableDriver = submarinerConfig.Spec.CableDriver
	}

	if submarinerConfig.Spec.IPSecIKEPort != 0 {
		brokerInfo.IPSecIKEPort = submarinerConfig.Spec.IPSecIKEPort
	}

	if submarinerConfig.Spec.IPSecNATTPort != 0 {
		brokerInfo.IPSecNATTPort = submarinerConfig.Spec.IPSecNATTPort
	}

	if submarinerConfig.Spec.SubscriptionConfig.Channel != "" {
		brokerInfo.CatalogChannel = submarinerConfig.Spec.SubscriptionConfig.Channel
	}

	if submarinerConfig.Spec.SubscriptionConfig.Source != "" {
		brokerInfo.CatalogSource = submarinerConfig.Spec.SubscriptionConfig.Source
	}

	if submarinerConfig.Spec.SubscriptionConfig.SourceNamespace != "" {
		brokerInfo.CatalogSourceNamespace = submarinerConfig.Spec.SubscriptionConfig.SourceNamespace
	}

	if submarinerConfig.Spec.SubscriptionConfig.StartingCSV != "" {
		brokerInfo.CatalogStartingCSV = submarinerConfig.Spec.SubscriptionConfig.StartingCSV
	}

	if submarinerConfig.Spec.ImagePullSpecs.SubmarinerImagePullSpec != "" {
		brokerInfo.SubmarinerGatewayImage = submarinerConfig.Spec.ImagePullSpecs.SubmarinerImagePullSpec
	}

	if submarinerConfig.Spec.ImagePullSpecs.SubmarinerRouteAgentImagePullSpec != "" {
		brokerInfo.SubmarinerRouteAgentImage = submarinerConfig.Spec.ImagePullSpecs.SubmarinerRouteAgentImagePullSpec
	}

	if submarinerConfig.Spec.ImagePullSpecs.LighthouseCoreDNSImagePullSpec != "" {
		brokerInfo.LighthouseCoreDNSImage = submarinerConfig.Spec.ImagePullSpecs.LighthouseCoreDNSImagePullSpec
	}

	if submarinerConfig.Spec.ImagePullSpecs.LighthouseAgentImagePullSpec != "" {
		brokerInfo.LighthouseAgentImage = submarinerConfig.Spec.ImagePullSpecs.LighthouseAgentImagePullSpec
	}

	condition := metav1.Condition{
		Type:    configv1alpha1.SubmarinerConfigConditionApplied,
		Status:  metav1.ConditionTrue,
		Reason:  "SubmarinerConfigApplied",
		Message: "SubmarinerConfig was applied",
	}
	_, updated, err := helpers.UpdateSubmarinerConfigStatus(
		configClient,
		submarinerConfig.Namespace, submarinerConfig.Name,
		helpers.UpdateSubmarinerConfigConditionFn(condition),
	)
	if err != nil {
		return err
	}

	if updated {
		recorder.Eventf("SubmarinerConfigApplied", "SubmarinerConfig %s was applied for manged cluster %s", submarinerConfig.Name, submarinerConfig.Namespace)
	}

	return nil
}
