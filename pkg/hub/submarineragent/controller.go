package submarineragent

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/ghodss/yaml"
	"github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	operatorhelpers "github.com/openshift/library-go/pkg/operator/v1helpers"
	"github.com/pkg/errors"
	"github.com/stolostron/submariner-addon/pkg/addon"
	"github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig"
	configv1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	configclient "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	configinformer "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/informers/externalversions/submarinerconfig/v1alpha1"
	configlister "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/listers/submarinerconfig/v1alpha1"
	"github.com/stolostron/submariner-addon/pkg/cloud"
	"github.com/stolostron/submariner-addon/pkg/constants"
	brokerinfo "github.com/stolostron/submariner-addon/pkg/hub/submarinerbrokerinfo"
	"github.com/stolostron/submariner-addon/pkg/manifestwork"
	"github.com/stolostron/submariner-addon/pkg/resource"
	"github.com/submariner-io/admiral/pkg/finalizer"
	submarinerv1a1 "github.com/submariner-io/submariner-operator/api/submariner/v1alpha1"
	"github.com/submariner-io/submariner-operator/pkg/discovery/globalnet"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"
	addoninformerv1alpha1 "open-cluster-management.io/api/client/addon/informers/externalversions/addon/v1alpha1"
	addonlisterv1alpha1 "open-cluster-management.io/api/client/addon/listers/addon/v1alpha1"
	clusterclient "open-cluster-management.io/api/client/cluster/clientset/versioned"
	clusterinformerv1 "open-cluster-management.io/api/client/cluster/informers/externalversions/cluster/v1"
	clusterinformerv1beta1 "open-cluster-management.io/api/client/cluster/informers/externalversions/cluster/v1beta1"
	clusterlisterv1 "open-cluster-management.io/api/client/cluster/listers/cluster/v1"
	clusterlisterv1beta1 "open-cluster-management.io/api/client/cluster/listers/cluster/v1beta1"
	workclient "open-cluster-management.io/api/client/work/clientset/versioned"
	workinformer "open-cluster-management.io/api/client/work/informers/externalversions/work/v1"
	worklister "open-cluster-management.io/api/client/work/listers/work/v1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	workv1 "open-cluster-management.io/api/work/v1"
)

const (
	serviceAccountLabel          = "cluster.open-cluster-management.io/submariner-cluster-sa"
	OperatorManifestWorkName     = "submariner-operator"
	SubmarinerCRManifestWorkName = "submariner-resource"
	AgentFinalizer               = "cluster.open-cluster-management.io/submariner-agent-cleanup"
	AddOnFinalizer               = "submarineraddon.open-cluster-management.io/submariner-addon-cleanup"
	submarinerConfigFinalizer    = "submarineraddon.open-cluster-management.io/config-cleanup"
	agentRBACFile                = "manifests/rbac/operatorgroup-aggregate-clusterrole.yaml"
	submarinerCRFile             = "manifests/operator/submariner.io-submariners-cr.yaml"
	BrokerCfgApplied             = "SubmarinerBrokerConfigApplied"
	brokerObjectName             = "submariner-broker"
)

var clusterRBACFiles = []string{
	"manifests/rbac/broker-cluster-serviceaccount.yaml",
	"manifests/rbac/broker-cluster-rolebinding.yaml",
}

var sccFiles = []string{
	"manifests/rbac/scc-aggregate-clusterrole.yaml",
	"manifests/rbac/submariner-agent-scc.yaml",
}

var operatorFiles = []string{
	"manifests/operator/submariner-operator-group.yaml",
	"manifests/operator/submariner-operator-subscription.yaml",
}

//go:embed manifests
var manifestFiles embed.FS

type clusterRBACConfig struct {
	ManagedClusterName        string
	SubmarinerBrokerNamespace string
}

// submarinerAgentController reconciles instances of ManagedCluster on the hub to deploy/remove
// corresponding submariner agent manifestworks.
type submarinerAgentController struct {
	kubeClient           kubernetes.Interface
	dynamicClient        dynamic.Interface
	clusterClient        clusterclient.Interface
	manifestWorkClient   workclient.Interface
	configClient         configclient.Interface
	addOnClient          addonclient.Interface
	clusterLister        clusterlisterv1.ManagedClusterLister
	clusterSetLister     clusterlisterv1beta1.ManagedClusterSetLister
	manifestWorkLister   worklister.ManifestWorkLister
	configLister         configlister.SubmarinerConfigLister
	addOnLister          addonlisterv1alpha1.ManagedClusterAddOnLister
	cloudProviderFactory cloud.ProviderFactory
	eventRecorder        events.Recorder
	knownConfigs         map[string]*configv1alpha1.SubmarinerConfig
}

