package submarineragent

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	operatorhelpers "github.com/openshift/library-go/pkg/operator/v1helpers"
	"github.com/pkg/errors"
	"github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig"
	configv1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	configclient "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	configinformer "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/informers/externalversions/submarinerconfig/v1alpha1"
	configlister "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/listers/submarinerconfig/v1alpha1"
	"github.com/stolostron/submariner-addon/pkg/cloud"
	"github.com/stolostron/submariner-addon/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/util/sets"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/retry"
	addoninformerv1alpha1 "open-cluster-management.io/api/client/addon/informers/externalversions/addon/v1alpha1"
	addonlisterv1alpha1 "open-cluster-management.io/api/client/addon/listers/addon/v1alpha1"
)

// TODO expose this as a flag to allow user to specify their zone label.
var defaultZoneLabel = ""

const submarinerGatewayCondition = "SubmarinerGatewaysLabeled"

const (
	submarinerUDPPortLabel = "gateway.submariner.io/udp-port"
	workerNodeLabel        = "node-role.kubernetes.io/worker"
)

type nodeLabelSelector struct {
	label string
	op    selection.Operator
}

// submarinerConfigController watches the SubmarinerConfigs API on the hub cluster and apply
// the related configuration on the manged cluster.
type submarinerConfigController struct {
	kubeClient           kubernetes.Interface
	configClient         configclient.Interface
	nodeLister           corev1lister.NodeLister
	addOnLister          addonlisterv1alpha1.ManagedClusterAddOnLister
	configLister         configlister.SubmarinerConfigLister
	clusterName          string
	cloudProviderFactory cloud.ProviderFactory
	onSyncDefer          func()
}

type SubmarinerConfigControllerInput struct {
	ClusterName          string
	KubeClient           kubernetes.Interface
	ConfigClient         configclient.Interface
	NodeInformer         corev1informers.NodeInformer
	AddOnInformer        addoninformerv1alpha1.ManagedClusterAddOnInformer
	ConfigInformer       configinformer.SubmarinerConfigInformer
	CloudProviderFactory cloud.ProviderFactory
	Recorder             events.Recorder
	// This is a hook for unit tests to invoke a defer (specifically GinkgoRecover) when the sync function is called.
	OnSyncDefer func()
}

// NewSubmarinerConfigController returns an instance of submarinerAgentConfigController.
func NewSubmarinerConfigController(input *SubmarinerConfigControllerInput) factory.Controller {
	c := &submarinerConfigController{
		kubeClient:           input.KubeClient,
		configClient:         input.ConfigClient,
		nodeLister:           input.NodeInformer.Lister(),
		addOnLister:          input.AddOnInformer.Lister(),
		configLister:         input.ConfigInformer.Lister(),
		clusterName:          input.ClusterName,
		cloudProviderFactory: input.CloudProviderFactory,
		onSyncDefer:          input.OnSyncDefer,
	}

	return factory.New().
		WithFilteredEventsInformers(func(obj interface{}) bool {
			metaObj := obj.(metav1.Object)

			return metaObj.GetName() == constants.SubmarinerAddOnName
		}, input.AddOnInformer.Informer()).
		WithFilteredEventsInformers(func(obj interface{}) bool {
			metaObj := obj.(metav1.Object)

			return metaObj.GetName() == constants.SubmarinerConfigName
		}, input.ConfigInformer.Informer()).
		WithFilteredEventsInformers(func(obj interface{}) bool {
			metaObj := obj.(metav1.Object)
			// only handle the changes of worker nodes
			if _, has := metaObj.GetLabels()[workerNodeLabel]; has {
				return true
			}

			return false
		}, input.NodeInformer.Informer()).
		WithSync(c.sync).
		ToController("SubmarinerAgentConfigController", input.Recorder)
}

