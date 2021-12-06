package submarinerbroker

import (
	"context"
	"crypto/rand"
	"embed"
	"net"
	"strconv"
	"strings"

	"github.com/open-cluster-management/submariner-addon/pkg/helpers"
	brokerinfo "github.com/open-cluster-management/submariner-addon/pkg/hub/submarinerbrokerinfo"
	"github.com/open-cluster-management/submariner-addon/pkg/resource"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/pkg/errors"
	"github.com/submariner-io/admiral/pkg/finalizer"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	clientset "open-cluster-management.io/api/client/cluster/clientset/versioned/typed/cluster/v1beta1"
	clusterinformerv1beta1 "open-cluster-management.io/api/client/cluster/informers/externalversions/cluster/v1beta1"
	clusterlisterv1beta1 "open-cluster-management.io/api/client/cluster/listers/cluster/v1beta1"
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
)

const (
	brokerFinalizer                                = "cluster.open-cluster-management.io/submariner-cleanup"
	brokerComponentsAnnotationKey                  = "cluster.open-cluster-management.io/brokerComponents"
	brokerDefaultCustomDomainsAnnotationKey        = "cluster.open-cluster-management.io/brokerDefaultCustomDomains"
	brokerGlobalnetCIDRRangeAnnotationKey          = "cluster.open-cluster-management.io/brokerGlobalnetCIDRRange"
	brokerDefaultGlobalnetClusterSizeAnnotationKey = "cluster.open-cluster-management.io/brokerDefaultGlobalnetClusterSize"
	brokerGlobalnetEnabledAnnotationKey            = "cluster.open-cluster-management.io/brokerGlobalnetEnabled"
	ipSecPSKSecretLength                           = 48
)

var staticResourceFiles = []string{
	"manifests/broker-namespace.yaml",
	"manifests/broker-cluster-role.yaml",
}

//go:embed manifests
var manifestFiles embed.FS

type submarinerBrokerController struct {
	kubeClient       kubernetes.Interface
	clustersetClient clientset.ManagedClusterSetInterface
	clusterSetLister clusterlisterv1beta1.ManagedClusterSetLister
	eventRecorder    events.Recorder
}

type brokerConfig struct {
	SubmarinerNamespace         string
	GlobalnetCIDRRange          string
	Components                  []string
	DefaultCustomDomains        []string
	DefaultGlobalnetClusterSize uint
	GlobalnetEnabled            bool
}

func NewController(
	clustersetClient clientset.ManagedClusterSetInterface,
	kubeClient kubernetes.Interface,
	clusterSetInformer clusterinformerv1beta1.ManagedClusterSetInformer,
	recorder events.Recorder) factory.Controller {
	c := &submarinerBrokerController{
		kubeClient:       kubeClient,
		clustersetClient: clustersetClient,
		clusterSetLister: clusterSetInformer.Lister(),
		eventRecorder:    recorder.WithComponentSuffix("submariner-broker-controller"),
	}

	return factory.New().
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			accessor, _ := meta.Accessor(obj)

			return accessor.GetName()
		}, clusterSetInformer.Informer()).
		WithSync(c.sync).
		ToController("SubmarinerBrokerController", recorder)
}

