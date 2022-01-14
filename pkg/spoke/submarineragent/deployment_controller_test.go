package submarineragent

import (
	"context"
	"testing"
	"time"

	addonv1alpha1 "github.com/open-cluster-management/api/addon/v1alpha1"
	addonfake "github.com/open-cluster-management/api/client/addon/clientset/versioned/fake"
	addoninformers "github.com/open-cluster-management/api/client/addon/informers/externalversions"
	testinghelpers "github.com/stolostron/submariner-addon/pkg/helpers/testing"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestDeploymentStatusControllerSync(t *testing.T) {
	cases := []struct {
		name            string
		addOns          []runtime.Object
		subscriptions   []runtime.Object
		deployemnts     []runtime.Object
		daemonsets      []runtime.Object
		validateActions func(t *testing.T, addOnActions []clienttesting.Action)
	}{
		{
			name: "submariner not deployed",
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "submariner",
					},
				},
			},
			subscriptions: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "operators.coreos.com/v1alpha1",
						"kind":       "Subscription",
						"metadata": map[string]interface{}{
							"namespace": "test",
							"name":      "submariner",
						},
						"spec":   map[string]interface{}{},
						"status": map[string]interface{}{},
					},
				},
			},
			validateActions: func(t *testing.T, addOnActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, addOnActions, "get", "update")
				actual := addOnActions[1].(clienttesting.UpdateActionImpl).Object
				addOn := actual.(*addonv1alpha1.ManagedClusterAddOn)
				if !meta.IsStatusConditionTrue(addOn.Status.Conditions, "SubmarinerAgentDegraded") {
					t.Errorf("expected SubmarinerAgentDegraded is true, but %v", addOn.Status.Conditions)
				}
			},
		},
		{
			name: "operator unavailable",
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "submariner",
					},
				},
			},
			subscriptions: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "operators.coreos.com/v1alpha1",
						"kind":       "Subscription",
						"metadata": map[string]interface{}{
							"namespace": "test",
							"name":      "submariner",
						},
						"spec": map[string]interface{}{},
						"status": map[string]interface{}{
							"installedCSV": "test",
						},
					},
				},
			},
			deployemnts: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "submariner-operator",
					},
				},
			},
			validateActions: func(t *testing.T, addOnActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, addOnActions, "get", "update")
				actual := addOnActions[1].(clienttesting.UpdateActionImpl).Object
				addOn := actual.(*addonv1alpha1.ManagedClusterAddOn)
				if !meta.IsStatusConditionTrue(addOn.Status.Conditions, "SubmarinerAgentDegraded") {
					t.Errorf("expected SubmarinerAgentDegraded is true, but %v", addOn.Status.Conditions)
				}
			},
		},
		{
			name: "submariner unavailable",
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "submariner",
					},
				},
			},
			subscriptions: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "operators.coreos.com/v1alpha1",
						"kind":       "Subscription",
						"metadata": map[string]interface{}{
							"namespace": "test",
							"name":      "submariner",
						},
						"spec": map[string]interface{}{},
						"status": map[string]interface{}{
							"installedCSV": "test",
						},
					},
				},
			},
			deployemnts: []runtime.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "submariner-operator",
					},
					Status: appsv1.DeploymentStatus{
						AvailableReplicas: 1,
					},
				},
			},
			daemonsets: []runtime.Object{
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "submariner-gateway",
					},
					Status: appsv1.DaemonSetStatus{
						NumberUnavailable: 1,
					},
				},
				&appsv1.DaemonSet{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "submariner-routeagent",
					},
					Status: appsv1.DaemonSetStatus{
						NumberUnavailable: 1,
					},
				},
			},
			validateActions: func(t *testing.T, addOnActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, addOnActions, "get", "update")
				actual := addOnActions[1].(clienttesting.UpdateActionImpl).Object
				addOn := actual.(*addonv1alpha1.ManagedClusterAddOn)
				if !meta.IsStatusConditionTrue(addOn.Status.Conditions, "SubmarinerAgentDegraded") {
					t.Errorf("expected SubmarinerAgentDegraded is true, but %v", addOn.Status.Conditions)
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			addOnClient := addonfake.NewSimpleClientset(c.addOns...)
			addOnInformerFactory := addoninformers.NewSharedInformerFactory(addOnClient, time.Minute*10)
			addOnStroe := addOnInformerFactory.Addon().V1alpha1().ManagedClusterAddOns().Informer().GetStore()
			for _, addOn := range c.addOns {
				addOnStroe.Add(addOn)
			}

			subscriptionGVR, _ := schema.ParseResourceArg("subscriptions.v1alpha1.operators.coreos.com")
			fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
			dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(fakeDynamicClient, time.Minute*10)
			subscriptionInformer := dynamicInformerFactory.ForResource(*subscriptionGVR)
			subscriptionStore := subscriptionInformer.Informer().GetStore()
			for _, subscription := range c.subscriptions {
				subscriptionStore.Add(subscription)
			}

			apps := append([]runtime.Object{}, c.daemonsets...)
			apps = append(apps, c.deployemnts...)
			kubeClient := kubefake.NewSimpleClientset(apps...)
			kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Minute*10)
			deploymentStore := kubeInformerFactory.Apps().V1().Deployments().Informer().GetStore()
			for _, deploy := range c.deployemnts {
				deploymentStore.Add(deploy)
			}
			daemonsetStore := kubeInformerFactory.Apps().V1().DaemonSets().Informer().GetStore()
			for _, ds := range c.daemonsets {
				daemonsetStore.Add(ds)
			}

			ctrl := &deploymentStatusController{
				addOnClient:        addOnClient,
				addOnLister:        addOnInformerFactory.Addon().V1alpha1().ManagedClusterAddOns().Lister(),
				deploymentLister:   kubeInformerFactory.Apps().V1().Deployments().Lister(),
				daemonSetLister:    kubeInformerFactory.Apps().V1().DaemonSets().Lister(),
				subscriptionLister: subscriptionInformer.Lister(),
				clusterName:        "test",
				namespace:          "test",
			}

			err := ctrl.sync(context.TODO(), testinghelpers.NewFakeSyncContext(t, "deployment"))
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}

			c.validateActions(t, addOnClient.Actions())
		})
	}
}
