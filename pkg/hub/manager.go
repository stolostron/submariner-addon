package hub

import (
	"context"
	"k8s.io/client-go/dynamic"
	"time"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"k8s.io/client-go/kubernetes"

	clusterv1client "github.com/open-cluster-management/api/client/cluster/clientset/versioned"
	clusterv1informers "github.com/open-cluster-management/api/client/cluster/informers/externalversions"
	workv1client "github.com/open-cluster-management/api/client/work/clientset/versioned"
	workv1informers "github.com/open-cluster-management/api/client/work/informers/externalversions"

	"github.com/open-cluster-management/submariner-addon/pkg/hub/submarineragent"
	"github.com/open-cluster-management/submariner-addon/pkg/hub/submarinerbroker"

	kubeinformers "k8s.io/client-go/informers"
)

// RunControllerManager starts the controllers on hub to manage spoke cluster registration.
func RunControllerManager(ctx context.Context, controllerContext *controllercmd.ControllerContext) error {
	kubeClient, err := kubernetes.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	dynamicClient, err := dynamic.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	clusterClient, err := clusterv1client.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	workClient, err := workv1client.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	clusterInformers := clusterv1informers.NewSharedInformerFactory(clusterClient, 10*time.Minute)
	workInformers := workv1informers.NewSharedInformerFactory(workClient, 10*time.Minute)
	kubeInfomers := kubeinformers.NewSharedInformerFactory(kubeClient, 10*time.Minute)

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
		clusterInformers.Cluster().V1().ManagedClusters(),
		clusterInformers.Cluster().V1alpha1().ManagedClusterSets(),
		workInformers.Work().V1().ManifestWorks(),
		controllerContext.EventRecorder,
	)

	go clusterInformers.Start(ctx.Done())
	go workInformers.Start(ctx.Done())
	go kubeInfomers.Start(ctx.Done())
	go submarinerBrokerController.Run(ctx, 1)
	go submarinerAgentController.Run(ctx, 1)

	<-ctx.Done()
	return nil
}
