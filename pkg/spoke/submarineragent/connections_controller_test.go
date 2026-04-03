package submarineragent_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/stolostron/submariner-addon/pkg/spoke/submarineragent"
	fakereactor "github.com/submariner-io/admiral/pkg/fake"
	"github.com/submariner-io/admiral/pkg/resource"
	submarinerv1alpha1 "github.com/submariner-io/submariner-operator/api/v1alpha1"
	submv1 "github.com/submariner-io/submariner/pkg/apis/submariner.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/utils/clock"
)

const (
	submarinerNS                 = "submariner-ns"
	connectionDegradedType       = "SubmarinerConnectionDegraded"
	routeAgentConnectionDegraded = "RouteAgentConnectionDegraded"
)

var _ = Describe("Connections Status Controller", func() {
	t := newConnStatusControllerTestDriver()

	When("all active gateway connections are established", func() {
		It("should update the ManagedClusterAddOn status condition to connections established", func(ctx context.Context) {
			t.awaitConnectionsEstablishedStatusCondition(ctx)
		})

		Context("after initially not established", func() {
			var origGateways *[]submv1.GatewayStatus

			BeforeEach(func() {
				origGateways = t.submariner.Status.Gateways
				t.submariner.Status.Gateways = nil
			})

			It("should transition the ManagedClusterAddOn status condition to connections established", func(ctx context.Context) {
				t.awaitConnectionsNotEstablishedStatusCondition(ctx)

				t.submariner.Status.Gateways = origGateways
				_, err := t.submarinerClient.Update(ctx, resource.MustToUnstructured(t.submariner), metav1.UpdateOptions{})
				Expect(err).To(Succeed())

				t.awaitConnectionsEstablishedStatusCondition(ctx)
			})
		})
	})

	When("an active gateway connection is in the process of connecting", func() {
		BeforeEach(func() {
			(*t.submariner.Status.Gateways)[0].Connections[0].Status = submv1.Connecting
		})

		It("should update the ManagedClusterAddOn status condition to degraded", func(ctx context.Context) {
			t.awaitConnectionsDegradedStatusCondition(ctx)
		})
	})

	When("an active gateway connection has an error", func() {
		BeforeEach(func() {
			(*t.submariner.Status.Gateways)[0].Connections[0].Status = submv1.ConnectionError
		})

		It("should update the ManagedClusterAddOn status condition as degraded", func(ctx context.Context) {
			t.awaitConnectionsDegradedStatusCondition(ctx)
		})
	})

	When("the gateway status isn't present", func() {
		BeforeEach(func() {
			t.submariner.Status.Gateways = nil
		})

		It("should update the ManagedClusterAddOn status condition to no connections present", func(ctx context.Context) {
			t.awaitConnectionsNotEstablishedStatusCondition(ctx)
		})
	})

	When("there are no gateways", func() {
		BeforeEach(func() {
			t.submariner.Status.Gateways = &[]submv1.GatewayStatus{}
		})

		It("should update the ManagedClusterAddOn status condition to no connections present", func(ctx context.Context) {
			t.awaitConnectionsNotEstablishedStatusCondition(ctx)
		})
	})

	When("there are no active gateway connections", func() {
		BeforeEach(func() {
			(*t.submariner.Status.Gateways)[0].Connections = nil
		})

		It("should update the ManagedClusterAddOn status condition to no connections present", func(ctx context.Context) {
			t.awaitConnectionsNotEstablishedStatusCondition(ctx)
		})
	})

	When("updating the ManagedClusterAddOn status initially fails", func() {
		Context("", func() {
			BeforeEach(func() {
				fakereactor.FailOnAction(&t.addOnClient.Fake, "managedclusteraddons", "update", nil, true)
			})

			It("should eventually update it", func(ctx context.Context) {
				t.awaitConnectionsEstablishedStatusCondition(ctx)
			})
		})

		Context("with a conflict error", func() {
			BeforeEach(func() {
				fakereactor.ConflictOnUpdateReactor(&t.addOnClient.Fake, "managedclusteraddons")
			})

			It("should eventually update it", func(ctx context.Context) {
				t.awaitConnectionsEstablishedStatusCondition(ctx)
			})
		})
	})

	When("when all RouteAgents have healthy connections", func() {
		It("should update the ManagedClusterAddOn status condition to connections established", func(ctx context.Context) {
			t.awaitRouteAgentsEstablishedStatusCondition(ctx)
		})
	})

	When("a RouteAgent's remote endpoint has a Connecting status", func() {
		BeforeEach(func() {
			t.routeAgents[0].Status.RemoteEndpoints[0].Status = submv1.Connecting
		})

		It("should update the ManagedClusterAddOn status condition to degraded", func(ctx context.Context) {
			t.awaitRouteAgentsDegradedStatusCondition(ctx)
		})
	})

	When("a RouteAgent's remote endpoint has a ConnectionNone status", func() {
		BeforeEach(func() {
			t.routeAgents[0].Status.RemoteEndpoints[0].Status = submv1.ConnectionNone
		})

		It("should update the ManagedClusterAddOn status condition to connections established", func(ctx context.Context) {
			t.awaitRouteAgentsEstablishedStatusCondition(ctx)
		})
	})

	When("a RouteAgent's remote endpoint has a ConnectionError status", func() {
		BeforeEach(func() {
			t.routeAgents[0].Status.RemoteEndpoints[0].Status = submv1.ConnectionError
		})

		It("should update the ManagedClusterAddOn status condition to degraded", func(ctx context.Context) {
			t.awaitRouteAgentsDegradedStatusCondition(ctx)
		})
	})
})

