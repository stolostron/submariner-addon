package submarineragent

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"slices"
	"time"

	"github.com/ghodss/yaml"
	"github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/pkg/errors"
	"github.com/stolostron/submariner-addon/pkg/addon"
	"github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig"
	configv1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	configclient "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	configinformer "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/informers/externalversions/submarinerconfig/v1alpha1"
	configlister "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/listers/submarinerconfig/v1alpha1"
	"github.com/stolostron/submariner-addon/pkg/constants"
	brokerinfo "github.com/stolostron/submariner-addon/pkg/hub/submarinerbrokerinfo"
	"github.com/stolostron/submariner-addon/pkg/manifestwork"
	"github.com/stolostron/submariner-addon/pkg/resource"
	"github.com/submariner-io/admiral/pkg/federate"
	"github.com/submariner-io/admiral/pkg/finalizer"
	"github.com/submariner-io/admiral/pkg/log"
	coreresource "github.com/submariner-io/admiral/pkg/resource"
	"github.com/submariner-io/admiral/pkg/util"
	submarinerv1a1 "github.com/submariner-io/submariner-operator/api/v1alpha1"
	"github.com/submariner-io/submariner-operator/pkg/discovery/globalnet"
	submarinerv1 "github.com/submariner-io/submariner/pkg/apis/submariner.io/v1"
	discovery "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	k8serrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/utils/ptr"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"
	addoninformerv1alpha1 "open-cluster-management.io/api/client/addon/informers/externalversions/addon/v1alpha1"
	addonlisterv1alpha1 "open-cluster-management.io/api/client/addon/listers/addon/v1alpha1"
	clusterclient "open-cluster-management.io/api/client/cluster/clientset/versioned"
	clusterinformerv1 "open-cluster-management.io/api/client/cluster/informers/externalversions/cluster/v1"
	clusterinformerv1beta2 "open-cluster-management.io/api/client/cluster/informers/externalversions/cluster/v1beta2"
	clusterlisterv1 "open-cluster-management.io/api/client/cluster/listers/cluster/v1"
	clusterlisterv1beta2 "open-cluster-management.io/api/client/cluster/listers/cluster/v1beta2"
	workclient "open-cluster-management.io/api/client/work/clientset/versioned"
	workinformer "open-cluster-management.io/api/client/work/informers/externalversions/work/v1"
	worklister "open-cluster-management.io/api/client/work/listers/work/v1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	clusterv1beta2 "open-cluster-management.io/api/cluster/v1beta2"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	mcsv1a1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"
)

const (
	serviceAccountLabel           = "cluster.open-cluster-management.io/submariner-cluster-sa"
	OperatorManifestWorkName      = "submariner-operator"
	SubmarinerCRManifestWorkName  = "submariner-resource"
	agentRBACFile                 = "manifests/rbac/operatorgroup-aggregate-clusterrole.yaml"
	submarinerCRFile              = "manifests/operator/submariner.io-submariners-cr.yaml"
	operatorNamespaceFile         = "manifests/operator/submariner-operator-namespace.yaml"
	BrokerCfgApplied              = "SubmarinerBrokerConfigApplied"
	BrokerObjectName              = "submariner-broker"
	BackupLabelKey                = "cluster.open-cluster-management.io/backup"
	BackupLabelValue              = "submariner"
	addonDeploymentConfigResource = "addondeploymentconfigs"
	addonDeploymentConfigGroup    = "addon.open-cluster-management.io"
)

var clusterRBACFiles = []string{
	"manifests/rbac/broker-cluster-serviceaccount.yaml",
	"manifests/rbac/broker-cluster-rolebinding.yaml",
}

var sccFiles = []string{
	"manifests/rbac/scc-aggregate-clusterrole.yaml",
}

var operatorAllFiles = []string{
	"manifests/operator/submariner-operator-group.yaml",
	"manifests/operator/submariner-operator-subscription.yaml",
}

