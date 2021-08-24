package submarineragent

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	configclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	configinformer "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/informers/externalversions/submarinerconfig/v1alpha1"
	configlister "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/listers/submarinerconfig/v1alpha1"
	gcpclient "github.com/open-cluster-management/submariner-addon/pkg/cloud/gcp/client"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"
	"k8s.io/apimachinery/pkg/util/sets"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"
	addoninformerv1alpha1 "open-cluster-management.io/api/client/addon/informers/externalversions/addon/v1alpha1"
	addonlisterv1alpha1 "open-cluster-management.io/api/client/addon/listers/addon/v1alpha1"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	operatorhelpers "github.com/openshift/library-go/pkg/operator/v1helpers"

	compute "google.golang.org/api/compute/v1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/util/retry"
)

// TODO expose this as a flag to allow user to specify their zone label
var defaultZoneLabel = ""

const (
	submarinerAddOnFinalizer  = "submarineraddon.open-cluster-management.io/submariner-addon-agent-cleanup"
	submarinerConfigFinalizer = "submarineraddon.open-cluster-management.io/config-addon-cleanup"
)

const submarinerGatewayCondition = "SubmarinerGatewaysLabeled"

const (
	submarinerUDPPortLabel = "gateway.submariner.io/udp-port"
	workerNodeLabel        = "node-role.kubernetes.io/worker"
	gcpZoneLabel           = "failure-domain.beta.kubernetes.io/zone"
)

const (
	ocpMachineAPINamespace = "openshift-machine-api"
	gcpCredentialsSecret   = "gcp-cloud-credentials"
)

type nodeLabelSelector struct {
	label string
	op    selection.Operator
}

// submarinerConfigController watches the SubmarinerConfigs API on the hub cluster and apply
// the related configuration on the manged cluster
type submarinerConfigController struct {
	kubeClient   kubernetes.Interface
	addOnClient  addonclient.Interface
	configClient configclient.Interface
	nodeLister   corev1lister.NodeLister
	addOnLister  addonlisterv1alpha1.ManagedClusterAddOnLister
	configLister configlister.SubmarinerConfigLister
	clusterName  string
}

// NewSubmarinerConfigController returns an instance of submarinerAgentConfigController
func NewSubmarinerConfigController(
	clusterName string,
	kubeClient kubernetes.Interface,
	addOnClient addonclient.Interface,
	configClient configclient.Interface,
	nodeInformer corev1informers.NodeInformer,
	addOnInformer addoninformerv1alpha1.ManagedClusterAddOnInformer,
	configInformer configinformer.SubmarinerConfigInformer,
	recorder events.Recorder) factory.Controller {
	c := &submarinerConfigController{
		kubeClient:   kubeClient,
		addOnClient:  addOnClient,
		configClient: configClient,
		nodeLister:   nodeInformer.Lister(),
		addOnLister:  addOnInformer.Lister(),
		configLister: configInformer.Lister(),
		clusterName:  clusterName,
	}

	return factory.New().
		WithFilteredEventsInformers(func(obj interface{}) bool {
			metaObj := obj.(metav1.Object)
			if metaObj.GetName() == helpers.SubmarinerAddOnName {
				return true
			}
			return false
		}, addOnInformer.Informer()).
		WithFilteredEventsInformers(func(obj interface{}) bool {
			metaObj := obj.(metav1.Object)
			if metaObj.GetName() == helpers.SubmarinerConfigName {
				return true
			}
			return false
		}, configInformer.Informer()).
		WithFilteredEventsInformers(func(obj interface{}) bool {
			metaObj := obj.(metav1.Object)
			// only handle the changes of worker nodes
			if _, has := metaObj.GetLabels()[workerNodeLabel]; has {
				return true
			}
			return false
		}, nodeInformer.Informer()).
		WithSync(c.sync).
		ToController("SubmarinerAgentConfigController", recorder)
}

