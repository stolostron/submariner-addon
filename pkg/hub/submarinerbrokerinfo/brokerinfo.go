package submarinerbrokerinfo

import (
	"fmt"

	clusterv1 "github.com/open-cluster-management/api/cluster/v1"
	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	configclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"

	"github.com/openshift/library-go/pkg/operator/events"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

const (
	SubmarinerVersion                       = "SUBMARINER_VERSION"
	SubmarinerDefaultVersion                = "0.8.0"
	SubmarinerCatalogName                   = "SUBMARINER_CATALOG_NAME"
	SubmarinerDefaultCatalogName            = "submariner"
	SubmarinerCatalogChannel                = "SUBMARINER_CATALOG_CHANNEL"
	SubmarinerDefaultCatalogChannel         = "alpha"
	SubmarinerCatalogSource                 = "SUBMARINER_CATALOG_SOURCE"
	SubmarinerDefaultCatalogSource          = "community-operators"
	SubmarinerCatalogSourceNamespace        = "SUBMARINER_CATALOG_SOURCE_NAMESPACE"
	SubmarinerDefaultCatalogSourceNamespace = "openshift-marketplace"
	SubmarinerCableDriver                   = "libreswan"
)

var (
	// catalogSource represents submariner catalog source, this can be set with go build ldflags
	catalogSource string
)

type SubmarinerBrokerInfo struct {
	NATEnabled             bool
	BrokerAPIServer        string
	BrokerNamespace        string
	BrokerToken            string
	BrokerCA               string
	IPSecPSK               string
	CableDriver            string
	IPSecIKEPort           int
	IPSecNATTPort          int
	ClusterName            string
	ClusterCIDR            string
	ServiceCIDR            string
	CatalogChannel         string
	CatalogName            string
	CatalogSource          string
	CatalogSourceNamespace string
	CatalogStartingCSV     string
}

// NewSubmarinerBrokerInfo creates submariner broker information with hub information
func NewSubmarinerBrokerInfo(
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	configClient configclient.Interface,
	recorder events.Recorder,
	managedCluster *clusterv1.ManagedCluster,
	brokeNamespace string,
	submarinerConfig *configv1alpha1.SubmarinerConfig) (*SubmarinerBrokerInfo, error) {
	brokerInfo := &SubmarinerBrokerInfo{
		CableDriver:            SubmarinerCableDriver,
		IPSecIKEPort:           helpers.SubmarinerIKEPort,
		IPSecNATTPort:          helpers.SubmarinerNatTPort,
		BrokerNamespace:        brokeNamespace,
		ClusterName:            managedCluster.Name,
		CatalogName:            helpers.GetEnv(SubmarinerCatalogName, SubmarinerDefaultCatalogName),
		CatalogChannel:         helpers.GetEnv(SubmarinerCatalogChannel, SubmarinerDefaultCatalogChannel),
		CatalogSource:          helpers.GetEnv(SubmarinerCatalogSource, SubmarinerDefaultCatalogSource),
		CatalogSourceNamespace: helpers.GetEnv(SubmarinerCatalogSourceNamespace, SubmarinerDefaultCatalogSourceNamespace),
		CatalogStartingCSV:     fmt.Sprintf("submariner.v%s", helpers.GetEnv(SubmarinerVersion, SubmarinerDefaultVersion)),
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

	token, ca, err := helpers.GetBrokerTokenAndCA(kubeClient, brokeNamespace, managedCluster.Name)
	if err != nil {
		return nil, err
	}
	brokerInfo.BrokerCA = ca
	brokerInfo.BrokerToken = token

	if err := applySubmarinerConfig(configClient, recorder, brokerInfo, submarinerConfig); err != nil {
		return nil, err
	}

	switch helpers.GetClusterType(managedCluster) {
	case helpers.ClusterTypeOCP:
		brokerInfo.NATEnabled = true
		if catalogSource != "" {
			brokerInfo.CatalogSource = catalogSource
		}
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