var operatorSkipFiles = []string{
	"manifests/operator/submariner-operator-subscription.yaml",
}

//go:embed manifests
var manifestFiles embed.FS

var BrokerGVR = schema.GroupVersionResource{
	Group:    "submariner.io",
	Version:  "v1alpha1",
	Resource: "brokers",
}

var (
	ManifestWorkDeletionRetryInterval = 15 * time.Second
	ManifestWorkDeletionTimeout       = 2 * time.Minute
)

var logger = log.Logger{Logger: logf.Log.WithName("SubmarinerAgentController")}

type clusterRBACConfig struct {
	ManagedClusterName        string
	SubmarinerBrokerNamespace string
}

// submarinerAgentController reconciles instances of ManagedCluster on the hub to deploy/remove
// corresponding submariner agent manifestworks.
type submarinerAgentController struct {
	kubeClient             kubernetes.Interface
	dynamicClient          dynamic.Interface
	controllerClient       client.Client
	clusterClient          clusterclient.Interface
	manifestWorkClient     workclient.Interface
	configClient           configclient.Interface
	addOnClient            addonclient.Interface
	clusterLister          clusterlisterv1.ManagedClusterLister
	clusterSetLister       clusterlisterv1beta2.ManagedClusterSetLister
	manifestWorkLister     worklister.ManifestWorkLister
	configLister           configlister.SubmarinerConfigLister
	clusterAddOnLister     addonlisterv1alpha1.ClusterManagementAddOnLister
	addOnLister            addonlisterv1alpha1.ManagedClusterAddOnLister
	deploymentConfigLister addonlisterv1alpha1.AddOnDeploymentConfigLister
	eventRecorder          events.Recorder
	resourceCache          resourceapply.ResourceCache
}

// NewSubmarinerAgentController returns a submarinerAgentController instance.
func NewSubmarinerAgentController(
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	controllerClient client.Client,
	clusterClient clusterclient.Interface,
	manifestWorkClient workclient.Interface,
	configClient configclient.Interface,
	addOnClient addonclient.Interface,
	clusterInformer clusterinformerv1.ManagedClusterInformer,
	clusterSetInformer clusterinformerv1beta2.ManagedClusterSetInformer,
	manifestWorkInformer workinformer.ManifestWorkInformer,
	configInformer configinformer.SubmarinerConfigInformer,
	clusterAddOnInformer addoninformerv1alpha1.ClusterManagementAddOnInformer,
	addOnInformer addoninformerv1alpha1.ManagedClusterAddOnInformer,
	deploymentConfigInformer addoninformerv1alpha1.AddOnDeploymentConfigInformer,
	recorder events.Recorder,
) factory.Controller {
	c := &submarinerAgentController{
		kubeClient:             kubeClient,
		dynamicClient:          dynamicClient,
		controllerClient:       controllerClient,
		clusterClient:          clusterClient,
		manifestWorkClient:     manifestWorkClient,
		configClient:           configClient,
		addOnClient:            addOnClient,
		clusterLister:          clusterInformer.Lister(),
		clusterSetLister:       clusterSetInformer.Lister(),
		manifestWorkLister:     manifestWorkInformer.Lister(),
		configLister:           configInformer.Lister(),
		clusterAddOnLister:     clusterAddOnInformer.Lister(),
		addOnLister:            addOnInformer.Lister(),
		deploymentConfigLister: deploymentConfigInformer.Lister(),
		eventRecorder:          recorder.WithComponentSuffix("submariner-agent-controller"),
		resourceCache:          resourceapply.NewResourceCache(),
	}

	return factory.New().
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			accessor, _ := meta.Accessor(obj)
			logger.V(log.DEBUG).Infof("Queuing ManagedCluster %q", accessor.GetName())

			return accessor.GetName()
		}, clusterInformer.Informer()).
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			// TODO: we may consider to use addon to deploy the submariner on the managed cluster instead of
			// using manifestwork, one problem should be considered - how to get the IPSECPSK
			accessor, _ := meta.Accessor(obj)
			if accessor.GetName() != OperatorManifestWorkName && accessor.GetName() != SubmarinerCRManifestWorkName {
				return ""
			}

			logger.V(log.DEBUG).Infof("Queuing ManifestWork \"%s/%s\"", accessor.GetNamespace(), accessor.GetName())

			return accessor.GetNamespace()
		}, manifestWorkInformer.Informer()).
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			// TODO: we may consider to use addon to set up the submariner env on the managed cluster instead of
			// using manifestwork, one problem should be considered - how to get the cloud credentials
			accessor, _ := meta.Accessor(obj)
			if accessor.GetName() != constants.SubmarinerConfigName {
				return ""
			}

			logger.V(log.DEBUG).Infof("Queuing SubmarinerConfig for managed cluster %q", accessor.GetNamespace())

			return accessor.GetNamespace()
		}, configInformer.Informer()).
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			accessor, _ := meta.Accessor(obj)
			if accessor.GetName() != constants.SubmarinerAddOnName {
				return ""
			}

			logger.V(log.DEBUG).Infof("Queuing ManagedClusterAddOn %q for cluster %q", accessor.GetName(), accessor.GetNamespace())

			return accessor.GetNamespace()
		}, addOnInformer.Informer()).
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			accessor, _ := meta.Accessor(obj)
			if accessor.GetName() != constants.SubmarinerAddOnName {
				return ""
			}

			logger.V(log.DEBUG).Infof("Queuing ClusterManagementAddon %q", accessor.GetName())

			return factory.DefaultQueueKey
		}, clusterAddOnInformer.Informer()).
		WithInformers(clusterSetInformer.Informer()).
		WithSync(c.sync).
		ToController("SubmarinerAgentController", recorder)
}

