package e2e

import (
	"context"
	"fmt"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/submariner-addon/test/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
)

var _ = Describe("[P1][sev1][server-foundation]Deploy a submariner on hub:", func() {
	var managedClusterSetName string
	var expectedBrokerNamespace string

	BeforeEach(func() {
		managedClusterSetName = fmt.Sprintf("e2e-set-%s", rand.String(6))
		By("Create a ManagedClusterSet")
		managedClusterSet := utils.NewManagedClusterSet(managedClusterSetName)
		_, err := clusterClient.ClusterV1alpha1().ManagedClusterSets().Create(context.Background(), managedClusterSet, metav1.CreateOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Check if the submariner broker is deployed")
		expectedBrokerNamespace = fmt.Sprintf("submariner-clusterset-%s-broker", managedClusterSetName)
		Eventually(func() bool {
			return utils.FindSubmarinerBrokerResources(kubeClient, expectedBrokerNamespace)
		}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())
	})
	AfterEach(func() {
		By("Delete the ManagedClusterSet")
		err := clusterClient.ClusterV1alpha1().ManagedClusterSets().Delete(context.Background(), managedClusterSetName, metav1.DeleteOptions{})
		Expect(err).NotTo(HaveOccurred())

		By("Check if the submariner broker is removed")
		Eventually(func() bool {
			ns, err := kubeClient.CoreV1().Namespaces().Get(context.Background(), expectedBrokerNamespace, metav1.GetOptions{})
			if errors.IsNotFound(err) {
				return true
			}
			if err != nil {
				return false
			}

			// the controller-runtime does not have a gc controller, so if the namespace is in terminating and
			// there is no broker finalizer on it, it will be consider as removed
			if ns.Status.Phase == corev1.NamespaceTerminating &&
				!utils.FindExpectedFinalizer(ns.Finalizers, "cluster.open-cluster-management.io/submariner-cleanup") {
				return true
			}
			return false
		}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())
	})

	It("Should deploy the submariner agent manifestworks on managed cluster namespace successfully", func() {
		By("Join a ManagedCluster to ManagedClusterSet")
		addLabels := map[string]string{
			"cluster.open-cluster-management.io/submariner-agent": "true",
			"cluster.open-cluster-management.io/clusterset":       managedClusterSetName,
		}

		err := utils.AddManagedClusterLabels(clusterClient, managedClusterName, addLabels)
		Expect(err).NotTo(HaveOccurred())

		By("Check if the submariner agent manifestworks are deployed")
		Eventually(func() error {
			return utils.CheckManifestWorks(workClient, clusterClient, managedClusterName)
		}, eventuallyTimeout*10, eventuallyInterval*5).Should(BeNil())

		By("Remove the cluster from clusterset")
		removeLabels := map[string]string{
			"cluster.open-cluster-management.io/submariner-agent": "true",
			"cluster.open-cluster-management.io/clusterset":       managedClusterSetName,
		}
		err = utils.RemoveManagedClusterLabels(clusterClient, managedClusterName, removeLabels)
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() bool {
			works, err := workClient.WorkV1().ManifestWorks(managedClusterName).List(context.Background(), metav1.ListOptions{})
			if err != nil {
				return false
			}
			return len(works.Items) == 0
		}, eventuallyTimeout*10, eventuallyInterval*5).Should(BeTrue())
	})
})
