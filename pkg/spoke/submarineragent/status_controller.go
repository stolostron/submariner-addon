package submarineragent

import (
	"context"
	"fmt"
	"strings"

	addonclient "github.com/open-cluster-management/api/client/addon/clientset/versioned"
	addoninformerv1alpha1 "github.com/open-cluster-management/api/client/addon/informers/externalversions/addon/v1alpha1"
	addonlisterv1alpha1 "github.com/open-cluster-management/api/client/addon/listers/addon/v1alpha1"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"

	submarinerv1alpha1 "github.com/submariner-io/submariner-operator/apis/submariner/v1alpha1"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	SubmarinerAddOnName         = "submariner-addon"
	SubmarinerOperatorNamespace = "submariner-operator"
)

const (
	submarinerGatewayNodesLabeled = "SubmarinerGatewayNodesLabeled"
	submarinerGatewayLabel        = "submariner.io/gateway"
	submarinerCRName              = "submariner"
)

// submarinerAgentStatusController watches the status of submariner CR and reflect the status
// to submariner-addon on the hub cluster
type submarinerAgentStatusController struct {
	addOnClient      addonclient.Interface
	addOnLister      addonlisterv1alpha1.ManagedClusterAddOnLister
	nodeLister       corev1lister.NodeLister
	submarinerLister cache.GenericLister
	clusterName      string
}

// NewSubmarinerAgentStatusController returns an instance of submarinerAgentStatusController
func NewSubmarinerAgentStatusController(
	clusterName string,
	addOnClient addonclient.Interface,
	addOnInformer addoninformerv1alpha1.ManagedClusterAddOnInformer,
	nodeInformer corev1informers.NodeInformer,
	submarinerInformer informers.GenericInformer,
	recorder events.Recorder) factory.Controller {
	c := &submarinerAgentStatusController{
		addOnClient:      addOnClient,
		addOnLister:      addOnInformer.Lister(),
		nodeLister:       nodeInformer.Lister(),
		submarinerLister: submarinerInformer.Lister(),
		clusterName:      clusterName,
	}

	return factory.New().
		WithInformers(submarinerInformer.Informer()).
		WithSync(c.sync).
		ToController("SubmarinerAgentStatusController", recorder)
}

func (c *submarinerAgentStatusController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	addOn, err := c.addOnLister.ManagedClusterAddOns(c.clusterName).Get(SubmarinerAddOnName)
	if errors.IsNotFound(err) {
		// addon is not found, could be deleted, ignore it.
		return nil
	}

	runtimeSubmariner, err := c.submarinerLister.ByNamespace(SubmarinerOperatorNamespace).Get(submarinerCRName)
	if errors.IsNotFound(err) {
		// submariner cr is not found, could be deleted, ignore it.
		return nil
	}
	if err != nil {
		return err
	}

	unstructuredSubmariner, err := runtime.DefaultUnstructuredConverter.ToUnstructured(runtimeSubmariner)
	if err != nil {
		return err
	}

	submariner := &submarinerv1alpha1.Submariner{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredSubmariner, &submariner); err != nil {
		return err
	}

	// check submariner agent status and update submariner-addon status on the hub cluster
	_, updated, err := helpers.UpdateManagedClusterAddOnStatus(
		ctx,
		c.addOnClient,
		c.clusterName,
		addOn.Name,
		helpers.UpdateManagedClusterAddOnStatusFn(c.checkGatewayNodes()),
		helpers.UpdateManagedClusterAddOnStatusFn(helpers.CheckSubmarinerDaemonSetsStatus(submariner)),
		helpers.UpdateManagedClusterAddOnStatusFn(helpers.CheckSubmarinerConnections(c.clusterName, submariner)),
	)
	if err != nil {
		return err
	}
	if updated {
		syncCtx.Recorder().Eventf("ManagedClusterAddOnStatusUpdated", "update managed cluster addon %q status", addOn.Name)
	}

	return nil
}

func (c *submarinerAgentStatusController) checkGatewayNodes() metav1.Condition {
	nodes, err := c.nodeLister.List(labels.SelectorFromSet(labels.Set{submarinerGatewayLabel: "true"}))
	if err != nil || len(nodes) == 0 {
		return metav1.Condition{
			Type:    submarinerGatewayNodesLabeled,
			Status:  metav1.ConditionFalse,
			Reason:  "SubmarinerGatewayNodesUnlabeled",
			Message: fmt.Sprintf("There are no nodes with label %q", submarinerGatewayLabel),
		}
	}

	nodeNames := []string{}
	for _, node := range nodes {
		nodeNames = append(nodeNames, node.Name)
	}
	return metav1.Condition{
		Type:    submarinerGatewayNodesLabeled,
		Status:  metav1.ConditionTrue,
		Reason:  "SubmarinerGatewayNodesLabeled",
		Message: fmt.Sprintf("The nodes %q are labeled with %q", strings.Join(nodeNames, ","), submarinerGatewayLabel),
	}
}
