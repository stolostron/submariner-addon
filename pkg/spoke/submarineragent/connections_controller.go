package submarineragent

import (
	"context"
	"fmt"
	"strings"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/pkg/errors"
	"github.com/stolostron/submariner-addon/pkg/addon"
	"github.com/submariner-io/admiral/pkg/log"
	"github.com/submariner-io/admiral/pkg/resource"
	submarinerv1alpha1 "github.com/submariner-io/submariner-operator/api/v1alpha1"
	submarinermv1 "github.com/submariner-io/submariner/pkg/apis/submariner.io/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	submarinerConnectionDegraded = "SubmarinerConnectionDegraded"
	routeAgentConnectionDegraded = "RouteAgentConnectionDegraded"
	connectionsDegraded          = "ConnectionsDegraded"
)

// connectionsStatusController watches the status of submariner CR and reflect the status
// to submariner-addon on the hub cluster.
type connectionsStatusController struct {
	addOnClient      addonclient.Interface
	submarinerLister cache.GenericLister
	routeAgentLister cache.GenericLister
	clusterName      string
	logger           log.Logger
}

// NewConnectionsStatusController returns an instance of submarinerAgentStatusController.
func NewConnectionsStatusController(clusterName string, addOnClient addonclient.Interface, submarinerInformer informers.GenericInformer,
	routeAgentInformer informers.GenericInformer, recorder events.Recorder,
) factory.Controller {
	name := "ConnectionsStatusController"
	c := &connectionsStatusController{
		addOnClient:      addOnClient,
		submarinerLister: submarinerInformer.Lister(),
		clusterName:      clusterName,
		routeAgentLister: routeAgentInformer.Lister(),
		logger:           log.Logger{Logger: logf.Log.WithName(name)},
	}

	return factory.New().
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			key, _ := cache.MetaNamespaceKeyFunc(obj)

			return key
		}, submarinerInformer.Informer()).
		WithSync(c.sync).
		ToController(name, recorder)
}

func (c *connectionsStatusController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	namespace, name, _ := cache.SplitMetaNamespaceKey(syncCtx.QueueKey())
	runtimeSubmariner, err := c.submarinerLister.ByNamespace(namespace).Get(name)
	if apierrors.IsNotFound(err) {
		// submariner cr is not found, could be deleted, ignore it.
		return nil
	}

	if err != nil {
		return errors.Wrapf(err, "error retrieving Submariner %q", name)
	}

	submariner := convert(runtimeSubmariner, &submarinerv1alpha1.Submariner{})

	// check submariner agent status and update submariner-addon status on the hub cluster
	gatewaycondition := c.checkSubmarinerConnections(submariner)

	routeAgents, err := c.routeAgentLister.ByNamespace(namespace).List(labels.Everything())
	if err != nil {
		return errors.Wrap(err, "error listing RouteAgents")
	}

	var allUnhealthyMessages []string

	for i := range routeAgents {
		routeAgent := convert(routeAgents[i], &submarinermv1.RouteAgent{})
		allUnhealthyMessages = append(allUnhealthyMessages, c.checkRouteAgentConnections(routeAgent)...)
	}

	routeAgentCondition := &metav1.Condition{
		Type:    routeAgentConnectionDegraded,
		Status:  metav1.ConditionFalse,
		Reason:  "ConnectionsEstablished",
		Message: "All RouteAgent connections to remote endpoints are established and healthy.",
	}

	if len(allUnhealthyMessages) > 0 {
		routeAgentCondition.Status = metav1.ConditionTrue
		routeAgentCondition.Reason = connectionsDegraded
		routeAgentCondition.Message = strings.Join(allUnhealthyMessages, "\n")
	}

	updatedStatus, updated, err := addon.UpdateStatus(ctx, c.addOnClient, c.clusterName, addon.UpdateConditionFn(gatewaycondition),
		addon.UpdateConditionFn(routeAgentCondition))
	if err != nil {
		return err //nolint:wrapcheck // No need to wrap here
	}

	if updated {
		c.logger.Infof("Updated submariner ManagedClusterAddOn status condition: %s", resource.ToJSON(updatedStatus.Conditions))
		syncCtx.Recorder().Eventf("ManagedClusterAddOnStatusUpdated", "Updated status condition: %#v",
			updatedStatus.Conditions)
	}

	return nil
}

func (c *connectionsStatusController) checkSubmarinerConnections(submariner *submarinerv1alpha1.Submariner) *metav1.Condition {
	condition := &metav1.Condition{
		Type: submarinerConnectionDegraded,
	}

	var gateways []submarinermv1.GatewayStatus
	if submariner.Status.Gateways != nil {
		gateways = *submariner.Status.Gateways
	}

	connectedMessages := []string{}
	unconnectedMessages := []string{}
	for i := range gateways {
		gateway := &gateways[i]
		if gateway.HAStatus != submarinermv1.HAStatusActive {
			continue
		}

		for i := range gateway.Connections {
			connection := &gateway.Connections[i]
			if connection.Status != submarinermv1.Connected {
				unconnectedMessages = append(unconnectedMessages,
					fmt.Sprintf("The connection between clusters %q and %q is not established (status=%s)", c.clusterName,
						connection.Endpoint.ClusterID, connection.Status))

				continue
			}

			connectedMessages = append(connectedMessages, fmt.Sprintf("The connection between clusters %q and %q is established",
				c.clusterName, connection.Endpoint.ClusterID))
		}
	}

	if len(connectedMessages) == 0 && len(unconnectedMessages) == 0 {
		condition.Status = metav1.ConditionTrue
		condition.Reason = "ConnectionsNotEstablished"
		condition.Message = "There are no connections on gateways"

		return condition
	}

	if len(unconnectedMessages) != 0 {
		condition.Status = metav1.ConditionTrue
		condition.Reason = connectionsDegraded

		connectedMessages = append(connectedMessages, unconnectedMessages...)
		condition.Message = strings.Join(connectedMessages, "\n")

		return condition
	}

	condition.Status = metav1.ConditionFalse
	condition.Reason = "ConnectionsEstablished"
	condition.Message = strings.Join(connectedMessages, "\n")

	return condition
}

func (c *connectionsStatusController) checkRouteAgentConnections(routeAgent *submarinermv1.RouteAgent) []string {
	var unhealthyMessages []string

	remoteEndpoints := routeAgent.Status.RemoteEndpoints
	for i := range remoteEndpoints {
		if remoteEndpoints[i].Status != submarinermv1.Connected && remoteEndpoints[i].Status != submarinermv1.ConnectionNone {
			unhealthyMessages = append(unhealthyMessages,
				fmt.Sprintf("The RouteAgent connection to remote endpoint %q from %q  is not established (status=%s): %s",
					remoteEndpoints[i].Spec.Hostname, routeAgent.Name, remoteEndpoints[i].Status, remoteEndpoints[i].StatusMessage))
		}
	}

	return unhealthyMessages
}

func convert[T runtime.Object](from runtime.Object, to T) T {
	u, err := runtime.DefaultUnstructuredConverter.ToUnstructured(from)
	utilruntime.Must(err)

	err = runtime.DefaultUnstructuredConverter.FromUnstructured(u, to)
	utilruntime.Must(err)

	return to
}
