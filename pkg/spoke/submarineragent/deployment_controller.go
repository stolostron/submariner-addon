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
	"github.com/submariner-io/admiral/pkg/names"
	"github.com/submariner-io/admiral/pkg/resource"
	"github.com/submariner-io/admiral/pkg/util"
	submarinerv1alpha1 "github.com/submariner-io/submariner-operator/api/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/informers"
	appsv1informers "k8s.io/client-go/informers/apps/v1"
	appsv1lister "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	subscriptionName = "submariner"
)

const submarinerAgentDegraded = "SubmarinerAgentDegraded"

// deploymentStatusController watches the status of submariner deployments and submariner daemonsets
// on the managed cluster and reports the status to the submariner-addon on the hub cluster.
type deploymentStatusController struct {
	addOnClient        addonclient.Interface
	daemonSetLister    appsv1lister.DaemonSetLister
	deploymentLister   appsv1lister.DeploymentLister
	subscriptionLister cache.GenericLister
	submarinerLister   cache.GenericLister
	clusterName        string
	namespace          string
	logger             log.Logger
}

// NewDeploymentStatusController returns an instance of deploymentStatusController.
func NewDeploymentStatusController(clusterName string, installationNamespace string, addOnClient addonclient.Interface,
	daemonsetInformer appsv1informers.DaemonSetInformer, deploymentInformer appsv1informers.DeploymentInformer,
	subscriptionInformer informers.GenericInformer, submarinerInformer informers.GenericInformer, recorder events.Recorder,
) factory.Controller {
	name := "DeploymentStatusController"
	c := &deploymentStatusController{
		addOnClient:        addOnClient,
		daemonSetLister:    daemonsetInformer.Lister(),
		deploymentLister:   deploymentInformer.Lister(),
		subscriptionLister: subscriptionInformer.Lister(),
		submarinerLister:   submarinerInformer.Lister(),
		clusterName:        clusterName,
		namespace:          installationNamespace,
		logger:             log.Logger{Logger: logf.Log.WithName(name)},
	}

	return factory.New().
		WithInformers(subscriptionInformer.Informer(), daemonsetInformer.Informer(), deploymentInformer.Informer()).
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			key, _ := cache.MetaNamespaceKeyFunc(obj)

			return key
		}, submarinerInformer.Informer()).
		WithSync(c.sync).
		ToController(name, recorder)
}

func getNestedString(obj *unstructured.Unstructured, fields ...string) string {
	s, _, err := unstructured.NestedString(obj.Object, fields...)
	utilruntime.Must(errors.Wrapf(err, "error retrieving %v field for %#v", fields, obj))

	return s
}

func (c *deploymentStatusController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	degradedConditionReasons := []string{}
	degradedConditionMessages := []string{}

	runtimeSub, err := c.subscriptionLister.ByNamespace(c.namespace).Get(subscriptionName)
	if apierrors.IsNotFound(err) {
		// submariner subscription is not found, could be deleted, ignore it.
		return nil
	}

	if err != nil {
		return err
	}

	unstructuredSub := resource.MustToUnstructured(runtimeSub)

	installedCSV := getNestedString(unstructuredSub, util.StatusField, "installedCSV")
	if installedCSV == "" {
		startingCSV := getNestedString(unstructuredSub, "spec", "startingCSV")
		if startingCSV == "" {
			startingCSV = "default"
		}

		channel := getNestedString(unstructuredSub, "spec", "channel")
		if channel == "" {
			channel = "default"
		}

		degradedConditionReasons = append(degradedConditionReasons, "CSVNotInstalled")
		degradedConditionMessages = append(degradedConditionMessages,
			fmt.Sprintf("The submariner-operator CSV (%s) is not installed from channel (%s) in catalog source (%s/%s)",
				startingCSV, channel, getNestedString(unstructuredSub, "spec", "sourceNamespace"),
				getNestedString(unstructuredSub, "spec", "source")))
	}

	err = c.checkDeployments(&degradedConditionReasons, &degradedConditionMessages)
	if err != nil {
		return err
	}

	err = c.checkDaemonSets(&degradedConditionReasons, &degradedConditionMessages)
	if err != nil {
		return err
	}

	err = c.checkOptionals(&degradedConditionReasons, &degradedConditionMessages)
	if err != nil {
		return err
	}

	submarinerAgentCondition := metav1.Condition{
		Type:    submarinerAgentDegraded,
		Status:  metav1.ConditionFalse,
		Reason:  "SubmarinerAgentDeployed",
		Message: fmt.Sprintf("Submariner (%s) is deployed on managed cluster.", installedCSV),
	}

	if len(degradedConditionReasons) != 0 {
		submarinerAgentCondition.Status = metav1.ConditionTrue
		submarinerAgentCondition.Reason = strings.Join(degradedConditionReasons, ",")
		submarinerAgentCondition.Message = strings.Join(degradedConditionMessages, "\n")
	}

	// check submariner agent status and update submariner-addon status on the hub cluster
	updatedStatus, updated, err := addon.UpdateStatus(ctx, c.addOnClient, c.clusterName, addon.UpdateConditionFn(&submarinerAgentCondition))
	if err != nil {
		return err
	}

	if updated {
		c.logger.Infof("Updated submariner ManagedClusterAddOn status condition: %s", resource.ToJSON(submarinerAgentCondition))

		syncCtx.Recorder().Eventf("ManagedClusterAddOnStatusUpdated", "Updated status conditions:  %#v",
			updatedStatus.Conditions)
	}

	return nil
}

