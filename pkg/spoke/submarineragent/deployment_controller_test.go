package submarineragent_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/stolostron/submariner-addon/pkg/spoke/submarineragent"
	fakereactor "github.com/submariner-io/admiral/pkg/fake"
	"github.com/submariner-io/admiral/pkg/names"
	"github.com/submariner-io/admiral/pkg/resource"
	"github.com/submariner-io/admiral/pkg/util"
	submarinerv1alpha1 "github.com/submariner-io/submariner-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	kubeInformers "k8s.io/client-go/informers"
	kubeFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/clock"
)

const (
	deploymentDegradedType = "SubmarinerAgentDegraded"
)

var _ = Describe("Deployment Status Controller", func() {
	t := newDeploymentControllerTestDriver()

	When("all components are deployed", func() {
		It("should update the ManagedClusterAddOn status condition to deployed", func(ctx context.Context) {
			t.awaitStatusConditionDeployed(ctx)
		})
	})

	When("the submariner subscription doesn't exist", func() {
		BeforeEach(func() {
			t.subscription = nil
		})

		It("should not update the ManagedClusterAddOn status condition", func(ctx context.Context) {
			t.awaitNoManagedClusterAddOnStatusCondition(ctx, deploymentDegradedType)
		})
	})

	When("the submariner subscription CSV isn't installed", func() {
		BeforeEach(func() {
			util.SetNestedField(t.subscription.Object, "", util.StatusField, "installedCSV")
		})

		It("should eventually update the ManagedClusterAddOn status condition to deployed", func(ctx context.Context) {
			t.awaitStatusCondition(ctx, metav1.ConditionTrue, "CSVNotInstalled")

			util.SetNestedField(t.subscription.Object, "submariner-csv", util.StatusField, "installedCSV")
			t.updateSubscription(ctx)

			t.awaitStatusConditionDeployed(ctx)
		})
	})

	When("the operator deployment doesn't initially exist", func() {
		BeforeEach(func() {
			t.operatorDeployment = nil
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func(ctx context.Context) {
			t.awaitStatusCondition(ctx, metav1.ConditionTrue, "NoOperatorDeployment")

			t.operatorDeployment = newOperatorDeployment()
			t.createOperatorDeployment(ctx)

			t.awaitStatusConditionDeployed(ctx)
		})
	})

	When("no operator deployment replica is initially available", func() {
		BeforeEach(func() {
			t.operatorDeployment.Status.AvailableReplicas = 0
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func(ctx context.Context) {
			t.awaitStatusCondition(ctx, metav1.ConditionTrue, "NoOperatorAvailable")

			t.operatorDeployment.Status.AvailableReplicas = 1
			t.updateDeployment(ctx, t.operatorDeployment)

			t.awaitStatusConditionDeployed(ctx)
		})
	})

	When("the gateway daemon set doesn't initially exist", func() {
		BeforeEach(func() {
			t.gatewayDaemonSet = nil
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func(ctx context.Context) {
			t.awaitStatusCondition(ctx, metav1.ConditionTrue, "NoGatewayDaemonSet")

			t.gatewayDaemonSet = newGatewayDaemonSet()
			t.createDaemonSet(ctx, t.gatewayDaemonSet)

			t.awaitStatusConditionDeployed(ctx)
		})
	})

	When("a gateway daemon set pod isn't initially available", func() {
		BeforeEach(func() {
			t.gatewayDaemonSet.Status.NumberUnavailable = 1
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func(ctx context.Context) {
			t.awaitStatusCondition(ctx, metav1.ConditionTrue, "GatewaysUnavailable")

			t.gatewayDaemonSet.Status.NumberUnavailable = 0
			t.updateDaemonSet(ctx, t.gatewayDaemonSet)

			t.awaitStatusConditionDeployed(ctx)
		})
	})

	When("no gateway daemon set pod is initially scheduled", func() {
		BeforeEach(func() {
			t.gatewayDaemonSet.Status.DesiredNumberScheduled = 0
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func(ctx context.Context) {
			t.awaitStatusCondition(ctx, metav1.ConditionTrue, "NoScheduledGateways")

			t.gatewayDaemonSet.Status.DesiredNumberScheduled = 1
			t.updateDaemonSet(ctx, t.gatewayDaemonSet)

			t.awaitStatusConditionDeployed(ctx)
		})
	})

	When("the route agent daemon set doesn't initially exist", func() {
		BeforeEach(func() {
			t.routeAgentDaemonSet = nil
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func(ctx context.Context) {
			t.awaitStatusCondition(ctx, metav1.ConditionTrue, "NoRouteAgentDaemonSet")

			t.routeAgentDaemonSet = newRouteAgentDaemonSet()
			t.createDaemonSet(ctx, t.routeAgentDaemonSet)

			t.awaitStatusConditionDeployed(ctx)
		})
	})

	When("a route agent daemon set pod isn't initially available", func() {
		BeforeEach(func() {
			t.routeAgentDaemonSet.Status.NumberUnavailable = 1
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func(ctx context.Context) {
			t.awaitStatusCondition(ctx, metav1.ConditionTrue, "RouteAgentsUnavailable")

			t.routeAgentDaemonSet.Status.NumberUnavailable = 0
			t.updateDaemonSet(ctx, t.routeAgentDaemonSet)

			t.awaitStatusConditionDeployed(ctx)
		})
	})

	When("the metrics proxy daemon set doesn't initially exist", func() {
		BeforeEach(func() {
			t.metricsProxyDaemonSet = nil
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func(ctx context.Context) {
			t.awaitStatusCondition(ctx, metav1.ConditionTrue, "NoMetricsProxyDaemonSet")

			t.metricsProxyDaemonSet = newMetricsProxyDaemonSet()
			t.createDaemonSet(ctx, t.metricsProxyDaemonSet)

			t.awaitStatusConditionDeployed(ctx)
		})
	})

	When("a metrics proxy daemon set pod isn't initially available", func() {
		BeforeEach(func() {
			t.metricsProxyDaemonSet.Status.NumberUnavailable = 1
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func(ctx context.Context) {
			t.awaitStatusCondition(ctx, metav1.ConditionTrue, "MetricsProxyUnavailable")

			t.metricsProxyDaemonSet.Status.NumberUnavailable = 0
			t.updateDaemonSet(ctx, t.metricsProxyDaemonSet)

			t.awaitStatusConditionDeployed(ctx)
		})
	})

	When("no metrics proxy daemon set pod is initially scheduled", func() {
		BeforeEach(func() {
			t.metricsProxyDaemonSet.Status.DesiredNumberScheduled = 0
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func(ctx context.Context) {
			t.awaitStatusCondition(ctx, metav1.ConditionTrue, "NoScheduledMetricsProxy")

			t.metricsProxyDaemonSet.Status.DesiredNumberScheduled = 1
			t.updateDaemonSet(ctx, t.metricsProxyDaemonSet)

			t.awaitStatusConditionDeployed(ctx)
		})
	})

	When("the lighthouse agent deployment doesn't initially exist", func() {
		BeforeEach(func() {
			t.lighthouseAgentDeployment = nil
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func(ctx context.Context) {
			t.awaitStatusCondition(ctx, metav1.ConditionTrue, "NoLighthouseAgentDeployment")

			t.lighthouseAgentDeployment = newLighthouseAgentDeployment()
			t.createLighthouseAgentDeployment(ctx)

			t.awaitStatusConditionDeployed(ctx)
		})
	})

	When("no lighthouse agent deployment replica is initially available", func() {
		BeforeEach(func() {
			t.lighthouseAgentDeployment.Status.AvailableReplicas = 0
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func(ctx context.Context) {
			t.awaitStatusCondition(ctx, metav1.ConditionTrue, "NoLighthouseAgentAvailable")

			t.lighthouseAgentDeployment.Status.AvailableReplicas = 1
			t.updateDeployment(ctx, t.lighthouseAgentDeployment)

			t.awaitStatusConditionDeployed(ctx)
		})
	})

	When("the lighthouse coredns deployment doesn't initially exist", func() {
		BeforeEach(func() {
			t.lighthouseCoreDNSDeployment = nil
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func(ctx context.Context) {
			t.awaitStatusCondition(ctx, metav1.ConditionTrue, "NoLighthouseCoreDNSDeployment")

			t.lighthouseCoreDNSDeployment = newLighthouseCoreDNSDeployment()
			t.createLighthouseCoreDNSDeployment(ctx)

			t.awaitStatusConditionDeployed(ctx)
		})
	})

	When("no lighthouse coredns deployment replica is initially available", func() {
		BeforeEach(func() {
			t.lighthouseCoreDNSDeployment.Status.AvailableReplicas = 0
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func(ctx context.Context) {
			t.awaitStatusCondition(ctx, metav1.ConditionTrue, "NoLighthouseCoreDNSAvailable")

			t.lighthouseCoreDNSDeployment.Status.AvailableReplicas = 1
			t.updateDeployment(ctx, t.lighthouseCoreDNSDeployment)

			t.awaitStatusConditionDeployed(ctx)
		})
	})

	When("globalnet is enabled", func() {
		BeforeEach(func() {
			t.submariner.Spec.GlobalCIDR = "242.0.0.0/16"
		})
		When("the globalnet daemon set doesn't initially exist", func() {
			It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func(ctx context.Context) {
				t.awaitStatusCondition(ctx, metav1.ConditionTrue, "NoGlobalnetDaemonSet")

				t.globalnetDaemonSet = newGlobalnetDaemonSet()
				t.createDaemonSet(ctx, t.globalnetDaemonSet)

				t.awaitStatusConditionDeployed(ctx)
			})
		})

		When("a globalnet daemon set pod isn't initially available", func() {
			BeforeEach(func() {
				t.globalnetDaemonSet = newGlobalnetDaemonSet()
				t.globalnetDaemonSet.Status.NumberUnavailable = 1
			})

			It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func(ctx context.Context) {
				t.awaitStatusCondition(ctx, metav1.ConditionTrue, "GlobalnetUnavailable")

				t.globalnetDaemonSet.Status.NumberUnavailable = 0
				t.updateDaemonSet(ctx, t.globalnetDaemonSet)

				t.awaitStatusConditionDeployed(ctx)
			})
		})

		When("no globalnet daemon set pod is initially scheduled", func() {
			BeforeEach(func() {
				t.globalnetDaemonSet = newGlobalnetDaemonSet()
				t.globalnetDaemonSet.Status.DesiredNumberScheduled = 0
			})

			It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func(ctx context.Context) {
				t.awaitStatusCondition(ctx, metav1.ConditionTrue, "NoScheduledGlobalnet")

				t.globalnetDaemonSet.Status.DesiredNumberScheduled = 1
				t.updateDaemonSet(ctx, t.globalnetDaemonSet)

				t.awaitStatusConditionDeployed(ctx)
			})
		})
	})

	When("updating the ManagedClusterAddOn status initially fails", func() {
		Context("", func() {
			BeforeEach(func() {
				fakereactor.FailOnAction(&t.addOnClient.Fake, "managedclusteraddons", "update", nil, true)
			})

			It("should eventually update it", func(ctx context.Context) {
				t.awaitStatusConditionDeployed(ctx)
			})
		})

		Context("with a conflict error", func() {
			BeforeEach(func() {
				fakereactor.ConflictOnUpdateReactor(&t.addOnClient.Fake, "managedclusteraddons")
			})

			It("should eventually update it", func(ctx context.Context) {
				t.awaitStatusConditionDeployed(ctx)
			})
		})
	})
})

type deploymentControllerTestDriver struct {
	managedClusterAddOnTestBase
	kubeClient                  *kubeFake.Clientset
	subscriptionClient          dynamic.ResourceInterface
	submarinerClient            dynamic.ResourceInterface
	subscription                *unstructured.Unstructured
	submariner                  *submarinerv1alpha1.Submariner
	operatorDeployment          *appsv1.Deployment
	gatewayDaemonSet            *appsv1.DaemonSet
	routeAgentDaemonSet         *appsv1.DaemonSet
	metricsProxyDaemonSet       *appsv1.DaemonSet
	lighthouseAgentDeployment   *appsv1.Deployment
	lighthouseCoreDNSDeployment *appsv1.Deployment
	globalnetDaemonSet          *appsv1.DaemonSet
}

func newDeploymentControllerTestDriver() *deploymentControllerTestDriver {
	t := &deploymentControllerTestDriver{}

	BeforeEach(func() {
		t.kubeClient = kubeFake.NewClientset()
		t.managedClusterAddOnTestBase.init()

		t.subscription = newSubscription()
		t.submariner = newSubmariner()
		t.operatorDeployment = newOperatorDeployment()
		t.gatewayDaemonSet = newGatewayDaemonSet()
		t.routeAgentDaemonSet = newRouteAgentDaemonSet()
		t.metricsProxyDaemonSet = newMetricsProxyDaemonSet()
		t.lighthouseAgentDeployment = newLighthouseAgentDeployment()
		t.lighthouseCoreDNSDeployment = newLighthouseCoreDNSDeployment()
		t.globalnetDaemonSet = nil
	})

	JustBeforeEach(func(ctx context.Context) {
		subscriptionClient, subscriptionInformerFactory, subscriptionInformer := newDynamicClientWithInformer(submarinerNS)
		t.subscriptionClient = subscriptionClient

		submarinerClient, submarinerInformerFactory, submarinerInformer := newDynamicClientWithInformer(submarinerNS)
		t.submarinerClient = submarinerClient

		if t.subscription != nil {
			t.createSubscription(ctx)
		}

		if t.submariner != nil {
			t.createSubmariner(ctx)
		}

		if t.operatorDeployment != nil {
			t.createOperatorDeployment(ctx)
		}

		if t.gatewayDaemonSet != nil {
			t.createDaemonSet(ctx, t.gatewayDaemonSet)
		}

		if t.routeAgentDaemonSet != nil {
			t.createDaemonSet(ctx, t.routeAgentDaemonSet)
		}

		if t.metricsProxyDaemonSet != nil {
			t.createDaemonSet(ctx, t.metricsProxyDaemonSet)
		}

		if t.lighthouseAgentDeployment != nil {
			t.createLighthouseAgentDeployment(ctx)
		}

		if t.lighthouseCoreDNSDeployment != nil {
			t.createLighthouseCoreDNSDeployment(ctx)
		}

		if t.globalnetDaemonSet != nil {
			t.createDaemonSet(ctx, t.globalnetDaemonSet)
		}

		kubeInformerFactory := kubeInformers.NewSharedInformerFactory(t.kubeClient, 0)

		t.managedClusterAddOnTestBase.run(ctx)

		controller := submarineragent.NewDeploymentStatusController(clusterName, submarinerNS, t.addOnClient,
			kubeInformerFactory.Apps().V1().DaemonSets(), kubeInformerFactory.Apps().V1().Deployments(),
			subscriptionInformer, submarinerInformer, events.NewLoggingEventRecorder("test", clock.RealClock{}))

		controllerCtx, stop := context.WithCancel(context.TODO())

		DeferCleanup(func() { stop() })

		kubeInformerFactory.Start(controllerCtx.Done())
		subscriptionInformerFactory.Start(controllerCtx.Done())
		submarinerInformerFactory.Start(controllerCtx.Done())

		cache.WaitForCacheSync(controllerCtx.Done(), kubeInformerFactory.Apps().V1().DaemonSets().Informer().HasSynced,
			kubeInformerFactory.Apps().V1().Deployments().Informer().HasSynced)

		//nolint:contextcheck // Need context.TODO() for long-running controller; passed ctx is request-scoped
		go controller.Run(controllerCtx, 1)
	})

	return t
}

func (t *deploymentControllerTestDriver) createSubscription(ctx context.Context) {
	_, err := t.subscriptionClient.Create(ctx, t.subscription.DeepCopy(), metav1.CreateOptions{})
	Expect(err).To(Succeed())
}

func (t *deploymentControllerTestDriver) createSubmariner(ctx context.Context) {
	_, err := t.submarinerClient.Create(ctx, resource.MustToUnstructured(t.submariner), metav1.CreateOptions{})
	Expect(err).To(Succeed())
}

func (t *deploymentControllerTestDriver) updateSubscription(ctx context.Context) {
	_, err := t.subscriptionClient.Update(ctx, t.subscription.DeepCopy(), metav1.UpdateOptions{})
	Expect(err).To(Succeed())
}

func (t *deploymentControllerTestDriver) createOperatorDeployment(ctx context.Context) {
	_, err := t.kubeClient.AppsV1().Deployments(submarinerNS).Create(ctx, t.operatorDeployment, metav1.CreateOptions{})
	Expect(err).To(Succeed())
}

func (t *deploymentControllerTestDriver) createLighthouseAgentDeployment(ctx context.Context) {
	_, err := t.kubeClient.AppsV1().Deployments(submarinerNS).Create(ctx, t.lighthouseAgentDeployment, metav1.CreateOptions{})
	Expect(err).To(Succeed())
}

func (t *deploymentControllerTestDriver) createLighthouseCoreDNSDeployment(ctx context.Context) {
	_, err := t.kubeClient.AppsV1().Deployments(submarinerNS).Create(ctx, t.lighthouseCoreDNSDeployment, metav1.CreateOptions{})
	Expect(err).To(Succeed())
}

func (t *deploymentControllerTestDriver) updateDeployment(ctx context.Context, deployment *appsv1.Deployment) {
	_, err := t.kubeClient.AppsV1().Deployments(submarinerNS).Update(ctx, deployment, metav1.UpdateOptions{})
	Expect(err).To(Succeed())
}

func (t *deploymentControllerTestDriver) createDaemonSet(ctx context.Context, d *appsv1.DaemonSet) {
	_, err := t.kubeClient.AppsV1().DaemonSets(submarinerNS).Create(ctx, d, metav1.CreateOptions{})
	Expect(err).To(Succeed())
}

func (t *deploymentControllerTestDriver) updateDaemonSet(ctx context.Context, d *appsv1.DaemonSet) {
	_, err := t.kubeClient.AppsV1().DaemonSets(submarinerNS).Update(ctx, d, metav1.UpdateOptions{})
	Expect(err).To(Succeed())
}

func (t *deploymentControllerTestDriver) awaitStatusCondition(ctx context.Context, status metav1.ConditionStatus, reason string) {
	t.awaitManagedClusterAddOnStatusCondition(ctx, &metav1.Condition{
		Type:   deploymentDegradedType,
		Status: status,
		Reason: reason,
	})
}

func (t *deploymentControllerTestDriver) awaitStatusConditionDeployed(ctx context.Context) {
	t.awaitStatusCondition(ctx, metav1.ConditionFalse, "SubmarinerAgentDeployed")
}

func newSubscription() *unstructured.Unstructured {
	sub := &unstructured.Unstructured{}
	sub.SetName("submariner")
	util.SetNestedField(sub.Object, "submariner-csv", util.StatusField, "installedCSV")

	return sub
}

func newSubmariner() *submarinerv1alpha1.Submariner {
	return &submarinerv1alpha1.Submariner{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: submarinerNS,
		},
		Spec: submarinerv1alpha1.SubmarinerSpec{
			GlobalCIDR: "",
		},
		Status: submarinerv1alpha1.SubmarinerStatus{
			NetworkPlugin: "OpenShiftSDN",
		},
	}
}

func newOperatorDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: submarinerNS,
			Name:      names.OperatorComponent,
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
			Name:      names.GatewayComponent,
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
			Name:      names.RouteAgentComponent,
		},
	}
}

func newMetricsProxyDaemonSet() *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: submarinerNS,
			Name:      names.MetricsProxyComponent,
		},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 1,
		},
	}
}

func newLighthouseAgentDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: submarinerNS,
			Name:      names.ServiceDiscoveryComponent,
		},
		Status: appsv1.DeploymentStatus{
			AvailableReplicas: 1,
		},
	}
}

func newLighthouseCoreDNSDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: submarinerNS,
			Name:      names.LighthouseCoreDNSComponent,
		},
		Status: appsv1.DeploymentStatus{
			AvailableReplicas: 1,
		},
	}
}

func newGlobalnetDaemonSet() *appsv1.DaemonSet {
	return &appsv1.DaemonSet{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: submarinerNS,
			Name:      names.GlobalnetComponent,
		},
		Status: appsv1.DaemonSetStatus{
			DesiredNumberScheduled: 1,
		},
	}
}
