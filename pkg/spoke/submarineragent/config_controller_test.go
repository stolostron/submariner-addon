package submarineragent_test

import (
	"context"
	"strconv"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	configv1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	configFake "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/clientset/versioned/fake"
	configInformers "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/informers/externalversions"
	cloudFake "github.com/stolostron/submariner-addon/pkg/cloud/fake"
	"github.com/stolostron/submariner-addon/pkg/helpers"
	"github.com/stolostron/submariner-addon/pkg/helpers/testing"
	"github.com/stolostron/submariner-addon/pkg/spoke/submarineragent"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeInformers "k8s.io/client-go/informers"
	kubeFake "k8s.io/client-go/kubernetes/fake"
	clientTesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	addonInformers "open-cluster-management.io/api/client/addon/informers/externalversions"
)

const (
	gatewayConditionType = "SubmarinerGatewaysLabeled"
)

var _ = Describe("Config Controller", func() {
	t := newConfigControllerTestDriver()

	testWorkerNodeLabeling(t)

	testSubmarinerConfig(t)

	testManagedClusterAddOn(t)
})

func testWorkerNodeLabeling(t *configControllerTestDriver) {
	When("no existing worker nodes are labeled as gateways", func() {
		It("should label the desired number of gateway nodes", func() {
			t.awaitLabeledNodes()
			t.awaitSuccessStatusCondition()
		})
	})

	When("the desired number of gateway nodes are already", func() {
		BeforeEach(func() {
			t.nodes[0].Labels["submariner.io/gateway"] = "true"
		})

		Context("partially labeled", func() {
			It("should fully label them", func() {
				t.awaitLabeledNodes()
				t.awaitSuccessStatusCondition()
			})
		})

		Context("fully labeled", func() {
			BeforeEach(func() {
				t.nodes[0].Labels["gateway.submariner.io/udp-port"] = strconv.Itoa(t.config.Spec.IPSecNATTPort)
			})

			It("should not try to update them", func() {
				testing.EnsureNoActionsForResource(&t.kubeClient.Fake, "nodes", "update")
			})
		})
	})

	When("initially there is an insufficient number of worker nodes", func() {
		BeforeEach(func() {
			t.config.Spec.Gateways = 3
			t.nodes[0].Labels = map[string]string{}
		})

		It("should eventually label the desired number of gateway nodes", func() {
			t.awaitSubmarinerConfigStatusCondition(metav1.Condition{
				Type:   gatewayConditionType,
				Status: metav1.ConditionFalse,
				Reason: "InsufficientNodes",
			})

			t.nodes[0].Labels["node-role.kubernetes.io/worker"] = ""
			_, err := t.kubeClient.CoreV1().Nodes().Update(context.TODO(), t.nodes[0], metav1.UpdateOptions{})
			Expect(err).To(Succeed())

			t.ensureNoLabeledNodes()

			node := newWorkerNode("worker-3")
			t.nodes = append(t.nodes, node)
			_, err = t.kubeClient.CoreV1().Nodes().Create(context.TODO(), node, metav1.CreateOptions{})
			Expect(err).To(Succeed())

			t.awaitLabeledNodes()
			t.awaitSuccessStatusCondition()
		})
	})

	When("the desired number of gateway nodes is increased", func() {
		BeforeEach(func() {
			t.nodes[0].Labels["submariner.io/gateway"] = "true"
			t.nodes[0].Labels["gateway.submariner.io/udp-port"] = strconv.Itoa(t.config.Spec.IPSecNATTPort)
		})

		It("should label the additional gateway nodes", func() {
			t.awaitSuccessStatusCondition()

			t.config.Spec.Gateways = 2
			_, err := t.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(t.config.Namespace).Update(context.TODO(),
				t.config, metav1.UpdateOptions{})
			Expect(err).To(Succeed())

			t.awaitLabeledNodes()
		})
	})

	When("the desired number of gateway nodes is decreased", func() {
		BeforeEach(func() {
			t.config.Spec.Gateways = 2
			t.nodes[0].Labels["submariner.io/gateway"] = "true"
			t.nodes[0].Labels["gateway.submariner.io/udp-port"] = strconv.Itoa(t.config.Spec.IPSecNATTPort)

			t.nodes[1].Labels["submariner.io/gateway"] = "true"
			t.nodes[1].Labels["gateway.submariner.io/udp-port"] = strconv.Itoa(t.config.Spec.IPSecNATTPort)
		})

		JustBeforeEach(func() {
			t.awaitSuccessStatusCondition()

			t.config.Spec.Gateways = 1
			_, err := t.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(t.config.Namespace).Update(context.TODO(),
				t.config, metav1.UpdateOptions{})
			Expect(err).To(Succeed())
		})

		It("should unlabel the subtracted gateway nodes", func() {
			t.awaitLabeledNodes()
		})

		Context("and unlabeling a node initially fails", func() {
			var reactor *testing.FailingReactor

			BeforeEach(func() {
				reactor = testing.FailOnAction(&t.kubeClient.Fake, "nodes", "update", nil, false)
			})

			It("should eventually unlabel it", func() {
				t.awaitFailureStatusCondition()
				reactor.Fail(false)
				t.awaitLabeledNodes()
				t.awaitSuccessStatusCondition()
			})
		})

		Context("and unlabeling a node initially fails with a conflict error", func() {
			BeforeEach(func() {
				testing.ConflictOnUpdateReactor(&t.kubeClient.Fake, "nodes")
			})

			It("should eventually unlabel it", func() {
				t.awaitLabeledNodes()
			})
		})
	})

	When("labeling a node initially fails", func() {
		var reactor *testing.FailingReactor

		BeforeEach(func() {
			reactor = testing.FailOnAction(&t.kubeClient.Fake, "nodes", "update", nil, false)
		})

		It("should eventually label it", func() {
			t.awaitFailureStatusCondition()

			reactor.Fail(false)

			t.awaitLabeledNodes()
			t.awaitSuccessStatusCondition()
		})
	})

	When("labeling a node initially fails with a conflict error", func() {
		BeforeEach(func() {
			testing.ConflictOnUpdateReactor(&t.kubeClient.Fake, "nodes")
		})

		It("should eventually label it", func() {
			t.awaitLabeledNodes()
			t.awaitSuccessStatusCondition()

			// Ensure there was no failure condition update due to the conflict error.
			for _, a := range t.configClient.Actions() {
				update, ok := a.(clientTesting.UpdateActionImpl)
				if ok {
					config, _ := update.Object.(*configv1alpha1.SubmarinerConfig)
					c := meta.FindStatusCondition(config.Status.Conditions, gatewayConditionType)
					if c != nil {
						Expect(c.Status).To(Equal(metav1.ConditionTrue))
					}
				}
			}
		})
	})
}

