package submarineragent

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	configclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	configinformer "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/informers/externalversions/submarinerconfig/v1alpha1"
	configlister "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/listers/submarinerconfig/v1alpha1"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	operatorhelpers "github.com/openshift/library-go/pkg/operator/v1helpers"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

const (
	submarinerConfigFinalizer     = "submarineraddon.open-cluster-management.io/config-addon-cleanup"
	submarinerGatewayUDPPortLabel = "gateway.submariner.io/udp-port"
	udpPortLabelConditionType     = "IPSecNATTPortLabled"
)

// submarinerAgentConfigController watches the SubmarinerConfigs API on the hub cluster and apply
// the related configuration on the manged cluster
type submarinerAgentConfigController struct {
	kubeClient   kubernetes.Interface
	configClient configclient.Interface
	nodeLister   corev1lister.NodeLister
	configLister configlister.SubmarinerConfigLister
	clusterName  string
}

// NewSubmarinerAgentConfigController returns an instance of submarinerAgentConfigController
func NewSubmarinerAgentConfigController(
	clusterName string,
	kubeClient kubernetes.Interface,
	configClient configclient.Interface,
	nodeInformer corev1informers.NodeInformer,
	configInformer configinformer.SubmarinerConfigInformer,
	recorder events.Recorder) factory.Controller {
	c := &submarinerAgentConfigController{
		kubeClient:   kubeClient,
		configClient: configClient,
		nodeLister:   nodeInformer.Lister(),
		configLister: configInformer.Lister(),
		clusterName:  clusterName,
	}

	return factory.New().
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			accessor, _ := meta.Accessor(obj)
			if accessor.GetName() != helpers.SubmarinerConfigName {
				return ""
			}
			return accessor.GetNamespace() + "/" + accessor.GetName()
		}, configInformer.Informer()).
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			accessor, _ := meta.Accessor(obj)
			return accessor.GetName()
		}, nodeInformer.Informer()).
		WithSync(c.sync).
		ToController("SubmarinerAgentConfigController", recorder)
}

func (c *submarinerAgentConfigController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	key := syncCtx.QueueKey()

	klog.V(4).Infof("Submariner agent config controller is reconciling, queue key: %s", key)

	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		// ignore bad format key
		return nil
	}

	// the sync is triggered by change of Node
	if namespace == "" {
		config, err := c.configLister.SubmarinerConfigs(c.clusterName).Get(helpers.SubmarinerConfigName)
		if errors.IsNotFound(err) {
			// the config not found, could be deleted, ignore
			return nil
		}
		if err != nil {
			return err
		}

		node, err := c.nodeLister.Get(name)
		if errors.IsNotFound(err) {
			// the node not found, could be deleted, ignore
			return nil
		}
		if err != nil {
			return err
		}

		if err := c.syncNode(ctx, config.DeepCopy(), node.DeepCopy()); err != nil {
			return err
		}
		return c.updateConfigStatus(config.DeepCopy())
	}

	config, err := c.configLister.SubmarinerConfigs(namespace).Get(name)
	if errors.IsNotFound(err) {
		// the config not found, could be deleted, ignore
		return nil
	}
	if err != nil {
		return err
	}

	if err := c.syncConfig(ctx, config.DeepCopy()); err != nil {
		return err
	}

	return c.updateConfigStatus(config.DeepCopy())
}

func (c *submarinerAgentConfigController) syncConfig(ctx context.Context, config *configv1alpha1.SubmarinerConfig) error {
	if config.DeletionTimestamp.IsZero() {
		hasFinalizer := false
		for i := range config.Finalizers {
			if config.Finalizers[i] == submarinerConfigFinalizer {
				hasFinalizer = true
				break
			}
		}
		if !hasFinalizer {
			config.Finalizers = append(config.Finalizers, submarinerConfigFinalizer)
			_, err := c.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(config.Namespace).Update(ctx, config, metav1.UpdateOptions{})
			return err
		}
	}

	// config is deleting, we remove its related resources
	if !config.DeletionTimestamp.IsZero() {
		if err := c.removeConfigFinalizer(ctx, config); err != nil {
			return err
		}

		nodes, err := c.getPortLabelNodes()
		if err != nil {
			return err
		}

		errs := []error{}
		for _, node := range nodes {
			errs = append(errs, c.removeUDPPortLabelFromNode(ctx, node.DeepCopy()))
		}

		return operatorhelpers.NewMultiLineAggregate(errs)
	}

	nodes, err := c.nodeLister.List(labels.Everything())
	if err != nil {
		return err
	}
	for _, node := range nodes {
		if err := c.syncNode(ctx, config, node.DeepCopy()); err != nil {
			return err
		}
	}

	return nil
}