// NewSubmarinerAgentController returns a submarinerAgentController instance.
func NewSubmarinerAgentController(
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	clusterClient clusterclient.Interface,
	manifestWorkClient workclient.Interface,
	configClient configclient.Interface,
	addOnClient addonclient.Interface,
	clusterInformer clusterinformerv1.ManagedClusterInformer,
	clusterSetInformer clusterinformerv1beta1.ManagedClusterSetInformer,
	manifestWorkInformer workinformer.ManifestWorkInformer,
	configInformer configinformer.SubmarinerConfigInformer,
	addOnInformer addoninformerv1alpha1.ManagedClusterAddOnInformer,
	cloudProviderFactory cloud.ProviderFactory,
	recorder events.Recorder,
) factory.Controller {
	c := &submarinerAgentController{
		kubeClient:           kubeClient,
		dynamicClient:        dynamicClient,
		clusterClient:        clusterClient,
		manifestWorkClient:   manifestWorkClient,
		configClient:         configClient,
		addOnClient:          addOnClient,
		clusterLister:        clusterInformer.Lister(),
		clusterSetLister:     clusterSetInformer.Lister(),
		manifestWorkLister:   manifestWorkInformer.Lister(),
		configLister:         configInformer.Lister(),
		addOnLister:          addOnInformer.Lister(),
		cloudProviderFactory: cloudProviderFactory,
		eventRecorder:        recorder.WithComponentSuffix("submariner-agent-controller"),
		knownConfigs:         make(map[string]*configv1alpha1.SubmarinerConfig),
	}

	return factory.New().
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			accessor, _ := meta.Accessor(obj)

			return accessor.GetName()
		}, clusterInformer.Informer()).
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			// TODO: we may consider to use addon to deploy the submariner on the managed cluster instead of
			// using manifestwork, one problem should be considered - how to get the IPSECPSK
			accessor, _ := meta.Accessor(obj)
			if accessor.GetName() != OperatorManifestWorkName && accessor.GetName() != SubmarinerCRManifestWorkName {
				return ""
			}

			return accessor.GetNamespace()
		}, manifestWorkInformer.Informer()).
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			// TODO: we may consider to use addon to set up the submariner env on the managed cluster instead of
			// using manifestwork, one problem should be considered - how to get the cloud credentials
			accessor, _ := meta.Accessor(obj)
			if accessor.GetName() != constants.SubmarinerConfigName {
				return ""
			}

			key, _ := cache.MetaNamespaceKeyFunc(obj)

			return key
		}, configInformer.Informer()).
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			accessor, _ := meta.Accessor(obj)
			if accessor.GetName() != constants.SubmarinerAddOnName {
				return ""
			}

			return accessor.GetNamespace()
		}, addOnInformer.Informer()).
		WithInformers(clusterSetInformer.Informer()).
		WithSync(c.sync).
		ToController("SubmarinerAgentController", recorder)
}

func (c *submarinerAgentController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	key := syncCtx.QueueKey()

	// if the sync is triggered by change of ManagedClusterSet, reconcile all managed clusters
	if key == factory.DefaultQueueKey {
		return c.onManagedClusterSetChange(syncCtx)
	}

	klog.V(4).Infof("Submariner agent controller is reconciling, queue key: %s", key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		// bad format key - shouldn't happen
		panic(err)
	}

	// if the sync is triggered by change of ManagedCluster, ManifestWork or ManagedClusterAddOn, reconcile the managed cluster
	if namespace == "" {
		managedCluster, err := c.clusterLister.Get(name)
		if apierrors.IsNotFound(err) {
			// managed cluster not found, could have been deleted, do nothing.
			return nil
		}

		if err != nil {
			return err
		}

		config, err := c.configLister.SubmarinerConfigs(name).Get(constants.SubmarinerConfigName)
		if apierrors.IsNotFound(err) {
			// only sync the managed cluster
			return c.syncManagedCluster(ctx, managedCluster.DeepCopy(), nil)
		}

		if err != nil {
			return err
		}

		if err := c.syncManagedCluster(ctx, managedCluster.DeepCopy(), config.DeepCopy()); err != nil {
			return err
		}

		// handle creating submariner config before creating managed cluster
		return c.syncSubmarinerConfig(ctx, managedCluster.DeepCopy(), config.DeepCopy())
	}

	// if the sync is triggered by change of SubmarinerConfig, reconcile the submariner config
	config, err := c.configLister.SubmarinerConfigs(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		// config is not found, could have been deleted, do nothing.
		return nil
	}

	if err != nil {
		return err
	}

	managedCluster, err := c.clusterLister.Get(namespace)
	if apierrors.IsNotFound(err) {
		// handle deleting submariner config after managed cluster was deleted.
		return c.syncSubmarinerConfig(ctx, nil, config.DeepCopy())
	}

	if err != nil {
		return err
	}

	// the submariner agent config maybe need to update.
	if err := c.syncManagedCluster(ctx, managedCluster.DeepCopy(), config.DeepCopy()); err != nil {
		return err
	}

	return c.syncSubmarinerConfig(ctx, managedCluster.DeepCopy(), config.DeepCopy())
}

