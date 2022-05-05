package submarineragent

import (
	"context"
	"fmt"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/stolostron/submariner-addon/pkg/apis/submarinerdiagnoseconfig"
	diagnosev1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerdiagnoseconfig/v1alpha1"
	diagnoseclient "github.com/stolostron/submariner-addon/pkg/client/submarinerdiagnoseconfig/clientset/versioned"
	diagnoseinformer "github.com/stolostron/submariner-addon/pkg/client/submarinerdiagnoseconfig/informers/externalversions/submarinerdiagnoseconfig/v1alpha1" //nolint
	diagnoselister "github.com/stolostron/submariner-addon/pkg/client/submarinerdiagnoseconfig/listers/submarinerdiagnoseconfig/v1alpha1"
	"github.com/stolostron/submariner-addon/pkg/constants"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	apiErrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	corev1informers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
	addoninformerv1alpha1 "open-cluster-management.io/api/client/addon/informers/externalversions/addon/v1alpha1"
	addonlisterv1alpha1 "open-cluster-management.io/api/client/addon/listers/addon/v1alpha1"
)

const (
	jobName        = "submariner-diagnose"
	imageName      = "quay.io/submariner/subctl:latest"
	serviceAccount = "submariner-operator"
	jobLabel       = "job-name"
)

// submarinerDiagnoseConfigController watches the SubmarinerDiagnoseConfigs API on the hub cluster
// and run Diagnose Job on the manged cluster.
type submarinerDiagnoseConfigController struct {
	kubeClient           kubernetes.Interface
	diagnoseClient       diagnoseclient.Interface
	addOnLister          addonlisterv1alpha1.ManagedClusterAddOnLister
	diagnoseConfigLister diagnoselister.SubmarinerDiagnoseConfigLister
	podLister            corev1lister.PodLister
	clusterName          string
	onSyncDefer          func()
}

type SubmarinerDiagnoseConfigControllerInput struct {
	ClusterName    string
	KubeClient     kubernetes.Interface
	ConfigClient   diagnoseclient.Interface
	AddOnInformer  addoninformerv1alpha1.ManagedClusterAddOnInformer
	ConfigInformer diagnoseinformer.SubmarinerDiagnoseConfigInformer
	PodInformer    corev1informers.PodInformer
	Recorder       events.Recorder
	// This is a hook for unit tests to invoke a defer (specifically GinkgoRecover) when the sync function is called.
	OnSyncDefer func()
}

// NewSubmarinerDiagnoseConfigController returns an instance of submarinerAgentConfigController.
func NewSubmarinerDiagnoseConfigController(input *SubmarinerDiagnoseConfigControllerInput) factory.Controller {
	c := &submarinerDiagnoseConfigController{
		kubeClient:           input.KubeClient,
		diagnoseClient:       input.ConfigClient,
		addOnLister:          input.AddOnInformer.Lister(),
		diagnoseConfigLister: input.ConfigInformer.Lister(),
		podLister:            input.PodInformer.Lister(),
		clusterName:          input.ClusterName,
		onSyncDefer:          input.OnSyncDefer,
	}

	return factory.New().
		WithFilteredEventsInformers(func(obj interface{}) bool {
			metaObj := obj.(metav1.Object)

			return metaObj.GetName() == constants.SubmarinerAddOnName
		}, input.AddOnInformer.Informer()).
		WithFilteredEventsInformers(func(obj interface{}) bool {
			metaObj := obj.(metav1.Object)

			return metaObj.GetName() == constants.SubmarinerConfigName
		}, input.ConfigInformer.Informer()).
		WithFilteredEventsInformers(func(obj interface{}) bool {
			metaObj := obj.(metav1.Object)
			if jobLabelValue, has := metaObj.GetLabels()[jobLabel]; has {
				if jobLabelValue == jobName {
					return true
				}
			}

			return false
		}, input.PodInformer.Informer()).
		WithSync(c.sync).
		ToController("SubmarinerAgentDiagnoseConfigController", input.Recorder)
}

