package gcp_test

import (
	"strconv"
	"testing"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/submariner-addon/pkg/cloud"
	"github.com/open-cluster-management/submariner-addon/pkg/cloud/gcp"
	"github.com/open-cluster-management/submariner-addon/pkg/cloud/gcp/client/mock"
	"github.com/openshift/library-go/pkg/operator/events"
	"google.golang.org/api/compute/v1"
	googleapi "google.golang.org/api/googleapi"
)

const (
	routePort   = "4800"
	metricsPort = "8080"
)

func TestGCP(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "GCP Suite")
}

var _ = Describe("PrepareSubmarinerClusterEnv", func() {
	t := newTestDriver()

	When("the firewall rules don't exist", func() {
		BeforeEach(func() {
			t.gcpClient.EXPECT().GetFirewallRule("test-x595d-submariner-ingress").Return(nil, &googleapi.Error{Code: 404})
			t.gcpClient.EXPECT().GetFirewallRule("test-x595d-submariner-egress").Return(nil, &googleapi.Error{Code: 404})
			t.gcpClient.EXPECT().GetFirewallRule("test-x595d-submariner-metrics-ingress").Return(nil, &googleapi.Error{Code: 404})
			t.gcpClient.EXPECT().GetFirewallRule("test-x595d-submariner-metrics-egress").Return(nil, &googleapi.Error{Code: 404})

			t.gcpClient.EXPECT().InsertFirewallRule(newIngressFirewallRule("test-x595d-submariner", "udp",
				t.ikePort, t.nattPort, routePort)).Return(nil)
			t.gcpClient.EXPECT().InsertFirewallRule(newEgressFirewallRule("test-x595d-submariner", "udp",
				t.ikePort, t.nattPort, routePort)).Return(nil)

			t.gcpClient.EXPECT().InsertFirewallRule(newIngressFirewallRule("test-x595d-submariner-metrics", "tcp",
				metricsPort)).Return(nil)
			t.gcpClient.EXPECT().InsertFirewallRule(newEgressFirewallRule("test-x595d-submariner-metrics", "tcp",
				metricsPort)).Return(nil)
		})

		It("should insert them", func() {
			Expect(t.provider.PrepareSubmarinerClusterEnv()).To(Succeed())
		})
	})

	When("the firewall rules exist", func() {
		BeforeEach(func() {
			t.gcpClient.EXPECT().GetFirewallRule("test-x595d-submariner-ingress").Return(
				newIngressFirewallRule("test-x595d-submariner", "udp", t.ikePort, t.nattPort, routePort), nil)
			t.gcpClient.EXPECT().GetFirewallRule("test-x595d-submariner-egress").Return(
				newEgressFirewallRule("test-x595d-submariner", "udp", t.ikePort, t.nattPort, routePort), nil)
			t.gcpClient.EXPECT().GetFirewallRule("test-x595d-submariner-metrics-ingress").Return(
				newIngressFirewallRule("test-x595d-submariner-metrics", "tcp", metricsPort), nil)
			t.gcpClient.EXPECT().GetFirewallRule("test-x595d-submariner-metrics-egress").Return(
				newEgressFirewallRule("test-x595d-submariner-metrics", "tcp", metricsPort), nil)
		})

		Context("and have not changed", func() {
			It("should not update them", func() {
				Expect(t.provider.PrepareSubmarinerClusterEnv()).To(Succeed())
			})
		})

		Context("and have changed", func() {
			BeforeEach(func() {
				t.ikePort = "501"
				t.nattPort = "4501"

				t.gcpClient.EXPECT().UpdateFirewallRule("test-x595d-submariner-ingress",
					newIngressFirewallRule("test-x595d-submariner", "udp", t.ikePort, t.nattPort, routePort)).Return(nil)
				t.gcpClient.EXPECT().UpdateFirewallRule("test-x595d-submariner-egress",
					newEgressFirewallRule("test-x595d-submariner", "udp", t.ikePort, t.nattPort, routePort)).Return(nil)
			})

			It("should update them", func() {
				Expect(t.provider.PrepareSubmarinerClusterEnv()).To(Succeed())
			})
		})
	})
})

var _ = Describe("CleanUpSubmarinerClusterEnv", func() {
	t := newTestDriver()

	BeforeEach(func() {
		t.gcpClient.EXPECT().DeleteFirewallRule("test-x595d-submariner-ingress").Return(nil)
		t.gcpClient.EXPECT().DeleteFirewallRule("test-x595d-submariner-egress").Return(nil)
		t.gcpClient.EXPECT().DeleteFirewallRule("test-x595d-submariner-metrics-ingress").Return(nil)
		t.gcpClient.EXPECT().DeleteFirewallRule("test-x595d-submariner-metrics-egress").Return(nil)
	})

	It("should delete the firewall rules", func() {
		Expect(t.provider.CleanUpSubmarinerClusterEnv()).To(Succeed())
	})
})

type testDriver struct {
	gcpClient *mock.MockInterface
	mockCtrl  *gomock.Controller
	provider  cloud.CloudProvider
	ikePort   string
	nattPort  string
}

func newTestDriver() *testDriver {
	t := &testDriver{}

	BeforeEach(func() {
		t.mockCtrl = gomock.NewController(GinkgoT())
		t.gcpClient = mock.NewMockInterface(t.mockCtrl)
		t.ikePort = "500"
		t.nattPort = "4500"

		t.gcpClient.EXPECT().GetProjectID().Return("test")
	})

	JustBeforeEach(func() {
		var err error

		t.provider, err = gcp.NewGCPProvider(t.gcpClient, events.NewLoggingEventRecorder("test"), "test-x595d",
			atoi(t.ikePort), atoi(t.nattPort))
		Expect(err).To(Succeed())
	})

	AfterEach(func() {
		t.mockCtrl.Finish()
	})

	return t
}

func newFirewallRule(name, dir, protocol string, ports ...string) *compute.Firewall {
	return &compute.Firewall{
		Name:      name,
		Network:   "projects/test/global/networks/test-x595d-network",
		Direction: dir,
		Allowed: []*compute.FirewallAllowed{
			{
				IPProtocol: protocol,
				Ports:      ports,
			},
		},
	}
}

func newIngressFirewallRule(namePrefix, protocol string, ports ...string) *compute.Firewall {
	return newFirewallRule(namePrefix+"-ingress", "INGRESS", protocol, ports...)
}

func newEgressFirewallRule(namePrefix, protocol string, ports ...string) *compute.Firewall {
	return newFirewallRule(namePrefix+"-egress", "EGRESS", protocol, ports...)
}

func atoi(s string) int {
	i, err := strconv.Atoi(s)
	Expect(err).To(Succeed())

	return i
}
