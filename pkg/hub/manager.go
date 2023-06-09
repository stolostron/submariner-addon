package hub

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/spf13/cobra"
	configclient "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	configinformers "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/informers/externalversions"
	"github.com/stolostron/submariner-addon/pkg/cloud"
	"github.com/stolostron/submariner-addon/pkg/hub/submarineraddonagent"
	"github.com/stolostron/submariner-addon/pkg/hub/submarineragent"
	"github.com/stolostron/submariner-addon/pkg/hub/submarinerbroker"
	"github.com/stolostron/submariner-addon/pkg/resource"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"
	addoninformers "open-cluster-management.io/api/client/addon/informers/externalversions"
	clusterclient "open-cluster-management.io/api/client/cluster/clientset/versioned"
	clusterinformers "open-cluster-management.io/api/client/cluster/informers/externalversions"
	workclient "open-cluster-management.io/api/client/work/clientset/versioned"
	workinformers "open-cluster-management.io/api/client/work/informers/externalversions"
	controllerclient "sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	containerName                = "submariner-addon"
	defaultNamespace             = "open-cluster-management"
	accessToBrokerCRDClusterRole = "access-to-brokers-submariner-crd"
)

type AddOnOptions struct {
	AgentImage string
}

func NewAddOnOptions() *AddOnOptions {
	return &AddOnOptions{}
}

func (o *AddOnOptions) AddFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	// TODO if downstream building supports to set downstream image, we could use this flag
	// to set agent image on building phase
	flags.StringVar(&o.AgentImage, "agent-image", o.AgentImage, "The image of addon agent.")
}

func (o *AddOnOptions) Complete(ctx context.Context, kubeClient kubernetes.Interface) error {
	if o.AgentImage != "" {
		return nil
	}

	namespace := resource.GetCurrentNamespace(defaultNamespace)
	podName := os.Getenv("POD_NAME")
	if podName == "" {
		return fmt.Errorf("the pod environment POD_NAME is required")
	}

	pod, err := kubeClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	for i := range pod.Spec.Containers {
		container := &pod.Spec.Containers[i]
		if container.Name == containerName {
			o.AgentImage = pod.Spec.Containers[0].Image

			return nil
		}
	}

	return fmt.Errorf("the agent image cannot be found from the container %q of the pod %q", containerName, podName)
}

// RunControllerManager starts the controllers on hub to manage submariner deployment.
func (o *AddOnOptions) RunControllerManager(ctx context.Context, controllerContext *controllercmd.ControllerContext) error {
	kubeClient, err := kubernetes.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	if err := o.Complete(ctx, kubeClient); err != nil {
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

	controllerClient, err := controllerclient.New(controllerContext.KubeConfig, controllerclient.Options{})
	if err != nil {
		return err
	}

	clusterInformers := clusterinformers.NewSharedInformerFactory(clusterClient, 10*time.Minute)
	workInformers := workinformers.NewSharedInformerFactory(workClient, 10*time.Minute)
	kubeInformers := kubeinformers.NewSharedInformerFactory(kubeClient, 10*time.Minute)
	configInformers := configinformers.NewSharedInformerFactory(configClient, 10*time.Minute)
	apiExtensionsInformers := apiextensionsinformers.NewSharedInformerFactory(apiExtensionClient, 10*time.Minute)
	addOnInformers := addoninformers.NewSharedInformerFactoryWithOptions(addOnClient, 10*time.Minute)

	submarinerBrokerCRDsController := submarinerbroker.NewCRDsController(
		apiExtensionClient,
		apiExtensionsInformers.Apiextensions().V1().CustomResourceDefinitions(),
		controllerContext.EventRecorder,
	)

	err = createClusterRoleToAllowBrokerCRD(ctx, kubeClient)
	if err != nil {
		return err
	}

	submarinerBrokerController := submarinerbroker.NewController(
		clusterClient.ClusterV1beta2().ManagedClusterSets(),
		kubeClient,
		clusterInformers.Cluster().V1beta2().ManagedClusterSets(),
		controllerContext.EventRecorder,
	)

	submarinerAgentController := submarineragent.NewSubmarinerAgentController(
		kubeClient,
		dynamicClient,
		controllerClient,
		clusterClient,
		workClient,
		configClient,
		addOnClient,
		clusterInformers.Cluster().V1().ManagedClusters(),
		clusterInformers.Cluster().V1beta2().ManagedClusterSets(),
		workInformers.Work().V1().ManifestWorks(),
		configInformers.Submarineraddon().V1alpha1().SubmarinerConfigs(),
		addOnInformers.Addon().V1alpha1().ManagedClusterAddOns(),
		cloud.NewProviderFactory(nil, kubeClient, workClient, dynamicClient, nil),
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

	err = mgr.AddAgent(submarineraddonagent.NewAddOnAgent(kubeClient, clusterClient, controllerContext.EventRecorder, o.AgentImage))
	if err != nil {
		return err
	}

	go func() {
		err := mgr.Start(ctx)
		if err != nil {
			klog.Fatalf("Error starting the manager: %v", err)
		}
	}()

	<-ctx.Done()

	return nil
}

func createClusterRoleToAllowBrokerCRD(ctx context.Context, kubeClient *kubernetes.Clientset) error {
	klog.Infof("Checking if ClusterRole %q exists", accessToBrokerCRDClusterRole)

	_, err := kubeClient.RbacV1().ClusterRoles().Get(ctx, accessToBrokerCRDClusterRole, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}

	if apierrors.IsNotFound(err) {
		klog.Infof("%q not found, creating it", accessToBrokerCRDClusterRole)
		// Create the ClusterRole
		brokerClusterRole := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: accessToBrokerCRDClusterRole,
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"submariner.io"},
					Resources: []string{"brokers"},
					Verbs:     []string{"create", "get", "update"},
				},
			},
		}
		_, err := kubeClient.RbacV1().ClusterRoles().Create(ctx, brokerClusterRole, metav1.CreateOptions{})
		if err != nil {
			return err
		}
	}

	return nil
}
