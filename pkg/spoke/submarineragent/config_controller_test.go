package submarineragent_test

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	configv1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	configFake "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/clientset/versioned/fake"
	configInformers "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/informers/externalversions"
	cloudFake "github.com/stolostron/submariner-addon/pkg/cloud/fake"
	"github.com/stolostron/submariner-addon/pkg/constants"
	"github.com/stolostron/submariner-addon/pkg/spoke/submarineragent"
	"github.com/submariner-io/admiral/pkg/fake"
	"github.com/submariner-io/admiral/pkg/test"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubeInformers "k8s.io/client-go/informers"
	kubeFake "k8s.io/client-go/kubernetes/fake"
	clientTesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	addonInformers "open-cluster-management.io/api/client/addon/informers/externalversions"
)

const (
	aws                  = "AWS"
	gcp                  = "GCP"
	gatewayConditionType = "SubmarinerGatewaysLabeled"
)

var _ = Describe("Config Controller", func() {
	t := newConfigControllerTestDriver()

	testWorkerNodeLabeling(t)

	testSubmarinerConfig(t)

	testManagedClusterAddOn(t)
})

func labelGateway(node *corev1.Node, isGateway bool) {
	node.Labels["submariner.io/gateway"] = strconv.FormatBool(isGateway)
}

