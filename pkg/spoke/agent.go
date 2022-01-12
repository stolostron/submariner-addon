package spoke

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/spf13/cobra"
	configclient "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	configinformers "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/informers/externalversions"
	"github.com/stolostron/submariner-addon/pkg/cloud"
	"github.com/stolostron/submariner-addon/pkg/constants"
	"github.com/stolostron/submariner-addon/pkg/resource"
	"github.com/stolostron/submariner-addon/pkg/spoke/submarineragent"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"open-cluster-management.io/addon-framework/pkg/lease"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"
	addoninformers "open-cluster-management.io/api/client/addon/informers/externalversions"
	workclient "open-cluster-management.io/api/client/work/clientset/versioned"
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
	o.InstallationNamespace = resource.GetCurrentNamespace(defaultInstallationNamespace)
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

	hubClient, err := kubernetes.NewForConfig(hubRestConfig)
	if err != nil {
		return err
	}

	restMapper, err := buildRestMapper(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	workClient, err := workclient.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	addOnInformers := addoninformers.NewSharedInformerFactoryWithOptions(addOnHubKubeClient, 10*time.Minute,
		addoninformers.WithNamespace(o.ClusterName))
	configInformers := configinformers.NewSharedInformerFactoryWithOptions(configHubKubeClient, 10*time.Minute,
		configinformers.WithNamespace(o.ClusterName))

	spokeKubeInformers := informers.NewSharedInformerFactoryWithOptions(spokeKubeClient, 10*time.Minute,
		informers.WithNamespace(o.InstallationNamespace))
	// TODO if submariner provides the informer in future, we will use it instead of dynamic informer
	dynamicInformers := dynamicinformer.NewFilteredDynamicSharedInformerFactory(spokeDynamicClient, 10*time.Minute, o.InstallationNamespace,
		nil)

	submarinerConfigController := submarineragent.NewSubmarinerConfigController(&submarineragent.SubmarinerConfigControllerInput{
		ClusterName:          o.ClusterName,
		KubeClient:           spokeKubeClient,
		ConfigClient:         configHubKubeClient,
		NodeInformer:         spokeKubeInformers.Core().V1().Nodes(),
		AddOnInformer:        addOnInformers.Addon().V1alpha1().ManagedClusterAddOns(),
		ConfigInformer:       configInformers.Submarineraddon().V1alpha1().SubmarinerConfigs(),
		CloudProviderFactory: cloud.NewProviderFactory(restMapper, spokeKubeClient, workClient, spokeDynamicClient, hubClient),
		Recorder:             controllerContext.EventRecorder,
	})

	gatewaysStatusController := submarineragent.NewGatewaysStatusController(
		o.ClusterName,
		addOnHubKubeClient,
		spokeKubeInformers.Core().V1().Nodes(),
		controllerContext.EventRecorder,
	)

	deploymentStatusController := submarineragent.NewDeploymentStatusController(o.ClusterName, o.InstallationNamespace,
		addOnHubKubeClient, spokeKubeInformers.Apps().V1().DaemonSets(), spokeKubeInformers.Apps().V1().Deployments(),
		dynamicInformers.ForResource(subscriptionGVR), controllerContext.EventRecorder)

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
		constants.SubmarinerAddOnName,
		o.InstallationNamespace,
	)
	go leaseUpdater.Start(ctx)

	<-ctx.Done()

	return nil
}

func buildRestMapper(restConfig *rest.Config) (meta.RESTMapper, error) {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("error creating discovery client: %w", err)
	}

	groupResources, err := restmapper.GetAPIGroupResources(discoveryClient)
	if err != nil {
		return nil, fmt.Errorf("error retrieving API group resources: %w", err)
	}

	return restmapper.NewDiscoveryRESTMapper(groupResources), nil
}
