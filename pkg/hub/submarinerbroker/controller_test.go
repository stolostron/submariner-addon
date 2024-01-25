package submarinerbroker_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/stolostron/submariner-addon/pkg/constants"
	"github.com/stolostron/submariner-addon/pkg/hub/submarinerbroker"
	"github.com/stolostron/submariner-addon/pkg/resource"
	fakereactor "github.com/submariner-io/admiral/pkg/fake"
	"github.com/submariner-io/admiral/pkg/finalizer"
	"github.com/submariner-io/admiral/pkg/test"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/ptr"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonfake "open-cluster-management.io/api/client/addon/clientset/versioned/fake"
	addoninformers "open-cluster-management.io/api/client/addon/informers/externalversions"
	clusterSetFake "open-cluster-management.io/api/client/cluster/clientset/versioned/fake"
	clusterSetInformers "open-cluster-management.io/api/client/cluster/informers/externalversions"
	clusterv1beta2 "open-cluster-management.io/api/cluster/v1beta2"
)

const (
	finalizerName  = "cluster.open-cluster-management.io/submariner-cleanup"
	clusterSetName = "east"
	brokerNS       = "east-broker"
	brokerRoleName = "submariner-k8s-broker-cluster"
)

var _ = Describe("Controller", func() {
	Describe("", testManagedClusterSet)
	Describe("", testClusterManagementAddOn)
})

func testManagedClusterSet() {
	t := newBrokerControllerTestDriver()

	When("a ManagedClusterSet is created", func() {
		It("should add the Finalizer", func() {
			test.AwaitFinalizer(resource.ForManagedClusterSet(t.clusterSetClient.ClusterV1beta2().ManagedClusterSets()),
				clusterSetName, finalizerName)
		})

		It("should create the broker Namespace resource", func() {
			t.awaitNamespace()
		})

		It("should create the broker Role resource", func() {
			Eventually(func() error {
				_, err := t.kubeClient.RbacV1().Roles(brokerNS).Get(context.TODO(), brokerRoleName, metav1.GetOptions{})

				return err
			}).Should(Succeed(), "Broker Role not found")
		})

		It("should create the IPsec PSK Secret resource", func() {
			t.awaitSecret()
		})

		It("should annotate the ManagedClusterSet with the broker namespace", func() {
			Eventually(func() string {
				cs, err := t.clusterSetClient.ClusterV1beta2().ManagedClusterSets().Get(context.Background(), t.clusterSet.Name,
					metav1.GetOptions{})
				Expect(err).To(Succeed())
				return cs.Annotations[submarinerbroker.SubmBrokerNamespaceKey]
			}, 3).Should(Equal(brokerNS))
		})

		Context("and creation of the broker Namespace resource initially fails", func() {
			BeforeEach(func() {
				t.justBeforeRun = func() {
					fakereactor.FailOnAction(&t.kubeClient.Fake, "namespaces", "create", nil, true)
				}
			})

			It("should eventually create it", func() {
				t.awaitNamespace()
			})
		})

		Context("and creation of the IPsec PSK Secret resource initially fails", func() {
			BeforeEach(func() {
				t.justBeforeRun = func() {
					fakereactor.FailOnAction(&t.kubeClient.Fake, "secrets", "create", nil, true)
				}
			})

			It("should eventually create it", func() {
				t.awaitSecret()
			})
		})
	})

	When("a ManagedClusterSet with SelectorType set to LabelSelector is created", func() {
		BeforeEach(func() {
			t.clusterSet.Spec.ClusterSelector.SelectorType = clusterv1beta2.LabelSelector
		})

		It("should not deploy the broker components", func() {
			t.ensureNoNamespace()
		})
	})

	When("a ManagedClusterSet with SelectorType set to ExclusiveClusterSetLabel is created", func() {
		BeforeEach(func() {
			t.clusterSet.Spec.ClusterSelector.SelectorType = clusterv1beta2.ExclusiveClusterSetLabel
		})

		It("should deploy the broker components", func() {
			t.awaitNamespace()
		})
	})

	When("a ManagedClusterSet is being deleted", func() {
		BeforeEach(func() {
			t.initBrokerSetup()
		})

		JustBeforeEach(func() {
			time.Sleep(100 * time.Millisecond)

			By("Deleting ManagedClusterSet")

			Expect(t.clusterSetClient.ClusterV1beta2().ManagedClusterSets().Delete(context.Background(), t.clusterSet.Name,
				metav1.DeleteOptions{})).To(Succeed())
		})

		It("should clean up the broker resources", func() {
			t.awaitNoNamespace()
			t.awaitNoBrokerRole()

			test.AwaitNoResource(resource.ForManagedClusterSet(t.clusterSetClient.ClusterV1beta2().ManagedClusterSets()), clusterSetName)
		})

		Context("and deletion of the broker Namespace initially fails", func() {
			BeforeEach(func() {
				t.justBeforeRun = func() {
					fakereactor.FailOnAction(&t.kubeClient.Fake, "namespaces", "delete", nil, true)
				}
			})

			It("should eventually delete it", func() {
				t.awaitNoNamespace()
			})
		})
	})
}

