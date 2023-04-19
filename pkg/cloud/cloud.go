package cloud

import (
	"fmt"

	"github.com/openshift/library-go/pkg/operator/events"
	configv1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	"github.com/stolostron/submariner-addon/pkg/cloud/aws"
	"github.com/stolostron/submariner-addon/pkg/cloud/azure"
	"github.com/stolostron/submariner-addon/pkg/cloud/gcp"
	"github.com/stolostron/submariner-addon/pkg/cloud/provider"
	"github.com/stolostron/submariner-addon/pkg/cloud/rhos"
	"github.com/stolostron/submariner-addon/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
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
	dynamicClient dynamic.Interface
	hubKubeClient kubernetes.Interface
}

type ProviderFn func(*provider.Info) (Provider, error)

var providers = map[string]ProviderFn{}

func init() {
	RegisterProvider("AWS", func(info *provider.Info) (Provider, error) {
		return aws.NewProvider(info)
	})

	RegisterProvider("GCP", func(info *provider.Info) (Provider, error) {
		return gcp.NewProvider(info)
	})

	RegisterProvider("OpenStack", func(info *provider.Info) (Provider, error) {
		return rhos.NewProvider(info)
	})

	RegisterProvider("Azure", func(info *provider.Info) (Provider, error) {
		return azure.NewProvider(info)
	})
}

func RegisterProvider(platform string, f ProviderFn) {
	providers[platform] = f
}

func NewProviderFactory(restMapper meta.RESTMapper, kubeClient kubernetes.Interface, dynamicClient dynamic.Interface,
	hubKubeClient kubernetes.Interface) ProviderFactory {
	return &providerFactory{
		restMapper:    restMapper,
		kubeClient:    kubeClient,
		dynamicClient: dynamicClient,
		hubKubeClient: hubKubeClient,
	}
}

func (f *providerFactory) Get(managedClusterInfo *configv1alpha1.ManagedClusterInfo, config *configv1alpha1.SubmarinerConfig,
	eventsRecorder events.Recorder,
) (Provider, bool, error) {
	klog.V(4).Infof("Get cloud provider: ManagedClusterInfo: %#v", managedClusterInfo)

	info := &provider.Info{
		RestMapper:           f.restMapper,
		KubeClient:           f.kubeClient,
		DynamicClient:        f.dynamicClient,
		HubKubeClient:        f.hubKubeClient,
		EventRecorder:        eventsRecorder,
		SubmarinerConfigSpec: config.Spec,
		ManagedClusterInfo:   *managedClusterInfo,
	}

	if info.SubmarinerConfigSpec.IPSecNATTPort == 0 {
		info.SubmarinerConfigSpec.IPSecNATTPort = constants.SubmarinerNatTPort
	}

	if info.SubmarinerConfigSpec.NATTDiscoveryPort == 0 {
		info.SubmarinerConfigSpec.NATTDiscoveryPort = constants.SubmarinerNatTDiscoveryPort
	}

	if info.CredentialsSecret == nil {
		info.CredentialsSecret = &corev1.LocalObjectReference{}
	}

	vendor := managedClusterInfo.Vendor
	if vendor == constants.ProductROSA || vendor == constants.ProductARO {
		return nil, false, nil
	}

	if vendor != constants.ProductOCP {
		return nil, false, fmt.Errorf("unsupported vendor %q for cluster %q", vendor, managedClusterInfo.ClusterName)
	}

	providerFn, found := providers[managedClusterInfo.Platform]
	if !found {
		return nil, false, nil
	}

	instance, err := providerFn(info)

	return instance, true, err
}