type connStatusControllerTestDriver struct {
	managedClusterAddOnTestBase
	submariner       *submarinerv1alpha1.Submariner
	routeAgents      []*submv1.RouteAgent
	submarinerClient dynamic.ResourceInterface
}

func newConnStatusControllerTestDriver() *connStatusControllerTestDriver {
	t := &connStatusControllerTestDriver{}

	BeforeEach(func() {
		t.submariner = &submarinerv1alpha1.Submariner{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "submariner",
				Namespace: submarinerNS,
			},
			Status: submarinerv1alpha1.SubmarinerStatus{
				Gateways: &[]submv1.GatewayStatus{
					{
						HAStatus: submv1.HAStatusActive,
						Connections: []submv1.Connection{
							{
								Status: submv1.Connected,
								Endpoint: submv1.EndpointSpec{
									ClusterID: "cluster1",
								},
							},
							{
								Status: submv1.Connected,
								Endpoint: submv1.EndpointSpec{
									ClusterID: "cluster2",
								},
							},
						},
					},
					{
						HAStatus: submv1.HAStatusPassive,
						Connections: []submv1.Connection{
							{
								Status: submv1.ConnectionError,
								Endpoint: submv1.EndpointSpec{
									ClusterID: "cluster1",
								},
							},
						},
					},
				},
			},
		}

		t.routeAgents = []*submv1.RouteAgent{
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "routeagent1",
					Namespace: submarinerNS,
				},
				Status: submv1.RouteAgentStatus{
					RemoteEndpoints: []submv1.RemoteEndpoint{
						{
							Status:        submv1.Connected,
							Spec:          submv1.EndpointSpec{Hostname: "remote1"},
							StatusMessage: "Success",
						},
						{
							Status:        submv1.Connected,
							Spec:          submv1.EndpointSpec{Hostname: "remote2"},
							StatusMessage: "Success",
						},
					},
				},
			},
			{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "routeagent2",
					Namespace: submarinerNS,
				},
				Status: submv1.RouteAgentStatus{
					RemoteEndpoints: []submv1.RemoteEndpoint{
						{
							Status:        submv1.Connected,
							Spec:          submv1.EndpointSpec{Hostname: "remote3"},
							StatusMessage: "Success3",
						},
					},
				},
			},
		}

		t.managedClusterAddOnTestBase.init()
	})

	JustBeforeEach(func(ctx context.Context) {
		submarinerClient, submarinerInformerFactory, submarinerInformer := newDynamicClientWithInformer(submarinerNS)
		t.submarinerClient = submarinerClient

		routeAgentClient, routeAgentInformerFactory, routeAgentInformer := newDynamicClientWithInformer(submarinerNS)

		_, err := t.submarinerClient.Create(ctx, resource.MustToUnstructured(t.submariner), metav1.CreateOptions{})
		Expect(err).To(Succeed())

		for i := range t.routeAgents {
			_, err := routeAgentClient.Create(ctx, resource.MustToUnstructured(t.routeAgents[i]), metav1.CreateOptions{})
			Expect(err).To(Succeed())
		}

		t.managedClusterAddOnTestBase.run(ctx)

		controller := submarineragent.NewConnectionsStatusController(clusterName, t.addOnClient, submarinerInformer,
			routeAgentInformer, events.NewLoggingEventRecorder("test", clock.RealClock{}))

		controllerCtx, stop := context.WithCancel(context.TODO())

		DeferCleanup(func() { stop() })

		submarinerInformerFactory.Start(controllerCtx.Done())
		routeAgentInformerFactory.Start(controllerCtx.Done())

		cache.WaitForCacheSync(controllerCtx.Done(), submarinerInformer.Informer().HasSynced, routeAgentInformer.Informer().HasSynced)

		//nolint:contextcheck // Need context.TODO() for long-running controller; passed ctx is request-scoped
		go controller.Run(controllerCtx, 1)
	})

	return t
}

func (t *connStatusControllerTestDriver) awaitStatusCondition(ctx context.Context, status metav1.ConditionStatus, reason string) {
	t.awaitManagedClusterAddOnStatusCondition(ctx, &metav1.Condition{
		Type:   connectionDegradedType,
		Status: status,
		Reason: reason,
	})
}

func (t *connStatusControllerTestDriver) awaitConnectionsEstablishedStatusCondition(ctx context.Context) {
	t.awaitStatusCondition(ctx, metav1.ConditionFalse, "ConnectionsEstablished")
}

func (t *connStatusControllerTestDriver) awaitConnectionsNotEstablishedStatusCondition(ctx context.Context) {
	t.awaitStatusCondition(ctx, metav1.ConditionTrue, "ConnectionsNotEstablished")
}

func (t *connStatusControllerTestDriver) awaitConnectionsDegradedStatusCondition(ctx context.Context) {
	t.awaitStatusCondition(ctx, metav1.ConditionTrue, "ConnectionsDegraded")
}

func (t *connStatusControllerTestDriver) awaitRouteAgentsDegradedStatusCondition(ctx context.Context) {
	t.awaitRouteAgentStatusCondition(ctx, metav1.ConditionTrue, "ConnectionsDegraded")
}

func (t *connStatusControllerTestDriver) awaitRouteAgentsEstablishedStatusCondition(ctx context.Context) {
	t.awaitRouteAgentStatusCondition(ctx, metav1.ConditionFalse, "ConnectionsEstablished")
}

func (t *connStatusControllerTestDriver) awaitRouteAgentStatusCondition(ctx context.Context, status metav1.ConditionStatus, reason string) {
	t.awaitManagedClusterAddOnStatusCondition(ctx, &metav1.Condition{
		Type:   routeAgentConnectionDegraded,
		Status: status,
		Reason: reason,
	})
}