func testClusterManagementAddOn() {
	t := newBrokerControllerTestDriver()

	BeforeEach(func() {
		t.initBrokerSetup()

		// Create a non-submariner ManagedClusterAddOn - should be ignored during cleanup.
		_, err := t.addOnClient.AddonV1alpha1().ManagedClusterAddOns("cluster1").Create(context.Background(),
			&addonv1alpha1.ManagedClusterAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name: "other",
				},
			}, metav1.CreateOptions{})
		Expect(err).To(Succeed())

		// Create another ManagedClusterSet with a label selector - should be ignored during cleanup.
		_, err = t.clusterSetClient.ClusterV1beta2().ManagedClusterSets().Create(context.Background(), &clusterv1beta2.ManagedClusterSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "west",
			},
			Spec: clusterv1beta2.ManagedClusterSetSpec{ClusterSelector: clusterv1beta2.ManagedClusterSelector{
				SelectorType: clusterv1beta2.LabelSelector,
			}},
		}, metav1.CreateOptions{})
		Expect(err).To(Succeed())
	})

	When("the ClusterManagementAddOn is being deleted", func() {
		JustBeforeEach(func() {
			time.Sleep(time.Millisecond * 100)

			By("Deleting ClusterManagementAddOn")

			Expect(t.addOnClient.AddonV1alpha1().ClusterManagementAddOns().Delete(context.Background(), t.clusterMgmtAddon.Name,
				metav1.DeleteOptions{})).To(Succeed())
		})

		Context("with owned ManagedClusterAddOns", func() {
			var (
				addOn1 *addonv1alpha1.ManagedClusterAddOn
				addOn2 *addonv1alpha1.ManagedClusterAddOn
			)

			BeforeEach(func() {
				ownerRef := metav1.OwnerReference{
					APIVersion:         addonv1alpha1.GroupVersion.String(),
					Kind:               "ClusterManagementAddOn",
					Name:               t.clusterMgmtAddon.Name,
					UID:                t.clusterMgmtAddon.UID,
					BlockOwnerDeletion: ptr.To(true),
					Controller:         ptr.To(true),
				}

				addOn1 = &addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:            constants.SubmarinerAddOnName,
						Namespace:       "cluster1",
						OwnerReferences: []metav1.OwnerReference{ownerRef},
						Finalizers:      []string{constants.SubmarinerAddOnFinalizer},
					},
				}

				_, err := t.addOnClient.AddonV1alpha1().ManagedClusterAddOns(addOn1.Namespace).Create(context.Background(), addOn1,
					metav1.CreateOptions{})
				Expect(err).To(Succeed())

				addOn2 = &addonv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:            constants.SubmarinerAddOnName,
						Namespace:       "cluster2",
						OwnerReferences: []metav1.OwnerReference{ownerRef},
					},
				}

				_, err = t.addOnClient.AddonV1alpha1().ManagedClusterAddOns(addOn2.Namespace).Create(context.Background(), addOn2,
					metav1.CreateOptions{})
				Expect(err).To(Succeed())
			})

			It("should delete the owned ManagedClusterAddOns and clean up the broker resources after all the ManagedClusterAddOns "+
				"are deleted", func() {
				t.ensureNamespace()

				Eventually(func() error {
					return finalizer.Remove(context.Background(), resource.ForAddon(t.addOnClient.AddonV1alpha1().
						ManagedClusterAddOns(addOn1.Namespace)), addOn1, constants.SubmarinerAddOnFinalizer)
				}).Should(Succeed())

				t.awaitNoNamespace()
				t.awaitNoBrokerRole()

				test.AwaitNoFinalizer(resource.ForManagedClusterSet(t.clusterSetClient.ClusterV1beta2().ManagedClusterSets()),
					clusterSetName, finalizerName)

				// Ensure broker setup doesn't happen after the ClusterManagementAddOn is actually deleted.

				test.AwaitNoResource(resource.ForClusterAddon(t.addOnClient.AddonV1alpha1().ClusterManagementAddOns()),
					t.clusterMgmtAddon.Name)

				Expect(t.clusterSetClient.ClusterV1beta2().ManagedClusterSets().Delete(context.Background(), t.clusterSet.Name,
					metav1.DeleteOptions{})).To(Succeed())

				_, err := t.clusterSetClient.ClusterV1beta2().ManagedClusterSets().Create(context.Background(), t.clusterSet,
					metav1.CreateOptions{})
				Expect(err).To(Succeed())

				t.ensureNoNamespace()
			})
		})

		Context("and deletion of the broker Namespace initially fails", func() {
			BeforeEach(func() {
				t.justBeforeRun = func() {
					fakereactor.FailOnAction(&t.kubeClient.Fake, "namespaces", "delete", nil, true)
				}
			})

			It("should eventually delete it", func() {
				t.awaitNoNamespace()
			})
		})
	})
}