func testSubmarinerConfig(t *configControllerTestDriver) {
	When("the SubmarinerConfig doesn't initially exist", func() {
		BeforeEach(func() {
			t.config = nil
		})

		It("should eventually label the gateway nodes", func() {
			t.ensureNoLabeledNodes()

			t.config = newSubmarinerConfig()
			_, err := t.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(t.config.Namespace).Create(context.TODO(),
				t.config, metav1.CreateOptions{})
			Expect(err).To(Succeed())

			t.awaitLabeledNodes()
		})
	})

	When("the SubmarinerConfig's Platform field isn't initially set", func() {
		BeforeEach(func() {
			t.config.Status.ManagedClusterInfo.Platform = ""
		})

		It("should eventually label the gateway nodes", func() {
			t.ensureNoLabeledNodes()

			t.config.Status.ManagedClusterInfo.Platform = "Other"
			_, err := t.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(t.config.Namespace).Update(context.TODO(),
				t.config, metav1.UpdateOptions{})
			Expect(err).To(Succeed())

			t.awaitLabeledNodes()
		})
	})

	When("the SubmarinerConfig's Platform field is set to AWS", func() {
		BeforeEach(func() {
			t.config.Status.ManagedClusterInfo.Platform = "AWS"
		})

		Context("and the number of labeled worker nodes matches the desired number", func() {
			BeforeEach(func() {
				t.nodes[0].Labels["submariner.io/gateway"] = "true"
			})

			It("should update the SubmarinerConfig status with a success condition", func() {
				t.awaitSuccessStatusCondition()
			})
		})

		Context("and the number of labeled worker nodes does not match the desired number", func() {
			It("should update the SubmarinerConfig status appropriately", func() {
				t.awaitSubmarinerConfigStatusCondition(metav1.Condition{
					Type:   gatewayConditionType,
					Status: metav1.ConditionFalse,
					Reason: "InsufficientNodes",
				})
			})
		})
	})

	When("the SubmarinerConfig is being deleted", func() {
		BeforeEach(func() {
			t.config.Spec.Gateways = 2
			t.nodes[0].Labels["submariner.io/gateway"] = "true"
			t.nodes[0].Labels["gateway.submariner.io/udp-port"] = strconv.Itoa(t.config.Spec.IPSecNATTPort)

			t.nodes[1].Labels["submariner.io/gateway"] = "true"
			t.nodes[1].Labels["gateway.submariner.io/udp-port"] = strconv.Itoa(t.config.Spec.IPSecNATTPort)
		})

		JustBeforeEach(func() {
			t.awaitSuccessStatusCondition()

			now := metav1.Now()
			t.config.DeletionTimestamp = &now
			_, err := t.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(t.config.Namespace).Update(context.TODO(),
				t.config, metav1.UpdateOptions{})
			Expect(err).To(Succeed())
		})

		It("should unlabel the gateway nodes", func() {
			t.awaitNoLabeledNodes()
		})

		Context("and unlabeling a node initially fails", func() {
			BeforeEach(func() {
				testing.FailOnAction(&t.kubeClient.Fake, "nodes", "update", nil, true)
			})

			It("should eventually unlabel it", func() {
				t.awaitNoLabeledNodes()
			})
		})

		Context("the SubmarinerConfig's Platform field is set to AWS", func() {
			BeforeEach(func() {
				t.config.Status.ManagedClusterInfo.Platform = "AWS"
			})

			It("should not unlabel the gateway nodes", func() {
				t.ensureLabeledNodes()
			})
		})
	})

	When("updating the SubmarinerConfig status initially fails", func() {
		BeforeEach(func() {
			testing.FailOnAction(&t.configClient.Fake, "*", "update", nil, true)
		})

		It("should eventually update it", func() {
			t.awaitSuccessStatusCondition()
		})
	})
}

