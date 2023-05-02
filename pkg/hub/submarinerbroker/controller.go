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
	"github.com/submariner-io/admiral/pkg/log"
	"github.com/submariner-io/admiral/pkg/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	clientset "open-cluster-management.io/api/client/cluster/clientset/versioned/typed/cluster/v1beta2"
	clusterinformerv1beta2 "open-cluster-management.io/api/client/cluster/informers/externalversions/cluster/v1beta2"
	clusterlisterv1beta2 "open-cluster-management.io/api/client/cluster/listers/cluster/v1beta2"
	clusterv1beta2 "open-cluster-management.io/api/cluster/v1beta2"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
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

var logger = log.Logger{Logger: logf.Log.WithName("SubmarinerBrokerController")}

type submarinerBrokerController struct {
	kubeClient       kubernetes.Interface
	clustersetClient clientset.ManagedClusterSetInterface
	clusterSetLister clusterlisterv1beta2.ManagedClusterSetLister
	eventRecorder    events.Recorder
}

type brokerConfig struct {
	SubmarinerNamespace string
}

func NewController(
	clustersetClient clientset.ManagedClusterSetInterface,
	kubeClient kubernetes.Interface,
	clusterSetInformer clusterinformerv1beta2.ManagedClusterSetInformer,
	recorder events.Recorder,
) factory.Controller {
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

	clusterSet, err := c.clusterSetLister.Get(clusterSetName)
	if apierrors.IsNotFound(err) {
		// ClusterSet not found, could have been deleted, do nothing.
		return nil
	}

	if err != nil {
		return err
	}

	clusterSet = clusterSet.DeepCopy()

	// ignore the non-legacy clusterset
	selectorType := clusterSet.Spec.ClusterSelector.SelectorType
	if len(selectorType) > 0 && selectorType != clusterv1beta2.ExclusiveClusterSetLabel {
		return nil
	}

	logger.V(log.DEBUG).Infof("Reconciling ManagedClusterSet %q", clusterSetName)

	// Update finalizer at first
	added, err := finalizer.Add(ctx, resource.ForManagedClusterSet(c.clustersetClient), clusterSet, brokerFinalizer)
	if added || err != nil {
		if added {
			logger.Infof("Added finalizer to ManagedClusterSet %q", clusterSet.Name)
		}

		return err
	}

	config := &brokerConfig{
		SubmarinerNamespace: brokerinfo.GenerateBrokerName(clusterSet.Name),
	}

	assetFunc := resource.AssetFromFile(manifestFiles, config)

	// ClusterSet is deleting, we remove its related resources on hub
	if !clusterSet.DeletionTimestamp.IsZero() {
		logger.Infof("ManagedClusterSet %q is deleting", clusterSet.Name)

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
		err = c.annotateClusterSetWithBrokerNamespace(ctx, config.SubmarinerNamespace, clusterSet)
		if err != nil {
			return err
		}
	}

	return c.createIPSecPSKSecret(ctx, config.SubmarinerNamespace)
}

func (c *submarinerBrokerController) createIPSecPSKSecret(ctx context.Context, brokerNamespace string) error {
	_, err := c.kubeClient.CoreV1().Secrets(brokerNamespace).Get(ctx, constants.IPSecPSKSecretName, metav1.GetOptions{})
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

		_, err = c.kubeClient.CoreV1().Secrets(brokerNamespace).Create(ctx, pskSecret, metav1.CreateOptions{})

		if err == nil {
			logger.Infof("Created IPSec PSK Secret %q in namespace %q", constants.IPSecPSKSecretName, brokerNamespace)
		}
	}

	return err
}

func (c *submarinerBrokerController) annotateClusterSetWithBrokerNamespace(ctx context.Context, brokerNamespace string,
	clusterSet *clusterv1beta2.ManagedClusterSet,
) error {
	err := util.Update(ctx, resource.ForManagedClusterSet(c.clustersetClient), clusterSet,
		func(existing runtime.Object) (runtime.Object, error) {
			clusterSet := existing.(*clusterv1beta2.ManagedClusterSet)

			annotations := clusterSet.GetAnnotations()
			if annotations == nil {
				annotations = map[string]string{}
			}

			annotations[submBrokerNamespace] = brokerNamespace
			clusterSet.SetAnnotations(annotations)

			return clusterSet, nil
		})
	if err != nil {
		return errors.Wrapf(err, "error annotating ManagedClusterSet %q with brokerNamespace %q",
			clusterSet.Name, brokerNamespace)
	}

	logger.Infof("Successfully annotated ManagedClusterSet %q with brokerNamespace %q", clusterSet.Name, brokerNamespace)

	return nil
}
