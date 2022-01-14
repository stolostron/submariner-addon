package gcp

import (
	"testing"

	"github.com/golang/mock/gomock"

	googleapi "google.golang.org/api/googleapi"

	"github.com/stolostron/submariner-addon/pkg/cloud/gcp/client/mock"

	"github.com/openshift/library-go/pkg/operator/events/eventstesting"
)

func TestPrepareSubmarinerClusterEnv(t *testing.T) {
	cases := []struct {
		name           string
		ikePort        string
		nattPort       string
		expectInvoking func(*mock.MockInterface)
	}{
		{
			name:     "build submariner env",
			ikePort:  "500",
			nattPort: "4500",
			expectInvoking: func(gcpClient *mock.MockInterface) {
				// get rules
				gcpClient.EXPECT().GetFirewallRule("test-x595d-submariner-ingress").Return(nil, &googleapi.Error{Code: 404})
				gcpClient.EXPECT().GetFirewallRule("test-x595d-submariner-egress").Return(nil, &googleapi.Error{Code: 404})
				gcpClient.EXPECT().GetFirewallRule("test-x595d-submariner-metrics-ingress").Return(nil, &googleapi.Error{Code: 404})
				gcpClient.EXPECT().GetFirewallRule("test-x595d-submariner-metrics-egress").Return(nil, &googleapi.Error{Code: 404})

				// instert rules
				ingress, egress := newFirewallRules(submarinerRuleName, "test", "test-x595d", "udp", []string{"500", "4500", "4800"})
				gcpClient.EXPECT().InsertFirewallRule(ingress).Return(nil)
				gcpClient.EXPECT().InsertFirewallRule(egress).Return(nil)
				mIngress, mEgress := newFirewallRules(submarinerMetricsRuleName, "test", "test-x595d", "tcp", []string{"8080"})
				gcpClient.EXPECT().InsertFirewallRule(mIngress).Return(nil)
				gcpClient.EXPECT().InsertFirewallRule(mEgress).Return(nil)
			},
		},
		{
			name:     "rebuild submariner env - no update",
			ikePort:  "500",
			nattPort: "4500",
			expectInvoking: func(gcpClient *mock.MockInterface) {
				// get rules
				ingress, egress := newFirewallRules(submarinerRuleName, "test", "test-x595d", "udp", []string{"500", "4500", "4800"})
				gcpClient.EXPECT().GetFirewallRule("test-x595d-submariner-ingress").Return(ingress, nil)
				gcpClient.EXPECT().GetFirewallRule("test-x595d-submariner-egress").Return(egress, nil)
				mIngress, mEgress := newFirewallRules(submarinerMetricsRuleName, "test", "test-x595d", "tcp", []string{"8080"})
				gcpClient.EXPECT().GetFirewallRule("test-x595d-submariner-metrics-ingress").Return(mIngress, nil)
				gcpClient.EXPECT().GetFirewallRule("test-x595d-submariner-metrics-egress").Return(mEgress, nil)
			},
		},
		{
			name:     "rebuild submariner env - update",
			ikePort:  "501",
			nattPort: "4501",
			expectInvoking: func(gcpClient *mock.MockInterface) {
				// get rules
				ingress, egress := newFirewallRules(submarinerRuleName, "test", "test-x595d", "udp", []string{"500", "4500", "4800"})
				gcpClient.EXPECT().GetFirewallRule("test-x595d-submariner-ingress").Return(ingress, nil)
				gcpClient.EXPECT().GetFirewallRule("test-x595d-submariner-egress").Return(egress, nil)
				mIngress, mEgress := newFirewallRules(submarinerMetricsRuleName, "test", "test-x595d", "tcp", []string{"8080"})
				gcpClient.EXPECT().GetFirewallRule("test-x595d-submariner-metrics-ingress").Return(mIngress, nil)
				gcpClient.EXPECT().GetFirewallRule("test-x595d-submariner-metrics-egress").Return(mEgress, nil)

				// udpate rules
				newIngress, newEgress := newFirewallRules(submarinerRuleName, "test", "test-x595d", "udp", []string{"501", "4501", "4800"})
				gcpClient.EXPECT().UpdateFirewallRule("test-x595d-submariner-ingress", newIngress).Return(nil)
				gcpClient.EXPECT().UpdateFirewallRule("test-x595d-submariner-egress", newEgress).Return(nil)
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			gcpClient := mock.NewMockInterface(mockCtrl)
			c.expectInvoking(gcpClient)

			gp := &gcpProvider{
				infraId:       "test-x595d",
				projectId:     "test",
				ikePort:       c.ikePort,
				nattPort:      c.nattPort,
				routePort:     "4800",
				metricsPort:   "8080",
				gcpClient:     gcpClient,
				eventRecorder: eventstesting.NewTestingEventRecorder(t),
			}

			err := gp.PrepareSubmarinerClusterEnv()
			if err != nil {
				t.Errorf("expect no err, bug got %v", err)
			}
		})
	}
}

func TestCleanUpSubmarinerClusterEnv(t *testing.T) {
	cases := []struct {
		name           string
		expectInvoking func(*mock.MockInterface)
	}{
		{
			name: "delete submariner env",
			expectInvoking: func(gcpClient *mock.MockInterface) {
				gcpClient.EXPECT().DeleteFirewallRule("test-x595d-submariner-ingress").Return(nil)
				gcpClient.EXPECT().DeleteFirewallRule("test-x595d-submariner-egress").Return(nil)
				gcpClient.EXPECT().DeleteFirewallRule("test-x595d-submariner-metrics-ingress").Return(nil)
				gcpClient.EXPECT().DeleteFirewallRule("test-x595d-submariner-metrics-egress").Return(nil)
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			gcpClient := mock.NewMockInterface(mockCtrl)
			c.expectInvoking(gcpClient)

			gp := &gcpProvider{
				infraId:       "test-x595d",
				projectId:     "test",
				gcpClient:     gcpClient,
				eventRecorder: eventstesting.NewTestingEventRecorder(t),
			}

			err := gp.CleanUpSubmarinerClusterEnv()
			if err != nil {
				t.Errorf("expect no err, bug got %v", err)
			}
		})
	}
}