func (c *submarinerAgentController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	key := syncCtx.QueueKey()

	// if the sync is triggered by change of ManagedClusterSet or ClusterManagementAddon, reconcile all managed clusters
	if key == factory.DefaultQueueKey {
		return c.onManagedClusterSetChange(syncCtx)
	}

	clusterName := key

	config, err := c.configLister.SubmarinerConfigs(clusterName).Get(constants.SubmarinerConfigName)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	return c.syncManagedCluster(ctx, clusterName, config, syncCtx)
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
	clusterName string,
	config *configv1alpha1.SubmarinerConfig,
	syncCtx factory.SyncContext,
) error {
	// Find the submariner ManagedClusterAddOn in the managed cluster namespace.
	addOn, err := c.addOnLister.ManagedClusterAddOns(clusterName).Get(constants.SubmarinerAddOnName)

	switch {
	case apierrors.IsNotFound(err):
		// No ManagedClusterAddOn, do nothing.
		return nil
	case err != nil:
		return err
	}

	var clusterSetName string

	// Find the corresponding ManagedCluster.
	managedCluster, err := c.clusterLister.Get(clusterName)

	switch {
	case apierrors.IsNotFound(err):
		managedCluster = nil
	case err != nil:
		return err
	default:
		clusterSetName = managedCluster.Labels[clusterv1beta2.ClusterSetLabel]
	}

	// The ManagedClusterAddOn is deleting, clean up its related resources.
	if !addOn.DeletionTimestamp.IsZero() {
		logger.Infof("ManagedClusterAddOn %q in cluster %q is deleting", addOn.Name, clusterName)

		return c.cleanUpSubmarinerAgent(ctx, clusterName, clusterSetName, syncCtx)
	}

	if managedCluster == nil {
		// ManagedCluster not found, probably deleted, do nothing. This probably shouldn't happen without the ManagedClusterAddOn being
		// deleted first but, if it does, we expect the ManagedClusterAddOn to also be deleted, so we don't do clean up here.
		return nil
	}

	if clusterSetName == "" {
		// We only deploy submariner on managed clusters that are part of only one exclusive cluster set so if it doesn't have the label,
		// we do clean in case submariner was previously deployed.
		logger.Infof("ManagedCluster %q is missing the cluster set label", managedCluster.Name)

		return c.cleanUpSubmarinerAgent(ctx, clusterName, "", syncCtx)
	}

	// Find the ManagedClusterSet containing the managed cluster.
	_, err = c.clusterSetLister.Get(clusterSetName)

	switch {
	case apierrors.IsNotFound(err):
		// ManagedClusterSet not found, probably deleted. We would expect the cluster set label on the ManagedCluster to also be removed,
		// but we'll do clean up here just in case.
		logger.Infof("ManagedClusterSet %q not found", clusterSetName)

		return c.cleanUpSubmarinerAgent(ctx, clusterName, "", syncCtx)
	case err != nil:
		return err
	}

	// Add the finalizer to the ManagedClusterAddOn.
	added, err := finalizer.Add(ctx, resource.ForAddon(c.addOnClient.AddonV1alpha1().ManagedClusterAddOns(clusterName)),
		addOn, constants.SubmarinerAddOnFinalizer)
	if added || err != nil {
		if added {
			logger.Infof("Added finalizer to ManagedClusterAddOn %q in cluster %q", addOn.Name, clusterName)
		}

		return err
	}

	return c.deploySubmarinerAgent(ctx, clusterSetName, managedCluster, addOn, config)
}

