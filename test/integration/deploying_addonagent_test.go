package integration

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/submariner-addon/pkg/spoke"
	"github.com/open-cluster-management/submariner-addon/test/util"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
)

const (
	expectedAgentWork = "addon-submariner-deploy"
	submarinerCRName  = "submariner"
)

var _ = Describe("Deploy a submariner-addon agent", func() {
	var managedClusterSetName string
	var managedClusterName string

	BeforeEach(func() {
		managedClusterSetName = fmt.Sprintf("set-%s", rand.String(6))
		managedClusterName = fmt.Sprintf("cluster-%s", rand.String(6))
	})

	Context("Deploy submariner-addon agent manifestworks on the hub", func() {
		BeforeEach(func() {
			By("Create a ManagedClusterSet")
			managedClusterSet := util.NewManagedClusterSet(managedClusterSetName)
			_, err := clusterClient.ClusterV1alpha1().ManagedClusterSets().Create(context.Background(), managedClusterSet, metav1.CreateOptions{})
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
			_, err = kubeClient.CoreV1().Namespaces().Create(context.Background(), util.NewManagedClusterNamespace(managedClusterName), metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Create a submariner-addon")
			_, err = addOnClient.AddonV1alpha1().ManagedClusterAddOns(managedClusterName).Create(context.TODO(), util.NewManagedClusterAddOn(managedClusterName), metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Setup the serviceaccount")
			err = util.SetupServiceAccount(kubeClient, brokerNamespace, managedClusterName)
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should deploy the submariner-addon agent manifestworks on managed cluster namespace successfully", func() {
			Eventually(func() bool {
				return util.FindManifestWorks(workClient, managedClusterName, expectedOperatorWork, expectedAgentWork)
			}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())
		})
	})

	Context("Run submariner-addon agent on the managed cluster", func() {
		var submarinerGVR, _ = schema.ParseResourceArg("submariners.v1alpha1.submariner.io")

		BeforeEach(func() {
			By("Setup the managed cluster namespace")
			_, err := kubeClient.CoreV1().Namespaces().Create(context.Background(), util.NewManagedClusterNamespace(managedClusterName), metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Create submariner-addon on managed cluster namespace")
			_, err = addOnClient.AddonV1alpha1().ManagedClusterAddOns(managedClusterName).Create(context.TODO(), util.NewManagedClusterAddOn(managedClusterName), metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should update submariner-addon status on the hub cluster after the submariner agent is deployed", func() {
			installationNamespace, err := util.GetCurrentNamespace(kubeClient, "submariner-operator")
			Expect(err).ToNot(HaveOccurred())

			// save the kubeconfig to a file, simulate agent mount hub kubeconfig secret that was created by addon framework
			Expect(util.CreateHubKubeConfig(cfg)).ToNot(HaveOccurred())

			By(fmt.Sprintf("Start submariner-addon agent on managed cluster namespace %q", installationNamespace))
			go func() {
				agentOptions := spoke.AgentOptions{
					InstallationNamespace: installationNamespace,
					HubKubeconfigFile:     util.HubKubeConfigPath,
					ClusterName:           managedClusterName,
				}
				err := agentOptions.RunAgent(context.Background(), &controllercmd.ControllerContext{
					KubeConfig:    cfg,
					EventRecorder: util.NewIntegrationTestEventRecorder("submariner-addon-agent-test"),
				})
				Expect(err).NotTo(HaveOccurred())
			}()

			By("Create a submariner on managed cluster")
			_, err = dynamicClient.Resource(*submarinerGVR).Namespace(installationNamespace).Create(context.TODO(), util.NewSubmariner(), metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Update the submariner status on managed cluster namespace")
			submariner, err := dynamicClient.Resource(*submarinerGVR).Namespace(installationNamespace).Get(context.TODO(), submarinerCRName, metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			util.SetSubmarinerDeployedStatus(submariner)
			_, err = dynamicClient.Resource(*submarinerGVR).Namespace(installationNamespace).UpdateStatus(context.TODO(), submariner, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				addOn, err := addOnClient.AddonV1alpha1().ManagedClusterAddOns(managedClusterName).Get(context.TODO(), "submariner", metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())
				if meta.IsStatusConditionTrue(addOn.Status.Conditions, "SubmarinerConnectionDegraded") {
					return false
				}
				return true
			}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())
		})
	})

})
