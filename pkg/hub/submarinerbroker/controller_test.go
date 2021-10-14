package submarinerbroker_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	helpers "github.com/open-cluster-management/submariner-addon/pkg/helpers/testing"
	"github.com/open-cluster-management/submariner-addon/pkg/hub/submarinerbroker"
	"github.com/openshift/library-go/pkg/operator/events"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	kubeFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	clusterSetFake "open-cluster-management.io/api/client/cluster/clientset/versioned/fake"
	clusterSetInformers "open-cluster-management.io/api/client/cluster/informers/externalversions"
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
)

const (
	finalizerName  = "cluster.open-cluster-management.io/submariner-cleanup"
	clusterSetName = "east"
	brokerNS       = "east-broker"
	brokerRoleName = "submariner-k8s-broker-cluster"
)

var _ = Describe("Controller", func() {
	t := newBrokerControllerTestDriver()

	When("a ManagedClusterSet is created", func() {
		It("should add the Finalizer", func() {
			helpers.AwaitFinalizer(finalizerName, func() (metav1.Object, error) {
				return t.clusterSetClient.ClusterV1beta1().ManagedClusterSets().Get(context.TODO(), clusterSetName, metav1.GetOptions{})
			})
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

		Context("and creation of the broker Namespace resource initially fails", func() {
			BeforeEach(func() {
				t.justBeforeRun = func() {
					helpers.FailOnAction(&t.kubeClient.Fake, "namespaces", "create", nil, true)
				}
			})

			It("should eventually create it", func() {
				t.awaitNamespace()
			})
		})

		Context("and creation of the IPsec PSK Secret resource initially fails", func() {
			BeforeEach(func() {
				t.justBeforeRun = func() {
					helpers.FailOnAction(&t.kubeClient.Fake, "secrets", "create", nil, true)
				}
			})

			It("should eventually create it", func() {
				t.awaitSecret()
			})
		})
	})

	When("a ManagedClusterSet is being deleted", func() {
		BeforeEach(func() {
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
		})

		JustBeforeEach(func() {
			time.Sleep(200 * time.Millisecond)

			now := metav1.Now()
			t.clusterSet.DeletionTimestamp = &now

			_, err := t.clusterSetClient.ClusterV1beta1().ManagedClusterSets().Update(context.TODO(), t.clusterSet, metav1.UpdateOptions{})
			Expect(err).To(Succeed())
		})

		It("should remove the Finalizer", func() {
			helpers.AwaitNoFinalizer(finalizerName, func() (metav1.Object, error) {
				return t.clusterSetClient.ClusterV1beta1().ManagedClusterSets().Get(context.TODO(), clusterSetName, metav1.GetOptions{})
			})
		})

		It("should delete the broker Namespace resource", func() {
			t.awaitNoNamespace()
		})

		It("should delete the broker Role resource", func() {
			Eventually(func() bool {
				_, err := t.kubeClient.RbacV1().Roles(brokerNS).Get(context.TODO(), brokerRoleName, metav1.GetOptions{})

				return errors.IsNotFound(err)
			}).Should(BeTrue(), "Broker Role still exists")
		})

		Context("and deletion of the broker Namespace initially fails", func() {
			BeforeEach(func() {
				t.justBeforeRun = func() {
					helpers.FailOnAction(&t.kubeClient.Fake, "namespaces", "delete", nil, true)
				}
			})

			It("should eventually delete it", func() {
				t.awaitNoNamespace()
			})
		})
	})
})

type brokerControllerTestDriver struct {
	kubeClient       *kubeFake.Clientset
	kubeObjs         []runtime.Object
	justBeforeRun    func()
	clusterSetClient *clusterSetFake.Clientset
	clusterSet       *clusterv1beta1.ManagedClusterSet
	stop             context.CancelFunc
}

func newBrokerControllerTestDriver() *brokerControllerTestDriver {
	t := &brokerControllerTestDriver{}

	BeforeEach(func() {
		t.clusterSet = &clusterv1beta1.ManagedClusterSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "east",
			},
		}

		t.kubeObjs = []runtime.Object{}
		t.justBeforeRun = nil
	})

	JustBeforeEach(func() {
		t.kubeClient = kubeFake.NewSimpleClientset(t.kubeObjs...)

		t.clusterSetClient = clusterSetFake.NewSimpleClientset(t.clusterSet)

		informerFactory := clusterSetInformers.NewSharedInformerFactory(t.clusterSetClient, 0)

		if t.justBeforeRun != nil {
			t.justBeforeRun()
		}

		controller := submarinerbroker.NewController(t.clusterSetClient.ClusterV1beta1().ManagedClusterSets(),
			t.kubeClient, informerFactory.Cluster().V1beta1().ManagedClusterSets(),
			events.NewLoggingEventRecorder("test"))

		var ctx context.Context

		ctx, t.stop = context.WithCancel(context.TODO())

		informerFactory.Start(ctx.Done())

		cache.WaitForCacheSync(ctx.Done(), informerFactory.Cluster().V1beta1().ManagedClusterSets().Informer().HasSynced)

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
