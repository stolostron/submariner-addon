package submarineragent

import (
	"context"

	addonclient "github.com/open-cluster-management/api/client/addon/clientset/versioned"
	addoninformerv1alpha1 "github.com/open-cluster-management/api/client/addon/informers/externalversions/addon/v1alpha1"
	addonlisterv1alpha1 "github.com/open-cluster-management/api/client/addon/listers/addon/v1alpha1"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"

	submarinerv1alpha1 "github.com/submariner-io/submariner-operator/apis/submariner/v1alpha1"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/tools/cache"
)

const submarinerCRName = "submariner"

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
		helpers.UpdateManagedClusterAddOnStatusFn(helpers.CheckSubmarinerConnections(c.clusterName, submariner)),
	)
	if err != nil {
		return err
	}
	if updated {
		syncCtx.Recorder().Eventf("ManagedClusterAddOnStatusUpdated", "update managed cluster addon %q status with connections status", addOn.Name)
	}

	return nil
}
