package integration_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/submariner-addon/pkg/hub/submarineragent"
	"github.com/open-cluster-management/submariner-addon/pkg/resource"
	"github.com/open-cluster-management/submariner-addon/test/util"
	"github.com/submariner-io/admiral/pkg/finalizer"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
)

var _ = Describe("Deploy a submariner on hub", func() {
	var managedClusterSetName string
	var managedClusterName string

	BeforeEach(func() {
		managedClusterSetName = fmt.Sprintf("set-%s", rand.String(6))
		managedClusterName = fmt.Sprintf("cluster-%s", rand.String(6))
	})

	Context("Deploy submariner agent manifestworks", func() {
		var expectedBrokerNamespace string

		BeforeEach(func() {
			By("Create a ManagedClusterSet")
			managedClusterSet := util.NewManagedClusterSet(managedClusterSetName)

			_, err := clusterClient.ClusterV1beta1().ManagedClusterSets().Create(context.Background(), managedClusterSet, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Check if the submariner broker is deployed")
			expectedBrokerNamespace = fmt.Sprintf("%s-broker", managedClusterSetName)
			Eventually(func() bool {
				return util.FindSubmarinerBrokerResources(kubeClient, expectedBrokerNamespace)
			}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())
		})

		It("Should deploy the submariner agent manifestworks on managed cluster namespace successfully", func() {
			By("Create a ManagedCluster")
			managedCluster := util.NewManagedCluster(managedClusterName, map[string]string{
				"cluster.open-cluster-management.io/clusterset": managedClusterSetName,
			})
			_, err := clusterClient.ClusterV1().ManagedClusters().Create(context.Background(), managedCluster, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Setup the managed cluster namespace")
			_, err = kubeClient.CoreV1().Namespaces().Create(context.Background(), util.NewManagedClusterNamespace(managedClusterName),
				metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Create a submariner-addon")
			_, err = addOnClient.AddonV1alpha1().ManagedClusterAddOns(managedClusterName).Create(context.TODO(),
				util.NewManagedClusterAddOn(managedClusterName), metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Setup the serviceaccount")
			err = util.SetupServiceAccount(kubeClient, expectedBrokerNamespace, managedClusterName)
			Expect(err).NotTo(HaveOccurred())

			awaitSubmarinerManifestMorks(managedClusterName)
		})
	})

	Context("Remove submariner agent manifestworks", func() {
		BeforeEach(func() {
			By("Create a ManagedClusterSet")
			managedClusterSet := util.NewManagedClusterSet(managedClusterSetName)

			_, err := clusterClient.ClusterV1beta1().ManagedClusterSets().Create(context.Background(), managedClusterSet, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			brokerNamespace := fmt.Sprintf("%s-broker", managedClusterSetName)
			Eventually(func() bool {
				return util.FindSubmarinerBrokerResources(kubeClient, brokerNamespace)
			}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())

			By("Create a ManagedCluster")
			managedCluster := util.NewManagedCluster(managedClusterName, map[string]string{
				"cluster.open-cluster-management.io/clusterset": managedClusterSetName,
			})
			_, err = clusterClient.ClusterV1().ManagedClusters().Create(context.Background(), managedCluster, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Setup the managed cluster namespace")
			_, err = kubeClient.CoreV1().Namespaces().Create(context.Background(), util.NewManagedClusterNamespace(managedClusterName),
				metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Create a submariner-addon")
			_, err = addOnClient.AddonV1alpha1().ManagedClusterAddOns(managedClusterName).Create(context.TODO(),
				util.NewManagedClusterAddOn(managedClusterName), metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Setup the serviceaccount")
			err = util.SetupServiceAccount(kubeClient, brokerNamespace, managedClusterName)
			Expect(err).NotTo(HaveOccurred())

			awaitSubmarinerManifestMorks(managedClusterName)
		})

		It("Should remove the submariner agent manifestworks after the submariner-addon is removed from the managed cluster", func() {
			By("Adding finalizer to the submariner-resource manifestwork")
			work, err := workClient.WorkV1().ManifestWorks(managedClusterName).Get(context.Background(),
				submarineragent.SubmarinerCRManifestWorkName, metav1.GetOptions{})
			Expect(err).To(Succeed())

			_, err = finalizer.Add(context.Background(), resource.ForManifestWork(workClient.WorkV1().ManifestWorks(managedClusterName)),
				work, "test-finalizer")
			Expect(err).To(Succeed())

			By("Remove the submariner-addon from the managed cluster")
			err = addOnClient.AddonV1alpha1().ManagedClusterAddOns(managedClusterName).Delete(context.TODO(), "submariner",
				metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			time.Sleep(time.Second * 3)

			By("Removing finalizer from the submariner-resource manifestwork")
			work, err = workClient.WorkV1().ManifestWorks(managedClusterName).Get(context.Background(),
				submarineragent.SubmarinerCRManifestWorkName, metav1.GetOptions{})
			Expect(err).To(Succeed())

			err = finalizer.Remove(context.Background(), resource.ForManifestWork(workClient.WorkV1().ManifestWorks(managedClusterName)),
				work, "test-finalizer")
			Expect(err).To(Succeed())

			awaitNoSubmarinerManifestMorks(managedClusterName)
		})

		It("Should remove the submariner agent manifestworks after the managedclusterset label is removed from the managed cluster", func() {
			By("Remove the managedclusterset label from the managed cluster")
			newLabels := map[string]string{}
			err := util.UpdateManagedClusterLabels(clusterClient, managedClusterName, newLabels)
			Expect(err).NotTo(HaveOccurred())

			awaitNoSubmarinerManifestMorks(managedClusterName)
		})

		It("Should remove the submariner agent manifestworks after the managedcluster is removed", func() {
			By("Remove the managedcluster")
			err := clusterClient.ClusterV1().ManagedClusters().Delete(context.Background(), managedClusterName, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			awaitNoSubmarinerManifestMorks(managedClusterName)
		})
	})

	Context("Remove submariner broker", func() {
		It("Should remove the submariner broker after the managedclusterset is removed", func() {
			By("Create a ManagedClusterSet")
			managedClusterSet := util.NewManagedClusterSet(managedClusterSetName)

			_, err := clusterClient.ClusterV1beta1().ManagedClusterSets().Create(context.Background(), managedClusterSet, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Check if the submariner broker is deployed")
			brokerNamespace := fmt.Sprintf("%s-broker", managedClusterSetName)
			Eventually(func() bool {
				return util.FindSubmarinerBrokerResources(kubeClient, brokerNamespace)
			}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())

			By("Remove the managedclusterset")
			err = clusterClient.ClusterV1beta1().ManagedClusterSets().Delete(context.Background(), managedClusterSetName, metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Check if the submariner broker is removed")
			Eventually(func() bool {
				ns, err := kubeClient.CoreV1().Namespaces().Get(context.Background(), brokerNamespace, metav1.GetOptions{})
				if errors.IsNotFound(err) {
					return true
				}
				if err != nil {
					return false
				}

				// the controller-runtime does not have a gc controller, so if the namespace is in terminating and
				// there is no broker finalizer on it, it will be consider as removed
				if ns.Status.Phase == corev1.NamespaceTerminating &&
					!util.FindExpectedFinalizer(ns.Finalizers, "cluster.open-cluster-management.io/submariner-cleanup") {
					return true
				}

				return false
			}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())
		})
	})

	Context("Create a SubmarinerConfig", func() {
		It("Should add finalizer to created SubmarinerConfig", func() {
			By("Setup the managed cluster namespace")
			_, err := kubeClient.CoreV1().Namespaces().Create(context.Background(), util.NewManagedClusterNamespace(managedClusterName),
				metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Create SubmarinerConfig")
			_, err = configClinet.SubmarineraddonV1alpha1().SubmarinerConfigs(managedClusterName).Create(context.Background(),
				util.NewSubmarinerConifg(managedClusterName), metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Check SubmarinerConfig finalizer")
			Eventually(func() bool {
				config, err := configClinet.SubmarineraddonV1alpha1().SubmarinerConfigs(managedClusterName).Get(context.Background(), "submariner",
					metav1.GetOptions{})
				if errors.IsNotFound(err) {
					return false
				}
				if err != nil {
					return false
				}

				if len(config.Finalizers) != 1 {
					return false
				}

				if config.Finalizers[0] != "submarineraddon.open-cluster-management.io/config-cleanup" {
					return false
				}

				return true
			}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())
		})
	})

	Context("Delete a SubmarinerConfig", func() {
		It("Should delete the created SubmarinerConfig", func() {
			By("Setup the managed cluster namespace")
			_, err := kubeClient.CoreV1().Namespaces().Create(context.Background(), util.NewManagedClusterNamespace(managedClusterName),
				metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Create SubmarinerConfig")
			_, err = configClinet.SubmarineraddonV1alpha1().SubmarinerConfigs(managedClusterName).Create(context.Background(),
				util.NewSubmarinerConifg(managedClusterName), metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Delete the created SubmarinerConfig")
			err = configClinet.SubmarineraddonV1alpha1().SubmarinerConfigs(managedClusterName).Delete(context.Background(), "submariner",
				metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Check if the SubmarinerConfig is deleted")
			Eventually(func() bool {
				_, err := configClinet.SubmarineraddonV1alpha1().SubmarinerConfigs(managedClusterName).Get(context.Background(), "submariner",
					metav1.GetOptions{})

				return errors.IsNotFound(err)
			}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())
		})
	})
})

func awaitSubmarinerManifestMorks(managedClusterName string) {
	By("Check if the submariner agent manifestworks are deployed")
	Eventually(func() bool {
		return util.FindManifestWorks(workClient, managedClusterName, submarineragent.OperatorManifestWorkName,
			submarineragent.SubmarinerCRManifestWorkName)
	}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())
}

func awaitNoSubmarinerManifestMorks(managedClusterName string) {
	By("Check if the submariner agent manifestworks are removed")
	Eventually(func() bool {
		return util.FindManifestWorks(workClient, managedClusterName, submarineragent.OperatorManifestWorkName,
			submarineragent.SubmarinerCRManifestWorkName)
	}, eventuallyTimeout, eventuallyInterval).Should(BeFalse())
}
