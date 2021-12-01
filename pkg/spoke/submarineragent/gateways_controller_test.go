package submarineragent_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/submariner-addon/pkg/spoke/submarineragent"
	"github.com/openshift/library-go/pkg/operator/events"
	fakereactor "github.com/submariner-io/admiral/pkg/fake"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeInformers "k8s.io/client-go/informers"
	kubeFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

const gatewayNodesLabeledType = "SubmarinerGatewayNodesLabeled"

var _ = Describe("Gateways Status Controller", func() {
	t := newGatewaysControllerTestDriver()

	When("there is a worker node labeled as a gateway", func() {
		BeforeEach(func() {
			t.nodes = []*corev1.Node{
				newWorkerNode("worker-1"),
				newGatewayNode("worker-2"),
				{
					ObjectMeta: metav1.ObjectMeta{
						Name: "non-worker",
					},
				},
			}
		})

		It("should add the ManagedClusterAddOn condition with status True", func() {
			t.awaitGatwaysLabeledCondition()
		})

		Context("and is subsequently unlabeled", func() {
			JustBeforeEach(func() {
				t.awaitGatwaysLabeledCondition()
			})

			Context("by setting to false", func() {
				It("should update the ManagedClusterAddOn condition with status False", func() {
					labelGateway(t.nodes[1], false)
					_, err := t.kubeClient.CoreV1().Nodes().Update(context.TODO(), t.nodes[1], metav1.UpdateOptions{})
					Expect(err).To(Succeed())

					t.awaitGatwaysNotLabeledCondition()
				})
			})

			Context("by removing it", func() {
				It("should update the ManagedClusterAddOn condition with status False", func() {
					t.awaitGatwaysLabeledCondition()

					delete(t.nodes[1].Labels, "submariner.io/gateway")
					_, err := t.kubeClient.CoreV1().Nodes().Update(context.TODO(), t.nodes[1], metav1.UpdateOptions{})
					Expect(err).To(Succeed())

					t.awaitGatwaysNotLabeledCondition()
				})
			})
		})
	})

	When("initially there are no worker nodes labeled as gateways and one is subsequently labeled", func() {
		BeforeEach(func() {
			t.nodes = []*corev1.Node{
				newWorkerNode("worker-1"),
			}
		})

		It("should update the ManagedClusterAddOn condition with status True", func() {
			t.awaitGatwaysNotLabeledCondition()
			labelGateway(t.nodes[0], true)
			_, err := t.kubeClient.CoreV1().Nodes().Update(context.TODO(), t.nodes[0], metav1.UpdateOptions{})
			Expect(err).To(Succeed())

			t.awaitGatwaysLabeledCondition()
		})
	})

	When("updating the ManagedClusterAddOn status initially fails", func() {
		Context("", func() {
			BeforeEach(func() {
				fakereactor.FailOnAction(&t.addOnClient.Fake, "managedclusteraddons", "update", nil, true)
			})

			It("should eventually update it", func() {
				t.awaitGatwaysLabeledCondition()
			})
		})

		Context("with a conflict error", func() {
			BeforeEach(func() {
				fakereactor.ConflictOnUpdateReactor(&t.addOnClient.Fake, "managedclusteraddons")
			})

			It("should eventually update it", func() {
				t.awaitGatwaysLabeledCondition()
			})
		})
	})
})

type gatewaysControllerTestDriver struct {
	managedClusterAddOnTestBase
	kubeClient *kubeFake.Clientset
	nodes      []*corev1.Node
	stop       context.CancelFunc
}

func newGatewaysControllerTestDriver() *gatewaysControllerTestDriver {
	t := &gatewaysControllerTestDriver{}

	BeforeEach(func() {
		t.kubeClient = kubeFake.NewSimpleClientset()
		t.managedClusterAddOnTestBase.init()
	})

	JustBeforeEach(func() {
		for _, node := range t.nodes {
			_, err := t.kubeClient.CoreV1().Nodes().Create(context.TODO(), node, metav1.CreateOptions{})
			Expect(err).To(Succeed())
		}

		kubeInformerFactory := kubeInformers.NewSharedInformerFactory(t.kubeClient, 0)

		t.managedClusterAddOnTestBase.run()

		controller := submarineragent.NewGatewaysStatusController(clusterName, t.addOnClient,
			kubeInformerFactory.Core().V1().Nodes(), events.NewLoggingEventRecorder("test"))

		var ctx context.Context

		ctx, t.stop = context.WithCancel(context.TODO())

		kubeInformerFactory.Start(ctx.Done())

		cache.WaitForCacheSync(ctx.Done(), kubeInformerFactory.Core().V1().Nodes().Informer().HasSynced)

		go controller.Run(ctx, 1)
	})

	AfterEach(func() {
		t.stop()
	})

	return t
}

func (t *gatewaysControllerTestDriver) awaitStatusCondition(status metav1.ConditionStatus, reason string) {
	t.awaitManagedClusterAddOnStatusCondition(&metav1.Condition{
		Type:   gatewayNodesLabeledType,
		Status: status,
		Reason: reason,
	})
}

func (t *gatewaysControllerTestDriver) awaitGatwaysNotLabeledCondition() {
	t.awaitStatusCondition(metav1.ConditionFalse, "SubmarinerGatewayNodesUnlabeled")
}

func (t *gatewaysControllerTestDriver) awaitGatwaysLabeledCondition() {
	t.awaitStatusCondition(metav1.ConditionTrue, "SubmarinerGatewayNodesLabeled")
}

func newGatewayNode(name string) *corev1.Node {
	node := newWorkerNode(name)
	labelGateway(node, true)

	return node
}