func (c *submarinerConfigController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	addOn, err := c.addOnLister.ManagedClusterAddOns(c.clusterName).Get(helpers.SubmarinerAddOnName)
	if errors.IsNotFound(err) {
		// the addon not found, could be deleted, ignore
		return nil
	}
	if err != nil {
		return err
	}

	config, err := c.configLister.SubmarinerConfigs(c.clusterName).Get(helpers.SubmarinerConfigName)
	if errors.IsNotFound(err) {
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
		// if the addon is deleted before config, clean up gateways config on the manged cluster

		if config.Status.ManagedClusterInfo.Platform == "AWS" {
			// for AWS, the gateway configuration will be operated on the hub, do nothing
			return nil
		}

		condition := metav1.Condition{
			Type:    submarinerGatewayCondition,
			Status:  metav1.ConditionFalse,
			Reason:  "ManagedClusterAddOnDeleted",
			Message: "There are no nodes labeled as gateways",
		}

		err = c.removeAllGateways(ctx, config)
		if err != nil {
			condition = failedCondition("Failed to unlabel the gatway nodes: %v", err)
		}

		updateErr := c.updateSubmarinerConfigStatus(syncCtx.Recorder(), config, condition)

		if err != nil {
			return err
		}

		return updateErr
	}

	if config.Status.ManagedClusterInfo.Platform == "AWS" {
		// for AWS, the gateway configuration will be operated on the hub
		// count the gateways status on the managed cluster and report it to the hub
		return c.updateGatewayStatus(syncCtx.Recorder(), config)
	}

	// config is deleting from hub, remove its related resources
	// TODO: add finalizer in next release
	if !config.DeletionTimestamp.IsZero() {
		return c.removeAllGateways(ctx, config)
	}

	// ensure the expected count of gateways
	condition, err := c.ensureGateways(ctx, config)

	updateErr := c.updateSubmarinerConfigStatus(syncCtx.Recorder(), config, condition)

	if err != nil {
		return err
	}

	return updateErr
}

func (c *submarinerConfigController) updateSubmarinerConfigStatus(recorder events.Recorder, config *configv1alpha1.SubmarinerConfig,
	condition metav1.Condition) error {
	updatedStatus, updated, err := helpers.UpdateSubmarinerConfigStatus(c.configClient, config.Namespace, config.Name,
		helpers.UpdateSubmarinerConfigConditionFn(condition))

	if updated {
		recorder.Eventf("SubmarinerConfigStatusUpdated", "Updated status conditions:  %#v", updatedStatus.Conditions)
	}

	return err
}

func (c *submarinerConfigController) ensureGateways(ctx context.Context, config *configv1alpha1.SubmarinerConfig) (metav1.Condition, error) {
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

		removed, err = c.removeGateways(ctx, config, currentGateways, -requiredGateways)

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
	if config.Status.ManagedClusterInfo.Platform == "GCP" {
		gc, instance, err := c.getGCPInstance(node)
		if err != nil {
			return err
		}

		if err := gc.EnablePublicIP(instance); err != nil {
			return err
		}
	}

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

func (c *submarinerConfigController) unlabelNode(ctx context.Context, config *configv1alpha1.SubmarinerConfig, node *corev1.Node) error {
	if config.Status.ManagedClusterInfo.Platform == "GCP" {
		gc, instance, err := c.getGCPInstance(node)
		if err != nil {
			return err
		}

		if err := gc.DisablePublicIP(instance); err != nil {
			return err
		}
	}

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
	expectedGateways int) ([]string, error) {
	var zoneLabel string
	// currently only gcp is supported
	switch config.Status.ManagedClusterInfo.Platform {
	case "GCP":
		zoneLabel = gcpZoneLabel
	default:
		// for other non-public cloud platform (vsphere) or native k8s
		zoneLabel = defaultZoneLabel
	}

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

func (c *submarinerConfigController) removeGateways(ctx context.Context, config *configv1alpha1.SubmarinerConfig,
	gateways []*corev1.Node, removedGateways int) ([]string, error) {
	if len(gateways) < removedGateways {
		removedGateways = len(gateways)
	}

	errs := []error{}
	removed := []string{}
	for i := 0; i < removedGateways; i++ {
		removed = append(removed, gateways[i].Name)
		errs = append(errs, c.unlabelNode(ctx, config, gateways[i]))
	}

	return removed, operatorhelpers.NewMultiLineAggregate(errs)
}

func (c *submarinerConfigController) removeAllGateways(ctx context.Context, config *configv1alpha1.SubmarinerConfig) error {
	gateways, err := c.getLabeledNodes(nodeLabelSelector{submarinerGatewayLabel, selection.Exists})
	if err != nil {
		return err
	}
	_, err = c.removeGateways(ctx, config, gateways, len(gateways))
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
			count = count + 1
		}
		nodeIndex = nodeIndex + 1
	}

	return gateways, nil
}

func (c *submarinerConfigController) updateGatewayStatus(recorder events.Recorder, config *configv1alpha1.SubmarinerConfig) error {
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

	return c.updateSubmarinerConfigStatus(recorder, config, condition)
}

//TODO consider to put this into the cloud-library
func (c *submarinerConfigController) getGCPInstance(node *corev1.Node) (gcpclient.Interface, *compute.Instance, error) {
	gc, err := gcpclient.NewOauth2Client(c.kubeClient, ocpMachineAPINamespace, gcpCredentialsSecret)
	if err != nil {
		return nil, nil, err
	}

	zone := node.Labels[gcpZoneLabel]
	instanceName := strings.Split(node.Name, ".")[0]
	instance, err := gc.GetInstance(zone, instanceName)
	if err != nil {
		return nil, nil, err
	}

	return gc, instance, nil
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