// clean up the submariner agent from this managedCluster.
func (c *submarinerAgentController) cleanUpSubmarinerAgent(ctx context.Context, managedClusterName, clusterSetName string,
	syncCtx factory.SyncContext,
) error {
	submarinerManifestWork, err := c.manifestWorkLister.ManifestWorks(managedClusterName).Get(SubmarinerCRManifestWorkName)

	switch {
	case apierrors.IsNotFound(err):
		if err := c.deleteManifestWork(ctx, OperatorManifestWorkName, managedClusterName); err != nil {
			return err
		}
	case err != nil:
		return errors.Wrapf(err, "error retrieving ManifestWork %q", SubmarinerCRManifestWorkName)
	case submarinerManifestWork.DeletionTimestamp.IsZero():
		return c.deleteManifestWork(ctx, SubmarinerCRManifestWorkName, managedClusterName)
	default:
		elapsed := time.Now().UTC().UnixMilli() - submarinerManifestWork.DeletionTimestamp.UTC().UnixMilli()
		if elapsed < ManifestWorkDeletionTimeout.Milliseconds() {
			logger.Infof("ManifestWork %q for cluster %q is still deleting after %v - re-queueing",
				SubmarinerCRManifestWorkName, managedClusterName, time.Millisecond*time.Duration(elapsed))
			syncCtx.Queue().AddAfter(managedClusterName, ManifestWorkDeletionRetryInterval)

			return nil
		}

		logger.Infof("ManifestWork %q for cluster %q did not complete deletion after %v - finishing clean up",
			SubmarinerCRManifestWorkName, managedClusterName, time.Millisecond*time.Duration(elapsed))
	}

	if err := c.deleteClusterBrokerResources(ctx, managedClusterName, clusterSetName); err != nil {
		return err
	}

	// remove service account and its rolebinding from broker namespace
	if err := c.removeClusterRBACFiles(ctx, managedClusterName); err != nil {
		return err
	}

	if err := c.deleteGlobalBrokerResourcesIfNecessary(ctx, clusterSetName); err != nil {
		return err
	}

	addOn, err := c.addOnLister.ManagedClusterAddOns(managedClusterName).Get(constants.SubmarinerAddOnName)
	if err == nil {
		return finalizer.Remove(ctx, resource.ForAddon(c.addOnClient.AddonV1alpha1().ManagedClusterAddOns(managedClusterName)),
			addOn, constants.SubmarinerAddOnFinalizer)
	}

	return nil
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

	err := c.createGNConfigMapIfNecessary(ctx, brokerNamespace)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if apierrors.IsNotFound(err) {
		_ = c.updateManagedClusterAddOnStatus(ctx, managedClusterAddOn, brokerNamespace, true)
		return fmt.Errorf("brokers.submariner.io object named %q missing in namespace %q", BrokerObjectName, brokerNamespace)
	}

	// broker object exists, add backup label if not already present
	err = addBackupLabel(ctx, c.controllerClient, &submarinerv1a1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name:      BrokerObjectName,
			Namespace: brokerNamespace,
		},
	})
	if err != nil {
		return err
	}

	_ = c.updateManagedClusterAddOnStatus(ctx, managedClusterAddOn, brokerNamespace, false)

	// create submariner broker info with submariner config
	brokerInfo, err := brokerinfo.Get(
		ctx,
		c.kubeClient,
		c.dynamicClient,
		c.controllerClient,
		managedCluster.Name,
		brokerNamespace,
		submarinerConfig,
		managedClusterAddOn.Spec.InstallNamespace,
	)
	if err != nil {
		return fmt.Errorf("failed to create submariner brokerInfo of cluster %v : %w", managedCluster.Name, err)
	}

	nodePlacements, err := c.getAddonDeploymentConfigs(managedClusterAddOn)
	if err != nil {
		return err
	}
	for _, nodePlacement := range nodePlacements {
		for k, v := range nodePlacement.NodeSelector {
			brokerInfo.NodeSelector[k] = v
		}

		brokerInfo.Tolerations = append(brokerInfo.Tolerations, nodePlacement.Tolerations...)
	}

	skipOperatorGroup := false

	if submarinerConfig != nil {
		err := c.updateSubmarinerConfigStatus(ctx, submarinerConfig, managedCluster)
		if err != nil {
			return err
		}

		_, skipOperatorGroup = submarinerConfig.GetAnnotations()["skipOperatorGroup"]
	}

	// Apply submariner operator manifest work
	operatorManifestWork, err := newOperatorManifestWork(managedCluster, brokerInfo, skipOperatorGroup)
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

	return manifestwork.Apply(ctx, c.manifestWorkClient, submarinerManifestWork, c.eventRecorder)
}

