package integration_test

import (
	"context"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/stolostron/submariner-addon/pkg/constants"
	"github.com/stolostron/submariner-addon/pkg/spoke"
	"github.com/stolostron/submariner-addon/test/util"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/rand"
)

const (
	expectedAgentWork = "addon-submariner-deploy-0"
	submarinerCRName  = "submariner"
)

var _ = Describe("Submariner addon agent", func() {
	var managedClusterName string

	BeforeEach(func() {
		managedClusterName = fmt.Sprintf("cluster-%s", rand.String(6))
	})

	When("the submariner addon agent is deployed", func() {
		BeforeEach(func() {
			DeferCleanup(startControllerManager())

			managedClusterSetName, brokerNamespace := deployManagedClusterSet()
			deployManagedClusterWithAddOn(managedClusterSetName, managedClusterName, brokerNamespace)
		})

		It("should deploy the addon agent ManifestWork resources", func() {
			Eventually(func() bool {
				return util.CheckManifestWorks(workClient, managedClusterName, true, expectedAgentWork)
			}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())
		})
	})

	When("the submariner spoke agent is run on a managed cluster", func() {
		submarinerGVR, _ := schema.ParseResourceArg("submariners.v1alpha1.submariner.io")

		var installationNamespace string

		BeforeEach(func() {
			By("Create the ManagedCluster's namespace")

			_, err := kubeClient.CoreV1().Namespaces().Create(context.Background(), util.NewNamespace(managedClusterName),
				metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Create a submariner ManagedClusterAddOn")

			_, err = addOnClient.AddonV1alpha1().ManagedClusterAddOns(managedClusterName).Create(context.Background(),
				util.NewManagedClusterAddOn(managedClusterName), metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			installationNamespace, err = util.GetCurrentNamespace(kubeClient, "submariner-operator")
			Expect(err).ToNot(HaveOccurred())

			By(fmt.Sprintf("Start submariner spoke agent on managed cluster namespace %q", installationNamespace))

			ctx, cancel := context.WithCancel(context.Background())

			go func() {
				defer GinkgoRecover()

				agentOptions := spoke.AgentOptions{
					InstallationNamespace: installationNamespace,
					HubRestConfig:         cfg,
					ClusterName:           managedClusterName,
				}

				err := agentOptions.RunAgent(ctx, &controllercmd.ControllerContext{
					KubeConfig:    cfg,
					EventRecorder: util.NewIntegrationTestEventRecorder("submariner-addon-agent-test"),
				})
				Expect(err).NotTo(HaveOccurred())
			}()

			DeferCleanup(cancel)
		})

		It("should update the ManagedClusterAddOn status after the Submariner resource status is updated", func() {
			By("Create Submariner resource on managed cluster")

			submariner, err := dynamicClient.Resource(*submarinerGVR).Namespace(installationNamespace).Create(context.Background(),
				util.NewSubmariner(submarinerCRName), metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())

			By("Update the Submariner status")

			util.SetSubmarinerDeployedStatus(submariner)
			_, err = dynamicClient.Resource(*submarinerGVR).Namespace(installationNamespace).UpdateStatus(context.Background(),
				submariner, metav1.UpdateOptions{})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				addOn, err := addOnClient.AddonV1alpha1().ManagedClusterAddOns(managedClusterName).Get(context.Background(),
					constants.SubmarinerAddOnName, metav1.GetOptions{})
				Expect(err).NotTo(HaveOccurred())

				return meta.IsStatusConditionTrue(addOn.Status.Conditions, "SubmarinerConnectionDegraded")
			}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())
		})
	})
})