func (c *submarinerAgentController) onManagedClusterSetChange(syncCtx factory.SyncContext) error {
	managedClusters, err := c.clusterLister.List(labels.Everything())
	if err != nil {
		return err
	}
	for _, managedCluster := range managedClusters {
		// enqueue the managed cluster to reconcile
		syncCtx.Queue().Add(managedCluster.Name)
	}

	return nil
}

// syncManagedCluster syncs one managed cluster.
func (c *submarinerAgentController) syncManagedCluster(
	ctx context.Context,
	managedCluster *clusterv1.ManagedCluster,
	config *configv1alpha1.SubmarinerConfig,
) error {
	// find the submariner-addon on the managed cluster namespace
	addOn, err := c.addOnLister.ManagedClusterAddOns(managedCluster.Name).Get(constants.SubmarinerAddOnName)

	switch {
	case apierrors.IsNotFound(err):
		// submariner-addon is not found, could have been deleted, do nothing.
		return nil
	case err != nil:
		return err
	}

	// managed cluster is deleting, we remove its related resources
	if !managedCluster.DeletionTimestamp.IsZero() {
		return c.cleanUpSubmarinerAgent(ctx, managedCluster, addOn)
	}

	clusterSetName, existed := managedCluster.Labels[clusterv1beta1.ClusterSetLabel]
	if !existed {
		// the cluster does not have the clusterset label, try to clean up the submariner agent
		return c.cleanUpSubmarinerAgent(ctx, managedCluster, addOn)
	}

	// find the clustersets that contains this managed cluster
	_, err = c.clusterSetLister.Get(clusterSetName)

	switch {
	case apierrors.IsNotFound(err):
		// if one cluster has clusterset label, but the clusterset is not found, it could have been deleted
		// try to clean up the submariner agent
		return c.cleanUpSubmarinerAgent(ctx, managedCluster, addOn)
	case err != nil:
		return err
	}

	// submariner-addon is deleting, we remove its related resources
	if !addOn.DeletionTimestamp.IsZero() {
		return c.cleanUpSubmarinerAgent(ctx, managedCluster, addOn)
	}

	// add a submariner agent finalizer to a managed cluster
	added, err := finalizer.Add(ctx, resource.ForManagedCluster(c.clusterClient.ClusterV1().ManagedClusters()), managedCluster, AgentFinalizer)
	if added || err != nil {
		return err
	}

	// add a finalizer to the submariner-addon
	added, err = finalizer.Add(ctx, resource.ForAddon(c.addOnClient.AddonV1alpha1().ManagedClusterAddOns(managedCluster.Name)),
		addOn, AddOnFinalizer)
	if added || err != nil {
		return err
	}

	return c.deploySubmarinerAgent(ctx, clusterSetName, managedCluster, addOn, config)
}