type brokerControllerTestDriver struct {
	kubeClient       *kubeFake.Clientset
	kubeObjs         []runtime.Object
	justBeforeRun    func()
	clusterSetClient *clusterSetFake.Clientset
	clusterSet       *clusterv1beta2.ManagedClusterSet
	addOnClient      *addonfake.Clientset
	clusterMgmtAddon *addonv1alpha1.ClusterManagementAddOn
	stop             context.CancelFunc
}

func newBrokerControllerTestDriver() *brokerControllerTestDriver {
	t := &brokerControllerTestDriver{}

	BeforeEach(func() {
		t.clusterSet = &clusterv1beta2.ManagedClusterSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "east",
			},
		}

		t.kubeObjs = []runtime.Object{}
		t.justBeforeRun = nil

		t.clusterMgmtAddon = &addonv1alpha1.ClusterManagementAddOn{
			ObjectMeta: metav1.ObjectMeta{
				Name:       constants.SubmarinerAddOnName,
				Finalizers: []string{constants.SubmarinerAddOnFinalizer},
			},
		}

		t.addOnClient = addonfake.NewSimpleClientset()
		fakereactor.AddBasicReactors(&t.addOnClient.Fake)

		var err error

		t.clusterMgmtAddon, err = t.addOnClient.AddonV1alpha1().ClusterManagementAddOns().Create(context.Background(), t.clusterMgmtAddon,
			metav1.CreateOptions{})
		Expect(err).To(Succeed())

		t.clusterSetClient = clusterSetFake.NewSimpleClientset()
		fakereactor.AddBasicReactors(&t.clusterSetClient.Fake)
	})

	JustBeforeEach(func() {
		t.kubeClient = kubeFake.NewSimpleClientset(t.kubeObjs...)

		_, err := t.clusterSetClient.ClusterV1beta2().ManagedClusterSets().Create(context.Background(), t.clusterSet,
			metav1.CreateOptions{})
		Expect(err).To(Succeed())

		clusterInformerFactory := clusterSetInformers.NewSharedInformerFactory(t.clusterSetClient, 0)

		addOnInformerFactory := addoninformers.NewSharedInformerFactory(t.addOnClient, 0)

		if t.justBeforeRun != nil {
			t.justBeforeRun()
		}

		controller := submarinerbroker.NewController(t.kubeClient,
			t.clusterSetClient.ClusterV1beta2().ManagedClusterSets(),
			clusterInformerFactory.Cluster().V1beta2().ManagedClusterSets(),
			t.addOnClient,
			addOnInformerFactory.Addon().V1alpha1(),
			events.NewLoggingEventRecorder("test"))

		var ctx context.Context

		ctx, t.stop = context.WithCancel(context.TODO())

		clusterInformerFactory.Start(ctx.Done())
		addOnInformerFactory.Start(ctx.Done())

		cache.WaitForCacheSync(ctx.Done(),
			clusterInformerFactory.Cluster().V1beta2().ManagedClusterSets().Informer().HasSynced,
			addOnInformerFactory.Addon().V1alpha1().ClusterManagementAddOns().Informer().HasSynced,
			addOnInformerFactory.Addon().V1alpha1().ManagedClusterAddOns().Informer().HasSynced)

		go controller.Run(ctx, 1)
	})

	AfterEach(func() {
		t.stop()
	})

	return t
}

