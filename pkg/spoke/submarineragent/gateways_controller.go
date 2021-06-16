package submarineragent

import (
	"context"
	"fmt"
	"sort"
	"strings"

	addonclient "github.com/open-cluster-management/api/client/addon/clientset/versioned"
	addoninformerv1alpha1 "github.com/open-cluster-management/api/client/addon/informers/externalversions/addon/v1alpha1"
	addonlisterv1alpha1 "github.com/open-cluster-management/api/client/addon/listers/addon/v1alpha1"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1lister "k8s.io/client-go/listers/core/v1"
)

const (
	submarinerGatewayLabel        = "submariner.io/gateway"
	submarinerGatewayNodesLabeled = "SubmarinerGatewayNodesLabeled"
)

// gatewaysStatusController watches the worker nodes on the managed cluster and reports
// whether the nodes are labeled with gateway to the submariner-addon on the hub cluster
type gatewaysStatusController struct {
	addOnClient addonclient.Interface
	addOnLister addonlisterv1alpha1.ManagedClusterAddOnLister
	nodeLister  corev1lister.NodeLister
	clusterName string
}

// NewGatewaysStatusController returns an instance of gatewaysStatusController
func NewGatewaysStatusController(
	clusterName string,
	addOnClient addonclient.Interface,
	addOnInformer addoninformerv1alpha1.ManagedClusterAddOnInformer,
	nodeInformer corev1informers.NodeInformer,
	recorder events.Recorder) factory.Controller {
	c := &gatewaysStatusController{
		addOnClient: addOnClient,
		addOnLister: addOnInformer.Lister(),
		nodeLister:  nodeInformer.Lister(),
		clusterName: clusterName,
	}

	return factory.New().
		WithFilteredEventsInformers(func(obj interface{}) bool {
			metaObj := obj.(metav1.Object)
			// only handle the changes of worker nodes
			if _, has := metaObj.GetLabels()[workerNodeLabel]; has {
				return true
			}
			return false
		}, nodeInformer.Informer()).
		WithSync(c.sync).
		ToController("SubmarinerAgentStatusController", recorder)
}

func (c *gatewaysStatusController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	addOn, err := c.addOnLister.ManagedClusterAddOns(c.clusterName).Get(helpers.SubmarinerAddOnName)
	if errors.IsNotFound(err) {
		// addon is not found, could be deleted, ignore it.
		return nil
	}
	if err != nil {
		return err
	}

	nodes, err := c.nodeLister.List(labels.SelectorFromSet(labels.Set{submarinerGatewayLabel: "true"}))
	if err != nil {
		return err
	}

	gatewayNodeCondtion := metav1.Condition{
		Type:    submarinerGatewayNodesLabeled,
		Status:  metav1.ConditionFalse,
		Reason:  "SubmarinerGatewayNodesUnlabeled",
		Message: fmt.Sprintf("There are no nodes with label %q", submarinerGatewayLabel),
	}

	if len(nodes) != 0 {
		nodeNames := []string{}
		for _, node := range nodes {
			nodeNames = append(nodeNames, node.Name)
		}

		// fixed the order of gateway names
		sort.Strings(nodeNames)

		gatewayNodeCondtion.Status = metav1.ConditionTrue
		gatewayNodeCondtion.Reason = "SubmarinerGatewayNodesLabeled"
		gatewayNodeCondtion.Message = fmt.Sprintf("The nodes %q are labeled with %q", strings.Join(nodeNames, ","), submarinerGatewayLabel)
	}

	// check submariner agent status and update submariner-addon status on the hub cluster
	_, updated, err := helpers.UpdateManagedClusterAddOnStatus(
		ctx,
		c.addOnClient,
		c.clusterName,
		addOn.Name,
		helpers.UpdateManagedClusterAddOnStatusFn(gatewayNodeCondtion),
	)
	if err != nil {
		return err
	}
	if updated {
		syncCtx.Recorder().Eventf("ManagedClusterAddOnStatusUpdated", "update managed cluster addon %q status with gateways status", addOn.Name)
	}

	return nil
}
