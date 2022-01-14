package submarineragent

import (
	"context"
	"testing"
	"time"

	addonv1alpha1 "github.com/open-cluster-management/api/addon/v1alpha1"
	addonfake "github.com/open-cluster-management/api/client/addon/clientset/versioned/fake"
	addoninformers "github.com/open-cluster-management/api/client/addon/informers/externalversions"
	testinghelpers "github.com/stolostron/submariner-addon/pkg/helpers/testing"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestConnectionsStatusControllerSync(t *testing.T) {
	cases := []struct {
		name            string
		addOns          []runtime.Object
		submariners     []runtime.Object
		validateActions func(t *testing.T, addOnActions []clienttesting.Action)
	}{
		{
			name: "submariner is deployed",
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "submariner",
					},
				},
			},
			submariners: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "submariner.io/v1alpha1",
						"kind":       "Submariner",
						"metadata": map[string]interface{}{
							"namespace": "submariner-operator",
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
				if !meta.IsStatusConditionTrue(addOn.Status.Conditions, "SubmarinerConnectionDegraded") {
					t.Errorf("expected SubmarinerConnectionDegraded is true, but %v", addOn.Status.Conditions)
				}
			},
		},
		{
			name: "submariner is deployed",
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "submariner",
					},
				},
			},
			submariners: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "submariner.io/v1alpha1",
						"kind":       "Submariner",
						"metadata": map[string]interface{}{
							"namespace": "submariner-operator",
							"name":      "submariner",
						},
						"spec": map[string]interface{}{},
						"status": map[string]interface{}{
							"gateways": []map[string]interface{}{
								{
									"connections": []map[string]interface{}{
										{
											"status": "connected",
											"endpoint": map[string]interface{}{
												"cluster_id": "cluster1",
												"hostname":   "ip-10-0-37-115",
											},
										},
									},
								},
							},
						},
					},
				},
			},
			validateActions: func(t *testing.T, addOnActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, addOnActions, "get", "update")
				actual := addOnActions[1].(clienttesting.UpdateActionImpl).Object
				addOn := actual.(*addonv1alpha1.ManagedClusterAddOn)
				if meta.IsStatusConditionTrue(addOn.Status.Conditions, "SubmarinerConnectionDegraded") {
					t.Errorf("expected SubmarinerConnectionDegraded is true, but %v", addOn.Status.Conditions)
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

			submarinerGVR, _ := schema.ParseResourceArg("submariners.v1alpha1.submariner.io")
			fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
			dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(fakeDynamicClient, time.Minute*10)
			submarinerInformer := dynamicInformerFactory.ForResource(*submarinerGVR)
			submarinerStore := submarinerInformer.Informer().GetStore()
			for _, submariner := range c.submariners {
				submarinerStore.Add(submariner)
			}

			ctrl := &connectionsStatusController{
				addOnClient:           addOnClient,
				addOnLister:           addOnInformerFactory.Addon().V1alpha1().ManagedClusterAddOns().Lister(),
				submarinerLister:      submarinerInformer.Lister(),
				clusterName:           "test",
				installationNamespace: "submariner-operator",
			}

			err := ctrl.sync(context.TODO(), testinghelpers.NewFakeSyncContext(t, "submariner"))
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
			c.validateActions(t, addOnClient.Actions())
		})
	}
}