func (c *submarinerBrokerController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	clusterSetName := syncCtx.QueueKey()
	klog.V(4).Infof("Reconciling ClusterSet %q", clusterSetName)

	clusterSet, err := c.clusterSetLister.Get(clusterSetName)
	if apierrors.IsNotFound(err) {
		// ClusterSet not found, could have been deleted, do nothing.
		return nil
	}

	if err != nil {
		return err
	}

	clusterSet = clusterSet.DeepCopy()

	// Update finalizer at first
	added, err := finalizer.Add(ctx, resource.ForManagedClusterSet(c.clustersetClient), clusterSet, brokerFinalizer)
	if added || err != nil {
		return err
	}

	config, err := buildBrokerConfig(clusterSet)
	if err != nil {
		return errors.WithMessagef(err, "Failed to build broker config")
	}

	assetFunc := resource.AssetFromFile(manifestFiles, config)

	// ClusterSet is deleting, we remove its related resources on hub
	if !clusterSet.DeletionTimestamp.IsZero() {
		if err := resource.DeleteFromManifests(c.kubeClient, syncCtx.Recorder(), assetFunc, staticResourceFiles...); err != nil {
			return err
		}

		return finalizer.Remove(ctx, resource.ForManagedClusterSet(c.clustersetClient), clusterSet, brokerFinalizer)
	}

	// Apply static files
	err = resource.ApplyManifests(c.kubeClient, syncCtx.Recorder(), assetFunc, staticResourceFiles...)
	if err != nil {
		return err
	}

	return c.createIPSecPSKSecret(config.SubmarinerNamespace)
}

func buildBrokerConfig(clusterSet *clusterv1beta1.ManagedClusterSet) (*brokerConfig, error) {
	config := &brokerConfig{
		SubmarinerNamespace:         brokerinfo.GenerateBrokerName(clusterSet.Name),
		Components:                  []string{"service-discovery", "connectivity"},
		DefaultCustomDomains:        nil,
		GlobalnetCIDRRange:          "242.0.0.0/8",
		DefaultGlobalnetClusterSize: 65536,
		GlobalnetEnabled:            false,
	}

	if v, ok := clusterSet.Annotations[brokerComponentsAnnotationKey]; ok {
		config.Components = strings.Split(strings.TrimSpace(v), ",")
	}

	if v, ok := clusterSet.Annotations[brokerDefaultCustomDomainsAnnotationKey]; ok {
		config.DefaultCustomDomains = strings.Split(strings.TrimSpace(v), ",")
	}

	if v, ok := clusterSet.Annotations[brokerGlobalnetCIDRRangeAnnotationKey]; ok {
		globalnetCIDRRange := strings.TrimSpace(v)
		if _, _, err := net.ParseCIDR(globalnetCIDRRange); err != nil {
			return nil, errors.WithMessagef(err, "Found invalid globalnet CIDR value %q", globalnetCIDRRange)
		}

		config.GlobalnetCIDRRange = globalnetCIDRRange
	}

	if v, ok := clusterSet.Annotations[brokerDefaultGlobalnetClusterSizeAnnotationKey]; ok {
		defaultGlobalnetClusterSize, err := strconv.Atoi(strings.TrimSpace(v))
		if err != nil || defaultGlobalnetClusterSize <= 0 {
			return nil, errors.WithMessagef(err, "Found invalid default globalnet cluster size value %q", v)
		}

		config.DefaultGlobalnetClusterSize = uint(defaultGlobalnetClusterSize)
	}

	if v, ok := clusterSet.Annotations[brokerGlobalnetEnabledAnnotationKey]; ok {
		globalnetEnabled, err := strconv.ParseBool(strings.TrimSpace(v))
		if err != nil {
			return nil, errors.WithMessagef(err, "Found invalid globalnet enabled value %q", v)
		}

		config.GlobalnetEnabled = globalnetEnabled
	}

	return config, nil
}

func (c *submarinerBrokerController) createIPSecPSKSecret(brokerNamespace string) error {
	_, err := c.kubeClient.CoreV1().Secrets(brokerNamespace).Get(context.TODO(), helpers.IPSecPSKSecretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		psk := make([]byte, ipSecPSKSecretLength)
		if _, err := rand.Read(psk); err != nil {
			return err
		}

		pskSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: helpers.IPSecPSKSecretName,
			},
			Data: map[string][]byte{
				"psk": psk,
			},
		}

		_, err = c.kubeClient.CoreV1().Secrets(brokerNamespace).Create(context.TODO(), pskSecret, metav1.CreateOptions{})
	}

	return err
}
