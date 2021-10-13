package submarineragent

import (
	"context"
	"os"
	"testing"
	"time"

	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	fakeconfigclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned/fake"
	configinformers "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/informers/externalversions"
	testinghelpers "github.com/open-cluster-management/submariner-addon/pkg/helpers/testing"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonfake "open-cluster-management.io/api/client/addon/clientset/versioned/fake"
	addoninformers "open-cluster-management.io/api/client/addon/informers/externalversions"
	fakeclusterclient "open-cluster-management.io/api/client/cluster/clientset/versioned/fake"
	clusterinformers "open-cluster-management.io/api/client/cluster/informers/externalversions"
	fakeworkclient "open-cluster-management.io/api/client/work/clientset/versioned/fake"
	workinformers "open-cluster-management.io/api/client/work/informers/externalversions"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	workv1 "open-cluster-management.io/api/work/v1"

	"github.com/openshift/library-go/pkg/operator/events/eventstesting"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

var now = metav1.Now()

func TestSyncManagedCluster(t *testing.T) {
	if err := os.Setenv("BROKER_API_SERVER", "127.0.0.1"); err != nil {
		t.Fatalf("cannot set env BROKER_API_SERVER")
	}

	defer os.Unsetenv("BROKER_API_SERVER")

	cases := []struct {
		name            string
		clusterName     string
		clusters        []runtime.Object
		clustersets     []runtime.Object
		addOns          []runtime.Object
		kubeObjs        []runtime.Object
		validateActions func(t *testing.T, kubeActions, clusterActions, workActions, addonActions []clienttesting.Action)
	}{
		{
			name:        "No managed cluster",
			clusterName: "cluster1",
			clusters:    []runtime.Object{},
			clustersets: []runtime.Object{},
			addOns:      []runtime.Object{},
			kubeObjs:    []runtime.Object{},
			validateActions: func(t *testing.T, kubeActions, clusterActions, workActions, addonActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, kubeActions)
				testinghelpers.AssertNoActions(t, clusterActions)
				testinghelpers.AssertNoActions(t, workActions)
				testinghelpers.AssertNoActions(t, addonActions)
			},
		},
		{
			name:        "No clusterset label on managed cluster",
			clusterName: "cluster1",
			clusters: []runtime.Object{
				newManagedCluster(map[string]string{}, []string{agentFinalizer}, false),
			},
			clustersets: []runtime.Object{},
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "submariner",
						Namespace:  "cluster1",
						Finalizers: []string{addOnFinalizer},
					},
				},
			},
			kubeObjs: []runtime.Object{},
			validateActions: func(t *testing.T, kubeActions, clusterActions, workActions, addonActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, kubeActions, "list")
				testinghelpers.AssertActions(t, clusterActions, "update")
				testinghelpers.AssertActions(t, workActions, "delete")
				testinghelpers.AssertActions(t, addonActions, "update")
			},
		},
		{
			name:        "No submariner on managed cluster namesapce",
			clusterName: "cluster1",
			clusters:    []runtime.Object{newManagedCluster(map[string]string{clusterSetLabel: "set1"}, []string{}, false)},
			clustersets: []runtime.Object{
				newManagedClusterSet(),
			},
			addOns:   []runtime.Object{},
			kubeObjs: []runtime.Object{},
			validateActions: func(t *testing.T, kubeActions, clusterActions, workActions, addonActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, kubeActions)
				testinghelpers.AssertNoActions(t, clusterActions)
				testinghelpers.AssertNoActions(t, workActions)
				testinghelpers.AssertNoActions(t, addonActions)
			},
		},
		{
			name:        "Clusterset is not found by clusterset label",
			clusterName: "cluster1",
			clusters: []runtime.Object{
				newManagedCluster(map[string]string{clusterSetLabel: "set1"}, []string{agentFinalizer}, false),
			},
			clustersets: []runtime.Object{},
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "cluster1",
					},
				},
			},
			kubeObjs: []runtime.Object{},
			validateActions: func(t *testing.T, kubeActions, clusterActions, workActions, addonActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, kubeActions, "list")
				testinghelpers.AssertActions(t, clusterActions, "update")
				testinghelpers.AssertActions(t, workActions, "delete")
				testinghelpers.AssertNoActions(t, addonActions)
			},
		},
		{
			name:        "Sync a labeled managed cluster",
			clusterName: "cluster1",
			clusters: []runtime.Object{
				newManagedCluster(map[string]string{clusterSetLabel: "set1"}, []string{}, false),
			},
			clustersets: []runtime.Object{newManagedClusterSet()},
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "cluster1",
					},
				},
			},
			kubeObjs: []runtime.Object{},
			validateActions: func(t *testing.T, kubeActions, clusterActions, workActions, addonActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, kubeActions)
				testinghelpers.AssertActions(t, clusterActions, "update")
				managedCluster := clusterActions[0].(clienttesting.UpdateActionImpl).Object
				testinghelpers.AssertFinalizers(t, managedCluster, []string{agentFinalizer})
				testinghelpers.AssertNoActions(t, workActions)
				testinghelpers.AssertNoActions(t, addonActions)
			},
		},
		{
			name:        "Sync submarienr-addon is created",
			clusterName: "cluster1",
			clusters: []runtime.Object{
				newManagedCluster(map[string]string{clusterSetLabel: "set1"}, []string{agentFinalizer}, false),
			},
			clustersets: []runtime.Object{newManagedClusterSet()},
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "cluster1",
					},
				},
			},
			kubeObjs: []runtime.Object{},
			validateActions: func(t *testing.T, kubeActions, clusterActions, workActions, addonActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, kubeActions)
				testinghelpers.AssertNoActions(t, clusterActions)
				testinghelpers.AssertNoActions(t, workActions)
				testinghelpers.AssertActions(t, addonActions, "update")
				addOn := addonActions[0].(clienttesting.UpdateActionImpl).Object
				testinghelpers.AssertFinalizers(t, addOn, []string{addOnFinalizer})
			},
		},
		{
			name:        "Deploy submariner agent",
			clusterName: "cluster1",
			clusters: []runtime.Object{
				newManagedCluster(map[string]string{clusterSetLabel: "set1"}, []string{agentFinalizer}, false),
			},
			clustersets: []runtime.Object{newManagedClusterSet()},
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "submariner",
						Namespace:  "cluster1",
						Finalizers: []string{addOnFinalizer},
					},
				},
			},
			kubeObjs: []runtime.Object{
				newSecret("set1-broker", "submariner-ipsec-psk", map[string][]byte{"psk": []byte("psk")}),
				newSecret("set1-broker", "cluster1-token-5pw5c", map[string][]byte{"ca.crt": []byte("ca"), "token": []byte("token")}),
				newServiceAccount("set1-broker", "cluster1", "cluster1-token-5pw5c", map[string]string{}),
			},
			validateActions: func(t *testing.T, kubeActions, clusterActions, workActions, addonActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, clusterActions)
				testinghelpers.AssertActions(t, workActions, "get", "create")
				work := (workActions[1].(clienttesting.CreateActionImpl).Object).(*workv1.ManifestWork)
				if work.Name != "submariner-operator" {
					t.Errorf("expected work submariner-agent-crds, but got %+v", work)
				}
				testinghelpers.AssertNoActions(t, addonActions)
			},
		},
		{
			name:        "Delete a mangaged cluster",
			clusterName: "cluster1",
			clusters: []runtime.Object{
				newManagedCluster(map[string]string{clusterSetLabel: "set1"}, []string{"test", agentFinalizer}, true),
			},
			clustersets: []runtime.Object{newManagedClusterSet()},
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "submariner",
						Namespace:  "cluster1",
						Finalizers: []string{addOnFinalizer},
					},
				},
			},
			kubeObjs: []runtime.Object{
				newServiceAccount("cluster1", "cluster1", "cluster1", map[string]string{serviceAccountLabel: "cluster1"}),
			},
			validateActions: func(t *testing.T, kubeActions, clusterActions, workActions, addonActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, kubeActions, "list", "delete", "delete")
				testinghelpers.AssertActions(t, clusterActions, "update")
				managedCluster := clusterActions[0].(clienttesting.UpdateActionImpl).Object
				testinghelpers.AssertFinalizers(t, managedCluster, []string{"test"})
				testinghelpers.AssertActions(t, workActions, "delete")
				testinghelpers.AssertActions(t, addonActions, "update")
			},
		},
		{
			name:        "Delete the submariner-addon",
			clusterName: "cluster1",
			clusters: []runtime.Object{
				newManagedCluster(map[string]string{clusterSetLabel: "set1"}, []string{"test", agentFinalizer}, false),
			},
			clustersets: []runtime.Object{newManagedClusterSet()},
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "submariner",
						Namespace:         "cluster1",
						Finalizers:        []string{addOnFinalizer},
						DeletionTimestamp: &now,
					},
				},
			},
			kubeObjs: []runtime.Object{
				newServiceAccount("cluster1", "cluster1", "cluster1", map[string]string{serviceAccountLabel: "cluster1"}),
			},
			validateActions: func(t *testing.T, kubeActions, clusterActions, workActions, addonActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, kubeActions, "list", "delete", "delete")
				testinghelpers.AssertActions(t, clusterActions, "update")
				managedCluster := clusterActions[0].(clienttesting.UpdateActionImpl).Object
				testinghelpers.AssertFinalizers(t, managedCluster, []string{"test"})
				testinghelpers.AssertActions(t, workActions, "delete")
				testinghelpers.AssertActions(t, addonActions, "update")
				addOn := addonActions[0].(clienttesting.UpdateActionImpl).Object
				testinghelpers.AssertFinalizers(t, addOn, []string{})
			},
		},
		{
			name:        "Sync all managed clusters",
			clusterName: "key",
			clusters: []runtime.Object{
				newManagedCluster(map[string]string{clusterSetLabel: "set1"}, []string{}, false),
			},
			clustersets: []runtime.Object{newManagedClusterSet()},
			addOns: []runtime.Object{
				&addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "cluster1",
					},
				},
			},
			kubeObjs: []runtime.Object{},
			validateActions: func(t *testing.T, kubeActions, clusterActions, workActions, addonActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, kubeActions)
				testinghelpers.AssertNoActions(t, clusterActions)
				testinghelpers.AssertNoActions(t, workActions)
				testinghelpers.AssertNoActions(t, addonActions)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeKubeClient := kubefake.NewSimpleClientset(c.kubeObjs...)
			fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())

			objects := []runtime.Object{}
			objects = append(objects, c.clusters...)
			objects = append(objects, c.clustersets...)
			fakeClusterClient := fakeclusterclient.NewSimpleClientset(objects...)
			clusterInformerFactory := clusterinformers.NewSharedInformerFactory(fakeClusterClient, 5*time.Minute)
			for _, cluster := range c.clusters {
				clusterInformerFactory.Cluster().V1().ManagedClusters().Informer().GetStore().Add(cluster)
			}
			for _, clusterset := range c.clustersets {
				clusterInformerFactory.Cluster().V1beta1().ManagedClusterSets().Informer().GetStore().Add(clusterset)
			}

			fakeWorkClient := fakeworkclient.NewSimpleClientset()
			workInformerFactory := workinformers.NewSharedInformerFactory(fakeWorkClient, 5*time.Minute)

			configClient := fakeconfigclient.NewSimpleClientset()
			configInformerFactory := configinformers.NewSharedInformerFactory(configClient, 5*time.Minute)

			addOnClient := addonfake.NewSimpleClientset(c.addOns...)
			addOnInformerFactory := addoninformers.NewSharedInformerFactory(addOnClient, time.Minute*10)
			addOnStroe := addOnInformerFactory.Addon().V1alpha1().ManagedClusterAddOns().Informer().GetStore()
			for _, addOn := range c.addOns {
				addOnStroe.Add(addOn)
			}

			ctrl := &submarinerAgentController{
				kubeClient:         fakeKubeClient,
				dynamicClient:      fakeDynamicClient,
				clusterClient:      fakeClusterClient,
				manifestWorkClient: fakeWorkClient,
				configClient:       configClient,
				addOnClient:        addOnClient,
				clusterLister:      clusterInformerFactory.Cluster().V1().ManagedClusters().Lister(),
				clusterSetLister:   clusterInformerFactory.Cluster().V1beta1().ManagedClusterSets().Lister(),
				manifestWorkLister: workInformerFactory.Work().V1().ManifestWorks().Lister(),
				configLister:       configInformerFactory.Submarineraddon().V1alpha1().SubmarinerConfigs().Lister(),
				addOnLister:        addOnInformerFactory.Addon().V1alpha1().ManagedClusterAddOns().Lister(),
				eventRecorder:      eventstesting.NewTestingEventRecorder(t),
			}

			err := ctrl.sync(context.TODO(), testinghelpers.NewFakeSyncContext(t, c.clusterName))
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}

			c.validateActions(t, fakeKubeClient.Actions(), fakeClusterClient.Actions(), fakeWorkClient.Actions(), addOnClient.Actions())
		})
	}
}