func (c *submarinerAgentConfigController) syncNode(ctx context.Context, config *configv1alpha1.SubmarinerConfig, node *corev1.Node) error {
	_, existed := node.Labels[submarinerGatewayLabel]
	if !existed {
		return c.removeUDPPortLabelFromNode(ctx, node)
	}

	// the node is submariner gateway node, we add the port label to it
	nattPort := strconv.Itoa(config.Spec.IPSecNATTPort)
	if nattPort == "0" {
		nattPort = "4500"
	}
	// the port label has been labeled
	if port, existed := node.Labels[submarinerGatewayUDPPortLabel]; existed && port == nattPort {
		return nil
	}

	node.Labels[submarinerGatewayUDPPortLabel] = nattPort
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		_, err := c.kubeClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
		return err
	})
}

func (c *submarinerAgentConfigController) removeUDPPortLabelFromNode(ctx context.Context, node *corev1.Node) error {
	_, existed := node.Labels[submarinerGatewayUDPPortLabel]
	if !existed {
		// the node dose not have port label, return
		return nil
	}

	// remove the port label
	delete(node.Labels, submarinerGatewayUDPPortLabel)

	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		_, err := c.kubeClient.CoreV1().Nodes().Update(ctx, node, metav1.UpdateOptions{})
		return err
	})
}

func (c *submarinerAgentConfigController) updateConfigStatus(config *configv1alpha1.SubmarinerConfig) error {
	nodes, err := c.getPortLabelNodes()
	if err != nil {
		return err
	}

	nodeNames := []string{}
	for _, node := range nodes {
		nodeNames = append(nodeNames, node.Name)
	}

	condition := metav1.Condition{
		Type:    udpPortLabelConditionType,
		Status:  metav1.ConditionTrue,
		Reason:  "IPSecNATTPortLabled",
		Message: fmt.Sprintf("The IPSec NAT-T port is labled on gateway nodes %q", strings.Join(nodeNames, ",")),
	}

	if len(nodeNames) == 0 {
		condition.Status = metav1.ConditionFalse
		condition.Reason = "IPSecNATTPortNotLabled"
		condition.Message = fmt.Sprintf("The IPSec NAT-T port is not labled")
	}

	// update config status
	_, _, err = helpers.UpdateSubmarinerConfigStatus(
		c.configClient,
		config.Namespace,
		config.Name,
		helpers.UpdateSubmarinerConfigConditionFn(condition),
	)

	return err
}

func (c *submarinerAgentConfigController) getPortLabelNodes() ([]*corev1.Node, error) {
	requirement, err := labels.NewRequirement(submarinerGatewayUDPPortLabel, selection.Exists, []string{})
	if err != nil {
		return nil, err
	}
	return c.nodeLister.List(labels.Everything().Add(*requirement))
}

func (c *submarinerAgentConfigController) removeConfigFinalizer(ctx context.Context, config *configv1alpha1.SubmarinerConfig) error {
	copiedFinalizers := []string{}
	for i := range config.Finalizers {
		if config.Finalizers[i] == submarinerConfigFinalizer {
			continue
		}
		copiedFinalizers = append(copiedFinalizers, config.Finalizers[i])
	}

	if len(config.Finalizers) != len(copiedFinalizers) {
		config.Finalizers = copiedFinalizers
		_, err := c.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(config.Namespace).Update(ctx, config, metav1.UpdateOptions{})
		return err
	}

	return nil
}
