package submarineragent

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/ghodss/yaml"

	addonv1alpha1 "github.com/open-cluster-management/api/addon/v1alpha1"
	addonclient "github.com/open-cluster-management/api/client/addon/clientset/versioned"
	addoninformerv1alpha1 "github.com/open-cluster-management/api/client/addon/informers/externalversions/addon/v1alpha1"
	addonlisterv1alpha1 "github.com/open-cluster-management/api/client/addon/listers/addon/v1alpha1"
	clusterclient "github.com/open-cluster-management/api/client/cluster/clientset/versioned"
	clusterinformerv1 "github.com/open-cluster-management/api/client/cluster/informers/externalversions/cluster/v1"
	clusterinformerv1alpha1 "github.com/open-cluster-management/api/client/cluster/informers/externalversions/cluster/v1alpha1"
	clusterlisterv1 "github.com/open-cluster-management/api/client/cluster/listers/cluster/v1"
	clusterlisterv1alpha1 "github.com/open-cluster-management/api/client/cluster/listers/cluster/v1alpha1"
	workclient "github.com/open-cluster-management/api/client/work/clientset/versioned"
	workinformer "github.com/open-cluster-management/api/client/work/informers/externalversions/work/v1"
	worklister "github.com/open-cluster-management/api/client/work/listers/work/v1"
	clusterv1 "github.com/open-cluster-management/api/cluster/v1"
	workv1 "github.com/open-cluster-management/api/work/v1"
	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	configclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	configinformer "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/informers/externalversions/submarinerconfig/v1alpha1"
	configlister "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/listers/submarinerconfig/v1alpha1"
	"github.com/open-cluster-management/submariner-addon/pkg/cloud"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"
	"github.com/open-cluster-management/submariner-addon/pkg/hub/submarineragent/bindata"
	brokerinfo "github.com/open-cluster-management/submariner-addon/pkg/hub/submarinerbrokerinfo"

	"github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	operatorhelpers "github.com/openshift/library-go/pkg/operator/v1helpers"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

const (
	submarinerAddOnName       = "submariner"
	agentFinalizer            = "cluster.open-cluster-management.io/submariner-agent-cleanup"
	clusterSetLabel           = "cluster.open-cluster-management.io/clusterset"
	manifestWorkName          = "submariner-operator"
	serviceAccountLabel       = "cluster.open-cluster-management.io/submariner-cluster-sa"
	submarinerConfigFinalizer = "submarineraddon.open-cluster-management.io/config-cleanup"
	addOnFinalizer            = "submarineraddon.open-cluster-management.io/submariner-addon-cleanup"
)

var clusterRBACFiles = []string{
	"manifests/agent/rbac/broker-cluster-serviceaccount.yaml",
	"manifests/agent/rbac/broker-cluster-rolebinding.yaml",
}

const agentRBACFile = "manifests/agent/rbac/operatorgroup-aggregate-clusterrole.yaml"

var sccFiles = []string{
	"manifests/agent/rbac/scc-aggregate-clusterrole.yaml",
	"manifests/agent/rbac/submariner-agent-scc.yaml",
}

var operatorFiles = []string{
	"manifests/agent/operator/submariner-operator-group.yaml",
	"manifests/agent/operator/submariner-operator-subscription.yaml",
	"manifests/agent/operator/submariner.io-submariners-cr.yaml",
}

type clusterRBACConfig struct {
	ManagedClusterName        string
	SubmarinerBrokerNamespace string
}

// submarinerAgentController reconciles instances of ManagedCluster on the hub to deploy/remove
// corresponding submariner agent manifestworks
type submarinerAgentController struct {
	kubeClient         kubernetes.Interface
	dynamicClient      dynamic.Interface
	clusterClient      clusterclient.Interface
	manifestWorkClient workclient.Interface
	configClient       configclient.Interface
	addOnClient        addonclient.Interface
	clusterLister      clusterlisterv1.ManagedClusterLister
	clusterSetLister   clusterlisterv1alpha1.ManagedClusterSetLister
	manifestWorkLister worklister.ManifestWorkLister
	configLister       configlister.SubmarinerConfigLister
	addOnLister        addonlisterv1alpha1.ManagedClusterAddOnLister
	eventRecorder      events.Recorder
}

