package hub

import (
	"context"
	"time"

	"k8s.io/client-go/dynamic"

	"github.com/openshift/library-go/pkg/controller/controllercmd"

	clusterclient "github.com/open-cluster-management/api/client/cluster/clientset/versioned"
	clusterinformers "github.com/open-cluster-management/api/client/cluster/informers/externalversions"
	workclient "github.com/open-cluster-management/api/client/work/clientset/versioned"
	workinformers "github.com/open-cluster-management/api/client/work/informers/externalversions"
	configclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	configinformers "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/informers/externalversions"
	"github.com/open-cluster-management/submariner-addon/pkg/hub/submarineragent"
	"github.com/open-cluster-management/submariner-addon/pkg/hub/submarinerbroker"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
)

// RunControllerManager starts the controllers on hub to manage submariner deployment.
func RunControllerManager(ctx context.Context, controllerContext *controllercmd.ControllerContext) error {
	kubeClient, err := kubernetes.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
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

	clusterInformers := clusterinformers.NewSharedInformerFactory(clusterClient, 10*time.Minute)
	workInformers := workinformers.NewSharedInformerFactory(workClient, 10*time.Minute)
	kubeInformers := kubeinformers.NewSharedInformerFactory(kubeClient, 10*time.Minute)
	configInformers := configinformers.NewSharedInformerFactory(configClient, 10*time.Minute)
	apiExtensionsInformers := apiextensionsinformers.NewSharedInformerFactory(apiExtensionClient, 10*time.Minute)

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
		clusterInformers.Cluster().V1().ManagedClusters(),
		clusterInformers.Cluster().V1alpha1().ManagedClusterSets(),
		workInformers.Work().V1().ManifestWorks(),
		configInformers.Submarineraddon().V1alpha1().SubmarinerConfigs(),
		controllerContext.EventRecorder,
	)

	go clusterInformers.Start(ctx.Done())
	go workInformers.Start(ctx.Done())
	go kubeInformers.Start(ctx.Done())
	go configInformers.Start(ctx.Done())
	go apiExtensionsInformers.Start(ctx.Done())
	go submarinerBrokerCRDsController.Run(ctx, 1)
	go submarinerBrokerController.Run(ctx, 1)
	go submarinerAgentController.Run(ctx, 1)

	<-ctx.Done()
	return nil
}
