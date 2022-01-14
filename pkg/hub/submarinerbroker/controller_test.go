package submarinerbroker

import (
	"context"
	"testing"
	"time"

	clusterfake "github.com/open-cluster-management/api/client/cluster/clientset/versioned/fake"
	clusterinformers "github.com/open-cluster-management/api/client/cluster/informers/externalversions"
	clusterv1alpha1 "github.com/open-cluster-management/api/cluster/v1alpha1"
	testinghelpers "github.com/stolostron/submariner-addon/pkg/helpers/testing"

	"github.com/openshift/library-go/pkg/operator/events/eventstesting"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestSync(t *testing.T) {
	cases := []struct {
		name            string
		clusterSetName  string
		clustersets     []runtime.Object
		validateActions func(t *testing.T, kubeActions, clusterSetActions []clienttesting.Action)
	}{
		{
			name:           "No clusterset",
			clusterSetName: "key",
			clustersets:    []runtime.Object{},
			validateActions: func(t *testing.T, kubeActions, clusterSetActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, kubeActions)
				testinghelpers.AssertNoActions(t, clusterSetActions)
			},
		},
		{
			name:           "Create a clusterset",
			clusterSetName: "set1",
			clustersets:    []runtime.Object{newManagedClusterSet("set1", []string{}, false)},
			validateActions: func(t *testing.T, kubeActions, clusterSetActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, kubeActions)
				testinghelpers.AssertActions(t, clusterSetActions, "update")
				managedClusterSet := clusterSetActions[0].(clienttesting.UpdateActionImpl).Object
				testinghelpers.AssertFinalizers(t, managedClusterSet, []string{brokerFinalizer})
			},
		},
		{
			name:           "Sync an existed clusterset",
			clusterSetName: "set1",
			clustersets:    []runtime.Object{newManagedClusterSet("set1", []string{brokerFinalizer}, false)},
			validateActions: func(t *testing.T, kubeActions, clusterSetActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, clusterSetActions)
				testinghelpers.AssertActions(t, kubeActions, "get", "create", "get", "create", "get", "create")
				namespace := (kubeActions[1].(clienttesting.CreateActionImpl).Object).(*corev1.Namespace)
				if namespace.Name != "set1-broker" {
					t.Errorf("expected set1-broker, but got %v", namespace)
				}
				testinghelpers.AssertActionResource(t, kubeActions[3], "roles")
				testinghelpers.AssertActionResource(t, kubeActions[5], "secrets")
			},
		},
		{
			name:           "Delete a clusterset",
			clusterSetName: "set1",
			clustersets:    []runtime.Object{newManagedClusterSet("set1", []string{"test", brokerFinalizer}, true)},
			validateActions: func(t *testing.T, kubeActions, clusterSetActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, kubeActions, "delete", "delete")
				testinghelpers.AssertActionResource(t, kubeActions[0], "namespaces")
				testinghelpers.AssertActionResource(t, kubeActions[1], "roles")
				testinghelpers.AssertActions(t, clusterSetActions, "update")
				managedClusterSet := clusterSetActions[0].(clienttesting.UpdateActionImpl).Object
				testinghelpers.AssertFinalizers(t, managedClusterSet, []string{"test"})
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeKubeClient := kubefake.NewSimpleClientset()

			fakeClusterClient := clusterfake.NewSimpleClientset(c.clustersets...)
			informerFactory := clusterinformers.NewSharedInformerFactory(fakeClusterClient, 5*time.Minute)
			for _, clusterset := range c.clustersets {
				informerFactory.Cluster().V1alpha1().ManagedClusterSets().Informer().GetStore().Add(clusterset)
			}

			ctrl := &submarinerBrokerController{
				kubeClient:       fakeKubeClient,
				clustersetClient: fakeClusterClient.ClusterV1alpha1().ManagedClusterSets(),
				clusterSetLister: informerFactory.Cluster().V1alpha1().ManagedClusterSets().Lister(),
				eventRecorder:    eventstesting.NewTestingEventRecorder(t),
			}

			err := ctrl.sync(context.TODO(), testinghelpers.NewFakeSyncContext(t, c.clusterSetName))
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}

			c.validateActions(t, fakeKubeClient.Actions(), fakeClusterClient.Actions())
		})
	}
}

func newManagedClusterSet(name string, finalizers []string, terminating bool) *clusterv1alpha1.ManagedClusterSet {
	clusterSet := &clusterv1alpha1.ManagedClusterSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:       name,
			Finalizers: finalizers,
		},
	}
	if terminating {
		now := metav1.Now()
		clusterSet.DeletionTimestamp = &now
	}

	return clusterSet
}
