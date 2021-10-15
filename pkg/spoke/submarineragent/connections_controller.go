package submarineragent

import (
	"context"
	"fmt"
	"strings"

	"github.com/open-cluster-management/submariner-addon/pkg/helpers"
	submarinerv1alpha1 "github.com/submariner-io/submariner-operator/apis/submariner/v1alpha1"
	submarinermv1 "github.com/submariner-io/submariner/pkg/apis/submariner.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

const (
	submarinerConnectionDegraded = "SubmarinerConnectionDegraded"
)

// connectionsStatusController watches the status of submariner CR and reflect the status
// to submariner-addon on the hub cluster
type connectionsStatusController struct {
	addOnClient      addonclient.Interface
	submarinerLister cache.GenericLister
	clusterName      string
}

// NewConnectionsStatusController returns an instance of submarinerAgentStatusController
func NewConnectionsStatusController(clusterName string, addOnClient addonclient.Interface, submarinerInformer informers.GenericInformer,
	recorder events.Recorder) factory.Controller {
	c := &connectionsStatusController{
		addOnClient:      addOnClient,
		submarinerLister: submarinerInformer.Lister(),
		clusterName:      clusterName,
	}

	return factory.New().
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			key, _ := cache.MetaNamespaceKeyFunc(obj)

			return key
		}, submarinerInformer.Informer()).
		WithSync(c.sync).
		ToController("SubmarinerConnectionsStatusController", recorder)
}

func (c *connectionsStatusController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	namespace, name, _ := cache.SplitMetaNamespaceKey(syncCtx.QueueKey())
	runtimeSubmariner, err := c.submarinerLister.ByNamespace(namespace).Get(name)
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
	updatedStatus, updated, err := helpers.UpdateManagedClusterAddOnStatus(ctx, c.addOnClient, c.clusterName,
		helpers.UpdateManagedClusterAddOnStatusFn(c.checkSubmarinerConnections(submariner)))
	if err != nil {
		return err
	}

	if updated {
		syncCtx.Recorder().Eventf("ManagedClusterAddOnStatusUpdated", "Updated status conditions:  %#v",
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
		condition.Reason = "ConnectionsDegraded"

		connectedMessages = append(connectedMessages, unconnectedMessages...)
		condition.Message = strings.Join(connectedMessages, "\n")

		return condition
	}

	condition.Status = metav1.ConditionFalse
	condition.Reason = "ConnectionsEstablished"
	condition.Message = strings.Join(connectedMessages, "\n")

	return condition
}
