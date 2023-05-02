package submarinerbroker_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/submariner-io/admiral/pkg/log/kzerolog"
)

var _ = BeforeSuite(func() {
	kzerolog.InitK8sLogging()
})

func TestSubmarinerbroker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Submariner Broker Suite")
}