func (c *submarinerConfigController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	if c.onSyncDefer != nil {
		defer c.onSyncDefer()
	}

	addOn, err := c.addOnLister.ManagedClusterAddOns(c.clusterName).Get(constants.SubmarinerAddOnName)
	if apiErrors.IsNotFound(err) {
		// the addon not found, could be deleted, ignore
		return nil
	}

	if err != nil {
		return err
	}

	config, err := c.configLister.SubmarinerConfigs(c.clusterName).Get(constants.SubmarinerConfigName)
	if apiErrors.IsNotFound(err) {
		// the config not found, could be deleted, do nothing
		return nil
	}

	if err != nil {
		return err
	}

	if config.Status.ManagedClusterInfo.Platform == "" {
		// no managed cluster info, do nothing
		return nil
	}
	// addon is deleting from the hub, remove its related resources on the managed cluster
	// TODO: add finalizer in next release
	if !addOn.DeletionTimestamp.IsZero() {
		// if the addon is deleted before config, clean up the submariner cluster environment.
		condition := metav1.Condition{
			Type:    submarinerGatewayCondition,
			Status:  metav1.ConditionFalse,
			Reason:  "ManagedClusterAddOnDeleted",
			Message: "There are no nodes labeled as gateways",
		}

		err := c.cleanupClusterEnvironment(ctx, config, syncCtx.Recorder())
		if err != nil {
			condition = failedCondition(err.Error())
		}

		updateErr := c.updateSubmarinerConfigStatus(ctx, syncCtx.Recorder(), config, &condition)

		if err != nil {
			return err
		}

		return updateErr
	}

	// config is deleting from hub, remove its related resources
	// TODO: add finalizer in next release
	if !config.DeletionTimestamp.IsZero() {
		return c.cleanupClusterEnvironment(ctx, config, syncCtx.Recorder())
	}

	if config.Status.ManagedClusterInfo.Platform == "AWS" {
		// for AWS, the gateway configuration will be operated on the hub
		// count the gateways status on the managed cluster and report it to the hub
		return c.updateGatewayStatus(ctx, syncCtx.Recorder(), config)
	}

	if isSpokePrepared(config.Status.ManagedClusterInfo.Platform) {
		return c.prepareForSubmariner(ctx, config, syncCtx)
	}

	// ensure the expected count of gateways
	condition, err := c.ensureGateways(ctx, config)

	updateErr := c.updateSubmarinerConfigStatus(ctx, syncCtx.Recorder(), config, &condition)

	if err != nil {
		return err
	}

	return updateErr
}

func (c *submarinerConfigController) prepareForSubmariner(ctx context.Context, config *configv1alpha1.SubmarinerConfig,
	syncCtx factory.SyncContext,
) error {
	if !meta.IsStatusConditionTrue(config.Status.Conditions, configv1alpha1.SubmarinerConfigConditionEnvPrepared) {
		errs := []error{}

		cloudProvider, preparedErr := c.cloudProviderFactory.Get(config.Status.ManagedClusterInfo, config, syncCtx.Recorder())
		if preparedErr == nil {
			preparedErr = cloudProvider.PrepareSubmarinerClusterEnv()
		}

		condition := metav1.Condition{
			Type:    configv1alpha1.SubmarinerConfigConditionEnvPrepared,
			Status:  metav1.ConditionTrue,
			Reason:  "SubmarinerClusterEnvPrepared",
			Message: "Submariner cluster environment was prepared",
		}

		if preparedErr != nil {
			condition.Status = metav1.ConditionFalse
			condition.Reason = "SubmarinerClusterEnvPreparationFailed"
			condition.Message = fmt.Sprintf("Failed to prepare submariner cluster environment: %v", preparedErr)
			errs = append(errs, preparedErr)
		}

		_, updated, updatedErr := submarinerconfig.UpdateStatus(ctx,
			c.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(config.Namespace), config.Name,
			submarinerconfig.UpdateConditionFn(&condition))

		if updatedErr != nil {
			errs = append(errs, updatedErr)
		}

		if updated {
			syncCtx.Recorder().Eventf("SubmarinerClusterEnvPrepared",
				"submariner cluster environment was prepared for managed cluster %s", config.Namespace)
		}

		if len(errs) > 0 {
			return operatorhelpers.NewMultiLineAggregate(errs)
		}
	}

	return c.updateGatewayStatus(ctx, syncCtx.Recorder(), config)
}

func (c *submarinerConfigController) cleanupClusterEnvironment(ctx context.Context, config *configv1alpha1.SubmarinerConfig,
	recorder events.Recorder,
) error {
	platform := config.Status.ManagedClusterInfo.Platform
	if isSpokePrepared(platform) {
		var cloudProvider cloud.Provider

		cloudProvider, err := c.cloudProviderFactory.Get(config.Status.ManagedClusterInfo, config, recorder)
		if err == nil {
			err = cloudProvider.CleanUpSubmarinerClusterEnv()
		}

		return errors.WithMessagef(err, "Failed to clean up the submariner cluster environment")
	} else if platform == "AWS" {
		// Cloud-prepare for AWS done from hub, Nothing to do on spoke
		return nil
	}

	return errors.WithMessagef(c.removeAllGateways(ctx), "Failed to unlabel the gateway nodes")
}

func (c *submarinerConfigController) updateSubmarinerConfigStatus(ctx context.Context, recorder events.Recorder,
	config *configv1alpha1.SubmarinerConfig, condition *metav1.Condition,
) error {
	updatedStatus, updated, err := submarinerconfig.UpdateStatus(ctx,
		c.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(config.Namespace), config.Name,
		submarinerconfig.UpdateConditionFn(condition))

	if updated {
		recorder.Eventf("SubmarinerConfigStatusUpdated", "Updated status conditions:  %#v", updatedStatus.Conditions)
	}

	return err
}