func testWorkerNodeLabeling(t *configControllerTestDriver) {
	When("no existing worker nodes are labeled as gateways", func() {
		It("should label the desired number of gateway nodes", func() {
			t.awaitLabeledNodes()
			t.awaitGatewaysLabeledSuccessCondition()
		})

		It("should update the SubmarinerConfig cluster environment prepared condition", func() {
			t.awaitClusterEnvPreparedSuccessCondition()
		})
	})

	When("the desired number of gateway nodes are already", func() {
		BeforeEach(func() {
			labelGateway(t.nodes[0], true)
		})

		Context("partially labeled", func() {
			It("should fully label them", func() {
				t.awaitLabeledNodes()
				t.awaitGatewaysLabeledSuccessCondition()
			})
		})

		Context("fully labeled", func() {
			BeforeEach(func() {
				t.nodes[0].Labels["gateway.submariner.io/udp-port"] = strconv.Itoa(t.config.Spec.IPSecNATTPort)
			})

			It("should not try to update them", func() {
				test.EnsureNoActionsForResource(&t.kubeClient.Fake, "nodes", "update")
			})
		})
	})

	When("initially there is an insufficient number of worker nodes", func() {
		BeforeEach(func() {
			t.config.Spec.Gateways = 3
			t.nodes[0].Labels = map[string]string{}
		})

		It("should eventually label the desired number of gateway nodes", func() {
			t.awaitSubmarinerConfigStatusCondition(&metav1.Condition{
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
			t.awaitGatewaysLabeledSuccessCondition()
		})
	})

	When("the desired number of gateway nodes is increased", func() {
		BeforeEach(func() {
			labelGateway(t.nodes[0], true)
			t.nodes[0].Labels["gateway.submariner.io/udp-port"] = strconv.Itoa(t.config.Spec.IPSecNATTPort)
		})

		It("should label the additional gateway nodes", func() {
			t.awaitGatewaysLabeledSuccessCondition()

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
			labelGateway(t.nodes[0], true)
			t.nodes[0].Labels["gateway.submariner.io/udp-port"] = strconv.Itoa(t.config.Spec.IPSecNATTPort)

			labelGateway(t.nodes[1], true)
			t.nodes[1].Labels["gateway.submariner.io/udp-port"] = strconv.Itoa(t.config.Spec.IPSecNATTPort)
		})

		JustBeforeEach(func() {
			t.awaitGatewaysLabeledSuccessCondition()

			t.config.Spec.Gateways = 1
			_, err := t.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(t.config.Namespace).Update(context.TODO(),
				t.config, metav1.UpdateOptions{})
			Expect(err).To(Succeed())
		})

		It("should unlabel the subtracted gateway nodes", func() {
			t.awaitLabeledNodes()
		})

		Context("and unlabeling a node initially fails", func() {
			var reactor *fake.FailOnActionReactor

			BeforeEach(func() {
				reactor = fake.FailOnAction(&t.kubeClient.Fake, "nodes", "update", nil, false)
			})

			It("should eventually unlabel it", func() {
				t.awaitGatewaysLabeledFailureCondition()
				reactor.Fail(false)
				t.awaitLabeledNodes()
				t.awaitGatewaysLabeledSuccessCondition()
			})
		})

		Context("and unlabeling a node initially fails with a conflict error", func() {
			BeforeEach(func() {
				fake.ConflictOnUpdateReactor(&t.kubeClient.Fake, "nodes")
			})

			It("should eventually unlabel it", func() {
				t.awaitLabeledNodes()
			})
		})
	})

	When("labeling a node initially fails", func() {
		var reactor *fake.FailOnActionReactor

		BeforeEach(func() {
			reactor = fake.FailOnAction(&t.kubeClient.Fake, "nodes", "update", nil, false)
		})

		It("should eventually label it", func() {
			t.awaitGatewaysLabeledFailureCondition()

			reactor.Fail(false)

			t.awaitLabeledNodes()
			t.awaitGatewaysLabeledSuccessCondition()
		})
	})

	When("labeling a node initially fails with a conflict error", func() {
		BeforeEach(func() {
			fake.ConflictOnUpdateReactor(&t.kubeClient.Fake, "nodes")
		})

		It("should eventually label it", func() {
			t.awaitLabeledNodes()
			t.awaitGatewaysLabeledSuccessCondition()

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
			t.expectProviderFactoryGet()

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
			t.expectProviderFactoryGet()

			_, err := t.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(t.config.Namespace).Update(context.TODO(),
				t.config, metav1.UpdateOptions{})
			Expect(err).To(Succeed())

			t.awaitLabeledNodes()
		})
	})

	When("the SubmarinerConfig's Platform field is set to AWS", func() {
		BeforeEach(func() {
			t.config.Status.ManagedClusterInfo.Platform = aws
			t.cloudProvider.EXPECT().PrepareSubmarinerClusterEnv().Return(nil).MinTimes(1)
		})

		It("should invoke the cloud provider and update the SubmarinerConfig status condition", func() {
			t.awaitClusterEnvPreparedSuccessCondition()
		})
	})

	When("the SubmarinerConfig's Platform field is set to GCP", func() {
		BeforeEach(func() {
			t.config.Status.ManagedClusterInfo.Platform = gcp
		})

		Context("", func() {
			BeforeEach(func() {
				t.cloudProvider.EXPECT().PrepareSubmarinerClusterEnv().Return(nil).MinTimes(1)
			})

			It("should invoke the cloud provider and update the SubmarinerConfig status condition", func() {
				t.awaitClusterEnvPreparedSuccessCondition()
			})

			Context("and the number of labeled worker nodes matches the desired number", func() {
				BeforeEach(func() {
					labelGateway(t.nodes[0], true)
				})

				It("should update the SubmarinerConfig status with a success condition", func() {
					t.awaitGatewaysLabeledSuccessCondition()
				})
			})

			Context("and the number of labeled worker nodes does not match the desired number", func() {
				It("should update the SubmarinerConfig status appropriately", func() {
					t.awaitSubmarinerConfigStatusCondition(&metav1.Condition{
						Type:   gatewayConditionType,
						Status: metav1.ConditionFalse,
						Reason: "InsufficientNodes",
					})
				})
			})
		})

		Context("and the cloud provider initially fails", func() {
			var waitCh chan struct{}

			BeforeEach(func() {
				waitCh = make(chan struct{})
				gomock.InOrder(
					t.cloudProvider.EXPECT().PrepareSubmarinerClusterEnv().Return(errors.New("fake error")).Times(1),
					t.cloudProvider.EXPECT().PrepareSubmarinerClusterEnv().DoAndReturn(func() error {
						<-waitCh

						return nil
					}).AnyTimes(),
				)
			})

			It("should initially set a failure status condition", func() {
				t.awaitSubmarinerConfigStatusCondition(&metav1.Condition{
					Type:   configv1alpha1.SubmarinerConfigConditionEnvPrepared,
					Status: metav1.ConditionFalse,
					Reason: "SubmarinerClusterEnvPreparationFailed",
				})

				close(waitCh)

				t.awaitSubmarinerConfigStatusCondition(&metav1.Condition{
					Type:   configv1alpha1.SubmarinerConfigConditionEnvPrepared,
					Status: metav1.ConditionTrue,
					Reason: "SubmarinerClusterEnvPrepared",
				})
			})
		})
	})

	When("the SubmarinerConfig is being deleted", func() {
		BeforeEach(func() {
			t.config.Spec.Gateways = 2
			labelGateway(t.nodes[0], true)
			t.nodes[0].Labels["gateway.submariner.io/udp-port"] = strconv.Itoa(t.config.Spec.IPSecNATTPort)

			labelGateway(t.nodes[1], true)
			t.nodes[1].Labels["gateway.submariner.io/udp-port"] = strconv.Itoa(t.config.Spec.IPSecNATTPort)

			now := metav1.Now()
			t.config.DeletionTimestamp = &now
		})

		It("should unlabel the gateway nodes", func() {
			t.awaitNoLabeledNodes()
		})

		Context("and unlabeling a node initially fails", func() {
			BeforeEach(func() {
				fake.FailOnAction(&t.kubeClient.Fake, "nodes", "update", nil, true)
			})

			It("should eventually unlabel it", func() {
				t.awaitNoLabeledNodes()
			})
		})

		Context("the SubmarinerConfig's Platform field is set to AWS", func() {
			var invoked chan bool

			BeforeEach(func() {
				t.config.Status.ManagedClusterInfo.Platform = aws
				invoked = make(chan bool)
				t.cloudProvider.EXPECT().CleanUpSubmarinerClusterEnv().DoAndReturn(func() error {
					invoked <- true

					return nil
				}).Times(1)
			})

			It("should invoke the cloud provider to clean up", func() {
				Eventually(invoked).Should(Receive())
			})

			It("should not unlabel the gateway nodes", func() {
				t.ensureLabeledNodes()
			})
		})

		Context("the SubmarinerConfig's Platform field is set to GCP", func() {
			BeforeEach(func() {
				t.config.Status.ManagedClusterInfo.Platform = gcp
			})

			Context("", func() {
				BeforeEach(func() {
					t.cloudProvider.EXPECT().CleanUpSubmarinerClusterEnv().AnyTimes()
				})

				It("should not unlabel the gateway nodes", func() {
					t.ensureLabeledNodes()
				})
			})

			Context("", func() {
				var invoked chan bool

				BeforeEach(func() {
					invoked = make(chan bool)
					t.cloudProvider.EXPECT().CleanUpSubmarinerClusterEnv().DoAndReturn(func() error {
						invoked <- true

						return nil
					}).Times(1)
				})

				It("should invoke the cloud provider to clean up", func() {
					Eventually(invoked).Should(Receive())
				})
			})
		})
	})

	When("updating the SubmarinerConfig status initially fails", func() {
		BeforeEach(func() {
			fake.FailOnAction(&t.configClient.Fake, "*", "update", nil, true)
		})

		It("should eventually update it", func() {
			t.awaitGatewaysLabeledSuccessCondition()
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
			labelGateway(t.nodes[0], true)
			t.nodes[0].Labels["gateway.submariner.io/udp-port"] = strconv.Itoa(t.config.Spec.IPSecNATTPort)

			labelGateway(t.nodes[1], true)
			t.nodes[1].Labels["gateway.submariner.io/udp-port"] = strconv.Itoa(t.config.Spec.IPSecNATTPort)

			now := metav1.Now()
			t.addOn.DeletionTimestamp = &now
		})

		It("should unlabel the gateway nodes", func() {
			t.awaitNoLabeledNodes()
			t.awaitSubmarinerConfigStatusCondition(&metav1.Condition{
				Type:   gatewayConditionType,
				Status: metav1.ConditionFalse,
				Reason: "ManagedClusterAddOnDeleted",
			})
		})

		Context("and unlabeling a node initially fails", func() {
			var reactor *fake.FailOnActionReactor

			BeforeEach(func() {
				reactor = fake.FailOnAction(&t.kubeClient.Fake, "nodes", "update", nil, false)
			})

			It("should eventually unlabel it", func() {
				t.awaitGatewaysLabeledFailureCondition()

				reactor.Fail(false)

				t.awaitNoLabeledNodes()
				t.awaitSubmarinerConfigStatusCondition(&metav1.Condition{
					Type:   gatewayConditionType,
					Status: metav1.ConditionFalse,
					Reason: "ManagedClusterAddOnDeleted",
				})
			})
		})

		Context("the SubmarinerConfig's Platform field is set to AWS", func() {
			BeforeEach(func() {
				t.config.Status.ManagedClusterInfo.Platform = aws
				t.cloudProvider.EXPECT().CleanUpSubmarinerClusterEnv().Return(nil).MinTimes(1)
			})

			It("should invoke the cloud provider to clean up", func() {
				t.awaitSubmarinerConfigStatusCondition(&metav1.Condition{
					Type:   gatewayConditionType,
					Status: metav1.ConditionFalse,
					Reason: "ManagedClusterAddOnDeleted",
				})
			})
		})

		Context("the SubmarinerConfig's Platform field is set to GCP", func() {
			BeforeEach(func() {
				t.config.Status.ManagedClusterInfo.Platform = gcp
			})

			Context("", func() {
				BeforeEach(func() {
					t.cloudProvider.EXPECT().CleanUpSubmarinerClusterEnv().Return(nil).MinTimes(1)
				})

				It("should invoke the cloud provider to clean up", func() {
					t.awaitSubmarinerConfigStatusCondition(&metav1.Condition{
						Type:   gatewayConditionType,
						Status: metav1.ConditionFalse,
						Reason: "ManagedClusterAddOnDeleted",
					})
				})
			})

			Context("", func() {
				BeforeEach(func() {
					t.cloudProvider.EXPECT().CleanUpSubmarinerClusterEnv().Return(nil).AnyTimes()
				})

				It("should not unlabel the gateway nodes", func() {
					t.ensureLabeledNodes()
				})
			})

			Context("and the cloud provider initially fails", func() {
				var waitCh chan struct{}

				BeforeEach(func() {
					waitCh = make(chan struct{})
					gomock.InOrder(
						t.cloudProvider.EXPECT().CleanUpSubmarinerClusterEnv().Return(errors.New("fake error")).Times(1),
						t.cloudProvider.EXPECT().CleanUpSubmarinerClusterEnv().DoAndReturn(func() error {
							<-waitCh

							return nil
						}).AnyTimes(),
					)
				})

				It("should initially set a failure status condition", func() {
					t.awaitGatewaysLabeledFailureCondition()

					close(waitCh)

					t.ensureLabeledNodes()
					t.awaitSubmarinerConfigStatusCondition(&metav1.Condition{
						Type:   gatewayConditionType,
						Status: metav1.ConditionFalse,
						Reason: "ManagedClusterAddOnDeleted",
					})
				})
			})
		})
	})
}

type configControllerTestDriver struct {
	managedClusterAddOnTestBase
	controller      factory.Controller
	config          *configv1alpha1.SubmarinerConfig
	nodes           []*corev1.Node
	stop            context.CancelFunc
	kubeClient      *kubeFake.Clientset
	configClient    *configFake.Clientset
	dynamicClient   *dynamicfake.FakeDynamicClient
	cloudProvider   *cloudFake.MockProvider
	providerFactory *cloudFake.MockProviderFactory
	mockCtrl        *gomock.Controller
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
		t.dynamicClient = dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())

		t.managedClusterAddOnTestBase.init()

		t.cloudProvider = cloudFake.NewMockProvider(t.mockCtrl)
		t.providerFactory = cloudFake.NewMockProviderFactory(t.mockCtrl)
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

		t.expectProviderFactoryGet()

		t.controller = submarineragent.NewSubmarinerConfigController(&submarineragent.SubmarinerConfigControllerInput{
			ClusterName:          clusterName,
			KubeClient:           t.kubeClient,
			ConfigClient:         t.configClient,
			DynamicClient:        t.dynamicClient,
			AddOnClient:          t.addOnClient,
			NodeInformer:         kubeInformerFactory.Core().V1().Nodes(),
			AddOnInformer:        addOnInformerFactory.Addon().V1alpha1().ManagedClusterAddOns(),
			ConfigInformer:       configInformerFactory.Submarineraddon().V1alpha1().SubmarinerConfigs(),
			CloudProviderFactory: t.providerFactory,
			Recorder:             events.NewLoggingEventRecorder("test"),
			OnSyncDefer:          GinkgoRecover,
		})

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

func (t *configControllerTestDriver) expectProviderFactoryGet() {
	if t.config != nil && t.config.Status.ManagedClusterInfo.Platform != "" {
		provider := t.cloudProvider
		if t.config.Status.ManagedClusterInfo.Platform == "Other" {
			provider = nil
		}

		t.providerFactory.EXPECT().Get(&t.config.Status.ManagedClusterInfo, gomock.Not(gomock.Nil()), gomock.Any()).
			Return(provider, provider != nil, nil).AnyTimes()
	}
}

func (t *configControllerTestDriver) awaitGatewaysLabeledSuccessCondition() {
	t.awaitSubmarinerConfigStatusCondition(&metav1.Condition{
		Type:   gatewayConditionType,
		Status: metav1.ConditionTrue,
		Reason: "Success",
	})
}

func (t *configControllerTestDriver) awaitGatewaysLabeledFailureCondition() {
	t.awaitSubmarinerConfigStatusCondition(&metav1.Condition{
		Type:   gatewayConditionType,
		Status: metav1.ConditionFalse,
		Reason: "Failure",
	})
}

func (t *configControllerTestDriver) awaitClusterEnvPreparedSuccessCondition() {
	t.awaitSubmarinerConfigStatusCondition(&metav1.Condition{
		Type:   configv1alpha1.SubmarinerConfigConditionEnvPrepared,
		Status: metav1.ConditionTrue,
		Reason: "SubmarinerClusterEnvPrepared",
	})
}

func (t *configControllerTestDriver) awaitSubmarinerConfigStatusCondition(expCond *metav1.Condition) {
	test.AwaitStatusCondition(expCond, func() ([]metav1.Condition, error) {
		config, err := t.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(clusterName).Get(context.TODO(),
			constants.SubmarinerConfigName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		return config.Status.Conditions, nil
	})
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
			Name:      constants.SubmarinerConfigName,
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