// syncSubmarinerConfig syncs submariner configuration.
func (c *submarinerAgentController) syncSubmarinerConfig(ctx context.Context,
	managedCluster *clusterv1.ManagedCluster,
	config *configv1alpha1.SubmarinerConfig,
) error {
	if c.skipSyncingUnchangedConfig(config) {
		klog.V(4).Infof("Skip syncing submariner config %q as it didn't change", config.Namespace+"/"+config.Name)
		return nil
	}

	// add a finalizer to the submarinerconfigfinalizer.Remove(ctx, resource.ForSubmarinerConfig(
	added, err := finalizer.Add(ctx, resource.ForSubmarinerConfig(
		c.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(config.Namespace)), config, submarinerConfigFinalizer)
	if added || err != nil {
		return err
	}

	// config is deleting, we remove its related resources
	if !config.DeletionTimestamp.IsZero() {
		if !isSpokePrepared(config.Status.ManagedClusterInfo.Platform) {
			if err := c.cleanUpSubmarinerClusterEnv(config); err != nil {
				return err
			}
		}

		err = finalizer.Remove(ctx, resource.ForSubmarinerConfig(
			c.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(config.Namespace)), config, submarinerConfigFinalizer)
		if err == nil {
			delete(c.knownConfigs, config.Namespace)
		}

		return err
	}

	if managedCluster == nil {
		return nil
	}

	managedClusterInfo := getManagedClusterInfo(managedCluster)

	// prepare submariner cluster environment
	errs := []error{}
	var condition *metav1.Condition
	if !isSpokePrepared(managedClusterInfo.Platform) {
		cloudProvider, _, preparedErr := c.cloudProviderFactory.Get(&managedClusterInfo, config, c.eventRecorder)
		if preparedErr == nil {
			preparedErr = cloudProvider.PrepareSubmarinerClusterEnv()
		}

		condition = &metav1.Condition{
			Type:    configv1alpha1.SubmarinerConfigConditionEnvPrepared,
			Status:  metav1.ConditionTrue,
			Reason:  "SubmarinerClusterEnvPrepared",
			Message: "Submariner cluster environment was prepared",
		}

		if preparedErr != nil {
			condition.Status = metav1.ConditionFalse
			condition.Reason = "SubmarinerClusterEnvPreparationFailed"
			condition.Message = fmt.Sprintf("Failed to prepare submariner cluster environment: %v", preparedErr)
			errs = append(errs, preparedErr)
		}
	}

	_, updated, updatedErr := submarinerconfig.UpdateStatus(ctx, c.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(config.Namespace),
		config.Name, submarinerconfig.UpdateStatusFn(condition, &managedClusterInfo))

	if updatedErr != nil {
		errs = append(errs, updatedErr)
	}

	if updated {
		c.eventRecorder.Eventf("SubmarinerClusterEnvPrepared",
			"submariner cluster environment was prepared for managed cluster %s", config.Namespace)
	}

	if len(errs) == 0 {
		c.knownConfigs[config.Namespace] = config
	}

	return operatorhelpers.NewMultiLineAggregate(errs)
}

// skipSyncingUnchangedConfig if last submariner config is known and is equal to the given config.
func (c *submarinerAgentController) skipSyncingUnchangedConfig(config *configv1alpha1.SubmarinerConfig) bool {
	lastConfig, known := c.knownConfigs[config.Namespace]
	return known && reflect.DeepEqual(lastConfig.Spec, config.Spec) && config.DeletionTimestamp.IsZero() &&
		reflect.DeepEqual(lastConfig.Finalizers, config.Finalizers)
}

// clean up the submariner agent from this managedCluster.
func (c *submarinerAgentController) cleanUpSubmarinerAgent(ctx context.Context, managedCluster *clusterv1.ManagedCluster,
	addOn *addonv1alpha1.ManagedClusterAddOn,
) error {
	submarinerManifestWork, err := c.manifestWorkLister.ManifestWorks(managedCluster.Name).Get(SubmarinerCRManifestWorkName)

	switch {
	case apierrors.IsNotFound(err):
		if err := c.deleteManifestWork(ctx, OperatorManifestWorkName, managedCluster.Name); err != nil {
			return err
		}
	case err != nil:
		return errors.Wrapf(err, "error retrieving ManifestWork %q", SubmarinerCRManifestWorkName)
	case submarinerManifestWork.DeletionTimestamp.IsZero():
		return c.deleteManifestWork(ctx, SubmarinerCRManifestWorkName, managedCluster.Name)
	default:
		return nil
	}

	// remove service account and its rolebinding from broker namespace
	if err := c.removeClusterRBACFiles(ctx, managedCluster.Name); err != nil {
		return err
	}

	if err := finalizer.Remove(ctx, resource.ForManagedCluster(c.clusterClient.ClusterV1().ManagedClusters()), managedCluster,
		AgentFinalizer); err != nil {
		return err
	}

	return finalizer.Remove(ctx, resource.ForAddon(c.addOnClient.AddonV1alpha1().ManagedClusterAddOns(managedCluster.Name)),
		addOn, AddOnFinalizer)
}

