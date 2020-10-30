package submarineragent

import (
	"context"
	"fmt"
	"path/filepath"

	clientset "github.com/open-cluster-management/api/client/cluster/clientset/versioned"
	clusterinformerv1 "github.com/open-cluster-management/api/client/cluster/informers/externalversions/cluster/v1"
	clusterinformerv1alpha1 "github.com/open-cluster-management/api/client/cluster/informers/externalversions/cluster/v1alpha1"
	clusterlisterv1 "github.com/open-cluster-management/api/client/cluster/listers/cluster/v1"
	clusterlisterv1alpha1 "github.com/open-cluster-management/api/client/cluster/listers/cluster/v1alpha1"
	workv1client "github.com/open-cluster-management/api/client/work/clientset/versioned"
	workinformer "github.com/open-cluster-management/api/client/work/informers/externalversions/work/v1"
	worklister "github.com/open-cluster-management/api/client/work/listers/work/v1"
	clusterv1 "github.com/open-cluster-management/api/cluster/v1"
	"k8s.io/client-go/dynamic"

	"github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	operatorhelpers "github.com/openshift/library-go/pkg/operator/v1helpers"

	"github.com/open-cluster-management/submariner-addon/pkg/helpers"
	"github.com/open-cluster-management/submariner-addon/pkg/hub/submarineragent/bindata"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

const (
	agentFinalizer      = "cluster.open-cluster-management.io/submariner-agent-cleanup"
	serviceAccountLabel = "cluster.open-cluster-management.io/submariner-cluster-sa"
	submarinerLabel     = "cluster.open-cluster-management.io/submariner-agent"
	clusterSetLabel     = "cluster.open-cluster-management.io/clusterset"
)

var clusterRBACFiles = []string{
	"manifests/agent/rbac/submariner-cluster-serviceaccount.yaml",
	"manifests/agent/rbac/submariner-cluster-rolebinding.yaml",
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
	clusterClient      clientset.Interface
	manifestWorkClient workv1client.Interface
	clusterLister      clusterlisterv1.ManagedClusterLister
	clusterSetLister   clusterlisterv1alpha1.ManagedClusterSetLister
	manifestWorkLister worklister.ManifestWorkLister
	eventRecorder      events.Recorder
}

// NewSubmarinerAgentController returns a submarinerAgentController instance
func NewSubmarinerAgentController(
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	clusterClient clientset.Interface,
	manifestWorkClient workv1client.Interface,
	clusterInformer clusterinformerv1.ManagedClusterInformer,
	clusterSetInformer clusterinformerv1alpha1.ManagedClusterSetInformer,
	manifestWorkInformer workinformer.ManifestWorkInformer,
	recorder events.Recorder) factory.Controller {
	c := &submarinerAgentController{
		kubeClient:         kubeClient,
		dynamicClient:      dynamicClient,
		clusterClient:      clusterClient,
		manifestWorkClient: manifestWorkClient,
		clusterLister:      clusterInformer.Lister(),
		clusterSetLister:   clusterSetInformer.Lister(),
		manifestWorkLister: manifestWorkInformer.Lister(),
		eventRecorder:      recorder.WithComponentSuffix("submariner-agent-controller"),
	}
	return factory.New().
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			accessor, _ := meta.Accessor(obj)
			return accessor.GetName()
		}, clusterInformer.Informer()).
		WithInformers(clusterSetInformer.Informer()).
		WithInformers(manifestWorkInformer.Informer()).
		WithSync(c.sync).
		ToController("SubmarinerAgentController", recorder)
}

func (c *submarinerAgentController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	key := syncCtx.QueueKey()

	// if the sync is triggered by change of ManagedClusterSet or ManifestWork, reconcile all managed clusters
	if key == "key" {
		if err := c.syncAllManagedClusters(ctx); err != nil {
			return err
		}
		return nil
	}

	managedCluster, err := c.clusterLister.Get(key)
	if errors.IsNotFound(err) {
		// managed cluster not found, could have been deleted, do nothing.
		return nil
	}
	if err != nil {
		return err
	}

	if err := c.syncManagedCluster(ctx, managedCluster); err != nil {
		return err
	}

	return nil
}