func (c *submarinerAgentController) updateSubmarinerConfigStatus(ctx context.Context, submarinerConfig *configv1alpha1.SubmarinerConfig,
	managedCluster *clusterv1.ManagedCluster,
) error {
	condition := &metav1.Condition{
		Type:    configv1alpha1.SubmarinerConfigConditionApplied,
		Status:  metav1.ConditionTrue,
		Reason:  "SubmarinerConfigApplied",
		Message: "SubmarinerConfig was applied",
	}

	managedClusterInfo := getManagedClusterInfo(managedCluster)

	// NetworkType is set by spoke cluster, make sure we don't reset it
	if submarinerConfig.Status.ManagedClusterInfo.NetworkType != "" {
		managedClusterInfo.NetworkType = submarinerConfig.Status.ManagedClusterInfo.NetworkType
	}

	_, updated, err := submarinerconfig.UpdateStatus(ctx,
		c.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(submarinerConfig.Namespace), submarinerConfig.Name,
		submarinerconfig.UpdateStatusFn(condition, managedClusterInfo))

	if updated {
		c.eventRecorder.Eventf("SubmarinerConfigApplied", "SubmarinerConfig %q was applied for managed cluster %q",
			submarinerConfig.Name, managedCluster.Name)
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
			BrokerObjectName, brokerNamespace)
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
		logger.Infof("Deleted manifestwork \"%s/%s\"", clusterName, name)
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

	return resource.ApplyManifests(ctx, c.kubeClient, c.eventRecorder, c.resourceCache, resource.AssetFromFile(manifestFiles, config),
		clusterRBACFiles...)
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

	// Delete created secret if present
	brokerNamespace := serviceAccounts.Items[0].Namespace
	secretName := brokerinfo.GenerateBrokerName(managedClusterName)
	err = c.kubeClient.CoreV1().Secrets(brokerNamespace).Delete(ctx, secretName, metav1.DeleteOptions{})

	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	config := &clusterRBACConfig{
		ManagedClusterName:        managedClusterName,
		SubmarinerBrokerNamespace: serviceAccounts.Items[0].Namespace,
	}

	return resource.DeleteFromManifests(ctx, c.kubeClient, c.eventRecorder, resource.AssetFromFile(manifestFiles, config),
		clusterRBACFiles...)
}