func (c *deploymentStatusController) checkDeployment(name, reasonName string, degradedConditionReasons,
	degradedConditionMessages *[]string,
) error {
	deployment, err := c.deploymentLister.Deployments(c.namespace).Get(name)
	msgName := strings.ReplaceAll(name, "-", " ")

	switch {
	case apierrors.IsNotFound(err):
		*degradedConditionReasons = append(*degradedConditionReasons, fmt.Sprintf("No%sDeployment", reasonName))
		*degradedConditionMessages = append(*degradedConditionMessages, fmt.Sprintf("The %s deployment does not exist", msgName))
	case err == nil:
		if deployment.Status.AvailableReplicas == 0 {
			*degradedConditionReasons = append(*degradedConditionReasons, fmt.Sprintf("No%sAvailable", reasonName))
			*degradedConditionMessages = append(*degradedConditionMessages, fmt.Sprintf("There are no %s replica available", msgName))
		}
	case err != nil:
		return err
	}

	return nil
}

func (c *deploymentStatusController) checkDeployments(degradedConditionReasons, degradedConditionMessages *[]string) error {
	err := c.checkDeployment(names.OperatorComponent, "Operator", degradedConditionReasons, degradedConditionMessages)
	if err != nil {
		return err
	}

	err = c.checkDeployment(names.ServiceDiscoveryComponent, "LighthouseAgent", degradedConditionReasons, degradedConditionMessages)
	if err != nil {
		return err
	}

	err = c.checkDeployment(names.LighthouseCoreDNSComponent, "LighthouseCoreDNS", degradedConditionReasons, degradedConditionMessages)
	if err != nil {
		return err
	}

	return nil
}

func (c *deploymentStatusController) checkOptionals(degradedConditionReasons, degradedConditionMessages *[]string,
) (err error) {
	submariner, err := c.getSubmariner()
	if err != nil {
		return err
	}

	if submariner == nil {
		return nil
	}

	if submariner.Spec.GlobalCIDR != "" {
		err = c.checkGlobalnetDaemonSet(degradedConditionReasons, degradedConditionMessages)
		if err != nil {
			return err
		}
	}

	return nil
}

func (c *deploymentStatusController) checkGatewayDaemonSet(degradedConditionReasons, degradedConditionMessages *[]string) error {
	gateways, err := c.daemonSetLister.DaemonSets(c.namespace).Get(names.GatewayComponent)

	switch {
	case apierrors.IsNotFound(err):
		*degradedConditionReasons = append(*degradedConditionReasons, "NoGatewayDaemonSet")
		*degradedConditionMessages = append(*degradedConditionMessages, "The gateway daemon set does not exist")
	case err == nil:
		if gateways.Status.DesiredNumberScheduled == 0 {
			*degradedConditionReasons = append(*degradedConditionReasons, "NoScheduledGateways")
			*degradedConditionMessages = append(*degradedConditionMessages, "There are no nodes to run the gateways")
		}

		if gateways.Status.NumberUnavailable != 0 {
			*degradedConditionReasons = append(*degradedConditionReasons, "GatewaysUnavailable")
			*degradedConditionMessages = append(*degradedConditionMessages,
				fmt.Sprintf("There are %d unavailable gateways", gateways.Status.NumberUnavailable))
		}
	case err != nil:
		return err
	}

	return nil
}

