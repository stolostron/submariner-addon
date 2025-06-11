package cloud

import (
	"context"
	"fmt"
	"strconv"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/pkg/errors"
	configv1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	"github.com/stolostron/submariner-addon/pkg/cloud/aws"
	"github.com/stolostron/submariner-addon/pkg/cloud/azure"
	"github.com/stolostron/submariner-addon/pkg/cloud/gcp"
	"github.com/stolostron/submariner-addon/pkg/cloud/provider"
	"github.com/stolostron/submariner-addon/pkg/cloud/rhos"
	"github.com/stolostron/submariner-addon/pkg/constants"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	Get(config *configv1alpha1.SubmarinerConfig, eventsRecorder events.Recorder) (Provider, bool, error)
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
	hubKubeClient kubernetes.Interface,
) ProviderFactory {
	return &providerFactory{
		restMapper:    restMapper,
		kubeClient:    kubeClient,
		dynamicClient: dynamicClient,
		hubKubeClient: hubKubeClient,
	}
}

func (f *providerFactory) Get(config *configv1alpha1.SubmarinerConfig, eventsRecorder events.Recorder) (Provider, bool, error) {
	if config.Annotations["submariner.io/skip-cloud-prepare"] == strconv.FormatBool(true) {
		return nil, false, nil
	}

	managedClusterInfo := &config.Status.ManagedClusterInfo

	klog.V(4).Infof("Get cloud provider: ManagedClusterInfo: %#v", managedClusterInfo)

	info := &provider.Info{
		RestMapper:           f.restMapper,
		KubeClient:           f.kubeClient,
		DynamicClient:        f.dynamicClient,
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

	vendor := managedClusterInfo.Vendor
	if vendor == constants.ProductROSA || vendor == constants.ProductARO || vendor == constants.ProductROKS {
		return nil, false, nil
	}

	if vendor != constants.ProductOCP {
		return nil, false, fmt.Errorf("unsupported vendor %q for cluster %q", vendor, managedClusterInfo.ClusterName)
	}

	providerFn, found := providers[managedClusterInfo.Platform]
	if !found {
		return nil, false, nil
	}

	if info.SubmarinerConfigSpec.CredentialsSecret == nil {
		return nil, true, errors.New("no CredentialsSecret reference provided")
	}

	var err error

	info.CredentialsSecret, err = f.hubKubeClient.CoreV1().Secrets(info.ClusterName).Get(context.TODO(),
		info.SubmarinerConfigSpec.CredentialsSecret.Name, metav1.GetOptions{})
	if err != nil {
		return nil, true, err
	}

	info.SubmarinerConfigAnnotations = config.Annotations

	instance, err := providerFn(info)

	return instance, true, err
}
