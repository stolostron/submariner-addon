package spoke

import (
	"context"
	"errors"
	"time"

	"github.com/open-cluster-management/addon-framework/pkg/lease"
	"github.com/spf13/cobra"

	addonclient "github.com/open-cluster-management/api/client/addon/clientset/versioned"
	addoninformers "github.com/open-cluster-management/api/client/addon/informers/externalversions"
	configclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	configinformers "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/informers/externalversions"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"
	"github.com/open-cluster-management/submariner-addon/pkg/spoke/submarineragent"

	"github.com/openshift/library-go/pkg/controller/controllercmd"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

const defaultInstallationNamespace = "submariner-operator"

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

	spokeKubeInformers := informers.NewSharedInformerFactory(spokeKubeClient, 10*time.Minute)
	// TODO if submariner provides the informer in future, we will use it instead of dynamic informer
	dynamicInformers := dynamicinformer.NewFilteredDynamicSharedInformerFactory(spokeDynamicClient, 10*time.Minute, submarineragent.SubmarinerOperatorNamespace, nil)

	submarinerGVR, _ := schema.ParseResourceArg("submariners.v1alpha1.submariner.io")
	submarinerAgentStatusController := submarineragent.NewSubmarinerAgentStatusController(
		o.ClusterName,
		addOnHubKubeClient,
		addOnInformers.Addon().V1alpha1().ManagedClusterAddOns(),
		spokeKubeInformers.Core().V1().Nodes(),
		dynamicInformers.ForResource(*submarinerGVR),
		controllerContext.EventRecorder,
	)

	submarinerAgentConfigController := submarineragent.NewSubmarinerAgentConfigController(
		o.ClusterName,
		spokeKubeClient,
		addOnHubKubeClient,
		configHubKubeClient,
		spokeKubeInformers.Core().V1().Nodes(),
		addOnInformers.Addon().V1alpha1().ManagedClusterAddOns(),
		configInformers.Submarineraddon().V1alpha1().SubmarinerConfigs(),
		controllerContext.EventRecorder,
	)

	go addOnInformers.Start(ctx.Done())
	go configInformers.Start(ctx.Done())
	go spokeKubeInformers.Start(ctx.Done())
	go dynamicInformers.Start(ctx.Done())
	go submarinerAgentStatusController.Run(ctx, 1)
	go submarinerAgentConfigController.Run(ctx, 1)

	// start lease updater
	leaseUpdater := lease.NewLeaseUpdater(
		spokeKubeClient,
		submarineragent.SubmarinerAddOnName,
		o.InstallationNamespace,
	)
	go leaseUpdater.Start(ctx)

	<-ctx.Done()
	return nil
}