func (c *submarinerAgentController) deploySubmarinerAgent(
	ctx context.Context,
	clusterSetName string,
	managedCluster *clusterv1.ManagedCluster,
	managedClusterAddOn *addonv1alpha1.ManagedClusterAddOn,
	submarinerConfig *configv1alpha1.SubmarinerConfig,
) error {
	// generate service account and bind it to `submariner-k8s-broker-cluster` role
	brokerNamespace := brokerinfo.GenerateBrokerName(clusterSetName)
	if err := c.applyClusterRBACFiles(ctx, brokerNamespace, managedCluster.Name); err != nil {
		return err
	}

	err := c.createGNConfigMapIfNecessary(brokerNamespace)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if apierrors.IsNotFound(err) {
		_ = c.updateManagedClusterAddOnStatus(ctx, managedClusterAddOn, brokerNamespace, true)
		return fmt.Errorf("brokers.submariner.io object named %q missing in namespace %q", brokerObjectName, brokerNamespace)
	}

	_ = c.updateManagedClusterAddOnStatus(ctx, managedClusterAddOn, brokerNamespace, false)

	// create submariner broker info with submariner config
	brokerInfo, err := brokerinfo.Get(
		c.kubeClient,
		c.dynamicClient,
		managedCluster.Name,
		brokerNamespace,
		submarinerConfig,
		managedClusterAddOn.Spec.InstallNamespace,
	)
	if err != nil {
		return fmt.Errorf("failed to create submariner brokerInfo of cluster %v : %w", managedCluster.Name, err)
	}

	if submarinerConfig != nil {
		err := c.updateSubmarinerConfigStatus(ctx, submarinerConfig, managedCluster.Name)
		if err != nil {
			return err
		}
	}

	// Apply submariner operator manifest work
	operatorManifestWork, err := newOperatorManifestWork(managedCluster, brokerInfo)
	if err != nil {
		return err
	}

	if err := manifestwork.Apply(ctx, c.manifestWorkClient, operatorManifestWork, c.eventRecorder); err != nil {
		return err
	}

	// Apply submariner resource manifest work
	submarinerManifestWork, err := newSubmarinerManifestWork(managedCluster, brokerInfo)
	if err != nil {
		return err
	}

	if err := manifestwork.Apply(ctx, c.manifestWorkClient, submarinerManifestWork, c.eventRecorder); err != nil {
		return err
	}

	return nil
}

func (c *submarinerAgentController) updateSubmarinerConfigStatus(ctx context.Context, submarinerConfig *configv1alpha1.SubmarinerConfig,
	clusterName string,
) error {
	condition := &metav1.Condition{
		Type:    configv1alpha1.SubmarinerConfigConditionApplied,
		Status:  metav1.ConditionTrue,
		Reason:  "SubmarinerConfigApplied",
		Message: "SubmarinerConfig was applied",
	}

	_, updated, err := submarinerconfig.UpdateStatus(ctx,
		c.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(submarinerConfig.Namespace), submarinerConfig.Name,
		submarinerconfig.UpdateConditionFn(condition))

	if updated {
		c.eventRecorder.Eventf("SubmarinerConfigApplied", "SubmarinerConfig %q was applied for managed cluster %q",
			submarinerConfig.Name, clusterName)
	}

	return err
}

func (c *submarinerAgentController) updateManagedClusterAddOnStatus(ctx context.Context,
	managedClusterAddon *addonv1alpha1.ManagedClusterAddOn, brokerNamespace string, missing bool,
) error {
	condition := metav1.Condition{
		Type: BrokerCfgApplied,
	}

	var message, reason string
	if missing {
		message = fmt.Sprintf("Waiting for brokers.submariner.io object named %q to be created in %q namespace",
			brokerObjectName, brokerNamespace)
		condition.Status = metav1.ConditionFalse
		reason = "BrokerConfigMissing"
	} else {
		message = fmt.Sprintf("Configuration applied from brokers.submariner.io object in %q namespace",
			brokerNamespace)
		condition.Status = metav1.ConditionTrue
		reason = "BrokerConfigApplied"
	}

	condition.Reason = reason

	_, updated, err := addon.UpdateStatus(ctx, c.addOnClient, managedClusterAddon.Namespace,
		addon.UpdateConditionFn(&condition))
	if err != nil {
		return err
	}

	if updated {
		c.eventRecorder.Eventf(reason, message)
	}

	return err
}

