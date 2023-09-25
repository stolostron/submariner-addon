package integration_test

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stolostron/submariner-addon/pkg/constants"
	"github.com/stolostron/submariner-addon/pkg/hub/submarineragent"
	"github.com/stolostron/submariner-addon/pkg/resource"
	"github.com/stolostron/submariner-addon/test/util"
	"github.com/submariner-io/admiral/pkg/finalizer"
	"github.com/submariner-io/submariner-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/utils/ptr"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"open-cluster-management.io/api/client/addon/clientset/versioned/scheme"
	clusterv1beta2 "open-cluster-management.io/api/cluster/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
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

			_, err := clusterClient.ClusterV1beta2().ManagedClusterSets().Create(context.Background(), managedClusterSet, metav1.CreateOptions{})
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
				clusterv1beta2.ClusterSetLabel: managedClusterSetName,
			})
			_, err := clusterClient.ClusterV1().ManagedClusters().Create(context.Background(), managedCluster, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Setup the managed cluster namespace")
			_, err = kubeClient.CoreV1().Namespaces().Create(context.Background(), util.NewManagedClusterNamespace(managedClusterName),
				metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Create the brokers.submariner.io config in the broker namespace")
			createBrokerConfiguration(expectedBrokerNamespace)

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

			_, err := clusterClient.ClusterV1beta2().ManagedClusterSets().Create(context.Background(), managedClusterSet, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			brokerNamespace := fmt.Sprintf("%s-broker", managedClusterSetName)
			Eventually(func() bool {
				return util.FindSubmarinerBrokerResources(kubeClient, brokerNamespace)
			}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())

			By("Create a ManagedCluster")
			managedCluster := util.NewManagedCluster(managedClusterName, map[string]string{
				clusterv1beta2.ClusterSetLabel: managedClusterSetName,
			})
			_, err = clusterClient.ClusterV1().ManagedClusters().Create(context.Background(), managedCluster, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Setup the managed cluster namespace")
			_, err = kubeClient.CoreV1().Namespaces().Create(context.Background(), util.NewManagedClusterNamespace(managedClusterName),
				metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Create the brokers.submariner.io config in the broker namespace")
			createBrokerConfiguration(brokerNamespace)

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

			_, err := clusterClient.ClusterV1beta2().ManagedClusterSets().Create(context.Background(), managedClusterSet, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Check if the submariner broker is deployed")
			brokerNamespace := fmt.Sprintf("%s-broker", managedClusterSetName)
			Eventually(func() bool {
				return util.FindSubmarinerBrokerResources(kubeClient, brokerNamespace)
			}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())

			By("Remove the managedclusterset")
			err = clusterClient.ClusterV1beta2().ManagedClusterSets().Delete(context.Background(), managedClusterSetName, metav1.DeleteOptions{})
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

	When("the ClusterManagementAddOn is deleted", func() {
		var clusterAddOn *addonv1alpha1.ClusterManagementAddOn

		BeforeEach(func() {
			By("Retrieve the ClusterManagementAddOn")

			var err error

			Eventually(func() error {
				clusterAddOn, err = addOnClient.AddonV1alpha1().ClusterManagementAddOns().Get(context.Background(),
					constants.SubmarinerAddOnName, metav1.GetOptions{})
				return err
			}, eventuallyTimeout, eventuallyInterval).ShouldNot(HaveOccurred())

			By(fmt.Sprintf("Create ManagedClusterSet %q", managedClusterSetName))

			_, err = clusterClient.ClusterV1beta2().ManagedClusterSets().Create(context.Background(),
				util.NewManagedClusterSet(managedClusterSetName), metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Check if the submariner broker is deployed")

			brokerNamespace := fmt.Sprintf("%s-broker", managedClusterSetName)
			Eventually(func() bool {
				return util.FindSubmarinerBrokerResources(kubeClient, brokerNamespace)
			}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())

			By("Create a managed cluster namespace")

			_, err = kubeClient.CoreV1().Namespaces().Create(context.Background(), util.NewManagedClusterNamespace(managedClusterName),
				metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By(fmt.Sprintf("Create ManagedClusterAddOn for cluster %q", managedClusterName))

			addOn := util.NewManagedClusterAddOn(managedClusterName)
			Expect(controllerutil.SetControllerReference(clusterAddOn, addOn, scheme.Scheme)).NotTo(HaveOccurred())

			_, err = addOnClient.AddonV1alpha1().ManagedClusterAddOns(managedClusterName).Create(context.Background(), addOn,
				metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		AfterEach(func() {
			if clusterAddOn != nil {
				By("Re-create the ClusterManagementAddOn")

				clusterAddOn.ResourceVersion = ""

				_, err := addOnClient.AddonV1alpha1().ClusterManagementAddOns().Create(context.Background(), clusterAddOn,
					metav1.CreateOptions{})
				if !errors.IsAlreadyExists(err) {
					Expect(err).NotTo(HaveOccurred())
				}
			}
		})

		It("should remove the broker resources for the associated ManagedClusterSets", func() {
			By("Delete the ClusterManagementAddOn")

			var propagationPolicy *metav1.DeletionPropagation

			// The default control plane set up by the K8s test environment does not run the garbage collector so foreground deletion
			// doesn't clean up owned references, so we need to do it manually. However, if using a real cluster then we can use
			// foreground deletion.
			if ptr.Deref(testEnv.UseExistingCluster, false) {
				propagationPolicy = ptr.To(metav1.DeletePropagationForeground)
			}

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*eventuallyTimeout)
			err := addOnClient.AddonV1alpha1().ClusterManagementAddOns().Delete(ctx, constants.SubmarinerAddOnName, metav1.DeleteOptions{
				PropagationPolicy: propagationPolicy,
			})

			cancel()
			Expect(err).NotTo(HaveOccurred())

			if !ptr.Deref(testEnv.UseExistingCluster, false) {
				By("Delete all ManagedClusterAddOns")

				list, err := addOnClient.AddonV1alpha1().ManagedClusterAddOns(metav1.NamespaceAll).List(context.Background(),
					metav1.ListOptions{})
				Expect(err).NotTo(HaveOccurred())

				for i := range list.Items {
					addOnInterface := addOnClient.AddonV1alpha1().ManagedClusterAddOns(list.Items[i].Namespace)
					Expect(finalizer.Remove(context.Background(), resource.ForAddon(addOnInterface), &list.Items[i],
						constants.SubmarinerAddOnFinalizer)).NotTo(HaveOccurred())

					Expect(addOnInterface.Delete(context.Background(), constants.SubmarinerAddOnName,
						metav1.DeleteOptions{})).NotTo(HaveOccurred())
				}
			}

			By("Ensure the ClusterManagementAddOn is deleted")

			Eventually(func() bool {
				_, err := addOnClient.AddonV1alpha1().ClusterManagementAddOns().Get(context.Background(),
					constants.SubmarinerAddOnName, metav1.GetOptions{})
				return errors.IsNotFound(err)
			}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())

			By("Ensure the ManagedClusterSet finalizer is removed")

			Eventually(func() bool {
				mcs, err := clusterClient.ClusterV1beta2().ManagedClusterSets().Get(context.Background(), managedClusterSetName,
					metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				return len(mcs.Finalizers) == 0
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

func createBrokerConfiguration(brokerNamespace string) {
	brokerCfg := &v1alpha1.Broker{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "submariner-broker",
			Namespace: brokerNamespace,
		},
		Spec: v1alpha1.BrokerSpec{
			GlobalnetEnabled: false,
		},
	}

	err := controllerClient.Create(context.TODO(), brokerCfg)
	Expect(err).NotTo(HaveOccurred())
}