func newSubmarinerManifestWork(managedCluster *clusterv1.ManagedCluster, config interface{}) (*workv1.ManifestWork, error) {
	return newManifestWork(SubmarinerCRManifestWorkName, managedCluster.Name, config, submarinerCRFile)
}

func newOperatorManifestWork(managedCluster *clusterv1.ManagedCluster, config interface{}, skipOperatorGroup bool,
) (*workv1.ManifestWork, error) {
	files := []string{operatorNamespaceFile, agentRBACFile}
	clusterProduct := getClusterProduct(managedCluster)
	if clusterProduct == constants.ProductOCP || clusterProduct == constants.ProductROSA ||
		clusterProduct == constants.ProductARO || clusterProduct == constants.ProductROKS ||
		clusterProduct == constants.ProductOSD {
		files = append(files, sccFiles...)
	}

	if skipOperatorGroup {
		files = append(files, operatorSkipFiles...)
	} else {
		files = append(files, operatorAllFiles...)
	}

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

func getManagedClusterInfo(managedCluster *clusterv1.ManagedCluster) *configv1alpha1.ManagedClusterInfo {
	clusterInfo := &configv1alpha1.ManagedClusterInfo{
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

func (c *submarinerAgentController) createGNConfigMapIfNecessary(ctx context.Context, brokerNamespace string) error {
	gmConfigMap, gnCmErr := globalnet.GetConfigMap(ctx, c.controllerClient, brokerNamespace)
	if gnCmErr != nil && !apierrors.IsNotFound(gnCmErr) {
		return errors.Wrapf(gnCmErr, "error getting globalnet configmap from broker namespace %q", brokerNamespace)
	}

	if gnCmErr == nil {
		// This should handle upgrade from a version that didn't add the label
		return addBackupLabel(ctx, c.controllerClient, gmConfigMap)
	}

	// globalnetConfig is missing in the broker-namespace, try creating it from submariner-broker object.

	brokerObj, err := c.getBrokerObject(ctx, brokerNamespace)
	if err != nil {
		return err
	}

	if brokerObj.Spec.GlobalnetEnabled {
		logger.Infof("Globalnet is enabled in the managedClusterSet namespace %q", brokerNamespace)

		if brokerObj.Spec.DefaultGlobalnetClusterSize == 0 {
			brokerObj.Spec.DefaultGlobalnetClusterSize = globalnet.DefaultGlobalnetClusterSize
		}

		if brokerObj.Spec.GlobalnetCIDRRange == "" {
			brokerObj.Spec.GlobalnetCIDRRange = globalnet.DefaultGlobalnetCIDR
		}
	} else {
		logger.Infof("Globalnet is disabled in the managedClusterSet namespace %q", brokerNamespace)
	}

	configMap, err := globalnet.NewGlobalnetConfigMap(brokerObj.Spec.GlobalnetEnabled,
		brokerObj.Spec.GlobalnetCIDRRange, brokerObj.Spec.DefaultGlobalnetClusterSize, brokerNamespace)
	if err == nil {
		configMap.Labels[BackupLabelKey] = BackupLabelValue
		err = c.controllerClient.Create(ctx, configMap)
	}

	return errors.Wrapf(err, "error creating globalnet configmap on Broker")
}

func (c *submarinerAgentController) deleteClusterBrokerResources(ctx context.Context, clusterName, clusterSetName string) error {
	if clusterSetName == "" {
		return nil
	}

	brokerNamespace := brokerinfo.GenerateBrokerName(clusterSetName)

	//nolint:prealloc // No need to pre-allocate since normally there won't be any errors.
	var errs []error

	deleteCollection := func(gvr schema.GroupVersionResource) {
		err := c.dynamicClient.Resource(gvr).Namespace(brokerNamespace).DeleteCollection(ctx, metav1.DeleteOptions{},
			metav1.ListOptions{
				LabelSelector: labels.Set(map[string]string{federate.ClusterIDLabelKey: clusterName}).String(),
			})
		if !apierrors.IsNotFound(err) {
			errs = append(errs, err)
		}
	}

	deleteCollection(submarinerv1.EndpointGVR)
	deleteCollection(submarinerv1.ClusterGVR)
	deleteCollection(discovery.SchemeGroupVersion.WithResource("endpointslices"))

	serviceImportClient := c.dynamicClient.Resource(schema.GroupVersionResource{
		Group:    mcsv1a1.GroupName,
		Version:  mcsv1a1.GroupVersion.Version,
		Resource: "serviceimports",
	}).Namespace(brokerNamespace)

	listServiceImports := func() []unstructured.Unstructured {
		siList, err := serviceImportClient.List(ctx, metav1.ListOptions{})
		errs = append(errs, err)

		if err != nil {
			return nil
		}

		return siList.Items
	}

	siList := listServiceImports()
	for i := range siList {
		err := util.Update(ctx, coreresource.ForDynamic(serviceImportClient), &siList[i],
			func(obj *unstructured.Unstructured) (*unstructured.Unstructured, error) {
				existing := coreresource.MustFromUnstructured(obj, &mcsv1a1.ServiceImport{})

				existing.Status.Clusters = slices.DeleteFunc(existing.Status.Clusters, func(s mcsv1a1.ClusterStatus) bool {
					return s.Cluster == clusterName
				})

				if len(existing.Status.Clusters) == 0 {
					err := serviceImportClient.Delete(ctx, existing.Name, metav1.DeleteOptions{
						Preconditions: &metav1.Preconditions{
							ResourceVersion: ptr.To(existing.ResourceVersion),
						},
					})
					if apierrors.IsNotFound(err) {
						err = nil
					}

					return obj, errors.Wrapf(err, "error deleting aggregated ServiceImport %q", existing.Name)
				}

				return coreresource.MustToUnstructured(existing), nil
			})
		errs = append(errs, err)
	}

	return errors.Wrapf(k8serrors.NewAggregate(errs), "error deleting broker resources for cluster %q", clusterName)
}

func (c *submarinerAgentController) deleteGlobalBrokerResourcesIfNecessary(ctx context.Context, clusterSetName string) error {
	if clusterSetName == "" {
		return nil
	}

	clusters, err := c.clusterLister.List(labels.SelectorFromSet(labels.Set{clusterv1beta2.ClusterSetLabel: clusterSetName}))
	if err != nil {
		return err
	}

	for _, cluster := range clusters {
		addOn, err := c.addOnLister.ManagedClusterAddOns(cluster.Name).Get(constants.SubmarinerAddOnName)
		if apierrors.IsNotFound(err) {
			continue
		}

		if err != nil {
			return err
		}

		if addOn.DeletionTimestamp.IsZero() {
			return nil
		}
	}

	brokerNamespace := brokerinfo.GenerateBrokerName(clusterSetName)

	logger.Infof("Deleting Globalnet ConfigMap and Broker resources from broker namespace %q in cluster set %q",
		brokerNamespace, clusterSetName)

	err = globalnet.DeleteConfigMap(ctx, c.controllerClient, brokerNamespace)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	err = c.controllerClient.Delete(ctx, &submarinerv1a1.Broker{ObjectMeta: metav1.ObjectMeta{
		Name:      BrokerObjectName,
		Namespace: brokerNamespace,
	}})
	if apierrors.IsNotFound(err) {
		err = nil
	}

	return err
}

func (c *submarinerAgentController) getBrokerObject(ctx context.Context, brokerNamespace string) (*submarinerv1a1.Broker, error) {
	broker := &submarinerv1a1.Broker{}

	err := c.controllerClient.Get(ctx, types.NamespacedName{Namespace: brokerNamespace, Name: BrokerObjectName}, broker)
	if err != nil {
		return nil, errors.Wrapf(err, "error getting broker object from namespace %q", brokerNamespace)
	}

	return broker, nil
}

func addBackupLabel[T client.Object](ctx context.Context, cl client.Client, to T) error {
	if to.GetLabels() != nil {
		if _, ok := to.GetLabels()[BackupLabelKey]; ok {
			return nil
		}
	}

	return errors.Wrapf(util.Update[T](ctx, coreresource.ForControllerClient[T](cl, to.GetNamespace(), to), to,
		func(obj T) (T, error) {
			existing := coreresource.MustToMeta(obj)

			existingLabels := existing.GetLabels()
			if existingLabels == nil {
				existingLabels = make(map[string]string)
			}

			if _, ok := existingLabels[BackupLabelKey]; !ok {
				existingLabels[BackupLabelKey] = BackupLabelValue
				existing.SetLabels(existingLabels)

				logger.Infof("Added backup label to %T \"%s/%s\"", to, to.GetNamespace(), to.GetName())
			}

			return obj, nil
		}), "error adding backup label to %T \"%s/%s\"", to, to.GetNamespace(), to.GetName())
}

func (c *submarinerAgentController) getAddonDeploymentConfigs(managedClusterAddon *addonv1alpha1.ManagedClusterAddOn) (
	[]*addonv1alpha1.NodePlacement, error,
) {
	var nodePlacements []*addonv1alpha1.NodePlacement

	for _, config := range managedClusterAddon.Spec.Configs {
		if config.Resource == addonDeploymentConfigResource && config.Group == addonDeploymentConfigGroup {
			deploymentConfig, err := c.deploymentConfigLister.AddOnDeploymentConfigs(config.Namespace).Get(config.Name)
			if err != nil {
				return nil, errors.Wrapf(err, "error getting AddonDeploymentConfig \"%s/%s\"", config.Namespace, config.Name)
			}

			nodePlacements = append(nodePlacements, deploymentConfig.Spec.NodePlacement)
		}
	}

	if len(nodePlacements) > 0 {
		return nodePlacements, nil
	}

	/* No deployment config on managedclusteraddon, check default
	   Ideally, we should get this from managedclusteraddon.status.configreferences but we don't for 2 reasons:
	     1. There can be race condition between MCH controller adding addondeploymentconfig to managedclusteraddon.status
	        vs when we read it, unless we watch for managedclusteraddon.status updates.
	     2. We update managedclusteraddon.status. Not a good pattern to watch for updates on something we're
	        updating as it will trigger extra cycles of update.
	   Revisit this after some more testing.
	*/

	clusterAddOn, err := c.clusterAddOnLister.Get(constants.SubmarinerAddOnName)

	if apierrors.IsNotFound(err) {
		return nodePlacements, nil
	} else if err != nil {
		return nil, errors.Wrapf(err, "error getting ClusterManagementAddon %q", constants.SubmarinerAddOnName)
	}

	for _, config := range clusterAddOn.Spec.SupportedConfigs {
		if config.Resource == addonDeploymentConfigResource && config.Group == addonDeploymentConfigGroup &&
			config.DefaultConfig != nil {
			name, namespace := config.DefaultConfig.Name, config.DefaultConfig.Namespace
			deploymentConfig, err := c.deploymentConfigLister.AddOnDeploymentConfigs(namespace).Get(name)
			if err != nil {
				return nil, errors.Wrapf(err, "error getting AddonDeploymentConfig %q:%q", namespace, name)
			}

			nodePlacements = append(nodePlacements, deploymentConfig.Spec.NodePlacement)
		}
	}

	return nodePlacements, nil
}
