package submarineragent

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	addonclient "github.com/open-cluster-management/api/client/addon/clientset/versioned"
	addoninformerv1alpha1 "github.com/open-cluster-management/api/client/addon/informers/externalversions/addon/v1alpha1"
	addonlisterv1alpha1 "github.com/open-cluster-management/api/client/addon/listers/addon/v1alpha1"
	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	configclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	configinformer "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/informers/externalversions/submarinerconfig/v1alpha1"
	configlister "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/listers/submarinerconfig/v1alpha1"
	gcpclient "github.com/open-cluster-management/submariner-addon/pkg/cloud/gcp/client"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"

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

const submarinerGatewayCondition = "SubmarinerGatewayLabled"

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

	// addon is deleting from the hub, remove its related resources on the managed cluster
	// TODO: add finalizer in next release
	if !addOn.DeletionTimestamp.IsZero() {
		// if the addon is deleted before config, clean up gateways config on the manged cluster
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

		if config.Status.ManagedClusterInfo.Platform == "AWS" {
			// for AWS, the gateway configuration will be operated on the hub, do nothing
			return nil
		}

		msg := "The gatways labels are unlabled from nodes after addon was deleted"
		if err := c.removeAllGateways(ctx, config); err != nil {
			msg = fmt.Sprintf("Failed to unlable the gatway labels from nodes: %v", err)
		}

		_, _, err = helpers.UpdateSubmarinerConfigStatus(
			c.configClient,
			config.Namespace,
			config.Name,
			helpers.UpdateSubmarinerConfigConditionFn(metav1.Condition{
				Type:    submarinerGatewayCondition,
				Status:  metav1.ConditionFalse,
				Reason:  "SubmarinerGatewayUnlabeled",
				Message: msg,
			}),
		)
		return err
	}

	config, err := c.configLister.SubmarinerConfigs(c.clusterName).Get(helpers.SubmarinerConfigName)
	if errors.IsNotFound(err) {
		// the config not found, could be deleted, ignore
		return nil
	}
	if err != nil {
		return err
	}

	if config.Status.ManagedClusterInfo.Platform == "" {
		// no managed cluster info, do nothing
		return nil
	}

	if config.Status.ManagedClusterInfo.Platform == "AWS" {
		// for AWS, the gateway configuration will be operated on the hub
		// count the gateways status on the managed cluster and report it to the hub
		return c.updateGatewayStatus(ctx, config)
	}

	// config is deleting from hub, remove its related resources
	// TODO: add finalizer in next release
	if !config.DeletionTimestamp.IsZero() {
		return c.removeAllGateways(ctx, config)
	}

	// ensure the expected count of gateways
	gatewayNames, err := c.ensureGateways(ctx, config)
	// fixed the order of gateway names
	sort.Strings(gatewayNames)

	// update config status according to current gateways
	condition := metav1.Condition{
		Type:    submarinerGatewayCondition,
		Status:  metav1.ConditionTrue,
		Reason:  "SubmarinerGatewayLabeled",
		Message: fmt.Sprintf("The nodes (%q) are labeled to gateways", strings.Join(gatewayNames, ",")),
	}

	if err != nil {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "SubmarinerGatewayNotLabeled"
		condition.Message = fmt.Sprintf("Failed to prepare gateways: %v", err)
	}

	_, _, err = helpers.UpdateSubmarinerConfigStatus(
		c.configClient,
		config.Namespace,
		config.Name,
		helpers.UpdateSubmarinerConfigConditionFn(condition),
	)

	return err
}