func (c *submarinerConfigController) ensureGateways(ctx context.Context,
	config *configv1alpha1.SubmarinerConfig,
) (metav1.Condition, error) {
	if config.Spec.Gateways < 1 {
		return metav1.Condition{
			Type:    submarinerGatewayCondition,
			Status:  metav1.ConditionFalse,
			Reason:  "InvalidInput",
			Message: "The desired number of gateways must be at least 1",
		}, nil
	}

	currentGateways, err := c.getLabeledNodes(
		nodeLabelSelector{submarinerGatewayLabel, selection.Exists},
	)
	if err != nil {
		return failedCondition("Error retrieving nodes: %v", err), err
	}

	currentGatewayNames := []string{}
	for _, gateway := range currentGateways {
		currentGatewayNames = append(currentGatewayNames, gateway.Name)
	}

	updatedGatewayNames := []string{}

	requiredGateways := config.Spec.Gateways - len(currentGateways)

	switch {
	case requiredGateways == 0:
		// number of gateways unchanged, ensure that the gateways are fully labeled
		errs := []error{}
		for _, gateway := range currentGateways {
			errs = append(errs, c.labelNode(ctx, config, gateway))
		}

		updatedGatewayNames = currentGatewayNames
		err = operatorhelpers.NewMultiLineAggregate(errs)
	case requiredGateways > 0:
		// gateways increased, need to label new ones
		updatedGatewayNames, err = c.addGateways(ctx, config, requiredGateways)
	default:
		// gateways decreased, need to unlabel some
		var removed []string

		removed, err = c.removeGateways(ctx, currentGateways, -requiredGateways)

		removedNames := sets.NewString(removed...)

		for _, name := range currentGatewayNames {
			if !removedNames.Has(name) {
				updatedGatewayNames = append(updatedGatewayNames, name)
			}
		}
	}

	if err != nil {
		return failedCondition("Unable to label the gateway nodes: %v", err), err
	}

	if len(updatedGatewayNames) == 0 {
		return metav1.Condition{
			Type:    submarinerGatewayCondition,
			Status:  metav1.ConditionFalse,
			Reason:  "InsufficientNodes",
			Message: "Insufficient number of worker nodes to satisfy the desired number of gateways",
		}, nil
	}

	sort.Strings(updatedGatewayNames)

	return successCondition(updatedGatewayNames), nil
}

func (c *submarinerConfigController) getLabeledNodes(nodeLabelSelectors ...nodeLabelSelector) ([]*corev1.Node, error) {
	requirements := []labels.Requirement{}

	for _, selector := range nodeLabelSelectors {
		requirement, err := labels.NewRequirement(selector.label, selector.op, []string{})
		if err != nil {
			return nil, err
		}

		requirements = append(requirements, *requirement)
	}

	return c.nodeLister.List(labels.Everything().Add(requirements...))
}

func (c *submarinerConfigController) labelNode(ctx context.Context, config *configv1alpha1.SubmarinerConfig, node *corev1.Node) error {
	_, hasGatewayLabel := node.Labels[submarinerGatewayLabel]
	labeledPort, hasPortLabel := node.Labels[submarinerUDPPortLabel]
	nattPort := strconv.Itoa(config.Spec.IPSecNATTPort)
	if hasGatewayLabel && (hasPortLabel && labeledPort == nattPort) {
		// the node has been labeled, do nothing
		return nil
	}

	return c.updateNode(ctx, node, func(node *corev1.Node) {
		node.Labels[submarinerGatewayLabel] = "true"
		node.Labels[submarinerUDPPortLabel] = nattPort
	})
}

func (c *submarinerConfigController) updateNode(ctx context.Context, node *corev1.Node, mutate func(node *corev1.Node)) error {
	name := node.Name

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var err error

		if node == nil {
			node, err = c.kubeClient.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				return err
			}
		}

		node = node.DeepCopy()
		mutate(node)

		_, err = c.kubeClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
		node = nil

		return err
	})
}

func (c *submarinerConfigController) unlabelNode(ctx context.Context, node *corev1.Node) error {
	_, hasGatewayLabel := node.Labels[submarinerGatewayLabel]
	_, hasPortLabel := node.Labels[submarinerUDPPortLabel]
	if !hasGatewayLabel && !hasPortLabel {
		// the node dose not have gateway and port labels, do nothing
		return nil
	}

	return c.updateNode(ctx, node, func(node *corev1.Node) {
		delete(node.Labels, submarinerGatewayLabel)
		delete(node.Labels, submarinerUDPPortLabel)
	})
}

