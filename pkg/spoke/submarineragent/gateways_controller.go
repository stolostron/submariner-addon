package submarineragent

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/stolostron/submariner-addon/pkg/addon"
	"github.com/submariner-io/admiral/pkg/log"
	"github.com/submariner-io/admiral/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1informers "k8s.io/client-go/informers/core/v1"
	corev1lister "k8s.io/client-go/listers/core/v1"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	submarinerGatewayLabel        = "submariner.io/gateway"
	submarinerGatewayNodesLabeled = "SubmarinerGatewayNodesLabeled"
)

// gatewaysStatusController watches the worker nodes on the managed cluster and reports
// whether the nodes are labeled with gateway to the submariner-addon on the hub cluster.
type gatewaysStatusController struct {
	addOnClient addonclient.Interface
	nodeLister  corev1lister.NodeLister
	clusterName string
	logger      log.Logger
}

// NewGatewaysStatusController returns an instance of gatewaysStatusController.
func NewGatewaysStatusController(
	clusterName string,
	addOnClient addonclient.Interface,
	nodeInformer corev1informers.NodeInformer,
	recorder events.Recorder,
) factory.Controller {
	name := "GatewaysStatusController"
	c := &gatewaysStatusController{
		addOnClient: addOnClient,
		nodeLister:  nodeInformer.Lister(),
		clusterName: clusterName,
		logger:      log.Logger{Logger: logf.Log.WithName(name)},
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
		ToController(name, recorder)
}

func (c *gatewaysStatusController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
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
	updatedStatus, updated, err := addon.UpdateStatus(ctx, c.addOnClient, c.clusterName, addon.UpdateConditionFn(&gatewayNodeCondtion))
	if err != nil {
		return err
	}

	if updated {
		c.logger.Infof("Updated submariner ManagedClusterAddOn status condition: %s", resource.ToJSON(gatewayNodeCondtion))

		syncCtx.Recorder().Eventf("ManagedClusterAddOnStatusUpdated", "Updated status conditions:  %#v",
			updatedStatus.Conditions)
	}

	return nil
}
