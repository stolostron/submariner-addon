package submarineragent_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers/testing"
	"github.com/open-cluster-management/submariner-addon/pkg/spoke/submarineragent"
	"github.com/openshift/library-go/pkg/operator/events"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	fakereactor "github.com/submariner-io/admiral/pkg/fake"
	"github.com/submariner-io/admiral/pkg/syncer/test"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	kubeInformers "k8s.io/client-go/informers"
	kubeFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

const deploymentDegradedType string = "SubmarinerAgentDegraded"

var _ = Describe("Deployment Status Controller", func() {
	t := newDeploymentControllerTestDriver()

	When("all components are deployed", func() {
		It("should update the ManagedClusterAddOn status condition to deployed", func() {
			t.awaitStatusConditionDeployed()
		})
	})

	When("the submariner subscription doesn't exist", func() {
		BeforeEach(func() {
			t.subscription = nil
		})

		It("should not update the ManagedClusterAddOn status condition", func() {
			t.awaitNoManagedClusterAddOnStatusCondition(deploymentDegradedType)
		})
	})

	When("the submariner subscription CSV isn't installed", func() {
		BeforeEach(func() {
			t.subscription.Status.InstalledCSV = ""
		})

		It("should eventually update the ManagedClusterAddOn status condition to deployed", func() {
			t.awaitStatusCondition(metav1.ConditionTrue, "CSVNotInstalled")

			t.subscription.Status.InstalledCSV = "submariner-csv"
			t.updateSubscription()

			t.awaitStatusConditionDeployed()
		})
	})

	When("the operator deployment doesn't initially exist", func() {
		BeforeEach(func() {
			t.operatorDeployment = nil
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func() {
			t.awaitStatusCondition(metav1.ConditionTrue, "NoOperatorDeployment")

			t.operatorDeployment = newOperatorDeployment()
			t.createOperatorDeployment()

			t.awaitStatusConditionDeployed()
		})
	})

	When("no operator deployment replica is initially available", func() {
		BeforeEach(func() {
			t.operatorDeployment.Status.AvailableReplicas = 0
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func() {
			t.awaitStatusCondition(metav1.ConditionTrue, "NoOperatorAvailable")

			t.operatorDeployment.Status.AvailableReplicas = 1
			t.updateOperatorDeployment()

			t.awaitStatusConditionDeployed()
		})
	})

	When("the gateway daemon set doesn't initially exist", func() {
		BeforeEach(func() {
			t.gatewayDaemonSet = nil
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func() {
			t.awaitStatusCondition(metav1.ConditionTrue, "NoGatewayDaemonSet")

			t.gatewayDaemonSet = newGatewayDaemonSet()
			t.createDaemonSet(t.gatewayDaemonSet)

			t.awaitStatusConditionDeployed()
		})
	})

	When("a gateway daemon set pod isn't initially available", func() {
		BeforeEach(func() {
			t.gatewayDaemonSet.Status.NumberUnavailable = 1
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func() {
			t.awaitStatusCondition(metav1.ConditionTrue, "GatewaysUnavailable")

			t.gatewayDaemonSet.Status.NumberUnavailable = 0
			t.updateDaemonSet(t.gatewayDaemonSet)

			t.awaitStatusConditionDeployed()
		})
	})

	When("no gateway daemon set pod is initially scheduled", func() {
		BeforeEach(func() {
			t.gatewayDaemonSet.Status.DesiredNumberScheduled = 0
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func() {
			t.awaitStatusCondition(metav1.ConditionTrue, "NoScheduledGateways")

			t.gatewayDaemonSet.Status.DesiredNumberScheduled = 1
			t.updateDaemonSet(t.gatewayDaemonSet)

			t.awaitStatusConditionDeployed()
		})
	})

	When("the route agent daemon set doesn't initially exist", func() {
		BeforeEach(func() {
			t.routeAgentDaemonSet = nil
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func() {
			t.awaitStatusCondition(metav1.ConditionTrue, "NoRouteAgentDaemonSet")

			t.routeAgentDaemonSet = newRouteAgentDaemonSet()
			t.createDaemonSet(t.routeAgentDaemonSet)

			t.awaitStatusConditionDeployed()
		})
	})

	When("a route agent daemon set pod isn't initially available", func() {
		BeforeEach(func() {
			t.routeAgentDaemonSet.Status.NumberUnavailable = 1
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func() {
			t.awaitStatusCondition(metav1.ConditionTrue, "RouteAgentsUnavailable")

			t.routeAgentDaemonSet.Status.NumberUnavailable = 0
			t.updateDaemonSet(t.routeAgentDaemonSet)

			t.awaitStatusConditionDeployed()
		})
	})

	When("updating the ManagedClusterAddOn status initially fails", func() {
		Context("", func() {
			BeforeEach(func() {
				fakereactor.FailOnAction(&t.addOnClient.Fake, "managedclusteraddons", "update", nil, true)
			})

			It("should eventually update it", func() {
				t.awaitStatusConditionDeployed()
			})
		})

		Context("with a conflict error", func() {
			BeforeEach(func() {
				fakereactor.ConflictOnUpdateReactor(&t.addOnClient.Fake, "managedclusteraddons")
			})

			It("should eventually update it", func() {
				t.awaitStatusConditionDeployed()
			})
		})
	})
})

type deploymentControllerTestDriver struct {
	managedClusterAddOnTestBase
	kubeClient          *kubeFake.Clientset
	subscriptionClient  dynamic.ResourceInterface
	subscription        *operatorsv1alpha1.Subscription
	operatorDeployment  *appsv1.Deployment
	gatewayDaemonSet    *appsv1.DaemonSet
	routeAgentDaemonSet *appsv1.DaemonSet
	stop                context.CancelFunc
}

func newDeploymentControllerTestDriver() *deploymentControllerTestDriver {
	t := &deploymentControllerTestDriver{}

	BeforeEach(func() {
		t.kubeClient = kubeFake.NewSimpleClientset()
		t.managedClusterAddOnTestBase.init()

		t.subscription = newSubscription()
		t.operatorDeployment = newOperatorDeployment()
		t.gatewayDaemonSet = newGatewayDaemonSet()
		t.routeAgentDaemonSet = newRouteAgentDaemonSet()
	})

	JustBeforeEach(func() {
		subscriptionClient, dynamicInformerFactory, subscriptionInformer := testing.NewDynamicClientWithInformer(submarinerNS)
		t.subscriptionClient = subscriptionClient

		if t.subscription != nil {
			t.createSubscription()
		}

		if t.operatorDeployment != nil {
			t.createOperatorDeployment()
		}

		if t.gatewayDaemonSet != nil {
			t.createDaemonSet(t.gatewayDaemonSet)
		}

		if t.routeAgentDaemonSet != nil {
			t.createDaemonSet(t.routeAgentDaemonSet)
		}

		kubeInformerFactory := kubeInformers.NewSharedInformerFactory(t.kubeClient, 0)

		t.managedClusterAddOnTestBase.run()

		controller := submarineragent.NewDeploymentStatusController(clusterName, submarinerNS, t.addOnClient,
			kubeInformerFactory.Apps().V1().DaemonSets(), kubeInformerFactory.Apps().V1().Deployments(),
			subscriptionInformer, events.NewLoggingEventRecorder("test"))

		var ctx context.Context

		ctx, t.stop = context.WithCancel(context.TODO())

		kubeInformerFactory.Start(ctx.Done())
		dynamicInformerFactory.Start(ctx.Done())

		cache.WaitForCacheSync(ctx.Done(), kubeInformerFactory.Apps().V1().DaemonSets().Informer().HasSynced,
			kubeInformerFactory.Apps().V1().Deployments().Informer().HasSynced)

		go controller.Run(ctx, 1)
	})

	AfterEach(func() {
		t.stop()
	})

	return t
}

func (t *deploymentControllerTestDriver) createSubscription() {
	_, err := t.subscriptionClient.Create(context.TODO(), test.ToUnstructured(t.subscription), metav1.CreateOptions{})
	Expect(err).To(Succeed())
}

func (t *deploymentControllerTestDriver) updateSubscription() {
	_, err := t.subscriptionClient.Update(context.TODO(), test.ToUnstructured(t.subscription), metav1.UpdateOptions{})
	Expect(err).To(Succeed())
}

func (t *deploymentControllerTestDriver) createOperatorDeployment() {
	_, err := t.kubeClient.AppsV1().Deployments(submarinerNS).Create(context.TODO(), t.operatorDeployment, metav1.CreateOptions{})
	Expect(err).To(Succeed())
}

func (t *deploymentControllerTestDriver) updateOperatorDeployment() {
	_, err := t.kubeClient.AppsV1().Deployments(submarinerNS).Update(context.TODO(), t.operatorDeployment, metav1.UpdateOptions{})
	Expect(err).To(Succeed())
}

func (t *deploymentControllerTestDriver) createDaemonSet(d *appsv1.DaemonSet) {
	_, err := t.kubeClient.AppsV1().DaemonSets(submarinerNS).Create(context.TODO(), d, metav1.CreateOptions{})
	Expect(err).To(Succeed())
}

func (t *deploymentControllerTestDriver) updateDaemonSet(d *appsv1.DaemonSet) {
	_, err := t.kubeClient.AppsV1().DaemonSets(submarinerNS).Update(context.TODO(), d, metav1.UpdateOptions{})
	Expect(err).To(Succeed())
}

func (t *deploymentControllerTestDriver) awaitStatusCondition(status metav1.ConditionStatus, reason string) {
	t.awaitManagedClusterAddOnStatusCondition(&metav1.Condition{
		Type:   deploymentDegradedType,
		Status: status,
		Reason: reason,
	})
}

func (t *deploymentControllerTestDriver) awaitStatusConditionDeployed() {
	t.awaitStatusCondition(metav1.ConditionFalse, "SubmarinerAgentDeployed")
}

func newSubscription() *operatorsv1alpha1.Subscription {
	return &operatorsv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "submariner",
			Namespace: submarinerNS,
		},
		Spec: &operatorsv1alpha1.SubscriptionSpec{},
		Status: operatorsv1alpha1.SubscriptionStatus{
			InstalledCSV: "submariner-csv",
		},
	}
}

func newOperatorDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: submarinerNS,
			Name:      "submariner-operator",
		},
		Status: appsv1.DeploymentStatus{
			AvailableReplicas: 1,
		},
	}
}

func newGatewayDaemonSet() *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: submarinerNS,
			Name:      "submariner-gateway",
		},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 1,
		},
	}
}

func newRouteAgentDaemonSet() *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: submarinerNS,
			Name:      "submariner-routeagent",
		},
	}
}
