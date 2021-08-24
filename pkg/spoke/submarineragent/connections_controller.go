package submarineragent

import (
	"context"
	"fmt"
	"strings"

	"github.com/open-cluster-management/submariner-addon/pkg/helpers"
	submarinermv1 "github.com/submariner-io/submariner/pkg/apis/submariner.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"
	addoninformerv1alpha1 "open-cluster-management.io/api/client/addon/informers/externalversions/addon/v1alpha1"
	addonlisterv1alpha1 "open-cluster-management.io/api/client/addon/listers/addon/v1alpha1"

	submarinerv1alpha1 "github.com/submariner-io/submariner-operator/apis/submariner/v1alpha1"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

const (
	submarinerCRName             = "submariner"
	submarinerConnectionDegraded = "SubmarinerConnectionDegraded"
)

// connectionsStatusController watches the status of submariner CR and reflect the status
// to submariner-addon on the hub cluster
type connectionsStatusController struct {
	addOnClient           addonclient.Interface
	addOnLister           addonlisterv1alpha1.ManagedClusterAddOnLister
	submarinerLister      cache.GenericLister
	clusterName           string
	installationNamespace string
}

// NewConnectionsStatusController returns an instance of submarinerAgentStatusController
func NewConnectionsStatusController(
	clusterName string,
	installationNamespace string,
	addOnClient addonclient.Interface,
	addOnInformer addoninformerv1alpha1.ManagedClusterAddOnInformer,
	submarinerInformer informers.GenericInformer,
	recorder events.Recorder) factory.Controller {
	c := &connectionsStatusController{
		addOnClient:           addOnClient,
		addOnLister:           addOnInformer.Lister(),
		submarinerLister:      submarinerInformer.Lister(),
		clusterName:           clusterName,
		installationNamespace: installationNamespace,
	}

	return factory.New().
		WithInformers(submarinerInformer.Informer()).
		WithSync(c.sync).
		ToController("SubmarinerConnectionsStatusController", recorder)
}

func (c *connectionsStatusController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	addOn, err := c.addOnLister.ManagedClusterAddOns(c.clusterName).Get(helpers.SubmarinerAddOnName)
	if errors.IsNotFound(err) {
		// addon is not found, could be deleted, ignore it.
		return nil
	}
	if err != nil {
		return err
	}

	runtimeSubmariner, err := c.submarinerLister.ByNamespace(c.installationNamespace).Get(submarinerCRName)
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
		helpers.UpdateManagedClusterAddOnStatusFn(c.checkSubmarinerConnections(submariner)),
	)
	if err != nil {
		return err
	}

	if updated {
		syncCtx.Recorder().Eventf("ManagedClusterAddOnStatusUpdated", "update managed cluster addon %q status with connections status", addOn.Name)
	}

	return nil
}

func (c *connectionsStatusController) checkSubmarinerConnections(submariner *submarinerv1alpha1.Submariner) metav1.Condition {
	condition := metav1.Condition{
		Type: submarinerConnectionDegraded,
	}

	var gateways []submarinermv1.GatewayStatus
	if submariner.Status.Gateways != nil {
		gateways = *submariner.Status.Gateways
	}

	connectedMessages := []string{}
	unconnectedMessages := []string{}
	for _, gateway := range gateways {
		for _, connection := range gateway.Connections {
			if connection.Status != submarinermv1.Connected {
				unconnectedMessages = append(unconnectedMessages, fmt.Sprintf("The connection between clusters %q and %q is not established (status=%s)",
					c.clusterName, connection.Endpoint.ClusterID, connection.Status))
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