func (c *deploymentStatusController) checkRouteAgentDaemonSet(degradedConditionReasons, degradedConditionMessages *[]string) error {
	routeAgent, err := c.daemonSetLister.DaemonSets(c.namespace).Get(names.RouteAgentComponent)

	switch {
	case apierrors.IsNotFound(err):
		*degradedConditionReasons = append(*degradedConditionReasons, "NoRouteAgentDaemonSet")
		*degradedConditionMessages = append(*degradedConditionMessages, "The route agents are not found")
	case err == nil:
		if routeAgent.Status.NumberUnavailable != 0 {
			*degradedConditionReasons = append(*degradedConditionReasons, "RouteAgentsUnavailable")
			*degradedConditionMessages = append(*degradedConditionMessages,
				fmt.Sprintf("There are %d unavailable route agents", routeAgent.Status.NumberUnavailable))
		}
	case err != nil:
		return err
	}

	return nil
}

func (c *deploymentStatusController) checkMetricsProxyDaemonSet(degradedConditionReasons, degradedConditionMessages *[]string) error {
	metricProxy, err := c.daemonSetLister.DaemonSets(c.namespace).Get(names.MetricsProxyComponent)

	switch {
	case apierrors.IsNotFound(err):
		*degradedConditionReasons = append(*degradedConditionReasons, "NoMetricsProxyDaemonSet")
		*degradedConditionMessages = append(*degradedConditionMessages, "The metrics proxy daemon set does not exist")
	case err == nil:
		if metricProxy.Status.DesiredNumberScheduled == 0 {
			*degradedConditionReasons = append(*degradedConditionReasons, "NoScheduledMetricsProxy")
			*degradedConditionMessages = append(*degradedConditionMessages, "There are no nodes to run the metrics proxy")
		}

		if metricProxy.Status.NumberUnavailable != 0 {
			*degradedConditionReasons = append(*degradedConditionReasons, "MetricsProxyUnavailable")
			*degradedConditionMessages = append(*degradedConditionMessages,
				fmt.Sprintf("There are %d unavailable metrics proxy pods", metricProxy.Status.NumberUnavailable))
		}
	case err != nil:
		return err
	}

	return nil
}

func (c *deploymentStatusController) checkGlobalnetDaemonSet(degradedConditionReasons, degradedConditionMessages *[]string) error {
	globalnet, err := c.daemonSetLister.DaemonSets(c.namespace).Get(names.GlobalnetComponent)

	switch {
	case apierrors.IsNotFound(err):
		*degradedConditionReasons = append(*degradedConditionReasons, "NoGlobalnetDaemonSet")
		*degradedConditionMessages = append(*degradedConditionMessages, "The globalnet daemon set does not exist")
	case err == nil:
		if globalnet.Status.DesiredNumberScheduled == 0 {
			*degradedConditionReasons = append(*degradedConditionReasons, "NoScheduledGlobalnet")
			*degradedConditionMessages = append(*degradedConditionMessages, "There are no nodes to run the globalnet pods")
		}

		if globalnet.Status.NumberUnavailable != 0 {
			*degradedConditionReasons = append(*degradedConditionReasons, "GlobalnetUnavailable")
			*degradedConditionMessages = append(*degradedConditionMessages,
				fmt.Sprintf("There are %d unavailable globalnet pods", globalnet.Status.NumberUnavailable))
		}
	case err != nil:
		return err
	}

	return nil
}

func (c *deploymentStatusController) checkDaemonSets(degradedConditionReasons, degradedConditionMessages *[]string) error {
	err := c.checkGatewayDaemonSet(degradedConditionReasons, degradedConditionMessages)
	if err != nil {
		return err
	}

	err = c.checkRouteAgentDaemonSet(degradedConditionReasons, degradedConditionMessages)
	if err != nil {
		return err
	}

	err = c.checkMetricsProxyDaemonSet(degradedConditionReasons, degradedConditionMessages)
	if err != nil {
		return err
	}

	return nil
}

func (c *deploymentStatusController) getSubmariner() (*submarinerv1alpha1.Submariner, error) {
	list, err := c.submarinerLister.ByNamespace(c.namespace).List(labels.Everything())
	if err != nil {
		return nil, err
	}

	if len(list) == 0 {
		return nil, nil //nolint:nilnil // No Submariner is not an error
	}

	unstructuredSubmariner, err := runtime.DefaultUnstructuredConverter.ToUnstructured(list[0])
	if err != nil {
		return nil, err
	}

	submariner := &submarinerv1alpha1.Submariner{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredSubmariner, &submariner); err != nil {
		return nil, err
	}

	return submariner, nil
}
