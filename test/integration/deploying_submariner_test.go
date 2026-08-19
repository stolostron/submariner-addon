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
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	"open-cluster-management.io/api/client/addon/clientset/versioned/scheme"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var _ = Describe("Submariner Deployment", func() {
	var (
		managedClusterSetName string
		managedClusterName    string
		brokerNamespace       string
	)

	BeforeEach(func() {
		managedClusterName = fmt.Sprintf("cluster-%s", rand.String(6))

		DeferCleanup(startControllerManager())

		managedClusterSetName, brokerNamespace = deployManagedClusterSet()
	})

	When("the submariner ManagedClusterAddOn is deployed on a ManagedCluster", func() {
		JustBeforeEach(func() {
			deployManagedClusterWithAddOn(managedClusterSetName, managedClusterName, brokerNamespace)
		})

		It("should successfully deploy the submariner ManifestWork resources", func() {
			works := awaitSubmarinerManifestWorks(managedClusterName)

			var submarinerCRWork *workv1.ManifestWork

			for _, w := range works {
				if w.Name == submarineragent.SubmarinerCRManifestWorkName {
					submarinerCRWork = w
					break
				}
			}

			Expect(submarinerCRWork).NotTo(BeNil(), "SubmarinerCR ManifestWork not found")

			Expect(submarinerCRWork.Spec.Workload.Manifests).To(HaveLen(3),
				"Expected 3 manifests (PSK Secret, Broker Secret, and Submariner CR)")

			manifestObjs := util.UnmarshallManifestObjs(submarinerCRWork)

			// Verify PSK Secret manifest
			pskSecret := &corev1.Secret{}
			Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(
				util.AssertManifestObj(manifestObjs, "Secret", constants.IPSecPSKSecretName).Object, pskSecret)).To(Succeed())
			Expect(pskSecret.Name).To(Equal(constants.IPSecPSKSecretName))
			Expect(pskSecret.Data["psk"]).NotTo(BeEmpty(), "PSK data should not be empty")

			// Verify Broker Secret manifest
			brokerSecret := &corev1.Secret{}
			Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(
				util.AssertManifestObj(manifestObjs, "Secret", constants.BrokerK8sSecretName).Object, brokerSecret)).To(Succeed())
			Expect(brokerSecret.Name).To(Equal(constants.BrokerK8sSecretName))
			Expect(brokerSecret.Data["token"]).NotTo(BeEmpty(), "Broker token should not be empty")
			Expect(brokerSecret.Data["ca.crt"]).NotTo(BeEmpty(), "Broker CA should not be empty")

			// Verify Submariner CR manifest
			submariner := &v1alpha1.Submariner{}
			Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(
				util.AssertManifestObj(manifestObjs, "Submariner", "").Object, submariner)).To(Succeed())
			Expect(submariner.Spec.CeIPSecPSKSecret).To(Equal(constants.IPSecPSKSecretName))
			Expect(submariner.Spec.BrokerK8sSecret).To(Equal(constants.BrokerK8sSecretName))
			Expect(submariner.Spec.BrokerK8sApiServerToken).To(BeEmpty())
			Expect(submariner.Spec.BrokerK8sCA).To(BeEmpty())
		})
	})

	Context("after submariner ManagedClusterAddOn deployment", func() {
		JustBeforeEach(func() {
			deployManagedClusterWithAddOn(managedClusterSetName, managedClusterName, brokerNamespace)
			awaitSubmarinerManifestWorks(managedClusterName)
		})

		When("the ManagedClusterAddOn is deleted", func() {
			It("should delete the submariner ManifestWork resources", func() {
				By(fmt.Sprintf("Add a finalizer to the %q ManifestWork", submarineragent.SubmarinerCRManifestWorkName))

				work, err := workClient.WorkV1().ManifestWorks(managedClusterName).Get(context.Background(),
					submarineragent.SubmarinerCRManifestWorkName, metav1.GetOptions{})
				Expect(err).To(Succeed())

				_, err = finalizer.Add(context.Background(), resource.ForManifestWork(workClient.WorkV1().ManifestWorks(managedClusterName)),
					work, "test-finalizer")
				Expect(err).To(Succeed())

				By("Delete the submariner ManagedClusterAddOn")

				Expect(addOnClient.AddonV1alpha1().ManagedClusterAddOns(managedClusterName).Delete(context.Background(),
					constants.SubmarinerAddOnName, metav1.DeleteOptions{})).NotTo(HaveOccurred())

				ensureSubmarinerManifestWorks(managedClusterName)

				By(fmt.Sprintf("Remove the finalizer from the %q ManifestWork", submarineragent.SubmarinerCRManifestWorkName))

				work, err = workClient.WorkV1().ManifestWorks(managedClusterName).Get(context.Background(),
					submarineragent.SubmarinerCRManifestWorkName, metav1.GetOptions{})
				Expect(err).To(Succeed())

				err = finalizer.Remove(context.Background(), resource.ForManifestWork(workClient.WorkV1().ManifestWorks(managedClusterName)),
					work, "test-finalizer")
				Expect(err).To(Succeed())

				awaitNoSubmarinerManifestWorks(managedClusterName)

				By("Await deletion of the submariner ManagedClusterAddOn")

				Eventually(func() bool {
					_, err := addOnClient.AddonV1alpha1().ManagedClusterAddOns(managedClusterName).Get(context.Background(),
						constants.SubmarinerAddOnName, metav1.GetOptions{})
					return apierrors.IsNotFound(err)
				}, eventuallyTimeout, eventuallyInterval).Should(BeTrue(), "ManagedClusterAddon was not deleted")
			})
		})

		When("the cluster set label is removed from the ManagedCluster", func() {
			It("should delete the submariner ManifestWork resources", func() {
				By("Remove the cluster set label from the ManagedCluster")

				Expect(util.UpdateManagedClusterLabels(clusterClient, managedClusterName, map[string]string{})).NotTo(HaveOccurred())

				awaitNoSubmarinerManifestWorks(managedClusterName)
			})
		})
	})

	When("the ManagedClusterSet is deleted", func() {
		It("should remove the broker resources", func() {
			By("Delete the ManagedClusterSet")

			Expect(clusterClient.ClusterV1beta2().ManagedClusterSets().Delete(context.Background(), managedClusterSetName,
				metav1.DeleteOptions{})).NotTo(HaveOccurred())

			By("Await removal of the broker resources")

			Eventually(func() bool {
				return util.CheckBrokerResources(kubeClient, brokerNamespace, false)
			}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())

			By("Await deletion of the ManagedClusterSet")

			Eventually(func() bool {
				_, err := clusterClient.ClusterV1beta2().ManagedClusterSets().Get(context.Background(), managedClusterSetName,
					metav1.GetOptions{})
				return apierrors.IsNotFound(err)
			}, eventuallyTimeout, eventuallyInterval).Should(BeTrue(), "ManagedClusterSet %q was not deleted", managedClusterSetName)
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

			By("Create a managed cluster namespace")

			_, err = kubeClient.CoreV1().Namespaces().Create(context.Background(), util.NewNamespace(managedClusterName),
				metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By(fmt.Sprintf("Create ManagedClusterAddOn for cluster %q", managedClusterName))

			addOn := util.NewManagedClusterAddOn(managedClusterName)
			Expect(controllerutil.SetControllerReference(clusterAddOn, addOn, scheme.Scheme)).NotTo(HaveOccurred())

			_, err = addOnClient.AddonV1alpha1().ManagedClusterAddOns(managedClusterName).Create(context.Background(), addOn,
				metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("should remove the broker resources for the associated ManagedClusterSets", func() {
			By("Delete the ClusterManagementAddOn")

			ctx, cancel := context.WithTimeout(context.Background(), time.Second*eventuallyTimeout)
			err := addOnClient.AddonV1alpha1().ClusterManagementAddOns().Delete(ctx, constants.SubmarinerAddOnName, metav1.DeleteOptions{})

			cancel()
			Expect(err).NotTo(HaveOccurred())

			By("Ensure the ClusterManagementAddOn is deleted")

			Eventually(func() bool {
				_, err := addOnClient.AddonV1alpha1().ClusterManagementAddOns().Get(context.Background(),
					constants.SubmarinerAddOnName, metav1.GetOptions{})
				return apierrors.IsNotFound(err)
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

func awaitSubmarinerManifestWorks(managedClusterName string) []*workv1.ManifestWork {
	By("Await deployment of the ManifestWorks")

	return awaitManifestWorks(workClient, managedClusterName, submarineragent.OperatorManifestWorkName,
		submarineragent.SubmarinerCRManifestWorkName)
}

func ensureSubmarinerManifestWorks(managedClusterName string) {
	By("Ensure deployment of the ManifestWorks")
	awaitManifestWorks(workClient, managedClusterName, submarineragent.OperatorManifestWorkName,
		submarineragent.SubmarinerCRManifestWorkName)
}

func awaitNoSubmarinerManifestWorks(managedClusterName string) {
	By("Await deletion of the ManifestWorks")
	awaitNoManifestWorks(workClient, managedClusterName, submarineragent.OperatorManifestWorkName,
		submarineragent.SubmarinerCRManifestWorkName)
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

	err := controllerClient.Create(context.Background(), brokerCfg)
	Expect(err).NotTo(HaveOccurred())
}