func (c *submarinerConfigController) addGateways(ctx context.Context, config *configv1alpha1.SubmarinerConfig,
	expectedGateways int,
) ([]string, error) {
	// for other non-public cloud platform (vsphere) or native k8s
	zoneLabel := defaultZoneLabel

	gateways, err := c.findGatewaysWithZone(expectedGateways, zoneLabel)
	if err != nil {
		return []string{}, err
	}

	names := []string{}
	errs := []error{}
	for _, gateway := range gateways {
		errs = append(errs, c.labelNode(ctx, config, gateway))
		names = append(names, gateway.Name)
	}

	return names, operatorhelpers.NewMultiLineAggregate(errs)
}

func (c *submarinerConfigController) removeGateways(ctx context.Context, gateways []*corev1.Node, removedGateways int) ([]string, error) {
	if len(gateways) < removedGateways {
		removedGateways = len(gateways)
	}

	errs := []error{}
	removed := []string{}

	for i := 0; i < removedGateways; i++ {
		removed = append(removed, gateways[i].Name)
		errs = append(errs, c.unlabelNode(ctx, gateways[i]))
	}

	return removed, operatorhelpers.NewMultiLineAggregate(errs)
}

func (c *submarinerConfigController) removeAllGateways(ctx context.Context) error {
	gateways, err := c.getLabeledNodes(nodeLabelSelector{submarinerGatewayLabel, selection.Exists})
	if err != nil {
		return err
	}

	_, err = c.removeGateways(ctx, gateways, len(gateways))

	return err
}

func (c *submarinerConfigController) findGatewaysWithZone(expected int, zoneLabel string) ([]*corev1.Node, error) {
	workers, err := c.getLabeledNodes(
		nodeLabelSelector{workerNodeLabel, selection.Exists},
		nodeLabelSelector{submarinerGatewayLabel, selection.DoesNotExist},
	)
	if err != nil {
		return nil, err
	}

	if len(workers) < expected {
		return []*corev1.Node{}, nil
	}

	// group the nodes with zone
	zoneNodes := map[string][]*corev1.Node{}

	for _, worker := range workers {
		zone, has := worker.Labels[zoneLabel]
		if !has {
			zone = "unknown"
		}

		nodes, has := zoneNodes[zone]
		if !has {
			zoneNodes[zone] = []*corev1.Node{worker}

			continue
		}

		nodes = append(nodes, worker)
		zoneNodes[zone] = nodes
	}

	count := 0
	nodeIndex := 0
	gateways := []*corev1.Node{}
	// find candidate gateways from different zones
	for count < expected {
		for _, nodes := range zoneNodes {
			if nodeIndex >= len(nodes) {
				continue
			}

			if count == expected {
				break
			}

			gateways = append(gateways, nodes[nodeIndex])
			count++
		}

		nodeIndex++
	}

	return gateways, nil
}

func (c *submarinerConfigController) updateGatewayStatus(ctx context.Context, recorder events.Recorder,
	config *configv1alpha1.SubmarinerConfig,
) error {
	gateways, err := c.getLabeledNodes(
		nodeLabelSelector{workerNodeLabel, selection.Exists},
		nodeLabelSelector{submarinerGatewayLabel, selection.Exists},
	)
	if err != nil {
		return err
	}

	gatewayNames := []string{}
	for _, gateway := range gateways {
		gatewayNames = append(gatewayNames, gateway.Name)
	}

	var condition metav1.Condition

	if config.Spec.Gateways != len(gateways) {
		condition = metav1.Condition{
			Type:   submarinerGatewayCondition,
			Status: metav1.ConditionFalse,
			Reason: "InsufficientNodes",
			Message: fmt.Sprintf("The %d worker nodes labeled as gateways (%q) does not match the desired number %d",
				len(gatewayNames), strings.Join(gatewayNames, ","), config.Spec.Gateways),
		}
	} else {
		condition = successCondition(gatewayNames)
	}

	return c.updateSubmarinerConfigStatus(ctx, recorder, config, &condition)
}

func failedCondition(formatMsg string, args ...interface{}) metav1.Condition {
	return metav1.Condition{
		Type:    submarinerGatewayCondition,
		Status:  metav1.ConditionFalse,
		Reason:  "Failure",
		Message: fmt.Sprintf(formatMsg, args...),
	}
}

func successCondition(gatewayNames []string) metav1.Condition {
	return metav1.Condition{
		Type:   submarinerGatewayCondition,
		Status: metav1.ConditionTrue,
		Reason: "Success",
		Message: fmt.Sprintf("%d node(s) (%q) are labeled as gateways", len(gatewayNames),
			strings.Join(gatewayNames, ",")),
	}
}

func isSpokePrepared(cloudName string) bool {
	if cloudName == "GCP" || cloudName == "OpenStack" || cloudName == "Azure" {
		return true
	}

	return false
}
