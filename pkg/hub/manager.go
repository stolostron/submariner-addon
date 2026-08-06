package hub

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	configclient "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	configinformers "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/informers/externalversions"
	"github.com/stolostron/submariner-addon/pkg/hub/submarineraddonagent"
	"github.com/stolostron/submariner-addon/pkg/hub/submarineragent"
	"github.com/stolostron/submariner-addon/pkg/hub/submarinerbroker"
	"github.com/stolostron/submariner-addon/pkg/resource"
	submarinerv1alpha1 "github.com/submariner-io/submariner-operator/api/v1alpha1"
	submarinerv1 "github.com/submariner-io/submariner/pkg/apis/submariner.io/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	"k8s.io/utils/clock"
	"open-cluster-management.io/addon-framework/pkg/addonmanager"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"
	addoninformers "open-cluster-management.io/api/client/addon/informers/externalversions"
	clusterclient "open-cluster-management.io/api/client/cluster/clientset/versioned"
	clusterinformers "open-cluster-management.io/api/client/cluster/informers/externalversions"
	workclient "open-cluster-management.io/api/client/work/clientset/versioned"
	workinformers "open-cluster-management.io/api/client/work/informers/externalversions"
	controllerclient "sigs.k8s.io/controller-runtime/pkg/client"
	mcsv1a1 "sigs.k8s.io/mcs-api/pkg/apis/v1alpha1"
)

const (
	containerName                = "submariner-addon"
	DefaultNamespace             = "open-cluster-management"
	accessToBrokerCRDClusterRole = "access-to-brokers-submariner-crd"
)

type Clients struct {
	kubeClient         kubernetes.Interface
	dynamicClient      dynamic.Interface
	clusterClient      clusterclient.Interface
	workClient         workclient.Interface
	configClient       configclient.Interface
	apiExtensionClient apiextensionsclientset.Interface
	addOnClient        addonclient.Interface
	controllerClient   controllerclient.Client
}

