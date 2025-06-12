package submarinerbroker

import (
	"context"
	"crypto/rand"
	"embed"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/pkg/errors"
	"github.com/stolostron/submariner-addon/pkg/constants"
	brokerinfo "github.com/stolostron/submariner-addon/pkg/hub/submarinerbrokerinfo"
	"github.com/stolostron/submariner-addon/pkg/resource"
	"github.com/submariner-io/admiral/pkg/finalizer"
	"github.com/submariner-io/admiral/pkg/log"
	coreresource "github.com/submariner-io/admiral/pkg/resource"
	"github.com/submariner-io/admiral/pkg/util"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"open-cluster-management.io/api/addon/v1alpha1"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"
	addoninformerv1alpha1 "open-cluster-management.io/api/client/addon/informers/externalversions/addon/v1alpha1"
	addonlisterv1alpha1 "open-cluster-management.io/api/client/addon/listers/addon/v1alpha1"
	clientset "open-cluster-management.io/api/client/cluster/clientset/versioned/typed/cluster/v1beta2"
	clusterinformerv1beta2 "open-cluster-management.io/api/client/cluster/informers/externalversions/cluster/v1beta2"
	clusterlisterv1beta2 "open-cluster-management.io/api/client/cluster/listers/cluster/v1beta2"
	clusterv1beta2 "open-cluster-management.io/api/cluster/v1beta2"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	brokerFinalizer        = "cluster.open-cluster-management.io/submariner-cleanup"
	SubmBrokerNamespaceKey = "cluster.open-cluster-management.io/submariner-broker-ns"
	ipSecPSKSecretLength   = 48
)

var staticResourceFiles = []string{
	"manifests/broker-namespace.yaml",
	"manifests/broker-cluster-role.yaml",
}

//go:embed manifests
var manifestFiles embed.FS

var logger = log.Logger{Logger: logf.Log.WithName("SubmarinerBrokerController")}

type submarinerBrokerController struct {
	kubeClient         kubernetes.Interface
	clustersetClient   clientset.ManagedClusterSetInterface
	clusterSetLister   clusterlisterv1beta2.ManagedClusterSetLister
	addOnClient        addonclient.Interface
	clusterAddOnLister addonlisterv1alpha1.ClusterManagementAddOnLister
	addOnLister        addonlisterv1alpha1.ManagedClusterAddOnLister
	eventRecorder      events.Recorder
	resourceCache      resourceapply.ResourceCache
}

type brokerConfig struct {
	SubmarinerNamespace string
}

func NewController(kubeClient kubernetes.Interface,
	clustersetClient clientset.ManagedClusterSetInterface,
	clusterSetInformer clusterinformerv1beta2.ManagedClusterSetInformer,
	addOnClient addonclient.Interface,
	addOnInformer addoninformerv1alpha1.Interface,
	recorder events.Recorder,
) factory.Controller {
	c := &submarinerBrokerController{
		kubeClient:         kubeClient,
		clustersetClient:   clustersetClient,
		clusterSetLister:   clusterSetInformer.Lister(),
		addOnClient:        addOnClient,
		clusterAddOnLister: addOnInformer.ClusterManagementAddOns().Lister(),
		addOnLister:        addOnInformer.ManagedClusterAddOns().Lister(),
		eventRecorder:      recorder.WithComponentSuffix("submariner-broker-controller"),
		resourceCache:      resourceapply.NewResourceCache(),
	}

	return factory.New().
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			accessor, _ := meta.Accessor(obj)
			logger.V(log.DEBUG).Infof("Queuing ManagedClusterSet %q", accessor.GetName())

			return accessor.GetName()
		}, clusterSetInformer.Informer()).
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			accessor, _ := meta.Accessor(obj)
			if accessor.GetName() != constants.SubmarinerAddOnName {
				return ""
			}

			logger.V(log.DEBUG).Infof("Queuing ClusterManagementAddon %q", accessor.GetName())

			return factory.DefaultQueueKey
		}, addOnInformer.ClusterManagementAddOns().Informer()).
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			accessor, _ := meta.Accessor(obj)
			if accessor.GetName() != constants.SubmarinerAddOnName {
				return ""
			}

			logger.V(log.DEBUG).Infof("Queuing ManagedClusterAddOn %q for cluster %q", accessor.GetName(), accessor.GetNamespace())

			return factory.DefaultQueueKey
		}, addOnInformer.ManagedClusterAddOns().Informer()).
		WithSync(c.sync).
		ToController("SubmarinerBrokerController", recorder)
}

func (c *submarinerBrokerController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	if syncCtx.QueueKey() == "" {
		return nil
	}

	logger.V(log.TRACE).Infof("Entering sync for %q", syncCtx.QueueKey())
	defer logger.V(log.TRACE).Infof("Exiting sync for %q", syncCtx.QueueKey())

	clusterAddOn, err := c.clusterAddOnLister.Get(constants.SubmarinerAddOnName)
	if apierrors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		return errors.Wrapf(err, "error retrieving ClusterManagementAddOn %q", constants.SubmarinerAddOnName)
	}

	if !clusterAddOn.DeletionTimestamp.IsZero() {
		logger.Infof("ClusterManagementAddOn %q is deleting", clusterAddOn.Name)

		return c.doAllClusterSetCleanup(ctx, clusterAddOn, syncCtx.Recorder())
	}

	if syncCtx.QueueKey() == factory.DefaultQueueKey {
		return nil
	}

	clusterSetName := syncCtx.QueueKey()

	clusterSet, err := c.clusterSetLister.Get(clusterSetName)
	if apierrors.IsNotFound(err) {
		// ClusterSet not found, could have been deleted, do nothing.
		return nil
	}

	if err != nil {
		return errors.Wrapf(err, "error retrieving ManagedClusterSe %q", clusterSetName)
	}

	clusterSet = clusterSet.DeepCopy()

	// ignore the non-legacy clusterset
	selectorType := clusterSet.Spec.ClusterSelector.SelectorType
	if len(selectorType) > 0 && selectorType != clusterv1beta2.ExclusiveClusterSetLabel {
		return nil
	}

	return c.reconcileManagedClusterSet(ctx, clusterSet, syncCtx.Recorder())
}

