package submarineragent_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/operator/events"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stolostron/submariner-addon/pkg/spoke/submarineragent"
	fakereactor "github.com/submariner-io/admiral/pkg/fake"
	"github.com/submariner-io/admiral/pkg/syncer/test"
	submarinerv1alpha1 "github.com/submariner-io/submariner-operator/api/v1alpha1"
	"github.com/submariner-io/submariner-operator/pkg/names"
	"github.com/submariner-io/submariner/pkg/cni"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	kubeInformers "k8s.io/client-go/informers"
	kubeFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
)

const (
	deploymentDegradedType = "SubmarinerAgentDegraded"
)

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
			t.updateDeployment(t.operatorDeployment)

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

	When("the metrics proxy daemon set doesn't initially exist", func() {
		BeforeEach(func() {
			t.metricsProxyDaemonSet = nil
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func() {
			t.awaitStatusCondition(metav1.ConditionTrue, "NoMetricsProxyDaemonSet")

			t.metricsProxyDaemonSet = newMetricsProxyDaemonSet()
			t.createDaemonSet(t.metricsProxyDaemonSet)

			t.awaitStatusConditionDeployed()
		})
	})

	When("a metrics proxy daemon set pod isn't initially available", func() {
		BeforeEach(func() {
			t.metricsProxyDaemonSet.Status.NumberUnavailable = 1
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func() {
			t.awaitStatusCondition(metav1.ConditionTrue, "MetricsProxyUnavailable")

			t.metricsProxyDaemonSet.Status.NumberUnavailable = 0
			t.updateDaemonSet(t.metricsProxyDaemonSet)

			t.awaitStatusConditionDeployed()
		})
	})

	When("no metrics proxy daemon set pod is initially scheduled", func() {
		BeforeEach(func() {
			t.metricsProxyDaemonSet.Status.DesiredNumberScheduled = 0
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func() {
			t.awaitStatusCondition(metav1.ConditionTrue, "NoScheduledMetricsProxy")

			t.metricsProxyDaemonSet.Status.DesiredNumberScheduled = 1
			t.updateDaemonSet(t.metricsProxyDaemonSet)

			t.awaitStatusConditionDeployed()
		})
	})

	When("the lighthouse agent deployment doesn't initially exist", func() {
		BeforeEach(func() {
			t.lighthouseAgentDeployment = nil
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func() {
			t.awaitStatusCondition(metav1.ConditionTrue, "NoLighthouseAgentDeployment")

			t.lighthouseAgentDeployment = newLighthouseAgentDeployment()
			t.createLighthouseAgentDeployment()

			t.awaitStatusConditionDeployed()
		})
	})

	When("no lighthouse agent deployment replica is initially available", func() {
		BeforeEach(func() {
			t.lighthouseAgentDeployment.Status.AvailableReplicas = 0
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func() {
			t.awaitStatusCondition(metav1.ConditionTrue, "NoLighthouseAgentAvailable")

			t.lighthouseAgentDeployment.Status.AvailableReplicas = 1
			t.updateDeployment(t.lighthouseAgentDeployment)

			t.awaitStatusConditionDeployed()
		})
	})

	When("the lighthouse coredns deployment doesn't initially exist", func() {
		BeforeEach(func() {
			t.lighthouseCoreDNSDeployment = nil
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func() {
			t.awaitStatusCondition(metav1.ConditionTrue, "NoLighthouseCoreDNSDeployment")

			t.lighthouseCoreDNSDeployment = newLighthouseCoreDNSDeployment()
			t.createLighthouseCoreDNSDeployment()

			t.awaitStatusConditionDeployed()
		})
	})

	When("no lighthouse coredns deployment replica is initially available", func() {
		BeforeEach(func() {
			t.lighthouseCoreDNSDeployment.Status.AvailableReplicas = 0
		})

		It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func() {
			t.awaitStatusCondition(metav1.ConditionTrue, "NoLighthouseCoreDNSAvailable")

			t.lighthouseCoreDNSDeployment.Status.AvailableReplicas = 1
			t.updateDeployment(t.lighthouseCoreDNSDeployment)

			t.awaitStatusConditionDeployed()
		})
	})

	When("globalnet is enabled", func() {
		BeforeEach(func() {
			t.submariner.Spec.GlobalCIDR = "242.0.0.0/16"
		})
		When("the globalnet daemon set doesn't initially exist", func() {
			It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func() {
				t.awaitStatusCondition(metav1.ConditionTrue, "NoGlobalnetDaemonSet")

				t.globalnetDaemonSet = newGlobalnetDaemonSet()
				t.createDaemonSet(t.globalnetDaemonSet)

				t.awaitStatusConditionDeployed()
			})
		})

		When("a globalnet daemon set pod isn't initially available", func() {
			BeforeEach(func() {
				t.globalnetDaemonSet = newGlobalnetDaemonSet()
				t.globalnetDaemonSet.Status.NumberUnavailable = 1
			})

			It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func() {
				t.awaitStatusCondition(metav1.ConditionTrue, "GlobalnetUnavailable")

				t.globalnetDaemonSet.Status.NumberUnavailable = 0
				t.updateDaemonSet(t.globalnetDaemonSet)

				t.awaitStatusConditionDeployed()
			})
		})

		When("no globalnet daemon set pod is initially scheduled", func() {
			BeforeEach(func() {
				t.globalnetDaemonSet = newGlobalnetDaemonSet()
				t.globalnetDaemonSet.Status.DesiredNumberScheduled = 0
			})

			It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func() {
				t.awaitStatusCondition(metav1.ConditionTrue, "NoScheduledGlobalnet")

				t.globalnetDaemonSet.Status.DesiredNumberScheduled = 1
				t.updateDaemonSet(t.globalnetDaemonSet)

				t.awaitStatusConditionDeployed()
			})
		})
	})

	When("network plugin is OVNKubernetes", func() {
		BeforeEach(func() {
			t.submariner.Status.NetworkPlugin = cni.OVNKubernetes
		})

		When("the network plugin syncer deployment doesn't initially exist", func() {
			It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func() {
				t.awaitStatusCondition(metav1.ConditionTrue, "NoNetworkPluginSyncerDeployment")

				t.networkPluginSyncerDeployment = newNetworkPluginsyncerDeployment()
				t.createNetworkPluginSyncerDeployment()

				t.awaitStatusConditionDeployed()
			})
		})

		When("no network plugin syncer deployment replica is initially available", func() {
			BeforeEach(func() {
				t.networkPluginSyncerDeployment = newNetworkPluginsyncerDeployment()
				t.networkPluginSyncerDeployment.Status.AvailableReplicas = 0
			})

			It("should eventually update the ManagedClusterAddOn status condition from degraded to deployed", func() {
				t.awaitStatusCondition(metav1.ConditionTrue, "NoNetworkPluginSyncerAvailable")

				t.networkPluginSyncerDeployment.Status.AvailableReplicas = 1
				t.updateDeployment(t.networkPluginSyncerDeployment)

				t.awaitStatusConditionDeployed()
			})
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
	kubeClient                    *kubeFake.Clientset
	subscriptionClient            dynamic.ResourceInterface
	submarinerClient              dynamic.ResourceInterface
	subscription                  *operatorsv1alpha1.Subscription
	submariner                    *submarinerv1alpha1.Submariner
	operatorDeployment            *appsv1.Deployment
	gatewayDaemonSet              *appsv1.DaemonSet
	routeAgentDaemonSet           *appsv1.DaemonSet
	metricsProxyDaemonSet         *appsv1.DaemonSet
	lighthouseAgentDeployment     *appsv1.Deployment
	lighthouseCoreDNSDeployment   *appsv1.Deployment
	globalnetDaemonSet            *appsv1.DaemonSet
	networkPluginSyncerDeployment *appsv1.Deployment
	stop                          context.CancelFunc
}

func newDeploymentControllerTestDriver() *deploymentControllerTestDriver {
	t := &deploymentControllerTestDriver{}

	BeforeEach(func() {
		t.kubeClient = kubeFake.NewSimpleClientset()
		t.managedClusterAddOnTestBase.init()

		t.subscription = newSubscription()
		t.submariner = newSubmariner()
		t.operatorDeployment = newOperatorDeployment()
		t.gatewayDaemonSet = newGatewayDaemonSet()
		t.routeAgentDaemonSet = newRouteAgentDaemonSet()
		t.metricsProxyDaemonSet = newMetricsProxyDaemonSet()
		t.lighthouseAgentDeployment = newLighthouseAgentDeployment()
		t.lighthouseCoreDNSDeployment = newLighthouseCoreDNSDeployment()
		t.networkPluginSyncerDeployment = nil
		t.globalnetDaemonSet = nil
	})

	JustBeforeEach(func() {
		subscriptionClient, subscriptionInformerFactory, subscriptionInformer := newDynamicClientWithInformer(submarinerNS)
		t.subscriptionClient = subscriptionClient

		submarinerClient, submarinerInformerFactory, submarinerInformer := newDynamicClientWithInformer(submarinerNS)
		t.submarinerClient = submarinerClient

		if t.subscription != nil {
			t.createSubscription()
		}

		if t.submariner != nil {
			t.createSubmariner()
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

		if t.metricsProxyDaemonSet != nil {
			t.createDaemonSet(t.metricsProxyDaemonSet)
		}

		if t.lighthouseAgentDeployment != nil {
			t.createLighthouseAgentDeployment()
		}

		if t.lighthouseCoreDNSDeployment != nil {
			t.createLighthouseCoreDNSDeployment()
		}

		if t.globalnetDaemonSet != nil {
			t.createDaemonSet(t.globalnetDaemonSet)
		}

		if t.networkPluginSyncerDeployment != nil {
			t.createNetworkPluginSyncerDeployment()
		}

		kubeInformerFactory := kubeInformers.NewSharedInformerFactory(t.kubeClient, 0)

		t.managedClusterAddOnTestBase.run()

		controller := submarineragent.NewDeploymentStatusController(clusterName, submarinerNS, t.addOnClient,
			kubeInformerFactory.Apps().V1().DaemonSets(), kubeInformerFactory.Apps().V1().Deployments(),
			subscriptionInformer, submarinerInformer, events.NewLoggingEventRecorder("test"))

		var ctx context.Context

		ctx, t.stop = context.WithCancel(context.TODO())

		kubeInformerFactory.Start(ctx.Done())
		subscriptionInformerFactory.Start(ctx.Done())
		submarinerInformerFactory.Start(ctx.Done())

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

func (t *deploymentControllerTestDriver) createSubmariner() {
	_, err := t.submarinerClient.Create(context.TODO(), test.ToUnstructured(t.submariner), metav1.CreateOptions{})
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

func (t *deploymentControllerTestDriver) createLighthouseAgentDeployment() {
	_, err := t.kubeClient.AppsV1().Deployments(submarinerNS).Create(context.TODO(), t.lighthouseAgentDeployment, metav1.CreateOptions{})
	Expect(err).To(Succeed())
}

func (t *deploymentControllerTestDriver) createLighthouseCoreDNSDeployment() {
	_, err := t.kubeClient.AppsV1().Deployments(submarinerNS).Create(context.TODO(), t.lighthouseCoreDNSDeployment, metav1.CreateOptions{})
	Expect(err).To(Succeed())
}

func (t *deploymentControllerTestDriver) createNetworkPluginSyncerDeployment() {
	_, err := t.kubeClient.AppsV1().Deployments(submarinerNS).Create(context.TODO(), t.networkPluginSyncerDeployment, metav1.CreateOptions{})
	Expect(err).To(Succeed())
}

func (t *deploymentControllerTestDriver) updateDeployment(deployment *appsv1.Deployment) {
	_, err := t.kubeClient.AppsV1().Deployments(submarinerNS).Update(context.TODO(), deployment, metav1.UpdateOptions{})
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

func newNetworkPluginsyncerDeployment() *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: submarinerNS,
			Name:      names.NetworkPluginSyncerComponent,
		},
		Status: appsv1.DeploymentStatus{
			AvailableReplicas: 1,
		},
	}
}