func testManagedClusterAddOn(t *configControllerTestDriver) {
	When("the ManagedClusterAddOn doesn't initially exist", func() {
		BeforeEach(func() {
			t.addOn = nil
		})

		It("should eventually label the gateway nodes", func() {
			t.ensureNoLabeledNodes()

			t.addOn = newAddOn()
			_, err := t.addOnClient.AddonV1alpha1().ManagedClusterAddOns(t.addOn.Namespace).Create(context.TODO(), t.addOn,
				metav1.CreateOptions{})
			Expect(err).To(Succeed())

			t.awaitLabeledNodes()
		})
	})

	When("the ManagedClusterAddOn is being deleted", func() {
		BeforeEach(func() {
			t.config.Spec.Gateways = 2
			t.nodes[0].Labels["submariner.io/gateway"] = "true"
			t.nodes[0].Labels["gateway.submariner.io/udp-port"] = strconv.Itoa(t.config.Spec.IPSecNATTPort)

			t.nodes[1].Labels["submariner.io/gateway"] = "true"
			t.nodes[1].Labels["gateway.submariner.io/udp-port"] = strconv.Itoa(t.config.Spec.IPSecNATTPort)
		})

		JustBeforeEach(func() {
			t.awaitSuccessStatusCondition()

			now := metav1.Now()
			t.addOn.DeletionTimestamp = &now
			_, err := t.addOnClient.AddonV1alpha1().ManagedClusterAddOns(t.addOn.Namespace).Update(context.TODO(),
				t.addOn, metav1.UpdateOptions{})
			Expect(err).To(Succeed())
		})

		It("should unlabel the gateway nodes", func() {
			t.awaitNoLabeledNodes()
			t.awaitSubmarinerConfigStatusCondition(metav1.Condition{
				Type:   gatewayConditionType,
				Status: metav1.ConditionFalse,
				Reason: "ManagedClusterAddOnDeleted",
			})
		})

		Context("and unlabeling a node initially fails", func() {
			var reactor *testing.FailingReactor

			BeforeEach(func() {
				reactor = testing.FailOnAction(&t.kubeClient.Fake, "nodes", "update", nil, false)
			})

			It("should eventually unlabel it", func() {
				t.awaitFailureStatusCondition()

				reactor.Fail(false)

				t.awaitNoLabeledNodes()
				t.awaitSubmarinerConfigStatusCondition(metav1.Condition{
					Type:   gatewayConditionType,
					Status: metav1.ConditionFalse,
					Reason: "ManagedClusterAddOnDeleted",
				})
			})
		})

		Context("the SubmarinerConfig's Platform field is set to AWS", func() {
			BeforeEach(func() {
				t.config.Status.ManagedClusterInfo.Platform = "AWS"
			})

			It("should not unlabel the gateway nodes", func() {
				t.ensureLabeledNodes()
			})
		})
	})
}

