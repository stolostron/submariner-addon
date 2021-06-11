package submarineragent

import (
	"context"
	"testing"
	"time"

	addonv1alpha1 "github.com/open-cluster-management/api/addon/v1alpha1"
	addonfake "github.com/open-cluster-management/api/client/addon/clientset/versioned/fake"
	addoninformers "github.com/open-cluster-management/api/client/addon/informers/externalversions"
	testinghelpers "github.com/open-cluster-management/submariner-addon/pkg/helpers/testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestGatewaysStatusControllerSync(t *testing.T) {
	cases := []struct {
		name            string
		addOns          []runtime.Object
		nodes           []runtime.Object
		validateActions func(t *testing.T, addOnActions []clienttesting.Action)
	}{
		{
			name: "nodes are labeled",
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "submariner",
					},
				},
			},
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
						Labels: map[string]string{
							"submariner.io/gateway": "true",
						},
					},
				},
			},
			validateActions: func(t *testing.T, addOnActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, addOnActions, "get", "update")
				actual := addOnActions[1].(clienttesting.UpdateActionImpl).Object
				addOn := actual.(*addonv1alpha1.ManagedClusterAddOn)
				if !meta.IsStatusConditionTrue(addOn.Status.Conditions, "SubmarinerGatewayNodesLabeled") {
					t.Errorf("expected SubmarinerGatewayNodesLabeled is true, but %v", addOn.Status.Conditions)
				}
			},
		},
		{
			name: "nodes are not labeled",
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "submariner",
					},
				},
			},
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
			validateActions: func(t *testing.T, addOnActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, addOnActions, "get", "update")
				actual := addOnActions[1].(clienttesting.UpdateActionImpl).Object
				addOn := actual.(*addonv1alpha1.ManagedClusterAddOn)
				if meta.IsStatusConditionTrue(addOn.Status.Conditions, "SubmarinerGatewayNodesLabeled") {
					t.Errorf("expected SubmarinerGatewayNodesLabeled is false, but %v", addOn.Status.Conditions)
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

			kubeClient := kubefake.NewSimpleClientset(c.nodes...)
			kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Minute*10)
			nodeStore := kubeInformerFactory.Core().V1().Nodes().Informer().GetStore()
			for _, node := range c.nodes {
				nodeStore.Add(node)
			}

			ctrl := &gatewaysStatusController{
				addOnClient: addOnClient,
				addOnLister: addOnInformerFactory.Addon().V1alpha1().ManagedClusterAddOns().Lister(),
				nodeLister:  kubeInformerFactory.Core().V1().Nodes().Lister(),
				clusterName: "test",
			}

			err := ctrl.sync(context.TODO(), testinghelpers.NewFakeSyncContext(t, "gateway"))
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}

			c.validateActions(t, addOnClient.Actions())
		})
	}
}
