package submarinerbroker

import (
	"context"
	"crypto/rand"
	"embed"
	"fmt"

	"github.com/open-cluster-management/submariner-addon/pkg/finalizer"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"
	"github.com/open-cluster-management/submariner-addon/pkg/resource"
	"github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	operatorhelpers "github.com/openshift/library-go/pkg/operator/v1helpers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	clientset "open-cluster-management.io/api/client/cluster/clientset/versioned/typed/cluster/v1alpha1"
	clusterinformerv1alpha1 "open-cluster-management.io/api/client/cluster/informers/externalversions/cluster/v1alpha1"
	clusterlisterv1alpha1 "open-cluster-management.io/api/client/cluster/listers/cluster/v1alpha1"
)

const (
	brokerFinalizer      = "cluster.open-cluster-management.io/submariner-cleanup"
	ipSecPSKSecretLength = 48
)

var (
	staticResourceFiles = []string{
		"manifests/broker-namespace.yaml",
		"manifests/broker-cluster-role.yaml",
	}
)

//go:embed manifests
var manifestFiles embed.FS

type submarinerBrokerController struct {
	kubeClient       kubernetes.Interface
	clustersetClient clientset.ManagedClusterSetInterface
	clusterSetLister clusterlisterv1alpha1.ManagedClusterSetLister
	eventRecorder    events.Recorder
}

type brokerConfig struct {
	SubmarinerNamespace string
}

func NewController(
	clustersetClient clientset.ManagedClusterSetInterface,
	kubeClient kubernetes.Interface,
	clusterSetInformer clusterinformerv1alpha1.ManagedClusterSetInformer,
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
	if errors.IsNotFound(err) {
		// ClusterSet not found, could have been deleted, do nothing.
		return nil
	}
	if err != nil {
		return err
	}
	clusterSet = clusterSet.DeepCopy()
	config := &brokerConfig{
		SubmarinerNamespace: helpers.GenerateBrokerName(clusterSet.Name),
	}

	// Update finalizer at first
	added, err := finalizer.Add(ctx, resource.ForManagedClusterSet(c.clustersetClient), clusterSet, brokerFinalizer)
	if added {
		return err
	}

	// ClusterSet is deleting, we remove its related resources on hub
	if !clusterSet.DeletionTimestamp.IsZero() {
		if err := c.cleanUp(ctx, syncCtx, config); err != nil {
			return err
		}
		return finalizer.Remove(ctx, resource.ForManagedClusterSet(c.clustersetClient), clusterSet, brokerFinalizer)
	}

	// Apply static files
	clientHolder := resourceapply.NewKubeClientHolder(c.kubeClient)
	applyResults := resourceapply.ApplyDirectly(
		clientHolder,
		syncCtx.Recorder(),
		func(name string) ([]byte, error) {
			template, err := manifestFiles.ReadFile(name)
			if err != nil {
				return nil, err
			}
			return assets.MustCreateAssetFromTemplate(name, template, config).Data, nil
		},
		staticResourceFiles...,
	)

	errs := []error{}
	for _, result := range applyResults {
		if result.Error != nil {
			errs = append(errs, fmt.Errorf("%q (%T): %v", result.File, result.Type, result.Error))
		}
	}

	if err := c.createIPSecPSKSecret(config.SubmarinerNamespace); err != nil {
		errs = append(errs, fmt.Errorf("unable to generate IPSECPSK secret : %v", err))
	}

	return operatorhelpers.NewMultiLineAggregate(errs)
}

func (c *submarinerBrokerController) cleanUp(ctx context.Context, controllerContext factory.SyncContext, config *brokerConfig) error {
	return helpers.CleanUpSubmarinerManifests(
		ctx,
		c.kubeClient,
		controllerContext.Recorder(),
		func(name string) ([]byte, error) {
			template, err := manifestFiles.ReadFile(name)
			if err != nil {
				return nil, err
			}
			return assets.MustCreateAssetFromTemplate(name, template, config).Data, nil
		},
		staticResourceFiles...,
	)
}

func (c *submarinerBrokerController) createIPSecPSKSecret(brokerNamespace string) error {
	_, err := c.kubeClient.CoreV1().Secrets(brokerNamespace).Get(context.TODO(), helpers.IPSecPSKSecretName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
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