func TestSyncSubmarinerConfig(t *testing.T) {
	now := metav1.Now()
	cases := []struct {
		name            string
		queueKey        string
		configs         []runtime.Object
		clusters        []runtime.Object
		expectedErr     bool
		validateActions func(t *testing.T, configActions []clienttesting.Action)
	}{
		{
			name:     "no submariner config",
			queueKey: "cluster1",
			configs:  []runtime.Object{},
			clusters: []runtime.Object{
				&clusterv1.ManagedCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster1",
					},
				},
			},
			validateActions: func(t *testing.T, configActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, configActions)
			},
		},
		{
			name:     "too many configs",
			queueKey: "",
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner-config-1",
						Namespace: "cluster1",
					},
				},
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner-config-2",
						Namespace: "cluster1",
					},
				},
			},
			clusters: []runtime.Object{
				&clusterv1.ManagedCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster1",
					},
				},
			},
			validateActions: func(t *testing.T, configActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, configActions)
			},
		},
		{
			name:     "create a submariner config",
			queueKey: "cluster1",
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "cluster1",
					},
				},
			},
			clusters: []runtime.Object{
				&clusterv1.ManagedCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster1",
					},
				},
			},
			validateActions: func(t *testing.T, configActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, configActions, "update")
				updatedObj := configActions[0].(clienttesting.UpdateActionImpl).Object
				config := updatedObj.(*configv1alpha1.SubmarinerConfig)
				if len(config.Finalizers) != 1 {
					t.Errorf("unexpect size of finalizers")
				}
				if config.Finalizers[0] != "submarineraddon.open-cluster-management.io/config-cleanup" {
					t.Errorf("unexpect finalizers")
				}
			},
		},
		{
			name:     "create a submariner config",
			queueKey: "cluster1/submariner",
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "cluster1",
					},
				},
			},
			clusters: []runtime.Object{
				&clusterv1.ManagedCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster1",
					},
				},
			},
			validateActions: func(t *testing.T, configActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, configActions, "update")
				updatedObj := configActions[0].(clienttesting.UpdateActionImpl).Object
				config := updatedObj.(*configv1alpha1.SubmarinerConfig)
				if len(config.Finalizers) != 1 {
					t.Errorf("unexpect size of finalizers")
				}
				if config.Finalizers[0] != "submarineraddon.open-cluster-management.io/config-cleanup" {
					t.Errorf("unexpect finalizers")
				}
			},
		},
		{
			name:     "no managed cluster",
			queueKey: "cluster1/submariner",
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "submariner",
						Namespace:  "cluster1",
						Finalizers: []string{"submarineraddon.open-cluster-management.io/config-cleanup"},
					},
					Spec: configv1alpha1.SubmarinerConfigSpec{
						CredentialsSecret: &corev1.LocalObjectReference{
							Name: "test",
						},
					},
				},
			},
			clusters: []runtime.Object{},
			validateActions: func(t *testing.T, configActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, configActions)
			},
		},
		{
			name:     "delete a submariner config",
			queueKey: "cluster1/submariner",
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "submariner",
						Namespace:         "cluster1",
						DeletionTimestamp: &now,
						Finalizers:        []string{"submarineraddon.open-cluster-management.io/config-cleanup"},
					},
					Status: configv1alpha1.SubmarinerConfigStatus{
						ManagedClusterInfo: configv1alpha1.ManagedClusterInfo{
							Platform: "GCP",
						},
					},
				},
			},
			clusters: []runtime.Object{
				&clusterv1.ManagedCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "cluster1",
						Labels:     map[string]string{"vendor": "unsupported"},
						Finalizers: []string{"cluster.open-cluster-management.io/submariner-config-cleanup"},
					},
				},
			},
			validateActions: func(t *testing.T, configActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, configActions, "update")
				workUpdatedObj := configActions[0].(clienttesting.UpdateActionImpl).Object
				config := workUpdatedObj.(*configv1alpha1.SubmarinerConfig)
				if len(config.Finalizers) != 0 {
					t.Errorf("unexpect size of finalizers")
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			configClient := fakeconfigclient.NewSimpleClientset(c.configs...)
			configInformerFactory := configinformers.NewSharedInformerFactory(configClient, 5*time.Minute)
			for _, config := range c.configs {
				configInformerFactory.Submarineraddon().V1alpha1().SubmarinerConfigs().Informer().GetStore().Add(config)
			}

			clusterClient := fakeclusterclient.NewSimpleClientset(c.clusters...)
			clusterInformerFactory := clusterinformers.NewSharedInformerFactory(clusterClient, 5*time.Minute)
			for _, cluster := range c.clusters {
				clusterInformerFactory.Cluster().V1().ManagedClusters().Informer().GetStore().Add(cluster)
			}

			addOnClient := addonfake.NewSimpleClientset()
			addOnInformerFactory := addoninformers.NewSharedInformerFactory(addOnClient, time.Minute*10)

			ctrl := &submarinerAgentController{
				kubeClient:         kubefake.NewSimpleClientset(),
				addOnClient:        addOnClient,
				addOnLister:        addOnInformerFactory.Addon().V1alpha1().ManagedClusterAddOns().Lister(),
				clusterClient:      clusterClient,
				configClient:       configClient,
				clusterLister:      clusterInformerFactory.Cluster().V1().ManagedClusters().Lister(),
				clusterSetLister:   clusterInformerFactory.Cluster().V1beta1().ManagedClusterSets().Lister(),
				manifestWorkClient: fakeworkclient.NewSimpleClientset(),
				configLister:       configInformerFactory.Submarineraddon().V1alpha1().SubmarinerConfigs().Lister(),
				eventRecorder:      eventstesting.NewTestingEventRecorder(t),
			}
			err := ctrl.sync(context.TODO(), testinghelpers.NewFakeSyncContext(t, c.queueKey))
			if c.expectedErr && err == nil {
				t.Errorf("expect err, but no error")
			}
			if !c.expectedErr && err != nil {
				t.Errorf("unexpect err: %v", err)
			}
			c.validateActions(t, configClient.Actions())
		})
	}
}

func newManagedCluster(labels map[string]string, finalizers []string, terminating bool) *clusterv1.ManagedCluster {
	cluster := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "cluster1",
			Labels:     labels,
			Finalizers: finalizers,
		},
	}

	if terminating {
		now := metav1.Now()
		cluster.DeletionTimestamp = &now
	}

	return cluster
}

func newManagedClusterSet() *clusterv1beta1.ManagedClusterSet {
	clusterSet := &clusterv1beta1.ManagedClusterSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: "set1",
		},
	}

	return clusterSet
}

func newSecret(namespace, name string, data map[string][]byte) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Data: data,
		Type: corev1.SecretTypeServiceAccountToken,
	}
}

func newServiceAccount(namespace, name, secretName string, labels map[string]string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels:    labels,
		},
		Secrets: []corev1.ObjectReference{{Name: secretName}},
	}
}