type configControllerTestDriver struct {
	managedClusterAddOnTestBase
	controller   factory.Controller
	config       *configv1alpha1.SubmarinerConfig
	nodes        []*corev1.Node
	stop         context.CancelFunc
	kubeClient   *kubeFake.Clientset
	configClient *configFake.Clientset
	mockCtrl     *gomock.Controller
}

func newConfigControllerTestDriver() *configControllerTestDriver {
	t := &configControllerTestDriver{}

	BeforeEach(func() {
		t.mockCtrl = gomock.NewController(GinkgoT())
		t.config = newSubmarinerConfig()

		t.nodes = []*corev1.Node{
			newWorkerNode("worker-1"),
			newWorkerNode("worker-2"),
			{
				ObjectMeta: metav1.ObjectMeta{
					Name: "non-worker",
				},
			},
		}

		t.kubeClient = kubeFake.NewSimpleClientset()
		t.configClient = configFake.NewSimpleClientset()

		t.managedClusterAddOnTestBase.init()
	})

	JustBeforeEach(func() {
		for _, node := range t.nodes {
			_, err := t.kubeClient.CoreV1().Nodes().Create(context.TODO(), node, metav1.CreateOptions{})
			Expect(err).To(Succeed())
		}

		t.kubeClient.ClearActions()

		defaultResync := 0 * time.Second
		kubeInformerFactory := kubeInformers.NewSharedInformerFactory(t.kubeClient, defaultResync)

		if t.config != nil {
			_, err := t.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(t.config.Namespace).Create(context.TODO(),
				t.config, metav1.CreateOptions{})
			Expect(err).To(Succeed())
		}

		t.configClient.ClearActions()

		configInformerFactory := configInformers.NewSharedInformerFactory(t.configClient, defaultResync)

		t.managedClusterAddOnTestBase.run()

		addOnInformerFactory := addonInformers.NewSharedInformerFactory(t.addOnClient, defaultResync)

		t.controller = submarineragent.NewSubmarinerConfigController(clusterName, t.kubeClient, t.configClient,
			kubeInformerFactory.Core().V1().Nodes(),
			addOnInformerFactory.Addon().V1alpha1().ManagedClusterAddOns(),
			configInformerFactory.Submarineraddon().V1alpha1().SubmarinerConfigs(),
			cloudFake.NewMockProviderFactory(t.mockCtrl), events.NewLoggingEventRecorder("test"))

		var ctx context.Context

		ctx, t.stop = context.WithCancel(context.TODO())

		kubeInformerFactory.Start(ctx.Done())
		configInformerFactory.Start(ctx.Done())
		addOnInformerFactory.Start(ctx.Done())

		cache.WaitForCacheSync(ctx.Done(), kubeInformerFactory.Core().V1().Nodes().Informer().HasSynced,
			configInformerFactory.Submarineraddon().V1alpha1().SubmarinerConfigs().Informer().HasSynced,
			addOnInformerFactory.Addon().V1alpha1().ManagedClusterAddOns().Informer().HasSynced)

		go t.controller.Run(ctx, 1)
	})

	AfterEach(func() {
		t.stop()
		t.mockCtrl.Finish()
	})

	return t
}

