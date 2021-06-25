package hub

import (
	"context"
	"fmt"
	"os"
	"time"

	"k8s.io/client-go/dynamic"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/spf13/cobra"

	"github.com/open-cluster-management/addon-framework/pkg/addonmanager"
	addonclient "github.com/open-cluster-management/api/client/addon/clientset/versioned"
	addoninformers "github.com/open-cluster-management/api/client/addon/informers/externalversions"
	clusterclient "github.com/open-cluster-management/api/client/cluster/clientset/versioned"
	clusterinformers "github.com/open-cluster-management/api/client/cluster/informers/externalversions"
	workclient "github.com/open-cluster-management/api/client/work/clientset/versioned"
	workinformers "github.com/open-cluster-management/api/client/work/informers/externalversions"
	configclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	configinformers "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/informers/externalversions"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"
	"github.com/open-cluster-management/submariner-addon/pkg/hub/submarineraddonagent"
	"github.com/open-cluster-management/submariner-addon/pkg/hub/submarineragent"
	"github.com/open-cluster-management/submariner-addon/pkg/hub/submarinerbroker"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

const (
	containerName    = "submariner-addon"
	defaultNamespace = "open-cluster-management"
)

type AddOnOptions struct {
	AgentImage string
}

func NewAddOnOptions() *AddOnOptions {
	return &AddOnOptions{}
}

func (o *AddOnOptions) AddFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	//TODO if downstream building supports to set downstream image, we could use this flag
	// to set agent image on building phase
	flags.StringVar(&o.AgentImage, "agent-image", o.AgentImage, "The image of addon agent.")
}

func (o *AddOnOptions) Complete(kubeClient kubernetes.Interface) error {
	if len(o.AgentImage) != 0 {
		return nil
	}

	namespace := helpers.GetCurrentNamespace(defaultNamespace)
	podName := os.Getenv("POD_NAME")
	if len(podName) == 0 {
		return fmt.Errorf("The pod enviroment POD_NAME is required")
	}

	pod, err := kubeClient.CoreV1().Pods(namespace).Get(context.TODO(), podName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	for _, container := range pod.Spec.Containers {
		if container.Name == containerName {
			o.AgentImage = pod.Spec.Containers[0].Image
			return nil
		}
	}
	return fmt.Errorf("The agent image cannot be found from the container %q of the pod %q", containerName, podName)
}

// RunControllerManager starts the controllers on hub to manage submariner deployment.
func (o *AddOnOptions) RunControllerManager(ctx context.Context, controllerContext *controllercmd.ControllerContext) error {
	kubeClient, err := kubernetes.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	if err := o.Complete(kubeClient); err != nil {
		return err
	}

	dynamicClient, err := dynamic.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	clusterClient, err := clusterclient.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	workClient, err := workclient.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	configClient, err := configclient.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	apiExtensionClient, err := apiextensionsclientset.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	addOnClient, err := addonclient.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	clusterInformers := clusterinformers.NewSharedInformerFactory(clusterClient, 10*time.Minute)
	workInformers := workinformers.NewSharedInformerFactory(workClient, 10*time.Minute)
	kubeInformers := kubeinformers.NewSharedInformerFactory(kubeClient, 10*time.Minute)
	configInformers := configinformers.NewSharedInformerFactory(configClient, 10*time.Minute)
	apiExtensionsInformers := apiextensionsinformers.NewSharedInformerFactory(apiExtensionClient, 10*time.Minute)
	addOnInformers := addoninformers.NewSharedInformerFactoryWithOptions(addOnClient, 10*time.Minute)

	submarinerBrokerCRDsController := submarinerbroker.NewSubmarinerBrokerCRDsController(
		apiExtensionClient,
		apiExtensionsInformers.Apiextensions().V1beta1().CustomResourceDefinitions(),
		controllerContext.EventRecorder,
	)

	submarinerBrokerController := submarinerbroker.NewSubmarinerBrokerController(
		clusterClient.ClusterV1alpha1().ManagedClusterSets(),
		kubeClient,
		clusterInformers.Cluster().V1alpha1().ManagedClusterSets(),
		controllerContext.EventRecorder,
	)

	submarinerAgentController := submarineragent.NewSubmarinerAgentController(
		kubeClient,
		dynamicClient,
		clusterClient,
		workClient,
		configClient,
		addOnClient,
		clusterInformers.Cluster().V1().ManagedClusters(),
		clusterInformers.Cluster().V1alpha1().ManagedClusterSets(),
		workInformers.Work().V1().ManifestWorks(),
		configInformers.Submarineraddon().V1alpha1().SubmarinerConfigs(),
		addOnInformers.Addon().V1alpha1().ManagedClusterAddOns(),
		controllerContext.EventRecorder,
	)
	go clusterInformers.Start(ctx.Done())
	go workInformers.Start(ctx.Done())
	go kubeInformers.Start(ctx.Done())
	go configInformers.Start(ctx.Done())
	go apiExtensionsInformers.Start(ctx.Done())
	go addOnInformers.Start(ctx.Done())
	go submarinerBrokerCRDsController.Run(ctx, 1)
	go submarinerBrokerController.Run(ctx, 1)
	go submarinerAgentController.Run(ctx, 1)

	mgr, err := addonmanager.New(controllerContext.KubeConfig)
	if err != nil {
		return err
	}
	if err := mgr.AddAgent(submarineraddonagent.NewAddOnAgent(kubeClient, controllerContext.EventRecorder, o.AgentImage)); err != nil {
		return err
	}
	go mgr.Start(ctx)

	<-ctx.Done()
	return nil
}
