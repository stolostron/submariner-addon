package submarineragent

import (
	"context"
	"embed"
	"fmt"

	"github.com/ghodss/yaml"
	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	configclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"

	//nolint:lll // Can't split line
	configinformer "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/informers/externalversions/submarinerconfig/v1alpha1"
	configlister "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/listers/submarinerconfig/v1alpha1"
	"github.com/open-cluster-management/submariner-addon/pkg/cloud"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"
	brokerinfo "github.com/open-cluster-management/submariner-addon/pkg/hub/submarinerbrokerinfo"
	"github.com/open-cluster-management/submariner-addon/pkg/manifestwork"
	"github.com/open-cluster-management/submariner-addon/pkg/resource"
	"github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	operatorhelpers "github.com/openshift/library-go/pkg/operator/v1helpers"
	"github.com/submariner-io/admiral/pkg/finalizer"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
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
	workv1 "open-cluster-management.io/api/work/v1"
)

const ManifestWorkName = "submariner-operator"

const (
	ClusterSetLabel     = "cluster.open-cluster-management.io/clusterset"
	serviceAccountLabel = "cluster.open-cluster-management.io/submariner-cluster-sa"
)

const (
	AgentFinalizer            = "cluster.open-cluster-management.io/submariner-agent-cleanup"
	AddOnFinalizer            = "submarineraddon.open-cluster-management.io/submariner-addon-cleanup"
	submarinerConfigFinalizer = "submarineraddon.open-cluster-management.io/config-cleanup"
)

var clusterRBACFiles = []string{
	"manifests/rbac/broker-cluster-serviceaccount.yaml",
	"manifests/rbac/broker-cluster-rolebinding.yaml",
}

const agentRBACFile = "manifests/rbac/operatorgroup-aggregate-clusterrole.yaml"

var sccFiles = []string{
	"manifests/rbac/scc-aggregate-clusterrole.yaml",
	"manifests/rbac/submariner-agent-scc.yaml",
}