func (c *submarinerBrokerController) reconcileManagedClusterSet(ctx context.Context, clusterSet *clusterv1beta2.ManagedClusterSet,
	recorder events.Recorder,
) error {
	logger.V(log.DEBUG).Infof("Reconciling ManagedClusterSet %q", clusterSet.Name)

	// ClusterSet is deleting, we remove its related resources on hub
	if !clusterSet.DeletionTimestamp.IsZero() {
		logger.Infof("ManagedClusterSet %q is deleting", clusterSet.Name)

		return c.doClusterSetCleanup(ctx, clusterSet, recorder)
	}

	brokerNS := brokerinfo.GenerateBrokerName(clusterSet.Name)

	if !finalizer.IsPresent(clusterSet, brokerFinalizer) || clusterSet.GetAnnotations()[SubmBrokerNamespaceKey] != brokerNS {
		err := util.Update(ctx, resource.ForManagedClusterSet(c.clustersetClient), clusterSet,
			func(existing *clusterv1beta2.ManagedClusterSet) (*clusterv1beta2.ManagedClusterSet, error) {
				objMeta := coreresource.MustToMeta(existing)

				annotations := objMeta.GetAnnotations()
				if annotations == nil {
					annotations = map[string]string{}
				}

				annotations[SubmBrokerNamespaceKey] = brokerNS
				objMeta.SetAnnotations(annotations)

				objMeta.SetFinalizers(append(objMeta.GetFinalizers(), brokerFinalizer))

				return existing, nil
			})
		if err != nil {
			return errors.Wrapf(err, "error updating ManagedClusterSet %q", clusterSet.Name)
		}

		logger.Infof("Updated ManagedClusterSet %q with the finalizer and/or brokerNamespace %q", clusterSet.Name, brokerNS)

		return nil
	}

	// Apply static files
	err := resource.ApplyManifests(ctx, c.kubeClient, recorder, c.resourceCache, assetFunc(brokerNS), staticResourceFiles...)
	if err != nil {
		return err //nolint:wrapcheck // No need to wrap here
	}

	return c.createIPSecPSKSecret(ctx, brokerNS)
}

func (c *submarinerBrokerController) createIPSecPSKSecret(ctx context.Context, brokerNamespace string) error {
	_, err := c.kubeClient.CoreV1().Secrets(brokerNamespace).Get(ctx, constants.IPSecPSKSecretName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		psk := make([]byte, ipSecPSKSecretLength)
		if _, err := rand.Read(psk); err != nil {
			return errors.Wrap(err, "error generating PSK secret")
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

	return errors.Wrapf(err, "error creating IPSec PSK Secret %q", constants.IPSecPSKSecretName)
}

func (c *submarinerBrokerController) doClusterSetCleanup(ctx context.Context, clusterSet *clusterv1beta2.ManagedClusterSet,
	recorder events.Recorder,
) error {
	brokerNS := clusterSet.GetAnnotations()[SubmBrokerNamespaceKey]
	if brokerNS == "" {
		return nil
	}

	if err := resource.DeleteFromManifests(ctx, c.kubeClient, recorder, assetFunc(brokerNS), staticResourceFiles...); err != nil {
		return err //nolint:wrapcheck // No need to wrap here
	}

	//nolint:wrapcheck // No need to wrap here
	return finalizer.Remove(ctx, resource.ForManagedClusterSet(c.clustersetClient), clusterSet, brokerFinalizer)
}

func (c *submarinerBrokerController) doAllClusterSetCleanup(ctx context.Context, clusterAddOn *v1alpha1.ClusterManagementAddOn,
	recorder events.Recorder,
) error {
	addOns, err := c.addOnLister.List(labels.Everything())
	if err != nil {
		return errors.Wrap(err, "error listing ManagedClusterAddOns")
	}

	addOnsExist := false

	for _, addOn := range addOns {
		if metav1.IsControlledBy(addOn, clusterAddOn) {
			logger.Infof("ManagedClusterAddOn in cluster %q still exists - deleting", addOn.Namespace)

			err = c.addOnClient.AddonV1alpha1().ManagedClusterAddOns(addOn.Namespace).Delete(ctx, addOn.Name, metav1.DeleteOptions{})
			if err != nil {
				return errors.Wrapf(err, "error deleting ManagedClusterAddOn %q", addOn.Name)
			}

			addOnsExist = true
		}
	}

	if addOnsExist {
		return nil
	}

	logger.Info("All ManagedClusterAddOns deleted - doing broker cleaning")

	clusterSets, err := c.clusterSetLister.List(labels.Everything())
	if err != nil {
		return errors.Wrap(err, "error listing ManagedClusterSets")
	}

	for _, clusterSet := range clusterSets {
		err = c.doClusterSetCleanup(ctx, clusterSet, recorder)
		if err != nil {
			return err
		}
	}

	//nolint:wrapcheck // No need to wrap here
	return finalizer.Remove(ctx, resource.ForClusterAddon(c.addOnClient.AddonV1alpha1().ClusterManagementAddOns()), clusterAddOn,
		constants.SubmarinerAddOnFinalizer)
}

func assetFunc(brokerNS string) resourceapply.AssetFunc {
	config := &brokerConfig{
		SubmarinerNamespace: brokerNS,
	}

	return resource.AssetFromFile(manifestFiles, config)
}
