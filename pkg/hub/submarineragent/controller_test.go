package submarineragent

import (
	"context"
	"os"
	"testing"
	"time"

	clusterfake "github.com/open-cluster-management/api/client/cluster/clientset/versioned/fake"
	fakeclusterclient "github.com/open-cluster-management/api/client/cluster/clientset/versioned/fake"
	clusterinformers "github.com/open-cluster-management/api/client/cluster/informers/externalversions"
	fakeworkclient "github.com/open-cluster-management/api/client/work/clientset/versioned/fake"
	workfake "github.com/open-cluster-management/api/client/work/clientset/versioned/fake"
	workinformers "github.com/open-cluster-management/api/client/work/informers/externalversions"
	clusterv1 "github.com/open-cluster-management/api/cluster/v1"
	clusterv1alpha1 "github.com/open-cluster-management/api/cluster/v1alpha1"
	workv1 "github.com/open-cluster-management/api/work/v1"
	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	fakeconfigclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned/fake"
	testinghelpers "github.com/open-cluster-management/submariner-addon/pkg/helpers/testing"

	"github.com/openshift/library-go/pkg/operator/events/eventstesting"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestSyncManagedCluster(t *testing.T) {
	if err := os.Setenv("BROKER_API_SERVER", "127.0.0.1"); err != nil {
		t.Fatalf("cannot set env BROKER_API_SERVER")
	}

	defer func() {
		os.Unsetenv("BROKER_API_SERVER")
	}()

	cases := []struct {
		name            string
		clusterName     string
		clusters        []runtime.Object
		clustersets     []runtime.Object
		kubeObjs        []runtime.Object
		validateActions func(t *testing.T, kubeActions, clusterActions, workActions []clienttesting.Action)
	}{
		{
			name:        "No managed cluster",
			clusterName: "cluster1",
			clusters:    []runtime.Object{},
			clustersets: []runtime.Object{},
			kubeObjs:    []runtime.Object{},
			validateActions: func(t *testing.T, kubeActions, clusterActions, workActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, kubeActions)
				testinghelpers.AssertNoActions(t, clusterActions)
				testinghelpers.AssertNoActions(t, workActions)
			},
		},
		{
			name:        "No submariner label on managed cluster",
			clusterName: "cluster1",
			clusters:    []runtime.Object{newManagedCluster("cluster1", map[string]string{}, []string{}, false)},
			clustersets: []runtime.Object{},
			kubeObjs:    []runtime.Object{},
			validateActions: func(t *testing.T, kubeActions, clusterActions, workActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, kubeActions, "list")
				testinghelpers.AssertActions(t, clusterActions, "get")
				testinghelpers.AssertActions(t, workActions, "delete", "delete", "delete")
			},
		},
		{
			name:        "No clusterset label on managed cluster",
			clusterName: "cluster1",
			clusters: []runtime.Object{
				newManagedCluster("cluster1", map[string]string{submarinerLabel: ""}, []string{}, false),
			},
			kubeObjs:    []runtime.Object{},
			clustersets: []runtime.Object{},
			validateActions: func(t *testing.T, kubeActions, clusterActions, workActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, kubeActions, "list")
				testinghelpers.AssertActions(t, clusterActions, "get")
				testinghelpers.AssertActions(t, workActions, "delete", "delete", "delete")
			},
		},
		{
			name:        "Clusterset is not found by clusterset label",
			clusterName: "cluster1",
			clusters: []runtime.Object{
				newManagedCluster("cluster1", map[string]string{submarinerLabel: "", clusterSetLabel: "set1"}, []string{}, false),
			},
			kubeObjs:    []runtime.Object{},
			clustersets: []runtime.Object{},
			validateActions: func(t *testing.T, kubeActions, clusterActions, workActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, kubeActions, "list")
				testinghelpers.AssertActions(t, clusterActions, "get", "get")
				testinghelpers.AssertActions(t, workActions, "delete", "delete", "delete")
			},
		},
		{
			name:        "Sync a labeled managed cluster",
			clusterName: "cluster1",
			clusters: []runtime.Object{
				newManagedCluster("cluster1", map[string]string{submarinerLabel: "", clusterSetLabel: "set1"}, []string{}, false),
			},
			clustersets: []runtime.Object{newManagedClusterSet("set1")},
			kubeObjs:    []runtime.Object{},
			validateActions: func(t *testing.T, kubeActions, clusterActions, workActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, kubeActions)
				testinghelpers.AssertNoActions(t, workActions)
				testinghelpers.AssertActions(t, clusterActions, "get", "update")
				managedCluster := clusterActions[1].(clienttesting.UpdateActionImpl).Object
				testinghelpers.AssertFinalizers(t, managedCluster, []string{agentFinalizer})
			},
		},
		{
			name:        "Deploy submariner agent",
			clusterName: "cluster1",
			clusters: []runtime.Object{
				newManagedCluster("cluster1", map[string]string{submarinerLabel: "", clusterSetLabel: "set1"}, []string{agentFinalizer}, false),
			},
			clustersets: []runtime.Object{newManagedClusterSet("set1")},
			kubeObjs: []runtime.Object{
				newSecret("submariner-clusterset-set1-broker", "submariner-ipsec-psk", map[string][]byte{"psk": []byte("psk")}),
				newSecret("submariner-clusterset-set1-broker", "cluster1-token-5pw5c", map[string][]byte{"ca.crt": []byte("ca"), "token": []byte("token")}),
				newServiceAccount("submariner-clusterset-set1-broker", "cluster1", "cluster1-token-5pw5c", map[string]string{}),
			},
			validateActions: func(t *testing.T, kubeActions, clusterActions, workActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, workActions, "get", "create", "get", "create", "get", "create")
				work1 := (workActions[1].(clienttesting.CreateActionImpl).Object).(*workv1.ManifestWork)
				if work1.Name != "submariner-agent-crds" {
					t.Errorf("expected work submariner-agent-crds, but got %v", work1)
				}
				work2 := (workActions[3].(clienttesting.CreateActionImpl).Object).(*workv1.ManifestWork)
				if work2.Name != "submariner-agent-rbac" {
					t.Errorf("expected work submariner-agent-rbac, but got %v", work2)

				}
				work3 := (workActions[5].(clienttesting.CreateActionImpl).Object).(*workv1.ManifestWork)
				if work3.Name != "submariner-agent-operator" {
					t.Errorf("expected work submariner-agent-operator, but got %v", work3)
				}
			},
		},
		{
			name:        "Delete a mangaged cluster",
			clusterName: "cluster1",
			clusters: []runtime.Object{
				newManagedCluster("cluster1", map[string]string{submarinerLabel: "", clusterSetLabel: "set1"}, []string{"test", agentFinalizer}, true),
			},
			clustersets: []runtime.Object{newManagedClusterSet("set1")},
			kubeObjs: []runtime.Object{
				newServiceAccount("cluster1", "cluster1", "cluster1", map[string]string{serviceAccountLabel: "cluster1"}),
			},
			validateActions: func(t *testing.T, kubeActions, clusterActions, workActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, kubeActions, "list", "delete", "delete")
				testinghelpers.AssertActions(t, clusterActions, "get", "get", "update")
				managedCluster := clusterActions[2].(clienttesting.UpdateActionImpl).Object
				testinghelpers.AssertFinalizers(t, managedCluster, []string{"test"})
			},
		},
		{
			name:        "Sync all managed clusters",
			clusterName: "key",
			clusters: []runtime.Object{
				newManagedCluster("cluster1", map[string]string{submarinerLabel: "", clusterSetLabel: "set1"}, []string{}, false),
			},
			clustersets: []runtime.Object{newManagedClusterSet("set1")},
			kubeObjs:    []runtime.Object{},
			validateActions: func(t *testing.T, kubeActions, clusterActions, workActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, kubeActions)
				testinghelpers.AssertNoActions(t, workActions)
				testinghelpers.AssertActions(t, clusterActions, "get", "update")
				managedCluster := clusterActions[1].(clienttesting.UpdateActionImpl).Object
				testinghelpers.AssertFinalizers(t, managedCluster, []string{agentFinalizer})
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
			fakeClusterClient := clusterfake.NewSimpleClientset(objects...)
			clusterInformerFactory := clusterinformers.NewSharedInformerFactory(fakeClusterClient, 5*time.Minute)
			for _, cluster := range c.clusters {
				clusterInformerFactory.Cluster().V1().ManagedClusters().Informer().GetStore().Add(cluster)
			}

			fakeWorkClient := workfake.NewSimpleClientset()
			workInformerFactory := workinformers.NewSharedInformerFactory(fakeWorkClient, 5*time.Minute)

			ctrl := &submarinerAgentController{
				kubeClient:         fakeKubeClient,
				dynamicClient:      fakeDynamicClient,
				clusterClient:      fakeClusterClient,
				manifestWorkClient: fakeWorkClient,
				clusterLister:      clusterInformerFactory.Cluster().V1().ManagedClusters().Lister(),
				clusterSetLister:   clusterInformerFactory.Cluster().V1alpha1().ManagedClusterSets().Lister(),
				manifestWorkLister: workInformerFactory.Work().V1().ManifestWorks().Lister(),
				configClient:       fakeconfigclient.NewSimpleClientset(),
				eventRecorder:      eventstesting.NewTestingEventRecorder(t),
			}

			err := ctrl.sync(context.TODO(), testinghelpers.NewFakeSyncContext(t, c.clusterName))
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}

			c.validateActions(t, fakeKubeClient.Actions(), fakeClusterClient.Actions(), fakeWorkClient.Actions())
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
		validateActions func(t *testing.T, workActions []clienttesting.Action)
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
			validateActions: func(t *testing.T, workActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, workActions, "list")
			},
		},
		{
			name:     "too many configs",
			queueKey: "cluster1",
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
			validateActions: func(t *testing.T, workActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, workActions, "list")
			},
		},
		{
			name:     "create a submariner config",
			queueKey: "cluster1",
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner-config",
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
			validateActions: func(t *testing.T, workActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, workActions, "list", "update")
				updatedObj := workActions[1].(clienttesting.UpdateActionImpl).Object
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
			queueKey: "cluster1/submariner-config",
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner-config",
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
			validateActions: func(t *testing.T, workActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, workActions, "get", "update")
				updatedObj := workActions[1].(clienttesting.UpdateActionImpl).Object
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
			queueKey: "cluster1/submariner-config",
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "submariner-config",
						Namespace:  "cluster1",
						Finalizers: []string{"submarineraddon.open-cluster-management.io/config-cleanup"},
					},
					Spec: configv1alpha1.SubmarinerConfigSpec{
						CredentialsSecret: &v1.LocalObjectReference{
							Name: "test",
						},
					},
				},
			},
			clusters: []runtime.Object{},
			validateActions: func(t *testing.T, workActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, workActions, "get")
			},
		},
		{
			name:     "unspported cluster vendor",
			queueKey: "cluster1/submariner-config",
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "submariner-config",
						Namespace:  "cluster1",
						Finalizers: []string{"submarineraddon.open-cluster-management.io/config-cleanup"},
					},
					Spec: configv1alpha1.SubmarinerConfigSpec{
						CredentialsSecret: &v1.LocalObjectReference{
							Name: "test",
						},
					},
				},
			},
			clusters: []runtime.Object{
				&clusterv1.ManagedCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:   "cluster1",
						Labels: map[string]string{"vendor": "unspported"},
					},
				},
			},
			expectedErr: true,
			validateActions: func(t *testing.T, workActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, workActions, "get", "get", "update")
			},
		},
		{
			name:     "delete a submariner config without credentials secret",
			queueKey: "cluster1/submariner-config",
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "submariner-config",
						Namespace:         "cluster1",
						DeletionTimestamp: &now,
						Finalizers:        []string{"submarineraddon.open-cluster-management.io/config-cleanup"},
					},
				},
			},
			clusters: []runtime.Object{},
			validateActions: func(t *testing.T, workActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, workActions, "get", "update")
				updatedObj := workActions[1].(clienttesting.UpdateActionImpl).Object
				config := updatedObj.(*configv1alpha1.SubmarinerConfig)
				if len(config.Finalizers) != 0 {
					t.Errorf("unexpect size of finalizers")
				}
			},
		},
		{
			name:     "delete a submariner config",
			queueKey: "cluster1/submariner-config",
			configs: []runtime.Object{
				&configv1alpha1.SubmarinerConfig{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "submariner-config",
						Namespace:         "cluster1",
						DeletionTimestamp: &now,
						Finalizers:        []string{"submarineraddon.open-cluster-management.io/config-cleanup"},
					},
					Spec: configv1alpha1.SubmarinerConfigSpec{
						CredentialsSecret: &v1.LocalObjectReference{
							Name: "aws-credentials-secret",
						},
					},
				},
			},
			clusters: []runtime.Object{
				&clusterv1.ManagedCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name:       "cluster1",
						Labels:     map[string]string{"vendor": "unspported"},
						Finalizers: []string{"cluster.open-cluster-management.io/submariner-config-cleanup"},
					},
				},
			},
			validateActions: func(t *testing.T, workActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, workActions, "get", "update")
				workUpdatedObj := workActions[1].(clienttesting.UpdateActionImpl).Object
				config := workUpdatedObj.(*configv1alpha1.SubmarinerConfig)
				if len(config.Finalizers) != 0 {
					t.Errorf("unexpect size of finalizers")
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clusterClient := fakeclusterclient.NewSimpleClientset(c.clusters...)
			configClient := fakeconfigclient.NewSimpleClientset(c.configs...)
			clusterInformerFactory := clusterinformers.NewSharedInformerFactory(clusterClient, 5*time.Minute)
			for _, cluster := range c.clusters {
				clusterInformerFactory.Cluster().V1().ManagedClusters().Informer().GetStore().Add(cluster)
			}
			ctrl := &submarinerAgentController{
				kubeClient:         kubefake.NewSimpleClientset(),
				clusterClient:      clusterClient,
				clusterLister:      clusterInformerFactory.Cluster().V1().ManagedClusters().Lister(),
				clusterSetLister:   clusterInformerFactory.Cluster().V1alpha1().ManagedClusterSets().Lister(),
				manifestWorkClient: fakeworkclient.NewSimpleClientset(),
				configClient:       configClient,
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

func newManagedCluster(name string, labels map[string]string, finalizers []string, terminating bool) *clusterv1.ManagedCluster {
	cluster := &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
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

func newManagedClusterSet(name string) *clusterv1alpha1.ManagedClusterSet {
	clusterSet := &clusterv1alpha1.ManagedClusterSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
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