// NewSubmarinerAgentController returns a submarinerAgentController instance
func NewSubmarinerAgentController(
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	clusterClient clusterclient.Interface,
	manifestWorkClient workclient.Interface,
	configClient configclient.Interface,
	addOnClient addonclient.Interface,
	clusterInformer clusterinformerv1.ManagedClusterInformer,
	clusterSetInformer clusterinformerv1alpha1.ManagedClusterSetInformer,
	manifestWorkInformer workinformer.ManifestWorkInformer,
	configInformer configinformer.SubmarinerConfigInformer,
	addOnInformer addoninformerv1alpha1.ManagedClusterAddOnInformer,
	recorder events.Recorder) factory.Controller {
	c := &submarinerAgentController{
		kubeClient:         kubeClient,
		dynamicClient:      dynamicClient,
		clusterClient:      clusterClient,
		manifestWorkClient: manifestWorkClient,
		configClient:       configClient,
		addOnClient:        addOnClient,
		clusterLister:      clusterInformer.Lister(),
		clusterSetLister:   clusterSetInformer.Lister(),
		manifestWorkLister: manifestWorkInformer.Lister(),
		configLister:       configInformer.Lister(),
		addOnLister:        addOnInformer.Lister(),
		eventRecorder:      recorder.WithComponentSuffix("submariner-agent-controller"),
	}
	return factory.New().
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			accessor, _ := meta.Accessor(obj)
			return accessor.GetName()
		}, clusterInformer.Informer()).
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			accessor, _ := meta.Accessor(obj)
			return accessor.GetNamespace()
		}, manifestWorkInformer.Informer()).
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			accessor, _ := meta.Accessor(obj)
			if accessor.GetName() != helpers.SubmarinerConfigName {
				return ""
			}
			return accessor.GetNamespace() + "/" + accessor.GetName()
		}, configInformer.Informer()).
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			accessor, _ := meta.Accessor(obj)
			if accessor.GetName() != submarinerAddOnName {
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

	klog.V(4).Infof("Submariner agent controller is reconciling, queue key: %s", key)

	// if the sync is triggered by change of ManagedClusterSet, reconcile all managed clusters
	if key == factory.DefaultQueueKey {
		if err := c.syncAllManagedClusters(ctx); err != nil {
			return err
		}
		return nil
	}

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

// syncAllManagedClusters syncs all managed clusters
func (c *submarinerAgentController) syncAllManagedClusters(ctx context.Context) error {
	managedClusters, err := c.clusterLister.List(labels.Everything())
	if err != nil {
		return err
	}

	errs := []error{}
	for _, managedCluster := range managedClusters {
		config, err := c.configLister.SubmarinerConfigs(managedCluster.ClusterName).Get(helpers.SubmarinerConfigName)
		if errors.IsNotFound(err) {
			// only sync managed clsuter
			errs = append(errs, c.syncManagedCluster(ctx, managedCluster.DeepCopy(), nil))
			continue
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}

		errs = append(errs, c.syncManagedCluster(ctx, managedCluster.DeepCopy(), config.DeepCopy()))
	}

	return operatorhelpers.NewMultiLineAggregate(errs)
}

// syncManagedCluster syncs one managed cluster
func (c *submarinerAgentController) syncManagedCluster(
	ctx context.Context,
	managedCluster *clusterv1.ManagedCluster,
	config *configv1alpha1.SubmarinerConfig) error {
	clusterSetName, existed := managedCluster.Labels[clusterSetLabel]
	if !existed {
		// the cluster does not have the clusterset label, try to clean up the submariner agent
		return c.cleanUpSubmarinerAgent(ctx, managedCluster)
	}

	// find the clustersets that contains this managed cluster
	_, err := c.clusterSetLister.Get(clusterSetName)
	switch {
	case errors.IsNotFound(err):
		// if the clusterset is not found, try to clean up the submariner agent
		return c.cleanUpSubmarinerAgent(ctx, managedCluster)
	case err != nil:
		return err
	}

	// find the submariner-addon on the managed cluster namespace
	addOn, err := c.addOnLister.ManagedClusterAddOns(managedCluster.Name).Get(submarinerAddOnName)
	switch {
	case errors.IsNotFound(err):
		// if the submariner-addon is not found, try to clean up the submariner agent
		return c.cleanUpSubmarinerAgent(ctx, managedCluster)
	case err != nil:
		return err
	}

	// add a submariner agent finalizer to a managed cluster
	if managedCluster.DeletionTimestamp.IsZero() {
		hasFinalizer := false
		for i := range managedCluster.Finalizers {
			if managedCluster.Finalizers[i] == agentFinalizer {
				hasFinalizer = true
				break
			}
		}
		if !hasFinalizer {
			managedCluster.Finalizers = append(managedCluster.Finalizers, agentFinalizer)
			_, err := c.clusterClient.ClusterV1().ManagedClusters().Update(ctx, managedCluster, metav1.UpdateOptions{})
			return err
		}
	}

	// managed cluster is deleting, we remove its related resources
	if !managedCluster.DeletionTimestamp.IsZero() {
		return c.cleanUpSubmarinerAgent(ctx, managedCluster)
	}

	// add a submariner agent finalizer to the submariner-addon
	if addOn.DeletionTimestamp.IsZero() {
		hasFinalizer := false
		for i := range addOn.Finalizers {
			if addOn.Finalizers[i] == addOnFinalizer {
				hasFinalizer = true
				break
			}
		}
		if !hasFinalizer {
			addOn.Finalizers = append(addOn.Finalizers, addOnFinalizer)
			_, err := c.addOnClient.AddonV1alpha1().ManagedClusterAddOns(managedCluster.Name).Update(ctx, addOn, metav1.UpdateOptions{})
			return err
		}
	}

	// submariner-addon is deleting, we remove its related resources
	if !addOn.DeletionTimestamp.IsZero() {
		if err := c.cleanUpSubmarinerAgent(ctx, managedCluster); err != nil {
			return err
		}
		return helpers.RemoveAddOnFinalizer(ctx, c.addOnClient, addOn, addOnFinalizer)
	}

	return c.deploySubmarinerAgent(ctx, clusterSetName, managedCluster, addOn, config)
}

// syncSubmarinerConfig syncs submariner configuration
func (c *submarinerAgentController) syncSubmarinerConfig(ctx context.Context,
	managedCluster *clusterv1.ManagedCluster,
	config *configv1alpha1.SubmarinerConfig) error {
	if config.DeletionTimestamp.IsZero() {
		hasFinalizer := false
		for i := range config.Finalizers {
			if config.Finalizers[i] == submarinerConfigFinalizer {
				hasFinalizer = true
				break
			}
		}
		if !hasFinalizer {
			config.Finalizers = append(config.Finalizers, submarinerConfigFinalizer)
			_, err := c.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(config.Namespace).Update(ctx, config, metav1.UpdateOptions{})
			return err
		}
	}

	// config is deleting, we remove its related resources
	if !config.DeletionTimestamp.IsZero() {
		if err := c.cleanUpSubmarinerClusterEnv(ctx, config); err != nil {
			return err
		}
		return helpers.RemoveConfigFinalizer(ctx, c.configClient, config, submarinerConfigFinalizer)
	}

	if config.Spec.CredentialsSecret == nil {
		// no platform credentials, the submariner cluster environment need not to be prepared
		return nil
	}

	if managedCluster == nil {
		return nil
	}

	managedClusterInfo := helpers.GetManagedClusterInfo(managedCluster)

	// prepare submariner cluster environment
	errs := []error{}
	cloudProvider, preparedErr := cloud.GetCloudProvider(c.kubeClient, c.manifestWorkClient, c.eventRecorder, managedClusterInfo, config)
	if preparedErr == nil {
		preparedErr = cloudProvider.PrepareSubmarinerClusterEnv()
	}

	condition := metav1.Condition{
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

	_, updated, updatedErr := helpers.UpdateSubmarinerConfigStatus(
		c.configClient,
		config.Namespace, config.Name,
		helpers.UpdateSubmarinerConfigStatusFn(condition, managedClusterInfo),
	)
	if updatedErr != nil {
		errs = append(errs, updatedErr)
	}
	if updated {
		c.eventRecorder.Eventf("SubmarinerClusterEnvPrepared", "submariner cluster environment was prepared for manged cluster %s", config.Namespace)
	}

	return operatorhelpers.NewMultiLineAggregate(errs)
}

// clean up the submariner agent from this managedCluster
func (c *submarinerAgentController) cleanUpSubmarinerAgent(ctx context.Context, managedCluster *clusterv1.ManagedCluster) error {
	if err := c.removeSubmarinerAgent(ctx, managedCluster.Name); err != nil {
		return err
	}
	return c.removeAgentFinalizer(ctx, managedCluster)
}

// removeAgentFinalizer removes the agent finalizer from a clusterset
func (c *submarinerAgentController) removeAgentFinalizer(ctx context.Context, managedCluster *clusterv1.ManagedCluster) error {
	copiedFinalizers := []string{}
	for i := range managedCluster.Finalizers {
		if managedCluster.Finalizers[i] == agentFinalizer {
			continue
		}
		copiedFinalizers = append(copiedFinalizers, managedCluster.Finalizers[i])
	}

	if len(managedCluster.Finalizers) != len(copiedFinalizers) {
		managedCluster.Finalizers = copiedFinalizers
		_, err := c.clusterClient.ClusterV1().ManagedClusters().Update(ctx, managedCluster, metav1.UpdateOptions{})
		return err
	}

	return nil
}

func (c *submarinerAgentController) deploySubmarinerAgent(
	ctx context.Context,
	clusterSetName string,
	managedCluster *clusterv1.ManagedCluster,
	managedClusterAddOn *addonv1alpha1.ManagedClusterAddOn,
	submarinerConfig *configv1alpha1.SubmarinerConfig) error {
	// generate service account and bind it to `submariner-k8s-broker-cluster` role
	brokerNamespace := helpers.GernerateBrokerName(clusterSetName)
	if err := c.applyClusterRBACFiles(brokerNamespace, managedCluster.Name); err != nil {
		return err
	}

	// create submariner broker info with submariner config
	brokerInfo, err := brokerinfo.NewSubmarinerBrokerInfo(
		c.kubeClient,
		c.dynamicClient,
		c.configClient,
		c.eventRecorder,
		managedCluster,
		brokerNamespace,
		submarinerConfig,
		managedClusterAddOn,
	)
	if err != nil {
		return fmt.Errorf("failed to create submariner brokerInfo of cluster %v : %v", managedCluster.Name, err)
	}

	// apply submariner operator manifest work
	operatorManifestWork, err := getManifestWork(managedCluster, brokerInfo)
	if err != nil {
		return err
	}
	if err := helpers.ApplyManifestWork(ctx, c.manifestWorkClient, operatorManifestWork, c.eventRecorder); err != nil {
		return err
	}

	return nil
}

func (c *submarinerAgentController) removeSubmarinerAgent(ctx context.Context, clusterName string) error {
	errs := []error{}
	// remove submariner manifestworks
	err := c.manifestWorkClient.WorkV1().ManifestWorks(clusterName).Delete(ctx, manifestWorkName, metav1.DeleteOptions{})
	switch {
	case errors.IsNotFound(err):
		//there is no submariner manifestworks, do noting
	case err == nil:
		c.eventRecorder.Eventf("SubmarinerManifestWorksDeleted", "Deleted manifestwork %q", fmt.Sprintf("%s/%s", clusterName, manifestWorkName))
	case err != nil:
		errs = append(errs, fmt.Errorf("failed to remove submariner agent from managed cluster %v: %v", clusterName, err))
	}
	if err != nil && !errors.IsNotFound(err) {
		errs = append(errs, fmt.Errorf("failed to remove submariner agent from managed cluster %v: %v", clusterName, err))
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
		clientHolder,
		c.eventRecorder,
		func(name string) ([]byte, error) {
			return assets.MustCreateAssetFromTemplate(name, bindata.MustAsset(filepath.Join("", name)), config).Data, nil
		},
		clusterRBACFiles...,
	)
	errs := []error{}
	for _, result := range applyResults {
		if result.Error != nil {
			errs = append(errs, fmt.Errorf("%q (%T): %v", result.File, result.Type, result.Error))
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
			return assets.MustCreateAssetFromTemplate(name, bindata.MustAsset(filepath.Join("", name)), config).Data, nil
		},
		clusterRBACFiles...,
	)
}

func (c *submarinerAgentController) cleanUpSubmarinerClusterEnv(ctx context.Context, config *configv1alpha1.SubmarinerConfig) error {
	// no platform credentials, the submariner cluster environment is not prepared
	if config.Spec.CredentialsSecret == nil {
		return nil
	}

	managedClusterInfo := config.Status.ManagedClusterInfo
	cloudProvider, err := cloud.GetCloudProvider(c.kubeClient, c.manifestWorkClient, c.eventRecorder, managedClusterInfo, config)
	if err != nil {
		//TODO handle the error gracefully in the future
		c.eventRecorder.Warningf("CleanUpSubmarinerClusterEnvFailed", "failed to create cloud provider: %v", err)
		return nil
	}
	if err := cloudProvider.CleanUpSubmarinerClusterEnv(); err != nil {
		//TODO handle the error gracefully in the future
		c.eventRecorder.Warningf("CleanUpSubmarinerClusterEnvFailed", "failed to clean up cloud environment: %v", err)
		return nil
	}
	c.eventRecorder.Eventf("SubmarinerClusterEnvDeleted", "the managed cluster %s submariner cluster environment is deleted", managedClusterInfo.ClusterName)
	return nil
}

func getManifestWork(managedCluster *clusterv1.ManagedCluster, config interface{}) (*workv1.ManifestWork, error) {
	files := []string{agentRBACFile}
	if helpers.GetClusterType(managedCluster) == helpers.ClusterTypeOCP {
		files = append(files, sccFiles...)
	}
	files = append(files, operatorFiles...)

	manifests := []workv1.Manifest{}
	for _, file := range files {
		yamlData := assets.MustCreateAssetFromTemplate(file, bindata.MustAsset(filepath.Join("", file)), config).Data
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
			Name:      manifestWorkName,
			Namespace: managedCluster.Name,
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: manifests,
			},
		},
	}, nil
}
