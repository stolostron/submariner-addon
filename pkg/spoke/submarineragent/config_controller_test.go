package submarineragent

import (
	"context"
	"testing"
	"time"

	addonv1alpha1 "github.com/open-cluster-management/api/addon/v1alpha1"
	addonfake "github.com/open-cluster-management/api/client/addon/clientset/versioned/fake"
	addoninformers "github.com/open-cluster-management/api/client/addon/informers/externalversions"
	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	fakeconfigclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned/fake"
	configinformers "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/informers/externalversions"
	testinghelpers "github.com/open-cluster-management/submariner-addon/pkg/helpers/testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeinformers "k8s.io/client-go/informers"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

var now = metav1.Now()

func TestConfigControllerSync(t *testing.T) {
	cases := []struct {
		name            string
		addOns          []runtime.Object
		configs         []runtime.Object
		nodes           []runtime.Object
		expectedErr     string
		validateActions func(t *testing.T, kubeActions, addOnActions, configActions []clienttesting.Action)
	}{
		{
			name:    "no resources",
			addOns:  []runtime.Object{},
			configs: []runtime.Object{},
			nodes:   []runtime.Object{},
			validateActions: func(t *testing.T, kubeActions, addOnActions, configActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, kubeActions)
				testinghelpers.AssertNoActions(t, addOnActions)
				testinghelpers.AssertNoActions(t, configActions)
			},
		},
		{
			name: "creating aws config",
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "test",
						Finalizers: []string{
							"submarineraddon.open-cluster-management.io/submariner-addon-cleanup",
							"submarineraddon.open-cluster-management.io/submariner-addon-agent-cleanup",
						},
					},
				},
			},
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					Status: configv1alpha1.SubmarinerConfigStatus{
						ManagedClusterInfo: configv1alpha1.ManagedClusterInfo{
							Platform: "AWS",
						},
					},
				},
			},
			nodes: []runtime.Object{},
			validateActions: func(t *testing.T, kubeActions, addOnActions, configActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, kubeActions)
				testinghelpers.AssertNoActions(t, addOnActions)
				// add finalizers
				testinghelpers.AssertNoActions(t, configActions)
			},
		},
		{
			name: "creating an addon",
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "test",
						Finalizers: []string{
							"submarineraddon.open-cluster-management.io/submariner-addon-cleanup",
						},
					},
				},
			},
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "submariner",
						Namespace:  "test",
						Finalizers: []string{"submarineraddon.open-cluster-management.io/config-cleanup"},
					},
					Spec: configv1alpha1.SubmarinerConfigSpec{},
				},
			},
			nodes: []runtime.Object{},
			validateActions: func(t *testing.T, kubeActions, addOnActions, configActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, kubeActions)
				testinghelpers.AssertNoActions(t, configActions)
				testinghelpers.AssertActions(t, addOnActions, "update")
				addOn := addOnActions[0].(clienttesting.UpdateActionImpl).Object
				testinghelpers.AssertFinalizers(t, addOn, []string{
					"submarineraddon.open-cluster-management.io/submariner-addon-cleanup",
					"submarineraddon.open-cluster-management.io/submariner-addon-agent-cleanup",
				})
			},
		},
		{
			name: "creating a config",
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "test",
						Finalizers: []string{
							"submarineraddon.open-cluster-management.io/submariner-addon-cleanup",
							"submarineraddon.open-cluster-management.io/submariner-addon-agent-cleanup",
						},
					},
				},
			},
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "submariner",
						Namespace:  "test",
						Finalizers: []string{"submarineraddon.open-cluster-management.io/config-cleanup"},
					},
					Spec: configv1alpha1.SubmarinerConfigSpec{},
				},
			},
			nodes: []runtime.Object{},
			validateActions: func(t *testing.T, kubeActions, addOnActions, configActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, kubeActions)
				testinghelpers.AssertNoActions(t, addOnActions)
				testinghelpers.AssertActions(t, configActions, "update")
			},
		},
		{
			name: "gateways should be greater than 1",
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "test",
						Finalizers: []string{
							"submarineraddon.open-cluster-management.io/submariner-addon-cleanup",
							"submarineraddon.open-cluster-management.io/submariner-addon-agent-cleanup",
						},
					},
				},
			},
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "test",
						Finalizers: []string{
							"submarineraddon.open-cluster-management.io/config-cleanup",
							"submarineraddon.open-cluster-management.io/config-addon-cleanup",
						},
					},
					Spec: configv1alpha1.SubmarinerConfigSpec{},
				},
			},
			nodes: []runtime.Object{},
			validateActions: func(t *testing.T, kubeActions, addOnActions, configActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, kubeActions)
				testinghelpers.AssertNoActions(t, addOnActions)
				testinghelpers.AssertActions(t, configActions, "get", "update")
			},
		},
		{
			name: "no worker nodes",
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "test",
						Finalizers: []string{
							"submarineraddon.open-cluster-management.io/submariner-addon-cleanup",
							"submarineraddon.open-cluster-management.io/submariner-addon-agent-cleanup",
						},
					},
				},
			},
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker": "",
							"submariner.io/gateway":          "true",
							"gateway.submariner.io/udp-port": "4500",
						},
					},
				},
			},
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "test",
						Finalizers: []string{
							"submarineraddon.open-cluster-management.io/config-cleanup",
							"submarineraddon.open-cluster-management.io/config-addon-cleanup",
						},
					},
					Spec: configv1alpha1.SubmarinerConfigSpec{
						IPSecNATTPort: 4500,
						GatewayConfig: configv1alpha1.GatewayConfig{
							Gateways: 2,
						},
					},
				},
			},
			validateActions: func(t *testing.T, kubeActions, addOnActions, configActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, kubeActions)
				testinghelpers.AssertNoActions(t, addOnActions)
				// update config status
				testinghelpers.AssertActions(t, configActions, "get", "update")
			},
		},
		{
			name: "gateways were labeled ",
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "test",
						Finalizers: []string{
							"submarineraddon.open-cluster-management.io/submariner-addon-cleanup",
							"submarineraddon.open-cluster-management.io/submariner-addon-agent-cleanup",
						},
					},
				},
			},
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "test",
						Finalizers: []string{
							"submarineraddon.open-cluster-management.io/config-cleanup",
							"submarineraddon.open-cluster-management.io/config-addon-cleanup",
						},
					},
					Spec: configv1alpha1.SubmarinerConfigSpec{
						IPSecNATTPort: 4500,
						GatewayConfig: configv1alpha1.GatewayConfig{
							Gateways: 1,
						},
					},
				},
			},
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker": "",
							"submariner.io/gateway":          "true",
							"gateway.submariner.io/udp-port": "4500",
						},
					},
				},
			},
			validateActions: func(t *testing.T, kubeActions, addOnActions, configActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, kubeActions)
				testinghelpers.AssertNoActions(t, addOnActions)
				// update config status
				testinghelpers.AssertActions(t, configActions, "get", "update")
			},
		},
		{
			name: "labeling gateways",
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "test",
						Finalizers: []string{
							"submarineraddon.open-cluster-management.io/submariner-addon-cleanup",
							"submarineraddon.open-cluster-management.io/submariner-addon-agent-cleanup",
						},
					},
				},
			},
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "test",
						Finalizers: []string{
							"submarineraddon.open-cluster-management.io/config-cleanup",
							"submarineraddon.open-cluster-management.io/config-addon-cleanup",
						},
					},
					Spec: configv1alpha1.SubmarinerConfigSpec{
						IPSecNATTPort: 4500,
						GatewayConfig: configv1alpha1.GatewayConfig{
							Gateways: 1,
						},
					},
				},
			},
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-1",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker": "",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-2",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker": "",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-3",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker": "",
						},
					},
				},
			},
			validateActions: func(t *testing.T, kubeActions, addOnActions, configActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, addOnActions)
				// label nodes
				testinghelpers.AssertActions(t, kubeActions, "update")
				// update config status
				testinghelpers.AssertActions(t, configActions, "get", "update")
			},
		},
		{
			name: "labeling gateways - gcp",
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "test",
						Finalizers: []string{
							"submarineraddon.open-cluster-management.io/submariner-addon-cleanup",
							"submarineraddon.open-cluster-management.io/submariner-addon-agent-cleanup",
						},
					},
				},
			},
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "test",
						Finalizers: []string{
							"submarineraddon.open-cluster-management.io/config-cleanup",
							"submarineraddon.open-cluster-management.io/config-addon-cleanup",
						},
					},
					Spec: configv1alpha1.SubmarinerConfigSpec{
						IPSecNATTPort: 4500,
						GatewayConfig: configv1alpha1.GatewayConfig{
							Gateways: 4,
						},
					},
					Status: configv1alpha1.SubmarinerConfigStatus{
						ManagedClusterInfo: configv1alpha1.ManagedClusterInfo{
							Platform: "GCP",
						},
					},
				},
			},
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-e1",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker":         "",
							"failure-domain.beta.kubernetes.io/zone": "east",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-e2",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker":         "",
							"failure-domain.beta.kubernetes.io/zone": "east",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-w1",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker":         "",
							"failure-domain.beta.kubernetes.io/zone": "west",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-n1",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker":         "",
							"failure-domain.beta.kubernetes.io/zone": "north",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-n2",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker":         "",
							"failure-domain.beta.kubernetes.io/zone": "north",
						},
					},
				},
			},
			validateActions: func(t *testing.T, kubeActions, addOnActions, configActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, addOnActions)
				// label four nodes
				testinghelpers.AssertActions(t, kubeActions, "update", "update", "update", "update")
				// update config status
				testinghelpers.AssertActions(t, configActions, "get", "update")
			},
		},
		{
			name: "reduce gateways",
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "test",
						Finalizers: []string{
							"submarineraddon.open-cluster-management.io/submariner-addon-cleanup",
							"submarineraddon.open-cluster-management.io/submariner-addon-agent-cleanup",
						},
					},
				},
			},
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "test",
						Finalizers: []string{
							"submarineraddon.open-cluster-management.io/config-cleanup",
							"submarineraddon.open-cluster-management.io/config-addon-cleanup",
						},
					},
					Spec: configv1alpha1.SubmarinerConfigSpec{
						IPSecNATTPort: 4500,
						GatewayConfig: configv1alpha1.GatewayConfig{
							Gateways: 1,
						},
					},
					Status: configv1alpha1.SubmarinerConfigStatus{
						ManagedClusterInfo: configv1alpha1.ManagedClusterInfo{
							Platform: "GCP",
						},
					},
				},
			},
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-e1",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker":         "",
							"failure-domain.beta.kubernetes.io/zone": "east",
							"submariner.io/gateway":                  "true",
							"gateway.submariner.io/udp-port":         "4500",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-w1",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker":         "",
							"failure-domain.beta.kubernetes.io/zone": "west",
							"submariner.io/gateway":                  "true",
							"gateway.submariner.io/udp-port":         "4500",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-n1",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker":         "",
							"failure-domain.beta.kubernetes.io/zone": "north",
						},
					},
				},
			},
			validateActions: func(t *testing.T, kubeActions, addOnActions, configActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, addOnActions)
				// unlabel one node
				testinghelpers.AssertActions(t, kubeActions, "update")
				// update config status
				testinghelpers.AssertActions(t, configActions, "get", "update")
			},
		},
		{
			name: "deleting config",
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "test",
						Finalizers: []string{
							"submarineraddon.open-cluster-management.io/submariner-addon-cleanup",
							"submarineraddon.open-cluster-management.io/submariner-addon-agent-cleanup",
						},
					},
				},
			},
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "test",
						Finalizers: []string{
							"submarineraddon.open-cluster-management.io/config-cleanup",
							"submarineraddon.open-cluster-management.io/config-addon-cleanup",
						},
						DeletionTimestamp: &now,
					},
				},
			},
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-e1",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker":         "",
							"failure-domain.beta.kubernetes.io/zone": "east",
							"submariner.io/gateway":                  "true",
							"gateway.submariner.io/udp-port":         "4500",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-w1",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker":         "",
							"failure-domain.beta.kubernetes.io/zone": "west",
							"submariner.io/gateway":                  "true",
							"gateway.submariner.io/udp-port":         "4500",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test-n1",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker":         "",
							"failure-domain.beta.kubernetes.io/zone": "north",
						},
					},
				},
			},
			validateActions: func(t *testing.T, kubeActions, addOnActions, configActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, addOnActions)
				// remove all gateways
				testinghelpers.AssertActions(t, kubeActions, "update", "update")
				// remove finalizer
				testinghelpers.AssertActions(t, configActions, "update")
			},
		},
		{
			name: "deleting addon",
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "test",
						Finalizers: []string{
							"submarineraddon.open-cluster-management.io/submariner-addon-cleanup",
							"submarineraddon.open-cluster-management.io/submariner-addon-agent-cleanup",
						},
						DeletionTimestamp: &now,
					},
				},
			},
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "test",
						Finalizers: []string{
							"submarineraddon.open-cluster-management.io/config-cleanup",
							"submarineraddon.open-cluster-management.io/config-addon-cleanup",
						},
					},
					Spec:   configv1alpha1.SubmarinerConfigSpec{},
					Status: configv1alpha1.SubmarinerConfigStatus{},
				},
			},
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
						Labels: map[string]string{
							"node-role.kubernetes.io/worker": "",
							"submariner.io/gateway":          "true",
							"gateway.submariner.io/udp-port": "4500",
						},
					},
				},
			},
			validateActions: func(t *testing.T, kubeActions, addOnActions, configActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, kubeActions, "update")
				testinghelpers.AssertActions(t, addOnActions, "update")
				testinghelpers.AssertActions(t, configActions, "update", "get", "update")
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			kubeClient := kubefake.NewSimpleClientset(c.nodes...)
			kubeInformerFactory := kubeinformers.NewSharedInformerFactory(kubeClient, time.Minute*10)
			nodeStore := kubeInformerFactory.Core().V1().Nodes().Informer().GetStore()
			for _, node := range c.nodes {
				nodeStore.Add(node)
			}

			addOnClient := addonfake.NewSimpleClientset(c.addOns...)
			addOnInformerFactory := addoninformers.NewSharedInformerFactory(addOnClient, time.Minute*10)
			addOnStroe := addOnInformerFactory.Addon().V1alpha1().ManagedClusterAddOns().Informer().GetStore()
			for _, addOn := range c.addOns {
				addOnStroe.Add(addOn)
			}

			configClient := fakeconfigclient.NewSimpleClientset(c.configs...)
			configInformerFactory := configinformers.NewSharedInformerFactory(configClient, 5*time.Minute)
			for _, config := range c.configs {
				configInformerFactory.Submarineraddon().V1alpha1().SubmarinerConfigs().Informer().GetStore().Add(config)
			}

			ctrl := &submarinerAgentConfigController{
				kubeClient:   kubeClient,
				addOnClient:  addOnClient,
				configClient: configClient,
				nodeLister:   kubeInformerFactory.Core().V1().Nodes().Lister(),
				addOnLister:  addOnInformerFactory.Addon().V1alpha1().ManagedClusterAddOns().Lister(),
				configLister: configInformerFactory.Submarineraddon().V1alpha1().SubmarinerConfigs().Lister(),
				clusterName:  "test",
			}

			err := ctrl.sync(context.TODO(), testinghelpers.NewFakeSyncContext(t, ""))
			if c.expectedErr == "" && err != nil {
				t.Errorf("unexpected err: %v", err)
			}
			if c.expectedErr != "" && err == nil {
				t.Errorf("expected err: %v, but failed", c.expectedErr)
			}
			if c.expectedErr != "" && err.Error() != c.expectedErr {
				t.Errorf("expected err: %q, but got: %q", c.expectedErr, err.Error())
			}

			c.validateActions(t, kubeClient.Actions(), addOnClient.Actions(), configClient.Actions())
		})
	}
}
