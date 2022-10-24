package cloud

import (
	"fmt"

	"github.com/openshift/library-go/pkg/operator/events"
	configv1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	"github.com/stolostron/submariner-addon/pkg/cloud/aws"
	"github.com/stolostron/submariner-addon/pkg/cloud/azure"
	"github.com/stolostron/submariner-addon/pkg/cloud/gcp"
	"github.com/stolostron/submariner-addon/pkg/cloud/rhos"
	"github.com/stolostron/submariner-addon/pkg/constants"
	v1 "k8s.io/api/core/v1"
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
	Get(managedClusterInfo *configv1alpha1.ManagedClusterInfo, config *configv1alpha1.SubmarinerConfig,
		eventsRecorder events.Recorder) (Provider, bool, error)
}

type providerFactory struct {
	restMapper    meta.RESTMapper
	kubeClient    kubernetes.Interface
	workClient    workclient.Interface
	dynamicClient dynamic.Interface
	hubKubeClient kubernetes.Interface
}

func NewProviderFactory(restMapper meta.RESTMapper, kubeClient kubernetes.Interface, workClient workclient.Interface,
	dynamicClient dynamic.Interface, hubKubeClient kubernetes.Interface,
) ProviderFactory {
	return &providerFactory{
		restMapper:    restMapper,
		kubeClient:    kubeClient,
		workClient:    workClient,
		dynamicClient: dynamicClient,
		hubKubeClient: hubKubeClient,
	}
}

func (f *providerFactory) Get(managedClusterInfo *configv1alpha1.ManagedClusterInfo, config *configv1alpha1.SubmarinerConfig,
	eventsRecorder events.Recorder,
) (Provider, bool, error) {
	platform := managedClusterInfo.Platform
	region := managedClusterInfo.Region
	infraID := managedClusterInfo.InfraID
	clusterName := managedClusterInfo.ClusterName
	vendor := managedClusterInfo.Vendor

	credentialsSecret := config.Spec.CredentialsSecret
	if credentialsSecret == nil {
		credentialsSecret = &v1.LocalObjectReference{}
	}

	if vendor == constants.ProductROSA || vendor == constants.ProductARO {
		return nil, false, nil
	}

	if vendor != constants.ProductOCP {
		return nil, false, fmt.Errorf("unsupported vendor %q of cluster %q", vendor, clusterName)
	}

	klog.V(4).Infof("get cloud provider: platform=%s,region=%s,infraID=%s,clusterName=%s,config=%s", platform,
		region, infraID, clusterName, config.Name)

	var provider Provider
	var err error

	found := true

	switch platform {
	case "AWS":
		provider, err = aws.NewAWSProvider(f.kubeClient, f.workClient, eventsRecorder, region, infraID, clusterName,
			credentialsSecret.Name, config.Spec.GatewayConfig.AWS.InstanceType,
			config.Spec.IPSecNATTPort, config.Spec.NATTDiscoveryPort, config.Spec.Gateways)
	case "GCP":
		provider, err = gcp.NewGCPProvider(f.restMapper, f.kubeClient, f.dynamicClient, f.hubKubeClient, eventsRecorder,
			region, infraID, clusterName, credentialsSecret.Name,
			config.Spec.GatewayConfig.GCP.InstanceType, config.Spec.IPSecNATTPort, config.Spec.NATTDiscoveryPort, config.Spec.Gateways)
	case "OpenStack":
		provider, err = rhos.NewRHOSProvider(f.restMapper, f.kubeClient, f.dynamicClient, f.hubKubeClient, eventsRecorder,
			region, infraID, clusterName, credentialsSecret.Name,
			config.Spec.GatewayConfig.RHOS.InstanceType, config.Spec.IPSecNATTPort, config.Spec.NATTDiscoveryPort, config.Spec.Gateways)
	case "Azure":
		provider, err = azure.NewAzureProvider(f.restMapper, f.kubeClient, f.dynamicClient, f.hubKubeClient, eventsRecorder,
			region, infraID, clusterName, credentialsSecret.Name,
			config.Spec.GatewayConfig.Azure.InstanceType, config.Spec.IPSecNATTPort, config.Spec.NATTDiscoveryPort, config.Spec.Gateways)
	default:
		found = false
	}

	return provider, found, err
}