func (c *submarinerAgentController) deleteManifestWork(ctx context.Context, name, clusterName string) error {
	err := c.manifestWorkClient.WorkV1().ManifestWorks(clusterName).Delete(ctx, name, metav1.DeleteOptions{})

	switch {
	case apierrors.IsNotFound(err):
		// there is no manifestwork, do nothing
	case err == nil:
		c.eventRecorder.Eventf("SubmarinerManifestWorksDeleted", "Deleted manifestwork %q",
			fmt.Sprintf("%s/%s", clusterName, name))
	case err != nil:
		return errors.Wrapf(err, "error deleting manifestwork %q from managed cluster %q", name, clusterName)
	}

	return nil
}

func (c *submarinerAgentController) applyClusterRBACFiles(ctx context.Context, brokerNamespace, managedClusterName string) error {
	config := &clusterRBACConfig{
		ManagedClusterName:        managedClusterName,
		SubmarinerBrokerNamespace: brokerNamespace,
	}

	return resource.ApplyManifests(ctx, c.kubeClient, c.eventRecorder, resource.AssetFromFile(manifestFiles, config), clusterRBACFiles...)
}

func (c *submarinerAgentController) removeClusterRBACFiles(ctx context.Context, managedClusterName string) error {
	serviceAccounts, err := c.kubeClient.CoreV1().ServiceAccounts(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", serviceAccountLabel, managedClusterName),
	})
	if err != nil {
		return err
	}

	// no serviceaccounts are found, do nothing
	if len(serviceAccounts.Items) == 0 {
		return nil
	}

	if len(serviceAccounts.Items) > 1 {
		return fmt.Errorf("one more than service accounts are found for %q", managedClusterName)
	}

	config := &clusterRBACConfig{
		ManagedClusterName:        managedClusterName,
		SubmarinerBrokerNamespace: serviceAccounts.Items[0].Namespace,
	}

	return resource.DeleteFromManifests(ctx, c.kubeClient, c.eventRecorder, resource.AssetFromFile(manifestFiles, config),
		clusterRBACFiles...)
}

func (c *submarinerAgentController) cleanUpSubmarinerClusterEnv(config *configv1alpha1.SubmarinerConfig) error {
	cloudProvider, _, err := c.cloudProviderFactory.Get(&config.Status.ManagedClusterInfo, config, c.eventRecorder)
	if err != nil {
		// TODO handle the error gracefully in the future
		c.eventRecorder.Warningf("CleanUpSubmarinerClusterEnvFailed", "failed to get cloud provider: %v", err)

		return nil
	}

	if err := cloudProvider.CleanUpSubmarinerClusterEnv(); err != nil {
		// TODO handle the error gracefully in the future
		c.eventRecorder.Warningf("CleanUpSubmarinerClusterEnvFailed", "failed to clean up cloud environment: %v", err)

		return nil
	}

	c.eventRecorder.Eventf("SubmarinerClusterEnvDeleted", "the managed cluster %s submariner cluster environment is deleted",
		config.Status.ManagedClusterInfo.ClusterName)

	return nil
}

func newSubmarinerManifestWork(managedCluster *clusterv1.ManagedCluster, config interface{}) (*workv1.ManifestWork, error) {
	return newManifestWork(SubmarinerCRManifestWorkName, managedCluster.Name, config, submarinerCRFile)
}

func newOperatorManifestWork(managedCluster *clusterv1.ManagedCluster, config interface{}) (*workv1.ManifestWork, error) {
	files := []string{agentRBACFile}
	clusterProduct := getClusterProduct(managedCluster)
	if clusterProduct == constants.ProductOCP || clusterProduct == constants.ProductROSA || clusterProduct == constants.ProductARO {
		files = append(files, sccFiles...)
	}

	files = append(files, operatorFiles...)

	return newManifestWork(OperatorManifestWorkName, managedCluster.Name, config, files...)
}

