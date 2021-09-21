package cloud

import (
	"fmt"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"

	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	"github.com/open-cluster-management/submariner-addon/pkg/cloud/aws"
	"github.com/open-cluster-management/submariner-addon/pkg/cloud/gcp"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"
	workclient "open-cluster-management.io/api/client/work/clientset/versioned"

	"github.com/openshift/library-go/pkg/operator/events"

	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type CloudProvider interface {
	// PrepareSubmarinerClusterEnv prepares submariner cluster environment
	PrepareSubmarinerClusterEnv() error
	// CleanUpSubmarinerClusterEnv clean up the prepared submariner cluster environment
	CleanUpSubmarinerClusterEnv() error
}

func GetCloudProvider(
	restConfig *rest.Config,
	kubeClient kubernetes.Interface,
	workClient workclient.Interface,
	dynamicClient dynamic.Interface,
	hubKubeClient kubernetes.Interface,
	eventsRecorder events.Recorder,
	managedClusterInfo configv1alpha1.ManagedClusterInfo, config *configv1alpha1.SubmarinerConfig) (CloudProvider, error) {
	clusterName := managedClusterInfo.ClusterName
	vendor := managedClusterInfo.Vendor
	if vendor != helpers.ProductOCP {
		return nil, fmt.Errorf("unsupported vendor %q of cluster %q", vendor, clusterName)
	}

	platform := managedClusterInfo.Platform
	region := managedClusterInfo.Region
	infraId := managedClusterInfo.InfraId

	klog.V(4).Infof("get cloud provider: platform=%s,region=%s,infraId=%s,clusterName=%s,config=%s", platform, region, infraId, clusterName, config.Name)

	switch platform {
	case "AWS":
		return aws.NewAWSProvider(
			kubeClient, workClient,
			eventsRecorder,
			region, infraId, clusterName, config.Spec.CredentialsSecret.Name,
			config.Spec.GatewayConfig.AWS.InstanceType,
			config.Spec.IPSecNATTPort, config.Spec.NATTDiscoveryPort,
			config.Spec.Gateways,
		)
	case "GCP":
		return gcp.NewGCPProvider(
			restConfig,
			kubeClient,
			dynamicClient,
			hubKubeClient,
			eventsRecorder,
			region, infraId, clusterName, config.Spec.CredentialsSecret.Name,
			"", config.Spec.IPSecNATTPort, config.Spec.NATTDiscoveryPort, config.Spec.Gateways,
		)
	}
	return nil, fmt.Errorf("unsupported cloud platform %q of cluster %q", platform, clusterName)
}