type AddOnOptions struct {
	AgentImage    string
	EventRecorder events.Recorder // Optional: for test injection
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

	namespace := resource.GetCurrentNamespace(DefaultNamespace)

	podName := os.Getenv("POD_NAME")
	if podName == "" {
		return errors.New("the pod environment POD_NAME is required")
	}

	pod, err := kubeClient.CoreV1().Pods(namespace).Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return errors.Wrapf(err, "error retrieving Pod %q", podName)
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
// The markReady callback is called after informer caches are synced (optional, can be nil).
func (o *AddOnOptions) RunControllerManager(ctx context.Context, kubeConfig *rest.Config, markReady func()) error {
	utilruntime.Must(submarinerv1alpha1.AddToScheme(scheme.Scheme))
	utilruntime.Must(submarinerv1.AddToScheme(scheme.Scheme))
	utilruntime.Must(mcsv1a1.Install(scheme.Scheme))

	clients, err := newClients(kubeConfig)
	if err != nil {
		return err
	}

	if err := o.Complete(ctx, clients.kubeClient); err != nil {
		return err
	}

	eventRecorder := getEventRecorder(ctx, o, clients.kubeClient)

	// Informer transform to trim ManagedFields for memory efficiency.
	// TODO: Apply trim to all informers when they support WithTransform.
	trim := func(obj any) (any, error) {
		if accessor, err := meta.Accessor(obj); err == nil {
			accessor.SetManagedFields(nil)
		}

		return obj, nil
	}

	clusterInformers := clusterinformers.NewSharedInformerFactoryWithOptions(clients.clusterClient, 10*time.Minute,
		clusterinformers.WithTransform(trim))
	workInformers := workinformers.NewSharedInformerFactoryWithOptions(clients.workClient, 10*time.Minute,
		workinformers.WithTransform(trim))
	kubeInformers := kubeinformers.NewSharedInformerFactoryWithOptions(
		clients.kubeClient, 10*time.Minute, kubeinformers.WithTransform(trim))
	configInformers := configinformers.NewSharedInformerFactoryWithOptions(
		clients.configClient, 10*time.Minute, configinformers.WithTransform(trim))
	apiExtensionsInformers := apiextensionsinformers.NewSharedInformerFactoryWithOptions(
		clients.apiExtensionClient, 10*time.Minute, apiextensionsinformers.WithTransform(trim))
	addOnInformers := addoninformers.NewSharedInformerFactoryWithOptions(clients.addOnClient, 10*time.Minute,
		addoninformers.WithTransform(trim))

	submarinerBrokerCRDsController := submarinerbroker.NewCRDsController(
		clients.apiExtensionClient,
		apiExtensionsInformers.Apiextensions().V1().CustomResourceDefinitions(),
		eventRecorder,
	)

	err = createClusterRoleToAllowBrokerCRD(ctx, clients.kubeClient)
	if err != nil {
		return err
	}

	submarinerBrokerController := submarinerbroker.NewController(clients.kubeClient,
		clients.clusterClient.ClusterV1beta2().ManagedClusterSets(),
		clusterInformers.Cluster().V1beta2().ManagedClusterSets(),
		clients.addOnClient,
		addOnInformers.Addon().V1beta1(),
		kubeConfig,
		eventRecorder)

	submarinerAgentController := submarineragent.NewSubmarinerAgentController(
		clients.kubeClient,
		clients.dynamicClient,
		clients.controllerClient,
		clients.clusterClient,
		clients.workClient,
		clients.configClient,
		clients.addOnClient,
		clusterInformers.Cluster().V1().ManagedClusters(),
		clusterInformers.Cluster().V1beta2().ManagedClusterSets(),
		workInformers.Work().V1().ManifestWorks(),
		configInformers.Submarineraddon().V1alpha1().SubmarinerConfigs(),
		addOnInformers.Addon().V1beta1().ClusterManagementAddOns(),
		addOnInformers.Addon().V1beta1().ManagedClusterAddOns(),
		addOnInformers.Addon().V1beta1().AddOnDeploymentConfigs(),
		eventRecorder,
	)

	clusterInformers.Start(ctx.Done())
	workInformers.Start(ctx.Done())
	kubeInformers.Start(ctx.Done())
	configInformers.Start(ctx.Done())
	apiExtensionsInformers.Start(ctx.Done())
	addOnInformers.Start(ctx.Done())

	// Wait for informer caches to sync before starting controllers
	clusterInformers.WaitForCacheSync(ctx.Done())
	workInformers.WaitForCacheSync(ctx.Done())
	kubeInformers.WaitForCacheSync(ctx.Done())
	configInformers.WaitForCacheSync(ctx.Done())
	apiExtensionsInformers.WaitForCacheSync(ctx.Done())
	addOnInformers.WaitForCacheSync(ctx.Done())

	// Check if context was cancelled during cache sync
	if ctx.Err() != nil {
		return errors.Wrap(ctx.Err(), "context cancelled during cache sync")
	}

	// Notify that informer caches are synced (for readiness probe)
	if markReady != nil {
		markReady()
	}

	go submarinerBrokerCRDsController.Run(ctx, 1)
	go submarinerBrokerController.Run(ctx, 1)
	go submarinerAgentController.Run(ctx, 1)

	mgr, err := addonmanager.New(kubeConfig)
	if err != nil {
		return errors.Wrap(err, "error creating addon manager")
	}

	agent, err := submarineraddonagent.NewAddOnAgent(clients.kubeClient, clients.clusterClient, clients.addOnClient,
		eventRecorder, o.AgentImage)
	if err != nil {
		return errors.Wrap(err, "error creating addon agent")
	}

	err = mgr.AddAgent(agent)
	if err != nil {
		return errors.Wrap(err, "error adding agent")
	}

	err = mgr.Start(ctx)
	if err != nil {
		return errors.Wrap(err, "error starting addon manager")
	}

	<-ctx.Done()

	return nil
}

func getEventRecorder(ctx context.Context, o *AddOnOptions, kubeClient kubernetes.Interface) events.Recorder {
	namespace := resource.GetCurrentNamespace(DefaultNamespace)

	// Use injected event recorder (for tests) or create one
	eventRecorder := o.EventRecorder
	if eventRecorder == nil {
		// Get controller reference for the current pod (walks ownership chain to find Deployment)
		controllerRef, err := events.GetControllerReferenceForCurrentPod(ctx, kubeClient, namespace, nil)
		if err != nil {
			klog.Warningf("unable to get owner reference (falling back to namespace): %v", err)
		}

		// Create event recorder using library-go events package
		eventRecorder = events.NewKubeRecorder(kubeClient.CoreV1().Events(namespace),
			"submariner-addon-controller", controllerRef, clock.RealClock{})
	}

	return eventRecorder
}

func createClusterRoleToAllowBrokerCRD(ctx context.Context, kubeClient kubernetes.Interface) error {
	klog.Infof("Checking if ClusterRole %q exists", accessToBrokerCRDClusterRole)

	_, err := kubeClient.RbacV1().ClusterRoles().Get(ctx, accessToBrokerCRDClusterRole, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrap(err, "error retrieving ClusterRoles")
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
			return errors.Wrap(err, "error creating broker ClusterRole")
		}
	}

	return nil
}

func newClients(kubeConfig *rest.Config) (*Clients, error) {
	kubeClient, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, errors.Wrap(err, "error creating kube client")
	}

	dynamicClient, err := dynamic.NewForConfig(kubeConfig)
	if err != nil {
		return nil, errors.Wrap(err, "error creating dynamic client")
	}

	clusterClient, err := clusterclient.NewForConfig(kubeConfig)
	if err != nil {
		return nil, errors.Wrap(err, "error creating controller client")
	}

	workClient, err := workclient.NewForConfig(kubeConfig)
	if err != nil {
		return nil, errors.Wrap(err, "error creating work client")
	}

	configClient, err := configclient.NewForConfig(kubeConfig)
	if err != nil {
		return nil, errors.Wrap(err, "error creating config client")
	}

	apiExtensionClient, err := apiextensionsclientset.NewForConfig(kubeConfig)
	if err != nil {
		return nil, errors.Wrap(err, "error creating apiExtension client")
	}

	addOnClient, err := addonclient.NewForConfig(kubeConfig)
	if err != nil {
		return nil, errors.Wrap(err, "error creating addon client")
	}

	controllerClient, err := controllerclient.New(kubeConfig, controllerclient.Options{})
	if err != nil {
		return nil, errors.Wrap(err, "error creating controller client")
	}

	return &Clients{
		kubeClient:         kubeClient,
		dynamicClient:      dynamicClient,
		clusterClient:      clusterClient,
		workClient:         workClient,
		configClient:       configClient,
		apiExtensionClient: apiExtensionClient,
		addOnClient:        addOnClient,
		controllerClient:   controllerClient,
	}, nil
}