func (c *submarinerConfigController) ensureGateways(ctx context.Context, config *configv1alpha1.SubmarinerConfig) ([]string, error) {
	if config.Spec.Gateways < 1 {
		return nil, fmt.Errorf("the count of gateways must be equal or greater than 1")
	}

	currentGateways, err := c.getLabeledNodes(
		nodeLabelSelector{submarinerGatewayLabel, selection.Exists},
	)
	if err != nil {
		return nil, err
	}

	currentGatewayNames := []string{}
	for _, gateway := range currentGateways {
		currentGatewayNames = append(currentGatewayNames, gateway.Name)
	}

	requiredGateways := config.Spec.Gateways - len(currentGateways)
	if requiredGateways == 0 {
		// current count of gateways are expected, ensure that the gateways are labled with nat-t lables
		errs := []error{}
		for _, gateway := range currentGateways {
			errs = append(errs, c.labelNode(ctx, config, gateway))
		}

		return currentGatewayNames, operatorhelpers.NewMultiLineAggregate(errs)
	}

	if requiredGateways > 0 {
		// require to create more gateways
		added, err := c.addGateways(ctx, config, requiredGateways)
		if err != nil {
			return nil, err
		}

		for _, gateway := range added {
			currentGatewayNames = append(currentGatewayNames, gateway.Name)
		}

		return currentGatewayNames, nil
	}

	// require to remove gateways
	removed, err := c.removeGateways(ctx, config, currentGateways, -requiredGateways)
	if err != nil {
		return nil, err
	}

	remainingGatewaysNames := []string{}
	for _, name := range currentGatewayNames {
		isNotRemoved := true

		for _, gateway := range removed {
			if name == gateway.Name {
				isNotRemoved = false
				break
			}
		}

		if isNotRemoved {
			remainingGatewaysNames = append(remainingGatewaysNames, name)
		}
	}

	return remainingGatewaysNames, nil
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

	copied := node.DeepCopy()
	copied.Labels[submarinerGatewayLabel] = "true"
	copied.Labels[submarinerUDPPortLabel] = nattPort

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		_, err := c.kubeClient.CoreV1().Nodes().Update(ctx, copied, metav1.UpdateOptions{})
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

	copied := node.DeepCopy()
	// remove the gateway and port label
	delete(copied.Labels, submarinerGatewayLabel)
	delete(copied.Labels, submarinerUDPPortLabel)

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		_, err := c.kubeClient.CoreV1().Nodes().Update(ctx, copied, metav1.UpdateOptions{})
		return err
	})
}

func (c *submarinerConfigController) addGateways(
	ctx context.Context, config *configv1alpha1.SubmarinerConfig, expectedGateways int) ([]*corev1.Node, error) {
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
		return nil, err
	}

	errs := []error{}
	for _, gateway := range gateways {
		errs = append(errs, c.labelNode(ctx, config, gateway))
	}

	return gateways, operatorhelpers.NewMultiLineAggregate(errs)
}

func (c *submarinerConfigController) removeGateways(
	ctx context.Context,
	config *configv1alpha1.SubmarinerConfig,
	gateways []*corev1.Node, removedGateways int) ([]*corev1.Node, error) {
	if len(gateways) < removedGateways {
		removedGateways = len(gateways)
	}

	errs := []error{}
	removed := []*corev1.Node{}
	for i := 0; i < removedGateways; i++ {
		removed = append(removed, gateways[i])
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
		return nil, fmt.Errorf("the candidate worker nodes (%d) are insufficient for required gateways (%d)", len(workers), expected)
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

func (c *submarinerConfigController) updateGatewayStatus(ctx context.Context, config *configv1alpha1.SubmarinerConfig) error {
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

	condition := metav1.Condition{
		Type:    submarinerGatewayCondition,
		Status:  metav1.ConditionTrue,
		Reason:  "SubmarinerGatewayLabeled",
		Message: fmt.Sprintf("The nodes (%q) are labeled to gateways", strings.Join(gatewayNames, ",")),
	}

	if config.Spec.Gateways != len(gateways) {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "SubmarinerGatewayNotLabeled"
		condition.Message = fmt.Sprintf("Expected %d gateways, but got %d gateways (%q)",
			config.Spec.Gateways, len(gateways), strings.Join(gatewayNames, ","))
	}

	_, _, err = helpers.UpdateSubmarinerConfigStatus(
		c.configClient,
		config.Namespace,
		config.Name,
		helpers.UpdateSubmarinerConfigConditionFn(condition),
	)

	return err
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