func (t *brokerControllerTestDriver) awaitSecret() {
	Eventually(func() error {
		_, err := t.kubeClient.CoreV1().Secrets(brokerNS).Get(context.TODO(), "submariner-ipsec-psk", metav1.GetOptions{})

		return err
	}).Should(Succeed(), "IPsec PSK Secret not found")
}

func (t *brokerControllerTestDriver) awaitNamespace() {
	Eventually(func() error {
		_, err := t.kubeClient.CoreV1().Namespaces().Get(context.TODO(), brokerNS, metav1.GetOptions{})

		return err
	}).Should(Succeed(), "Broker Namespace not found")
}

func (t *brokerControllerTestDriver) awaitNoNamespace() {
	Eventually(func() bool {
		_, err := t.kubeClient.CoreV1().Namespaces().Get(context.TODO(), brokerNS, metav1.GetOptions{})

		return errors.IsNotFound(err)
	}).Should(BeTrue(), "Broker Namespace still exists")
}

func (t *brokerControllerTestDriver) ensureNoNamespace() {
	Consistently(func() bool {
		_, err := t.kubeClient.CoreV1().Namespaces().Get(context.TODO(), brokerNS, metav1.GetOptions{})

		return errors.IsNotFound(err)
	}).Should(BeTrue(), "Broker Namespace exists")
}

func (t *brokerControllerTestDriver) ensureNamespace() {
	Consistently(func() bool {
		_, err := t.kubeClient.CoreV1().Namespaces().Get(context.TODO(), brokerNS, metav1.GetOptions{})

		return err == nil
	}).Should(BeTrue(), "Broker Namespace does not exist")
}

func (t *brokerControllerTestDriver) awaitNoBrokerRole() {
	Eventually(func() bool {
		_, err := t.kubeClient.RbacV1().Roles(brokerNS).Get(context.TODO(), brokerRoleName, metav1.GetOptions{})

		return errors.IsNotFound(err)
	}).Should(BeTrue(), "Broker Role still exists")
}

func (t *brokerControllerTestDriver) initBrokerSetup() {
	t.clusterSet.Annotations = map[string]string{submarinerbroker.SubmBrokerNamespaceKey: brokerNS}
	t.clusterSet.Finalizers = []string{finalizerName}
	t.kubeObjs = []runtime.Object{
		&corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: brokerNS,
			},
		},
		&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      brokerRoleName,
				Namespace: brokerNS,
			},
		},
	}
}