// syncAllManagedClusters syncs all managed clusters
func (c *submarinerAgentController) syncAllManagedClusters(ctx context.Context) error {
	managedClusters, err := c.clusterLister.List(labels.Everything())
	if err != nil {
		return err
	}

	errs := []error{}
	for _, managedCluster := range managedClusters {
		if err = c.syncManagedCluster(ctx, managedCluster); err != nil {
			errs = append(errs, err)
		}
	}

	return operatorhelpers.NewMultiLineAggregate(errs)
}

// syncManagedCluster syncs one managed cluster
func (c *submarinerAgentController) syncManagedCluster(ctx context.Context, managedCluster *clusterv1.ManagedCluster) error {
	// the cluster does not have the submariner label, ignore it
	if _, existed := managedCluster.Labels[submarinerLabel]; !existed {
		return c.removeSubmarinerAgent(ctx, managedCluster.Name)
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
		// remove the submariner agent from this managedCluster
		if err := c.removeSubmarinerAgent(ctx, managedCluster.Name); err != nil {
			return err
		}
		return c.removeAgentFinalizer(ctx, managedCluster)
	}

	// find the clustersets that contains this managed cluster
	clusterSetName, existed := managedCluster.Labels[clusterSetLabel]
	if !existed {
		return c.removeSubmarinerAgent(ctx, managedCluster.Name)
	}
	_, err := c.clusterClient.ClusterV1alpha1().ManagedClusterSets().Get(ctx, clusterSetName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			c.eventRecorder.Warning("SubmarinerUndeployed",
				fmt.Sprintf("There are no managedClusterSets %q related to managedCluster %q", clusterSetName, managedCluster.Name))
			return c.removeSubmarinerAgent(ctx, managedCluster.Name)
		}
		return err
	}

	return c.deploySubmarinerAgent(ctx, clusterSetName, managedCluster.Name)
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

func (c *submarinerAgentController) deploySubmarinerAgent(ctx context.Context, clusterSetName, clusterName string) error {
	// generate service account and bind it to `submariner-k8s-broker-cluster` role
	brokerNamespace := fmt.Sprintf("submariner-clusterset-%s-broker", clusterSetName)
	if err := c.applyClusterRBACFiles(brokerNamespace, clusterName); err != nil {
		return err
	}

	if err := ApplySubmarinerManifestWorks(
		c.kubeClient,
		c.dynamicClient,
		c.manifestWorkClient,
		c.clusterClient,
		clusterName, brokerNamespace, ctx); err != nil {
		c.eventRecorder.Warning("SubmarinerAgentDeployedFailed",
			fmt.Sprintf("failed to deploy submariner agent on managed cluster %v: %v", clusterName, err))
		return err
	}

	c.eventRecorder.Event("SubmarinerAgentDeployed", fmt.Sprintf("submariner agent was deployed on managed cluster %q", clusterName))
	return nil
}

func (c *submarinerAgentController) removeSubmarinerAgent(ctx context.Context, clusterName string) error {
	errs := []error{}
	if err := RemoveSubmarinerManifestWorks(clusterName, c.manifestWorkClient, c.clusterClient, ctx); err != nil {
		errs = append(errs, fmt.Errorf("failed to remove submariner agent from managed cluster %v: %v", clusterName, err))
	}

	// remove service account and its rolebinding from broker namespace
	if err := c.removeClusterRBACFiles(clusterName, ctx); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		c.eventRecorder.Event("SubmarinerAgentRemoved", fmt.Sprintf("there is no submariner agent or submariner agent was removed from managed cluster %q", clusterName))
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

func (c *submarinerAgentController) removeClusterRBACFiles(managedClusterName string, ctx context.Context) error {
	serviceAccounts, err := c.kubeClient.CoreV1().ServiceAccounts(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", serviceAccountLabel, managedClusterName),
	})
	if err != nil {
		return err
	}
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
		context.TODO(),
		c.kubeClient,
		c.eventRecorder,
		func(name string) ([]byte, error) {
			return assets.MustCreateAssetFromTemplate(name, bindata.MustAsset(filepath.Join("", name)), config).Data, nil
		},
		clusterRBACFiles...,
	)
}
