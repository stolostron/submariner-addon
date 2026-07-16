package spoke

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/serviceability"
	"github.com/spf13/cobra"
	configclient "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	configinformers "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/informers/externalversions"
	"github.com/stolostron/submariner-addon/pkg/cloud"
	"github.com/stolostron/submariner-addon/pkg/constants"
	"github.com/stolostron/submariner-addon/pkg/resource"
	"github.com/stolostron/submariner-addon/pkg/spoke/submarineragent"
	"github.com/stolostron/submariner-addon/pkg/version"
	submarinermv1 "github.com/submariner-io/submariner/pkg/apis/submariner.io/v1"
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
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	"open-cluster-management.io/addon-framework/pkg/lease"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"
	addoninformers "open-cluster-management.io/api/client/addon/informers/externalversions"
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
	routeAgentGVR = submarinermv1.SchemeGroupVersion.WithResource("routeagents")
)

type AgentOptions struct {
	InstallationNamespace string
	HubKubeconfigFile     string
	HubRestConfig         *rest.Config
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
	if o.HubRestConfig == nil && o.HubKubeconfigFile == "" {
		return errors.New("hub-kubeconfig is required")
	}

	if o.ClusterName == "" {
		return errors.New("cluster name is empty")
	}

	return nil
}

func (o *AgentOptions) RunAgent(ctx context.Context, spokeConfig *rest.Config) error {
	o.Complete()

	if err := o.Validate(); err != nil {
		return err
	}

	var err error

	hubRestConfig := o.HubRestConfig

	if hubRestConfig == nil {
		hubRestConfig, err = clientcmd.BuildConfigFromFlags("" /* leave masterurl as empty */, o.HubKubeconfigFile)
		if err != nil {
			return fmt.Errorf("error creating hub REST config: %w", err)
		}
	}

	addOnHubKubeClient, err := addonclient.NewForConfig(hubRestConfig)
	if err != nil {
		return fmt.Errorf("error creating addon client: %w", err)
	}

	configHubKubeClient, err := configclient.NewForConfig(hubRestConfig)
	if err != nil {
		return fmt.Errorf("error creating hub kube client: %w", err)
	}

	spokeKubeClient, err := kubernetes.NewForConfig(spokeConfig)
	if err != nil {
		return fmt.Errorf("error creating spoke kube client: %w", err)
	}

	spokeDynamicClient, err := dynamic.NewForConfig(spokeConfig)
	if err != nil {
		return fmt.Errorf("error creating spoke dynamic client: %w", err)
	}

	hubClient, err := kubernetes.NewForConfig(hubRestConfig)
	if err != nil {
		return fmt.Errorf("error creating hub client: %w", err)
	}

	restMapper, err := buildRestMapper(spokeConfig)
	if err != nil {
		return fmt.Errorf("error creating REST mapper: %w", err)
	}

	eventRecorder := o.getEventRecorder(ctx, spokeKubeClient)

	// Informer transform to trim ManagedFields for memory efficiency.
	trim := func(obj any) (any, error) {
		if accessor, err := meta.Accessor(obj); err == nil {
			accessor.SetManagedFields(nil)
		}

		return obj, nil
	}

	addOnInformers := addoninformers.NewSharedInformerFactoryWithOptions(addOnHubKubeClient, 10*time.Minute,
		addoninformers.WithNamespace(o.ClusterName), addoninformers.WithTransform(trim))
	configInformers := configinformers.NewSharedInformerFactoryWithOptions(configHubKubeClient, 10*time.Minute,
		configinformers.WithNamespace(o.ClusterName), configinformers.WithTransform(trim))

	spokeKubeInformers := informers.NewSharedInformerFactoryWithOptions(spokeKubeClient, 10*time.Minute,
		informers.WithNamespace(o.InstallationNamespace), informers.WithTransform(trim))
	// TODO if submariner provides the informer in future, we will use it instead of dynamic informer
	dynamicInformers := dynamicinformer.NewFilteredDynamicSharedInformerFactory(spokeDynamicClient, 10*time.Minute, o.InstallationNamespace,
		nil)
	submarinerInformer := dynamicInformers.ForResource(submarinerGVR)
	routeAgentInformer := dynamicInformers.ForResource(routeAgentGVR)

	submarinerConfigController := submarineragent.NewSubmarinerConfigController(&submarineragent.SubmarinerConfigControllerInput{
		ClusterName:          o.ClusterName,
		Namespace:            o.InstallationNamespace,
		KubeClient:           spokeKubeClient,
		ConfigClient:         configHubKubeClient,
		AddOnClient:          addOnHubKubeClient,
		DynamicClient:        spokeDynamicClient,
		NodeInformer:         spokeKubeInformers.Core().V1().Nodes(),
		AddOnInformer:        addOnInformers.Addon().V1beta1().ManagedClusterAddOns(),
		ConfigInformer:       configInformers.Submarineraddon().V1alpha1().SubmarinerConfigs(),
		SubmarinerInformer:   submarinerInformer,
		CloudProviderFactory: cloud.NewProviderFactory(restMapper, spokeKubeClient, spokeDynamicClient, hubClient),
		Recorder:             eventRecorder,
	})

	gatewaysStatusController := submarineragent.NewGatewaysStatusController(
		o.ClusterName,
		addOnHubKubeClient,
		spokeKubeInformers.Core().V1().Nodes(),
		eventRecorder,
	)

	deploymentStatusController := submarineragent.NewDeploymentStatusController(o.ClusterName, o.InstallationNamespace,
		addOnHubKubeClient, spokeKubeInformers.Apps().V1().DaemonSets(), spokeKubeInformers.Apps().V1().Deployments(),
		dynamicInformers.ForResource(subscriptionGVR), submarinerInformer, eventRecorder)

	connectionsStatusController := submarineragent.NewConnectionsStatusController(o.ClusterName, addOnHubKubeClient,
		dynamicInformers.ForResource(submarinerGVR), routeAgentInformer, eventRecorder)

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

	defer serviceability.BehaviorOnPanic(os.Getenv("OPENSHIFT_ON_PANIC"), version.Get())()
	defer serviceability.Profile(os.Getenv("OPENSHIFT_PROFILE")).Stop()

	serviceability.StartProfiler()

	<-ctx.Done()

	return nil
}

func (o *AgentOptions) getEventRecorder(ctx context.Context, spokeKubeClient *kubernetes.Clientset) events.Recorder {
	// Get controller reference for the current pod (walks ownership chain to find Deployment)
	// Use a short timeout to avoid blocking agent startup if API server is slow
	lookupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	controllerRef, err := events.GetControllerReferenceForCurrentPod(lookupCtx, spokeKubeClient, o.InstallationNamespace, nil)
	if err != nil {
		klog.Warningf("unable to get owner reference (falling back to namespace): %v", err)
	}

	// Create event recorder using library-go events package
	return events.NewKubeRecorder(spokeKubeClient.CoreV1().Events(o.InstallationNamespace),
		"submariner-agent", controllerRef, clock.RealClock{})
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