func (c *submarinerDiagnoseConfigController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	if c.onSyncDefer != nil {
		defer c.onSyncDefer()
	}

	addOn, err := c.addOnLister.ManagedClusterAddOns(c.clusterName).Get(constants.SubmarinerAddOnName)
	if apiErrors.IsNotFound(err) {
		// the addon not found, could be deleted, ignore
		return nil
	}

	if err != nil {
		return err
	}

	diagnoseConfig, err := c.diagnoseConfigLister.SubmarinerDiagnoseConfigs(c.clusterName).Get(constants.SubmarinerConfigName)
	if apiErrors.IsNotFound(err) {
		syncCtx.Recorder().Eventf("TestDiagnoseConfig", "Deleting Diagnose Job")

		return c.deleteDiagnoseJob(addOn.Spec.InstallNamespace)
	}

	if err != nil {
		return err
	}

	if diagnoseConfig.Status.Conditions != nil {
		return nil
	}

	podList, err := c.podLister.Pods(addOn.Spec.InstallNamespace).List(labels.SelectorFromSet(labels.Set{jobLabel: jobName}))
	if err != nil && !apiErrors.IsNotFound(err) {
		syncCtx.Recorder().Event("TestDiagnoseConfig", "failed to get diagnose podlist")
		return err
	}

	if apiErrors.IsNotFound(err) || len(podList) == 0 {
		syncCtx.Recorder().Event("TestDiagnoseConfig", "creating Diagnose Job")

		err = c.deployDiagnoseJob(addOn.Spec.InstallNamespace)

		return err
	}

	// Pod deployed, update status
	condition := diagnoseSuccessCondition("Pod %s available on cluster %s", podList[0].Name, addOn.ClusterName)
	updateErr := c.updateSubmarinerDiagnoseStatus(ctx, syncCtx.Recorder(), diagnoseConfig, &condition)
	if updateErr != nil {
		syncCtx.Recorder().Event("TestDiagnoseConfig", "failed to update status")
	}

	return updateErr
}

func (c *submarinerDiagnoseConfigController) updateSubmarinerDiagnoseStatus(ctx context.Context, recorder events.Recorder,
	config *diagnosev1alpha1.SubmarinerDiagnoseConfig, condition *metav1.Condition,
) error {
	updatedStatus, updated, err := submarinerdiagnoseconfig.UpdateStatus(ctx,
		c.diagnoseClient.SubmarineraddonV1alpha1().SubmarinerDiagnoseConfigs(config.Namespace), config.Name,
		submarinerdiagnoseconfig.UpdateConditionFn(condition))

	if updated {
		recorder.Eventf("SubmarinerDiagnoseConfigStatusUpdated", "Updated status conditions:  %#v", updatedStatus.Conditions)
	}

	return err
}

func (c *submarinerDiagnoseConfigController) deployDiagnoseJob(namespace string) error {
	var backOffLimit int32 = 0

	jobSpec := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      jobName,
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			Template: v1.PodTemplateSpec{
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:            jobName,
							Image:           imageName,
							ImagePullPolicy: v1.PullIfNotPresent,
							Command:         []string{"subctl", "diagnose", "all", "--in-cluster"},
						},
					},
					RestartPolicy:      v1.RestartPolicyNever,
					ServiceAccountName: serviceAccount,
				},
			},
			BackoffLimit: &backOffLimit,
		},
	}

	_, err := c.kubeClient.BatchV1().Jobs(jobSpec.Namespace).Create(context.TODO(), jobSpec, metav1.CreateOptions{})

	return err
}

func (c *submarinerDiagnoseConfigController) deleteDiagnoseJob(namespace string) error {
	propagationPolicy := metav1.DeletePropagationBackground // GC will delete pods in background
	err := c.kubeClient.BatchV1().Jobs(namespace).Delete(context.TODO(), jobName, metav1.DeleteOptions{
		PropagationPolicy: &propagationPolicy,
	})
	if !apiErrors.IsNotFound(err) {
		return err
	}

	return nil
}

func diagnoseSuccessCondition(formatMsg string, args ...interface{}) metav1.Condition {
	return metav1.Condition{
		Type:    "diagnoseJobRun",
		Status:  metav1.ConditionTrue,
		Reason:  "Success",
		Message: fmt.Sprintf(formatMsg, args...),
	}
}
