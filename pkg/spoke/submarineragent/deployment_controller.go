package submarineragent

import (
	"context"
	"fmt"
	"strings"

	addonclient "github.com/open-cluster-management/api/client/addon/clientset/versioned"
	addoninformerv1alpha1 "github.com/open-cluster-management/api/client/addon/informers/externalversions/addon/v1alpha1"
	addonlisterv1alpha1 "github.com/open-cluster-management/api/client/addon/listers/addon/v1alpha1"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	appsv1lister "k8s.io/client-go/listers/apps/v1"
)

const (
	operatorName   = "submariner-operator"
	gatewayName    = "submariner-gateway"
	routeAgentName = "submariner-routeagent"
)

const submarinerAgentDegraded = "SubmarinerAgentDegraded"

// deploymentStatusController watches the status of submariner-operator deployment and submariner daemonsets
// on the managed cluster and reports the status to the submariner-addon on the hub cluster
type deploymentStatusController struct {
	addOnClient      addonclient.Interface
	addOnLister      addonlisterv1alpha1.ManagedClusterAddOnLister
	daemonSetLister  appsv1lister.DaemonSetLister
	deploymentLister appsv1lister.DeploymentLister
	clusterName      string
	namespace        string
}

// NewDeploymentStatusController returns an instance of deploymentStatusController
func NewDeploymentStatusController(
	clusterName string,
	installationNamespace string,
	addOnClient addonclient.Interface,
	addOnInformer addoninformerv1alpha1.ManagedClusterAddOnInformer,
	daemonsetInformer appsv1informers.DaemonSetInformer,
	deploymentInformer appsv1informers.DeploymentInformer,
	recorder events.Recorder) factory.Controller {
	c := &deploymentStatusController{
		addOnClient:      addOnClient,
		addOnLister:      addOnInformer.Lister(),
		daemonSetLister:  daemonsetInformer.Lister(),
		deploymentLister: deploymentInformer.Lister(),
		clusterName:      clusterName,
		namespace:        installationNamespace,
	}

	return factory.New().
		WithInformers(daemonsetInformer.Informer(), deploymentInformer.Informer()).
		WithSync(c.sync).
		ToController("SubmarinerAgentStatusController", recorder)
}

func (c *deploymentStatusController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	addOn, err := c.addOnLister.ManagedClusterAddOns(c.clusterName).Get(helpers.SubmarinerAddOnName)
	if errors.IsNotFound(err) {
		// addon is not found, could be deleted, ignore it.
		return nil
	}
	if err != nil {
		return err
	}

	degradedConditionReasons := []string{}
	degradedConditionMessages := []string{}

	// TODO print the submariner version in the SubmarinerAgentDegraded condition
	operator, err := c.deploymentLister.Deployments(c.namespace).Get(operatorName)
	switch {
	case errors.IsNotFound(err):
		degradedConditionReasons = append(degradedConditionReasons, "OperatorNotDeployed")
		degradedConditionMessages = append(degradedConditionMessages, "The submariner-operator is not deployed")
	case err == nil:
		if operator.Status.AvailableReplicas == 0 {
			degradedConditionReasons = append(degradedConditionReasons, "OperatorDegraded")
			degradedConditionMessages = append(degradedConditionMessages, "There are no available submariner-operator")
		}
	case err != nil:
		return err
	}

	gateways, err := c.daemonSetLister.DaemonSets(c.namespace).Get(gatewayName)
	switch {
	case errors.IsNotFound(err):
		degradedConditionReasons = append(degradedConditionReasons, "GatewaysNotDeployed")
		degradedConditionMessages = append(degradedConditionMessages, "The gateways are not deployed")
	case err == nil:
		if gateways.Status.DesiredNumberScheduled == 0 {
			degradedConditionReasons = append(degradedConditionReasons, "GatewaysDegraded")
			degradedConditionMessages = append(degradedConditionMessages, "There are no nodes to run the gateways")
		}

		if gateways.Status.NumberUnavailable != 0 {
			degradedConditionReasons = append(degradedConditionReasons, "GatewaysDegraded")
			degradedConditionMessages = append(degradedConditionMessages,
				fmt.Sprintf("There are %d unavailable gateways", gateways.Status.NumberUnavailable))
		}
	case err != nil:
		return err
	}

	routeAgent, err := c.daemonSetLister.DaemonSets(c.namespace).Get(routeAgentName)
	switch {
	case errors.IsNotFound(err):
		degradedConditionReasons = append(degradedConditionReasons, "RouteAgentsNotDeployed")
		degradedConditionMessages = append(degradedConditionMessages, "The route agents are not deployed")
	case err == nil:
		if routeAgent.Status.NumberUnavailable != 0 {
			degradedConditionReasons = append(degradedConditionReasons, "RouteAgentsDegraded")
			degradedConditionMessages = append(degradedConditionMessages,
				fmt.Sprintf("There are %d unavailable route agents", routeAgent.Status.NumberUnavailable))
		}
	case err != nil:
		return err
	}

	//TODO check globalnet daemonset status, if global is enabled

	submarinerAgentCondtion := metav1.Condition{
		Type:    submarinerAgentDegraded,
		Status:  metav1.ConditionFalse,
		Reason:  "SubmarinerAgentDeployed",
		Message: "Submariner agent is deployed on managed cluster.",
	}

	if len(degradedConditionReasons) != 0 {
		submarinerAgentCondtion.Status = metav1.ConditionTrue
		submarinerAgentCondtion.Reason = strings.Join(degradedConditionReasons, ",")
		submarinerAgentCondtion.Message = strings.Join(degradedConditionMessages, "\n")
	}

	// check submariner agent status and update submariner-addon status on the hub cluster
	_, updated, err := helpers.UpdateManagedClusterAddOnStatus(
		ctx,
		c.addOnClient,
		c.clusterName,
		addOn.Name,
		helpers.UpdateManagedClusterAddOnStatusFn(submarinerAgentCondtion),
	)
	if err != nil {
		return err
	}
	if updated {
		syncCtx.Recorder().Eventf("ManagedClusterAddOnStatusUpdated", "update managed cluster addon %q status with submariner agent status", addOn.Name)
	}

	return nil
}