func newManifestWork(name, namespace string, config interface{}, files ...string) (*workv1.ManifestWork, error) {
	manifests := []workv1.Manifest{}

	for _, file := range files {
		template, err := manifestFiles.ReadFile(file)
		if err != nil {
			return nil, err
		}

		yamlData := assets.MustCreateAssetFromTemplate(file, template, config).Data
		jsonData, err := yaml.YAMLToJSON(yamlData)
		if err != nil {
			return nil, err
		}

		manifest := workv1.Manifest{RawExtension: runtime.RawExtension{Raw: jsonData}}
		manifests = append(manifests, manifest)
	}

	return &workv1.ManifestWork{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: manifests,
			},
		},
	}, nil
}

func getClusterProduct(managedCluster *clusterv1.ManagedCluster) string {
	for _, claim := range managedCluster.Status.ClusterClaims {
		if claim.Name == "product.open-cluster-management.io" {
			return claim.Value
		}
	}

	return ""
}

func getManagedClusterInfo(managedCluster *clusterv1.ManagedCluster) configv1alpha1.ManagedClusterInfo {
	clusterInfo := configv1alpha1.ManagedClusterInfo{
		ClusterName: managedCluster.Name,
	}

	for _, claim := range managedCluster.Status.ClusterClaims {
		if claim.Name == "product.open-cluster-management.io" {
			clusterInfo.Vendor = claim.Value
		}

		if claim.Name == "platform.open-cluster-management.io" {
			clusterInfo.Platform = claim.Value
		}

		if claim.Name == "region.open-cluster-management.io" {
			clusterInfo.Region = claim.Value
		}

		if claim.Name == "infrastructure.openshift.io" {
			var infraInfo map[string]interface{}
			if err := json.Unmarshal([]byte(claim.Value), &infraInfo); err == nil {
				clusterInfo.InfraID = fmt.Sprintf("%v", infraInfo["infraName"])
			}
		}

		if claim.Name == "version.openshift.io" {
			clusterInfo.VendorVersion = claim.Value
		}
	}

	return clusterInfo
}

func (c *submarinerAgentController) createGNConfigMapIfNecessary(brokerNamespace string) error {
	_, gnCmErr := globalnet.GetConfigMap(c.kubeClient, brokerNamespace)
	if gnCmErr != nil && !apierrors.IsNotFound(gnCmErr) {
		return errors.Wrapf(gnCmErr, "error getting globalnet configmap from broker namespace %q", brokerNamespace)
	}

	if gnCmErr == nil {
		return nil
	}

	// globalnetConfig is missing in the broker-namespace, try creating it from submariner-broker object.
	brokerGVR := schema.GroupVersionResource{
		Group:    "submariner.io",
		Version:  "v1alpha1",
		Resource: "brokers",
	}

	brokerCfg, brokerErr := c.dynamicClient.Resource(brokerGVR).Namespace(brokerNamespace).Get(context.TODO(),
		brokerObjectName, metav1.GetOptions{})
	if brokerErr != nil {
		return errors.Wrapf(brokerErr, "error getting broker object from namespace %q", brokerNamespace)
	}

	brokerObj := &submarinerv1a1.Broker{}

	err := runtime.DefaultUnstructuredConverter.FromUnstructured(brokerCfg.Object, brokerObj)
	if err != nil {
		return errors.Wrapf(err, "error converting broker object in namespace %q", brokerNamespace)
	}

	if brokerObj.Spec.GlobalnetEnabled {
		klog.Infof("Globalnet is enabled in the managedClusterSet namespace %q", brokerNamespace)

		if brokerObj.Spec.DefaultGlobalnetClusterSize == 0 {
			brokerObj.Spec.DefaultGlobalnetClusterSize = globalnet.DefaultGlobalnetClusterSize
		}

		if brokerObj.Spec.GlobalnetCIDRRange == "" {
			brokerObj.Spec.GlobalnetCIDRRange = globalnet.DefaultGlobalnetCIDR
		}
	} else {
		klog.Infof("Globalnet is disabled in the managedClusterSet namespace %q", brokerNamespace)
	}

	if err := globalnet.CreateConfigMap(c.kubeClient, brokerObj.Spec.GlobalnetEnabled,
		brokerObj.Spec.GlobalnetCIDRRange, brokerObj.Spec.DefaultGlobalnetClusterSize, brokerNamespace); err != nil {
		return errors.Wrapf(err, "error creating globalnet configmap on Broker")
	}

	return nil
}

func isSpokePrepared(cloudName string) bool {
	return cloudName != "AWS"
}
