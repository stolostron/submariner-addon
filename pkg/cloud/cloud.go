package cloud

import (
	"fmt"

	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	"github.com/open-cluster-management/submariner-addon/pkg/cloud/aws"
	"github.com/open-cluster-management/submariner-addon/pkg/cloud/gcp"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"
	"github.com/openshift/library-go/pkg/operator/events"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	workclient "open-cluster-management.io/api/client/work/clientset/versioned"
)

//go:generate mockgen -source=./cloud.go -destination=./fake/cloud.go -package=fake

type Provider interface {
	// PrepareSubmarinerClusterEnv prepares submariner cluster environment
	PrepareSubmarinerClusterEnv() error
	// CleanUpSubmarinerClusterEnv clean up the prepared submariner cluster environment
	CleanUpSubmarinerClusterEnv() error
}

type ProviderFactory interface {
	Get(managedClusterInfo configv1alpha1.ManagedClusterInfo, config *configv1alpha1.SubmarinerConfig,
		eventsRecorder events.Recorder) (Provider, error)
}

type providerFactory struct {
	restMapper    meta.RESTMapper
	kubeClient    kubernetes.Interface
	workClient    workclient.Interface
	dynamicClient dynamic.Interface
	hubKubeClient kubernetes.Interface
}

func NewProviderFactory(restMapper meta.RESTMapper, kubeClient kubernetes.Interface, workClient workclient.Interface,
	dynamicClient dynamic.Interface, hubKubeClient kubernetes.Interface) ProviderFactory {
	return &providerFactory{
		restMapper:    restMapper,
		kubeClient:    kubeClient,
		workClient:    workClient,
		dynamicClient: dynamicClient,
		hubKubeClient: hubKubeClient,
	}
}

func (f *providerFactory) Get(managedClusterInfo configv1alpha1.ManagedClusterInfo, config *configv1alpha1.SubmarinerConfig,
	eventsRecorder events.Recorder) (Provider, error) {
	clusterName := managedClusterInfo.ClusterName
	vendor := managedClusterInfo.Vendor
	if vendor != helpers.ProductOCP {
		return nil, fmt.Errorf("unsupported vendor %q of cluster %q", vendor, clusterName)
	}

	platform := managedClusterInfo.Platform
	region := managedClusterInfo.Region
	infraId := managedClusterInfo.InfraId

	klog.V(4).Infof("get cloud provider: platform=%s,region=%s,infraId=%s,clusterName=%s,config=%s", platform,
		region, infraId, clusterName, config.Name)

	switch platform {
	case "AWS":
		return aws.NewAWSProvider(f.kubeClient, f.workClient, eventsRecorder, region, infraId, clusterName,
			config.Spec.CredentialsSecret.Name, config.Spec.GatewayConfig.AWS.InstanceType,
			config.Spec.IPSecNATTPort, config.Spec.NATTDiscoveryPort, config.Spec.Gateways)
	case "GCP":
		return gcp.NewGCPProvider(f.restMapper, f.kubeClient, f.dynamicClient, f.hubKubeClient, eventsRecorder,
			region, infraId, clusterName, config.Spec.CredentialsSecret.Name,
			"", config.Spec.IPSecNATTPort, config.Spec.NATTDiscoveryPort, config.Spec.Gateways)
	}

	return nil, fmt.Errorf("unsupported cloud platform %q of cluster %q", platform, clusterName)
}
