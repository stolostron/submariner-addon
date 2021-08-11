package gcp

import (
	"fmt"
	"reflect"
	"strconv"

	"github.com/open-cluster-management/submariner-addon/pkg/cloud/gcp/client"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"

	"google.golang.org/api/compute/v1"
	googleapi "google.golang.org/api/googleapi"

	"github.com/openshift/library-go/pkg/operator/events"
	operatorhelpers "github.com/openshift/library-go/pkg/operator/v1helpers"
)

const (
	submarinerRuleName        = "submariner"
	submarinerMetricsRuleName = "submariner-metrics"
)

type gcpProvider struct {
	infraId       string
	projectId     string
	ikePort       string
	nattPort      string
	routePort     string
	metricsPort   string
	gcpClient     client.Interface
	eventRecorder events.Recorder
}

func NewGCPProvider(gcpClient client.Interface, eventRecorder events.Recorder, infraId string, ikePort,
	nattPort int) (*gcpProvider, error) {
	if infraId == "" {
		return nil, fmt.Errorf("cluster infraId is empty")
	}

	if ikePort == 0 {
		ikePort = helpers.SubmarinerIKEPort
	}

	if nattPort == 0 {
		nattPort = helpers.SubmarinerNatTPort
	}

	return &gcpProvider{
		infraId:       infraId,
		projectId:     gcpClient.GetProjectID(),
		ikePort:       strconv.Itoa(ikePort),
		nattPort:      strconv.Itoa(nattPort),
		routePort:     strconv.Itoa(helpers.SubmarinerRoutePort),
		metricsPort:   strconv.Itoa(helpers.SubmarinerMetricsPort),
		gcpClient:     gcpClient,
		eventRecorder: eventRecorder,
	}, nil
}

// PrepareSubmarinerClusterEnv prepares submariner cluster environment on GCP
// The below tasks will be executed
// 1. create the inbound and outbound firewall rules for submariner, below ports will be opened
//    - IPsec IKE port (by default 500/UDP)
//    - NAT traversal port (by default 4500/UDP)
//    - 4800/UDP port to encapsulate Pod traffic from worker and master nodes to the Submariner Gateway nodes
// 2. create the inbound and outbound firewall rules to open 8080/TCP port to export metrics service from the Submariner gateway
func (g *gcpProvider) PrepareSubmarinerClusterEnv() error {
	// open IPsec IKE port (by default 500/UDP), NAT traversal port (by default 4500/UDP) and route port (4800/UDP)
	ports := []string{g.ikePort, g.nattPort, g.routePort}
	ingress, egress := newFirewallRules(submarinerRuleName, g.projectId, g.infraId, "udp", ports)
	if err := g.openPorts(ingress, egress); err != nil {
		return fmt.Errorf("failed to open submariner ports: %v", err)
	}

	// open metrics port (8080/TCP)
	metricsIngress, metricsEgress := newFirewallRules(submarinerMetricsRuleName, g.projectId, g.infraId, "tcp", []string{g.metricsPort})
	if err := g.openPorts(metricsIngress, metricsEgress); err != nil {
		return fmt.Errorf("failed to open submariner metrics ports: %v", err)
	}

	g.eventRecorder.Eventf("SubmarinerClusterEnvBuild", "the submariner cluster env is build on gcp")
	return nil
}

// CleanUpSubmarinerClusterEnv clean up submariner cluster environment on GCP after the SubmarinerConfig was deleted
// 1. delete the inbound and outbound firewall rules to close submariner ports
// 2. delete the inbound and outbound firewall rules to close submariner metrics port
func (g *gcpProvider) CleanUpSubmarinerClusterEnv() error {
	var errs []error

	//close submariner ports
	ingressName, egressName := generateRuleNames(g.infraId, submarinerRuleName)
	if err := g.gcpClient.DeleteFirewallRule(ingressName); err != nil {
		errs = append(errs, err)
	}
	if err := g.gcpClient.DeleteFirewallRule(egressName); err != nil {
		errs = append(errs, err)
	}

	//close submariner metrics port
	metricsIngressName, metricsEgressName := generateRuleNames(g.infraId, submarinerMetricsRuleName)
	if err := g.gcpClient.DeleteFirewallRule(metricsIngressName); err != nil {
		errs = append(errs, err)
	}
	if err := g.gcpClient.DeleteFirewallRule(metricsEgressName); err != nil {
		errs = append(errs, err)
	}

	if len(errs) == 0 {
		g.eventRecorder.Eventf("SubmarinerClusterEnvCleanedUp", "the submariner cluster env is cleaned up on gcp")
	}

	return operatorhelpers.NewMultiLineAggregate(errs)
}

func (g *gcpProvider) openPorts(ingress, egress *compute.Firewall) error {
	var errs []error

	for _, rule := range []*compute.Firewall{ingress, egress} {
		current, err := g.gcpClient.GetFirewallRule(rule.Name)
		if isNotFound(err) {
			errs = append(errs, g.gcpClient.InsertFirewallRule(rule))
			continue
		}

		if err != nil {
			errs = append(errs, err)
			continue
		}

		if !isChanged(current, rule) {
			continue
		}

		errs = append(errs, g.gcpClient.UpdateFirewallRule(rule.Name, rule))
	}

	return operatorhelpers.NewMultiLineAggregate(errs)
}

func isNotFound(err error) bool {
	if gerr, ok := err.(*googleapi.Error); ok && gerr.Code == 404 {
		return true
	}
	return false
}

func isChanged(oldRule, newRule *compute.Firewall) bool {
	if len(oldRule.Allowed) != len(newRule.Allowed) {
		return true
	}

	if oldRule.Allowed[0].IPProtocol != newRule.Allowed[0].IPProtocol {
		return true
	}

	return !reflect.DeepEqual(oldRule.Allowed[0].Ports, newRule.Allowed[0].Ports)
}

func newFirewallRules(name, projectID, infraId, protocol string, ports []string) (ingress, egress *compute.Firewall) {
	ingressName, egressName := generateRuleNames(infraId, name)
	return &compute.Firewall{
			Name:      ingressName,
			Network:   fmt.Sprintf("projects/%s/global/networks/%s-network", projectID, infraId),
			Direction: "INGRESS",
			Allowed: []*compute.FirewallAllowed{
				{
					IPProtocol: protocol,
					Ports:      ports,
				},
			},
		},
		&compute.Firewall{
			Name:      egressName,
			Network:   fmt.Sprintf("projects/%s/global/networks/%s-network", projectID, infraId),
			Direction: "EGRESS",
			Allowed: []*compute.FirewallAllowed{
				{
					IPProtocol: protocol,
					Ports:      ports,
				},
			},
		}
}

func generateRuleNames(infraId, name string) (ingressName, egressName string) {
	return fmt.Sprintf("%s-%s-ingress", infraId, name), fmt.Sprintf("%s-%s-egress", infraId, name)
}