func (t *configControllerTestDriver) awaitSuccessStatusCondition() {
	t.awaitSubmarinerConfigStatusCondition(metav1.Condition{
		Type:   gatewayConditionType,
		Status: metav1.ConditionTrue,
		Reason: "Success",
	})
}

func (t *configControllerTestDriver) awaitFailureStatusCondition() {
	t.awaitSubmarinerConfigStatusCondition(metav1.Condition{
		Type:   gatewayConditionType,
		Status: metav1.ConditionFalse,
		Reason: "Failure",
	})
}

func (t *configControllerTestDriver) awaitSubmarinerConfigStatusCondition(expCond metav1.Condition) {
	testing.AwaitStatusCondition(expCond, func() ([]metav1.Condition, error) {
		config, err := t.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(clusterName).Get(context.TODO(),
			helpers.SubmarinerConfigName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		return config.Status.Conditions, nil
	})
}

func (t *configControllerTestDriver) ensureNoSubmarinerConfigStatusCondition(condType string) {
	Consistently(func() *metav1.Condition {
		config, err := t.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(clusterName).Get(context.TODO(),
			helpers.SubmarinerConfigName, metav1.GetOptions{})
		Expect(err).To(Succeed())

		return meta.FindStatusCondition(config.Status.Conditions, condType)
	}, 300*time.Millisecond).Should(BeNil())
}

func (t *configControllerTestDriver) getLabeledWorkerNodes() []*corev1.Node {
	foundNodes := []*corev1.Node{}

	for _, expected := range t.nodes {
		actual, err := t.kubeClient.CoreV1().Nodes().Get(context.TODO(), expected.Name, metav1.GetOptions{})
		Expect(err).To(Succeed())

		if _, ok := actual.Labels["node-role.kubernetes.io/worker"]; !ok {
			continue
		}

		if actual.Labels["submariner.io/gateway"] == "true" &&
			actual.Labels["gateway.submariner.io/udp-port"] == strconv.Itoa(t.config.Spec.IPSecNATTPort) {
			foundNodes = append(foundNodes, actual)
		}
	}

	return foundNodes
}

func (t *configControllerTestDriver) awaitLabeledNodes() {
	Eventually(func() int {
		return len(t.getLabeledWorkerNodes())
	}, 2).Should(Equal(t.config.Spec.Gateways), "The expected number of worker nodes weren't labeled")
}

func (t *configControllerTestDriver) awaitNoLabeledNodes() {
	Eventually(func() int {
		return len(t.getLabeledWorkerNodes())
	}, 2).Should(BeZero(), "Expected no labeled worker nodes")
}

func (t *configControllerTestDriver) ensureNoLabeledNodes() {
	Consistently(func() int {
		return len(t.getLabeledWorkerNodes())
	}, 300*time.Millisecond).Should(BeZero(), "Expected no labeled worker nodes")
}

func (t *configControllerTestDriver) ensureLabeledNodes() {
	Consistently(func() int {
		return len(t.getLabeledWorkerNodes())
	}, 300*time.Millisecond).Should(Equal(t.config.Spec.Gateways))
}

func newWorkerNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"node-role.kubernetes.io/worker": "",
			},
		},
	}
}

func newSubmarinerConfig() *configv1alpha1.SubmarinerConfig {
	return &configv1alpha1.SubmarinerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      helpers.SubmarinerConfigName,
			Namespace: clusterName,
		},
		Spec: configv1alpha1.SubmarinerConfigSpec{
			IPSecNATTPort: 4500,
			GatewayConfig: configv1alpha1.GatewayConfig{
				Gateways: 1,
			},
		},
		Status: configv1alpha1.SubmarinerConfigStatus{
			ManagedClusterInfo: configv1alpha1.ManagedClusterInfo{
				Platform: "Other",
			},
		},
	}
}
