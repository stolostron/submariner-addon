package submarineragent

import (
	"context"
	"fmt"
	"strings"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stolostron/submariner-addon/pkg/helpers"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	appsv1lister "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
)

const (
	subscriptionName = "submariner"
	operatorName     = "submariner-operator"
	gatewayName      = "submariner-gateway"
	routeAgentName   = "submariner-routeagent"
)

const submarinerAgentDegraded = "SubmarinerAgentDegraded"

// deploymentStatusController watches the status of submariner-operator deployment and submariner daemonsets
// on the managed cluster and reports the status to the submariner-addon on the hub cluster
type deploymentStatusController struct {
	addOnClient        addonclient.Interface
	daemonSetLister    appsv1lister.DaemonSetLister
	deploymentLister   appsv1lister.DeploymentLister
	subscriptionLister cache.GenericLister
	clusterName        string
	namespace          string
}

// NewDeploymentStatusController returns an instance of deploymentStatusController
func NewDeploymentStatusController(clusterName string, installationNamespace string, addOnClient addonclient.Interface,
	daemonsetInformer appsv1informers.DaemonSetInformer, deploymentInformer appsv1informers.DeploymentInformer,
	subscriptionInformer informers.GenericInformer, recorder events.Recorder) factory.Controller {
	c := &deploymentStatusController{
		addOnClient:        addOnClient,
		daemonSetLister:    daemonsetInformer.Lister(),
		deploymentLister:   deploymentInformer.Lister(),
		subscriptionLister: subscriptionInformer.Lister(),
		clusterName:        clusterName,
		namespace:          installationNamespace,
	}

	return factory.New().
		WithInformers(subscriptionInformer.Informer(), daemonsetInformer.Informer(), deploymentInformer.Informer()).
		WithSync(c.sync).
		ToController("SubmarinerAgentStatusController", recorder)
}

func (c *deploymentStatusController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	degradedConditionReasons := []string{}
	degradedConditionMessages := []string{}

	runtimeSub, err := c.subscriptionLister.ByNamespace(c.namespace).Get(subscriptionName)
	if errors.IsNotFound(err) {
		// submariner subscription is not found, could be deleted, ignore it.
		return nil
	}
	if err != nil {
		return err
	}

	unstructuredSub, err := runtime.DefaultUnstructuredConverter.ToUnstructured(runtimeSub)
	if err != nil {
		return err
	}

	sub := &operatorsv1alpha1.Subscription{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredSub, &sub); err != nil {
		return err
	}

	if len(sub.Status.InstalledCSV) == 0 {
		startingCSV := sub.Spec.StartingCSV
		if len(startingCSV) == 0 {
			startingCSV = "default"
		}

		channel := sub.Spec.Channel
		if len(channel) == 0 {
			channel = "default"
		}

		degradedConditionReasons = append(degradedConditionReasons, "CSVNotInstalled")
		degradedConditionMessages = append(degradedConditionMessages,
			fmt.Sprintf("The submariner-operator CSV (%s) is not installed from channel (%s) in catalog source (%s/%s)",
				startingCSV, channel, sub.Spec.CatalogSourceNamespace, sub.Spec.CatalogSource))
	}

	operator, err := c.deploymentLister.Deployments(c.namespace).Get(operatorName)
	switch {
	case errors.IsNotFound(err):
		degradedConditionReasons = append(degradedConditionReasons, "NoOperatorDeployment")
		degradedConditionMessages = append(degradedConditionMessages, "The submariner operator deployment does not exist")
	case err == nil:
		if operator.Status.AvailableReplicas == 0 {
			degradedConditionReasons = append(degradedConditionReasons, "NoOperatorAvailable")
			degradedConditionMessages = append(degradedConditionMessages, "There is no submariner operator replica available")
		}
	case err != nil:
		return err
	}

	gateways, err := c.daemonSetLister.DaemonSets(c.namespace).Get(gatewayName)
	switch {
	case errors.IsNotFound(err):
		degradedConditionReasons = append(degradedConditionReasons, "NoGatewayDaemonSet")
		degradedConditionMessages = append(degradedConditionMessages, "The gateway daemon set does not exist")
	case err == nil:
		if gateways.Status.DesiredNumberScheduled == 0 {
			degradedConditionReasons = append(degradedConditionReasons, "NoScheduledGateways")
			degradedConditionMessages = append(degradedConditionMessages, "There are no nodes to run the gateways")
		}

		if gateways.Status.NumberUnavailable != 0 {
			degradedConditionReasons = append(degradedConditionReasons, "GatewaysUnavailable")
			degradedConditionMessages = append(degradedConditionMessages,
				fmt.Sprintf("There are %d unavailable gateways", gateways.Status.NumberUnavailable))
		}
	case err != nil:
		return err
	}

	routeAgent, err := c.daemonSetLister.DaemonSets(c.namespace).Get(routeAgentName)
	switch {
	case errors.IsNotFound(err):
		degradedConditionReasons = append(degradedConditionReasons, "NoRouteAgentDaemonSet")
		degradedConditionMessages = append(degradedConditionMessages, "The route agents are not found")
	case err == nil:
		if routeAgent.Status.NumberUnavailable != 0 {
			degradedConditionReasons = append(degradedConditionReasons, "RouteAgentsUnavailable")
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
		Message: fmt.Sprintf("Submariner (%s) is deployed on managed cluster.", sub.Status.InstalledCSV),
	}

	if len(degradedConditionReasons) != 0 {
		submarinerAgentCondtion.Status = metav1.ConditionTrue
		submarinerAgentCondtion.Reason = strings.Join(degradedConditionReasons, ",")
		submarinerAgentCondtion.Message = strings.Join(degradedConditionMessages, "\n")
	}

	// check submariner agent status and update submariner-addon status on the hub cluster
	updatedStatus, updated, err := helpers.UpdateManagedClusterAddOnStatus(ctx, c.addOnClient, c.clusterName,
		helpers.UpdateManagedClusterAddOnStatusFn(submarinerAgentCondtion))
	if err != nil {
		return err
	}

	if updated {
		syncCtx.Recorder().Eventf("ManagedClusterAddOnStatusUpdated", "Updated status conditions:  %#v",
			updatedStatus.Conditions)
	}

	return nil
}
