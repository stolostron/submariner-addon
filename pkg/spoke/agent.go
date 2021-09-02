package spoke

import (
	"context"
	"errors"
	"time"

	"github.com/spf13/cobra"
	"open-cluster-management.io/addon-framework/pkg/lease"

	configclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	configinformers "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/informers/externalversions"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"
	"github.com/open-cluster-management/submariner-addon/pkg/spoke/submarineragent"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"
	addoninformers "open-cluster-management.io/api/client/addon/informers/externalversions"

	"github.com/openshift/library-go/pkg/controller/controllercmd"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const defaultInstallationNamespace = "submariner-operator"

var (
	submarinerGVR = schema.GroupVersionResource{
		Group:    "submariner.io",
		Version:  "v1alpha1",
		Resource: "submariners",
	}
	subscriptionGVR = schema.GroupVersionResource{
		Group:    "operators.coreos.com",
		Version:  "v1alpha1",
		Resource: "subscriptions",
	}
)

type AgentOptions struct {
	InstallationNamespace string
	HubKubeconfigFile     string
	ClusterName           string
}

func NewAgentOptions() *AgentOptions {
	return &AgentOptions{}
}

func (o *AgentOptions) AddFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	flags.StringVar(&o.HubKubeconfigFile, "hub-kubeconfig", o.HubKubeconfigFile, "Location of kubeconfig file to connect to hub cluster.")
	flags.StringVar(&o.ClusterName, "cluster-name", o.ClusterName, "Name of managed cluster.")
}

func (o *AgentOptions) Complete() {
	o.InstallationNamespace = helpers.GetCurrentNamespace(defaultInstallationNamespace)
}

func (o *AgentOptions) Validate() error {
	if o.HubKubeconfigFile == "" {
		return errors.New("hub-kubeconfig is required")
	}

	if o.ClusterName == "" {
		return errors.New("cluster name is empty")
	}

	return nil
}

func (o *AgentOptions) RunAgent(ctx context.Context, controllerContext *controllercmd.ControllerContext) error {
	o.Complete()

	if err := o.Validate(); err != nil {
		return err
	}

	hubRestConfig, err := clientcmd.BuildConfigFromFlags("" /* leave masterurl as empty */, o.HubKubeconfigFile)
	if err != nil {
		return err
	}

	addOnHubKubeClient, err := addonclient.NewForConfig(hubRestConfig)
	if err != nil {
		return err
	}

	configHubKubeClient, err := configclient.NewForConfig(hubRestConfig)
	if err != nil {
		return err
	}

	spokeKubeClient, err := kubernetes.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	spokeDynamicClient, err := dynamic.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	addOnInformers := addoninformers.NewSharedInformerFactoryWithOptions(addOnHubKubeClient, 10*time.Minute, addoninformers.WithNamespace(o.ClusterName))
	configInformers := configinformers.NewSharedInformerFactoryWithOptions(configHubKubeClient, 10*time.Minute, configinformers.WithNamespace(o.ClusterName))

	spokeKubeInformers := informers.NewSharedInformerFactoryWithOptions(spokeKubeClient, 10*time.Minute, informers.WithNamespace(o.InstallationNamespace))
	// TODO if submariner provides the informer in future, we will use it instead of dynamic informer
	dynamicInformers := dynamicinformer.NewFilteredDynamicSharedInformerFactory(spokeDynamicClient, 10*time.Minute, o.InstallationNamespace, nil)

	submarinerConfigController := submarineragent.NewSubmarinerConfigController(
		o.ClusterName,
		spokeKubeClient,
		addOnHubKubeClient,
		configHubKubeClient,
		spokeKubeInformers.Core().V1().Nodes(),
		addOnInformers.Addon().V1alpha1().ManagedClusterAddOns(),
		configInformers.Submarineraddon().V1alpha1().SubmarinerConfigs(),
		controllerContext.EventRecorder,
	)

	gatewaysStatusController := submarineragent.NewGatewaysStatusController(
		o.ClusterName,
		addOnHubKubeClient,
		addOnInformers.Addon().V1alpha1().ManagedClusterAddOns(),
		spokeKubeInformers.Core().V1().Nodes(),
		controllerContext.EventRecorder,
	)

	deploymentStatusController := submarineragent.NewDeploymentStatusController(
		o.ClusterName,
		o.InstallationNamespace,
		addOnHubKubeClient,
		addOnInformers.Addon().V1alpha1().ManagedClusterAddOns(),
		spokeKubeInformers.Apps().V1().DaemonSets(),
		spokeKubeInformers.Apps().V1().Deployments(),
		dynamicInformers.ForResource(subscriptionGVR),
		controllerContext.EventRecorder,
	)

	connectionsStatusController := submarineragent.NewConnectionsStatusController(o.ClusterName, addOnHubKubeClient,
		dynamicInformers.ForResource(submarinerGVR), controllerContext.EventRecorder)

	go addOnInformers.Start(ctx.Done())
	go configInformers.Start(ctx.Done())
	go spokeKubeInformers.Start(ctx.Done())
	go dynamicInformers.Start(ctx.Done())

	go submarinerConfigController.Run(ctx, 1)
	go gatewaysStatusController.Run(ctx, 1)
	go deploymentStatusController.Run(ctx, 1)
	go connectionsStatusController.Run(ctx, 1)

	// start lease updater
	leaseUpdater := lease.NewLeaseUpdater(
		spokeKubeClient,
		helpers.SubmarinerAddOnName,
		o.InstallationNamespace,
	)
	go leaseUpdater.Start(ctx)

	<-ctx.Done()
	return nil
}
