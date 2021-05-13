package submarineragent

import (
	"context"
	"testing"
	"time"

	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	fakeconfigclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned/fake"
	configinformers "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/informers/externalversions"
	testinghelpers "github.com/open-cluster-management/submariner-addon/pkg/helpers/testing"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
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
		queueKey        string
		nodes           []runtime.Object
		configs         []runtime.Object
		validateActions func(t *testing.T, actions []clienttesting.Action)
	}{
		{
			name:     "no labled submariner gateway nodes",
			queueKey: "test",
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
					},
				},
			},
			configs: []runtime.Object{},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, actions)
			},
		},
		{
			name:     "no configs",
			queueKey: "",
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
			configs: []runtime.Object{},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, actions)
			},
		},
		{
			name:     "create a submariner config",
			queueKey: "test/submariner",
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
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "submariner",
						Namespace:  "test",
						Finalizers: []string{"submarineraddon.open-cluster-management.io/config-cleanup"},
					},
					Spec: configv1alpha1.SubmarinerConfigSpec{
						CredentialsSecret: &v1.LocalObjectReference{
							Name: "aws-credentials-secret",
						},
					},
				},
			},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, actions)
			},
		},
		{
			name:     "default natt port",
			queueKey: "test/submariner",
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
						CredentialsSecret: &v1.LocalObjectReference{
							Name: "aws-credentials-secret",
						},
					},
				},
			},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testinghelpers.AssertActions(t, actions, "update")
			},
		},
		{
			name:     "label gateway nodes",
			queueKey: "test2",
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test1",
						Labels: map[string]string{
							"submariner.io/gateway":          "true",
							"gateway.submariner.io/udp-port": "4501",
						},
					},
				},
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test2",
						Labels: map[string]string{
							"submariner.io/gateway": "true",
						},
					},
				},
			},
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "test",
					},
					Spec: configv1alpha1.SubmarinerConfigSpec{
						IPSecNATTPort: 4501,
						CredentialsSecret: &v1.LocalObjectReference{
							Name: "aws-credentials-secret",
						},
					},
				},
			},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testinghelpers.AssertActions(t, actions, "update")
			},
		},
		{
			name:     "remvoe config",
			queueKey: "test/submariner",
			nodes: []runtime.Object{
				&corev1.Node{
					ObjectMeta: metav1.ObjectMeta{
						Name: "test",
						Labels: map[string]string{
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
						DeletionTimestamp: &now,
					},
					Spec: configv1alpha1.SubmarinerConfigSpec{
						CredentialsSecret: &v1.LocalObjectReference{
							Name: "aws-credentials-secret",
						},
					},
				},
			},
			validateActions: func(t *testing.T, actions []clienttesting.Action) {
				testinghelpers.AssertActions(t, actions, "update")
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

			configClient := fakeconfigclient.NewSimpleClientset(c.configs...)
			configInformerFactory := configinformers.NewSharedInformerFactory(configClient, 5*time.Minute)
			for _, config := range c.configs {
				configInformerFactory.Submarineraddon().V1alpha1().SubmarinerConfigs().Informer().GetStore().Add(config)
			}

			ctrl := &submarinerAgentConfigController{
				kubeClient:   kubeClient,
				configClient: configClient,
				nodeLister:   kubeInformerFactory.Core().V1().Nodes().Lister(),
				configLister: configInformerFactory.Submarineraddon().V1alpha1().SubmarinerConfigs().Lister(),
				clusterName:  "test",
			}

			err := ctrl.sync(context.TODO(), testinghelpers.NewFakeSyncContext(t, c.queueKey))
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
			c.validateActions(t, kubeClient.Actions())
		})
	}
}
