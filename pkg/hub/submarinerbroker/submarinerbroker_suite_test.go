package submarinerbroker_test

import (
	"flag"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/submariner-io/admiral/pkg/log/kzerolog"
)

var _ = BeforeSuite(func() {
	// set logging verbosity of agent in unit test to DEBUG
	flags := flag.NewFlagSet("kzerolog", flag.ExitOnError)
	kzerolog.AddFlags(flags)
	_ = flags.Parse([]string{"-v=2"})
	kzerolog.InitK8sLogging()
})

func TestSubmarinerbroker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Submariner Broker Suite")
}
