package submarineragent_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers/testing"
	"github.com/open-cluster-management/submariner-addon/pkg/spoke/submarineragent"
	"github.com/openshift/library-go/pkg/operator/events"
	submarinerv1alpha1 "github.com/submariner-io/submariner-operator/api/submariner/v1alpha1"
	submv1 "github.com/submariner-io/submariner/pkg/apis/submariner.io/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
)

const (
	submarinerNS           = "submariner-ns"
	connectionDegradedType = "SubmarinerConnectionDegraded"
)

var _ = Describe("Connections Status Controller", func() {
	t := newConnStatusControllerTestDriver()

	When("all active gateway connections are established", func() {
		It("should update the ManagedClusterAddOn status condition to connections established", func() {
			t.awaitConnectionsEstablishedStatusCondition()
		})

		Context("after initially not established", func() {
			var origGateways *[]submv1.GatewayStatus

			BeforeEach(func() {
				origGateways = t.submariner.Status.Gateways
				t.submariner.Status.Gateways = nil
			})

			It("should transition the ManagedClusterAddOn status condition to connections established", func() {
				t.awaitConnectionsNotEstablishedStatusCondition()

				t.submariner.Status.Gateways = origGateways
				_, err := t.submarinerClient.Update(context.TODO(), testing.ToUnstructured(t.submariner), metav1.UpdateOptions{})
				Expect(err).To(Succeed())

				t.awaitConnectionsEstablishedStatusCondition()
			})
		})
	})

	When("an active gateway connection is in the process of connecting", func() {
		BeforeEach(func() {
			(*t.submariner.Status.Gateways)[0].Connections[0].Status = submv1.Connecting
		})

		It("should update the ManagedClusterAddOn status condition to degraded", func() {
			t.awaitConnectionsDegradedStatusCondition()
		})
	})

	When("an active gateway connection has an error", func() {
		BeforeEach(func() {
			(*t.submariner.Status.Gateways)[0].Connections[0].Status = submv1.ConnectionError
		})

		It("should update the ManagedClusterAddOn status condition as degraded", func() {
			t.awaitConnectionsDegradedStatusCondition()
		})
	})

	When("the gateway status isn't present", func() {
		BeforeEach(func() {
			t.submariner.Status.Gateways = nil
		})

		It("should update the ManagedClusterAddOn status condition to no connections present", func() {
			t.awaitConnectionsNotEstablishedStatusCondition()
		})
	})

	When("there are no gateways", func() {
		BeforeEach(func() {
			t.submariner.Status.Gateways = &[]submv1.GatewayStatus{}
		})

		It("should update the ManagedClusterAddOn status condition to no connections present", func() {
			t.awaitConnectionsNotEstablishedStatusCondition()
		})
	})

	When("there are no active gateway connections", func() {
		BeforeEach(func() {
			(*t.submariner.Status.Gateways)[0].Connections = nil
		})

		It("should update the ManagedClusterAddOn status condition to no connections present", func() {
			t.awaitConnectionsNotEstablishedStatusCondition()
		})
	})

	When("updating the ManagedClusterAddOn status initially fails", func() {
		Context("", func() {
			BeforeEach(func() {
				testing.FailOnAction(&t.addOnClient.Fake, "managedclusteraddons", "update", nil, true)
			})

			It("should eventually update it", func() {
				t.awaitConnectionsEstablishedStatusCondition()
			})
		})

		Context("with a conflict error", func() {
			BeforeEach(func() {
				testing.ConflictOnUpdateReactor(&t.addOnClient.Fake, "managedclusteraddons")
			})

			It("should eventually update it", func() {
				t.awaitConnectionsEstablishedStatusCondition()
			})
		})
	})
})

type connStatusControllerTestDriver struct {
	managedClusterAddOnTestBase
	submariner       *submarinerv1alpha1.Submariner
	submarinerClient dynamic.ResourceInterface
	stop             context.CancelFunc
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

		t.managedClusterAddOnTestBase.init()
	})

	JustBeforeEach(func() {
		submarinerClient, dynamicInformerFactory, submarinerInformer := testing.NewDynamicClientWithInformer(submarinerNS)
		t.submarinerClient = submarinerClient

		_, err := t.submarinerClient.Create(context.TODO(), testing.ToUnstructured(t.submariner), metav1.CreateOptions{})
		Expect(err).To(Succeed())

		t.managedClusterAddOnTestBase.run()

		controller := submarineragent.NewConnectionsStatusController(clusterName, t.addOnClient, submarinerInformer,
			events.NewLoggingEventRecorder("test"))

		var ctx context.Context

		ctx, t.stop = context.WithCancel(context.TODO())

		dynamicInformerFactory.Start(ctx.Done())

		cache.WaitForCacheSync(ctx.Done(), submarinerInformer.Informer().HasSynced)

		go controller.Run(ctx, 1)
	})

	AfterEach(func() {
		t.stop()
	})

	return t
}

func (t *connStatusControllerTestDriver) awaitStatusCondition(status metav1.ConditionStatus, reason string) {
	t.awaitManagedClusterAddOnStatusCondition(&metav1.Condition{
		Type:   connectionDegradedType,
		Status: status,
		Reason: reason,
	})
}

func (t *connStatusControllerTestDriver) awaitConnectionsEstablishedStatusCondition() {
	t.awaitStatusCondition(metav1.ConditionFalse, "ConnectionsEstablished")
}

func (t *connStatusControllerTestDriver) awaitConnectionsNotEstablishedStatusCondition() {
	t.awaitStatusCondition(metav1.ConditionTrue, "ConnectionsNotEstablished")
}

func (t *connStatusControllerTestDriver) awaitConnectionsDegradedStatusCondition() {
	t.awaitStatusCondition(metav1.ConditionTrue, "ConnectionsDegraded")
}