var operatorFiles = []string{
	"manifests/operator/submariner-operator-group.yaml",
	"manifests/operator/submariner-operator-subscription.yaml",
	"manifests/operator/submariner.io-submariners-cr.yaml",
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
	recorder events.Recorder) factory.Controller {
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
			if accessor.GetName() != ManifestWorkName {
				return ""
			}

			return accessor.GetNamespace()
		}, manifestWorkInformer.Informer()).
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			// TODO: we may consider to use addon to set up the submariner env on the managed cluster instead of
			// using manifestwork, one problem should be considered - how to get the cloud credentials
			accessor, _ := meta.Accessor(obj)
			if accessor.GetName() != helpers.SubmarinerConfigName {
				return ""
			}

			return accessor.GetNamespace() + "/" + accessor.GetName()
		}, configInformer.Informer()).
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			accessor, _ := meta.Accessor(obj)
			if accessor.GetName() != helpers.SubmarinerAddOnName {
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
		// ignore bad format key
		return nil
	}

	// if the sync is triggered by change of ManagedCluster, ManifestWork or ManagedClusterAddOn, reconcile the managed cluster
	if namespace == "" {
		managedCluster, err := c.clusterLister.Get(name)
		if errors.IsNotFound(err) {
			// managed cluster not found, could have been deleted, do nothing.
			return nil
		}

		if err != nil {
			return err
		}

		config, err := c.configLister.SubmarinerConfigs(name).Get(helpers.SubmarinerConfigName)
		if errors.IsNotFound(err) {
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
	if errors.IsNotFound(err) {
		// config is not found, could have been deleted, do nothing.
		return nil
	}

	if err != nil {
		return err
	}

	managedCluster, err := c.clusterLister.Get(namespace)
	if errors.IsNotFound(err) {
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
	config *configv1alpha1.SubmarinerConfig) error {
	// find the submariner-addon on the managed cluster namespace
	addOn, err := c.addOnLister.ManagedClusterAddOns(managedCluster.Name).Get(helpers.SubmarinerAddOnName)

	switch {
	case errors.IsNotFound(err):
		// submariner-addon is not found, could have been deleted, do nothing.
		return nil
	case err != nil:
		return err
	}

	// add a submariner agent finalizer to a managed cluster
	added, err := finalizer.Add(ctx, resource.ForManagedCluster(c.clusterClient.ClusterV1().ManagedClusters()), managedCluster, AgentFinalizer)
	if added || err != nil {
		return err
	}

	// managed cluster is deleting, we remove its related resources
	if !managedCluster.DeletionTimestamp.IsZero() {
		return c.cleanUpSubmarinerAgent(ctx, managedCluster, addOn)
	}

	clusterSetName, existed := managedCluster.Labels[ClusterSetLabel]
	if !existed {
		// the cluster does not have the clusterset label, try to clean up the submariner agent
		return c.cleanUpSubmarinerAgent(ctx, managedCluster, addOn)
	}

	// find the clustersets that contains this managed cluster
	_, err = c.clusterSetLister.Get(clusterSetName)

	switch {
	case errors.IsNotFound(err):
		// if one cluster has clusterset label, but the clusterset is not found, it could have been deleted
		// try to clean up the submariner agent
		return c.cleanUpSubmarinerAgent(ctx, managedCluster, addOn)
	case err != nil:
		return err
	}

	// add a finalizer to the submariner-addon
	added, err = finalizer.Add(ctx, resource.ForAddon(c.addOnClient.AddonV1alpha1().ManagedClusterAddOns(managedCluster.Name)),
		addOn, AddOnFinalizer)
	if added || err != nil {
		return err
	}

	// submariner-addon is deleting, we remove its related resources
	if !addOn.DeletionTimestamp.IsZero() {
		return c.cleanUpSubmarinerAgent(ctx, managedCluster, addOn)
	}

	return c.deploySubmarinerAgent(ctx, clusterSetName, managedCluster, addOn, config)
}

// syncSubmarinerConfig syncs submariner configuration.
func (c *submarinerAgentController) syncSubmarinerConfig(ctx context.Context,
	managedCluster *clusterv1.ManagedCluster,
	config *configv1alpha1.SubmarinerConfig) error {
	// add a finalizer to the submarinerconfigfinalizer.Remove(ctx, resource.ForSubmarinerConfig(
	added, err := finalizer.Add(ctx, resource.ForSubmarinerConfig(
		c.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(config.Namespace)), config, submarinerConfigFinalizer)
	if added || err != nil {
		return err
	}

	// config is deleting, we remove its related resources
	if !config.DeletionTimestamp.IsZero() {
		if config.Status.ManagedClusterInfo.Platform != "GCP" {
			if err := c.cleanUpSubmarinerClusterEnv(config); err != nil {
				return err
			}
		}

		return finalizer.Remove(ctx, resource.ForSubmarinerConfig(
			c.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(config.Namespace)), config, submarinerConfigFinalizer)
	}

	if managedCluster == nil {
		return nil
	}

	managedClusterInfo := helpers.GetManagedClusterInfo(managedCluster)

	// prepare submariner cluster environment
	errs := []error{}
	var condition *metav1.Condition
	if managedClusterInfo.Platform != "GCP" {
		cloudProvider, preparedErr := c.cloudProviderFactory.Get(managedClusterInfo, config, c.eventRecorder)
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

	_, updated, updatedErr := helpers.UpdateSubmarinerConfigStatus(
		c.configClient,
		config.Namespace, config.Name,
		helpers.UpdateSubmarinerConfigStatusFn(condition, &managedClusterInfo),
	)

	if updatedErr != nil {
		errs = append(errs, updatedErr)
	}

	if updated {
		c.eventRecorder.Eventf("SubmarinerClusterEnvPrepared",
			"submariner cluster environment was prepared for managed cluster %s", config.Namespace)
	}

	return operatorhelpers.NewMultiLineAggregate(errs)
}

// clean up the submariner agent from this managedCluster.
func (c *submarinerAgentController) cleanUpSubmarinerAgent(ctx context.Context, managedCluster *clusterv1.ManagedCluster,
	addOn *addonv1alpha1.ManagedClusterAddOn) error {
	if err := c.removeSubmarinerAgent(ctx, managedCluster.Name); err != nil {
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
	submarinerConfig *configv1alpha1.SubmarinerConfig) error {
	// generate service account and bind it to `submariner-k8s-broker-cluster` role
	brokerNamespace := helpers.GenerateBrokerName(clusterSetName)
	if err := c.applyClusterRBACFiles(brokerNamespace, managedCluster.Name); err != nil {
		return err
	}

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
		err := c.updateSubmarinerConfigStatus(submarinerConfig, managedCluster.Name)
		if err != nil {
			return err
		}
	}

	// apply submariner operator manifest work
	operatorManifestWork, err := getManifestWork(managedCluster, brokerInfo)
	if err != nil {
		return err
	}

	if err := manifestwork.Apply(ctx, c.manifestWorkClient, operatorManifestWork, c.eventRecorder); err != nil {
		return err
	}

	return nil
}

func (c *submarinerAgentController) updateSubmarinerConfigStatus(submarinerConfig *configv1alpha1.SubmarinerConfig,
	clusterName string) error {
	condition := &metav1.Condition{
		Type:    configv1alpha1.SubmarinerConfigConditionApplied,
		Status:  metav1.ConditionTrue,
		Reason:  "SubmarinerConfigApplied",
		Message: "SubmarinerConfig was applied",
	}

	_, updated, err := helpers.UpdateSubmarinerConfigStatus(c.configClient, submarinerConfig.Namespace, submarinerConfig.Name,
		helpers.UpdateSubmarinerConfigConditionFn(condition))

	if updated {
		c.eventRecorder.Eventf("SubmarinerConfigApplied", "SubmarinerConfig %q was applied for managed cluster %q",
			submarinerConfig.Name, clusterName)
	}

	return err
}

func (c *submarinerAgentController) removeSubmarinerAgent(ctx context.Context, clusterName string) error {
	errs := []error{}
	// remove submariner manifestworks
	err := c.manifestWorkClient.WorkV1().ManifestWorks(clusterName).Delete(ctx, ManifestWorkName, metav1.DeleteOptions{})

	switch {
	case errors.IsNotFound(err):
		// there is no submariner manifestworks, do noting
	case err == nil:
		c.eventRecorder.Eventf("SubmarinerManifestWorksDeleted", "Deleted manifestwork %q", fmt.Sprintf("%s/%s", clusterName, ManifestWorkName))
	case err != nil:
		errs = append(errs, fmt.Errorf("failed to remove submariner agent from managed cluster %v: %w", clusterName, err))
	}

	// remove service account and its rolebinding from broker namespace
	if err := c.removeClusterRBACFiles(ctx, clusterName); err != nil {
		errs = append(errs, err)
	}

	return operatorhelpers.NewMultiLineAggregate(errs)
}

func (c *submarinerAgentController) applyClusterRBACFiles(brokerNamespace, managedClusterName string) error {
	config := &clusterRBACConfig{
		ManagedClusterName:        managedClusterName,
		SubmarinerBrokerNamespace: brokerNamespace,
	}
	clientHolder := resourceapply.NewKubeClientHolder(c.kubeClient)
	applyResults := resourceapply.ApplyDirectly(
		context.TODO(),
		clientHolder,
		c.eventRecorder,
		func(name string) ([]byte, error) {
			template, err := manifestFiles.ReadFile(name)
			if err != nil {
				return nil, err
			}

			return assets.MustCreateAssetFromTemplate(name, template, config).Data, nil
		},
		clusterRBACFiles...,
	)
	errs := []error{}
	for _, result := range applyResults {
		if result.Error != nil {
			errs = append(errs, fmt.Errorf("%q (%T): %w", result.File, result.Type, result.Error))
		}
	}

	return operatorhelpers.NewMultiLineAggregate(errs)
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

	return helpers.CleanUpSubmarinerManifests(
		ctx,
		c.kubeClient,
		c.eventRecorder,
		func(name string) ([]byte, error) {
			template, err := manifestFiles.ReadFile(name)
			if err != nil {
				return nil, err
			}

			return assets.MustCreateAssetFromTemplate(name, template, config).Data, nil
		},
		clusterRBACFiles...,
	)
}

func (c *submarinerAgentController) cleanUpSubmarinerClusterEnv(config *configv1alpha1.SubmarinerConfig) error {
	cloudProvider, err := c.cloudProviderFactory.Get(config.Status.ManagedClusterInfo, config, c.eventRecorder)
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

func getManifestWork(managedCluster *clusterv1.ManagedCluster, config interface{}) (*workv1.ManifestWork, error) {
	files := []string{agentRBACFile}
	if helpers.GetClusterProduct(managedCluster) == helpers.ProductOCP {
		files = append(files, sccFiles...)
	}

	files = append(files, operatorFiles...)

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
			Name:      ManifestWorkName,
			Namespace: managedCluster.Name,
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: manifests,
			},
		},
	}, nil
}
