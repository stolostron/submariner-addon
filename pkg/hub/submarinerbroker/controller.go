package submarinerbroker

import (
	"context"
	"crypto/rand"
	"embed"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/pkg/errors"
	"github.com/stolostron/submariner-addon/pkg/constants"
	brokerinfo "github.com/stolostron/submariner-addon/pkg/hub/submarinerbrokerinfo"
	"github.com/stolostron/submariner-addon/pkg/resource"
	"github.com/submariner-io/admiral/pkg/finalizer"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
	clientset "open-cluster-management.io/api/client/cluster/clientset/versioned/typed/cluster/v1beta1"
	clusterinformerv1beta1 "open-cluster-management.io/api/client/cluster/informers/externalversions/cluster/v1beta1"
	clusterlisterv1beta1 "open-cluster-management.io/api/client/cluster/listers/cluster/v1beta1"
)

const (
	brokerFinalizer      = "cluster.open-cluster-management.io/submariner-cleanup"
	submBrokerNamespace  = "cluster.open-cluster-management.io/submariner-broker-ns"
	ipSecPSKSecretLength = 48
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
	SubmarinerNamespace string
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

	config := &brokerConfig{
		SubmarinerNamespace: brokerinfo.GenerateBrokerName(clusterSet.Name),
	}

	assetFunc := resource.AssetFromFile(manifestFiles, config)

	// ClusterSet is deleting, we remove its related resources on hub
	if !clusterSet.DeletionTimestamp.IsZero() {
		if err := resource.DeleteFromManifests(ctx, c.kubeClient, syncCtx.Recorder(), assetFunc, staticResourceFiles...); err != nil {
			return err
		}

		return finalizer.Remove(ctx, resource.ForManagedClusterSet(c.clustersetClient), clusterSet, brokerFinalizer)
	}

	// Apply static files
	err = resource.ApplyManifests(ctx, c.kubeClient, syncCtx.Recorder(), assetFunc, staticResourceFiles...)
	if err != nil {
		return err
	}

	if clusterSet.GetAnnotations()[submBrokerNamespace] != config.SubmarinerNamespace {
		err = c.annotateClusterSetWithBrokerNamespace(config.SubmarinerNamespace, clusterSetName)
		if err != nil {
			return err
		}
	}

	return c.createIPSecPSKSecret(config.SubmarinerNamespace)
}

func (c *submarinerBrokerController) createIPSecPSKSecret(brokerNamespace string) error {
	_, err := c.kubeClient.CoreV1().Secrets(brokerNamespace).Get(context.TODO(), constants.IPSecPSKSecretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		psk := make([]byte, ipSecPSKSecretLength)
		if _, err := rand.Read(psk); err != nil {
			return err
		}

		pskSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: constants.IPSecPSKSecretName,
			},
			Data: map[string][]byte{
				"psk": psk,
			},
		}

		_, err = c.kubeClient.CoreV1().Secrets(brokerNamespace).Create(context.TODO(), pskSecret, metav1.CreateOptions{})
	}

	return err
}

func (c *submarinerBrokerController) annotateClusterSetWithBrokerNamespace(brokerNamespace, clusterSetName string) error {
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		clusterSet, err := c.clustersetClient.Get(context.TODO(), clusterSetName, metav1.GetOptions{})
		if err != nil {
			return errors.Wrapf(err, "unable to get clusterSet info for %q", clusterSetName)
		}

		annotations := clusterSet.GetAnnotations()
		if annotations == nil {
			annotations = map[string]string{}
		}

		annotations[submBrokerNamespace] = brokerNamespace
		clusterSet.SetAnnotations(annotations)
		_, updateErr := c.clustersetClient.Update(context.TODO(), clusterSet, metav1.UpdateOptions{})

		return updateErr
	})

	if retryErr != nil {
		return errors.Wrapf(retryErr, "error updating clusterSet annotation %q", clusterSetName)
	}

	klog.V(4).Infof("Successfully annotated clusterSet %q with brokerNamespace %q", clusterSetName, brokerNamespace)

	return nil
}
