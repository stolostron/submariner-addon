package integration

import (
	"context"
	"fmt"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	"github.com/open-cluster-management/submariner-addon/pkg/spoke"
	"github.com/open-cluster-management/submariner-addon/test/util"

	"github.com/openshift/library-go/pkg/controller/controllercmd"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
)

const (
	expectedAgentWork           = "addon-submariner-deploy"
	submarinerOperatorNamespace = "submariner-operator"
	submarinerCRName            = "submariner"
)

var _ = ginkgo.Describe("Deploy a submariner-addon agent", func() {
	var managedClusterSetName string
	var managedClusterName string

	ginkgo.BeforeEach(func() {
		managedClusterSetName = fmt.Sprintf("set-%s", rand.String(6))
		managedClusterName = fmt.Sprintf("cluster-%s", rand.String(6))
	})

	ginkgo.Context("Deploy submariner-addon agent manifestworks on the hub", func() {
		ginkgo.BeforeEach(func() {
			ginkgo.By("Create a ManagedClusterSet")
			managedClusterSet := util.NewManagedClusterSet(managedClusterSetName)
			_, err := clusterClient.ClusterV1alpha1().ManagedClusterSets().Create(context.Background(), managedClusterSet, metav1.CreateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			brokerNamespace := fmt.Sprintf("%s-broker", managedClusterSetName)
			gomega.Eventually(func() bool {
				return util.FindSubmarinerBrokerResources(kubeClient, brokerNamespace)
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())

			ginkgo.By("Create a ManagedCluster")
			managedCluster := util.NewManagedCluster(managedClusterName, map[string]string{
				"cluster.open-cluster-management.io/clusterset": managedClusterSetName,
			})
			_, err = clusterClient.ClusterV1().ManagedClusters().Create(context.Background(), managedCluster, metav1.CreateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ginkgo.By("Setup the managed cluster namespace")
			_, err = kubeClient.CoreV1().Namespaces().Create(context.Background(), util.NewManagedClusterNamespace(managedClusterName), metav1.CreateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ginkgo.By("Create a submariner-addon")
			_, err = addOnClient.AddonV1alpha1().ManagedClusterAddOns(managedClusterName).Create(context.TODO(), util.NewManagedClusterAddOn(managedClusterName), metav1.CreateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ginkgo.By("Setup the serviceaccount")
			err = util.SetupServiceAccount(kubeClient, brokerNamespace, managedClusterName)
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("Should deploy the submariner-addon agent manifestworks on managed cluster namespace successfully", func() {
			gomega.Eventually(func() bool {
				return util.FindManifestWorks(workClient, managedClusterName, expectedOperatorWork, expectedAgentWork)
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
		})
	})

	ginkgo.Context("Run submariner-addon agent on the managed cluster", func() {
		var installationNamespace string
		var submarinerGVR, _ = schema.ParseResourceArg("submariners.v1alpha1.submariner.io")

		ginkgo.BeforeEach(func() {
			ginkgo.By("Setup the managed cluster namespace")
			_, err := kubeClient.CoreV1().Namespaces().Create(context.Background(), util.NewManagedClusterNamespace(managedClusterName), metav1.CreateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ginkgo.By("Create submariner-addon on managed cluster namespace")
			_, err = addOnClient.AddonV1alpha1().ManagedClusterAddOns(managedClusterName).Create(context.TODO(), util.NewManagedClusterAddOn(managedClusterName), metav1.CreateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ginkgo.By("Create submariner-addon installation namespace on managed cluster namespace")
			installationNamespace = fmt.Sprintf("submariner-addon-%s", managedClusterName)
			_, err = kubeClient.CoreV1().Namespaces().Create(context.Background(), util.NewManagedClusterNamespace(installationNamespace), metav1.CreateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ginkgo.By("Create submariner-operator namespace on managed cluster namespace")
			_, err = kubeClient.CoreV1().Namespaces().Create(context.Background(), util.NewManagedClusterNamespace(submarinerOperatorNamespace), metav1.CreateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			// save the kubeconfig to a file, simulate agent mount hub kubeconfig secret that was created by addon framework
			err = util.CreateHubKubeConfig(cfg)
			gomega.Expect(err).ToNot(gomega.HaveOccurred())
		})

		ginkgo.AfterEach(func() {
			err := dynamicClient.Resource(*submarinerGVR).Namespace(submarinerOperatorNamespace).Delete(context.TODO(), submarinerCRName, metav1.DeleteOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
		})

		ginkgo.It("Should update submariner-addon status on the hub cluster after the submariner agent is deployed", func() {
			ginkgo.By("Start submariner-addon agent on managed cluster namespace")
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
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			}()

			ginkgo.By("Create a submariner on managed cluster namespace")
			_, err := dynamicClient.Resource(*submarinerGVR).Namespace(submarinerOperatorNamespace).Create(context.TODO(), util.NewSubmariner(), metav1.CreateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			ginkgo.By("Update the submariner status on managed cluster namespace")
			submariner, err := dynamicClient.Resource(*submarinerGVR).Namespace(submarinerOperatorNamespace).Get(context.TODO(), submarinerCRName, metav1.GetOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())
			util.SetSubmarinerDeployedStatus(submariner)
			_, err = dynamicClient.Resource(*submarinerGVR).Namespace(submarinerOperatorNamespace).UpdateStatus(context.TODO(), submariner, metav1.UpdateOptions{})
			gomega.Expect(err).NotTo(gomega.HaveOccurred())

			gomega.Eventually(func() bool {
				addOn, err := addOnClient.AddonV1alpha1().ManagedClusterAddOns(managedClusterName).Get(context.TODO(), "submariner", metav1.GetOptions{})
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
				if !meta.IsStatusConditionFalse(addOn.Status.Conditions, "SubmarinerAgentDegraded") {
					return false
				}
				return true
			}, eventuallyTimeout, eventuallyInterval).Should(gomega.BeTrue())
		})
	})

})
