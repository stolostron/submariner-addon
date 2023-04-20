package submarineragent_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/submariner-io/admiral/pkg/log/kzerolog"
)

var _ = BeforeSuite(func() {
	kzerolog.InitK8sLogging()

	if err := os.Setenv("BROKER_API_SERVER", "127.0.0.1"); err != nil {
		panic(err)
	}
})

var _ = AfterSuite(func() {
	_ = os.Unsetenv("BROKER_API_SERVER")
})

func TestSubmarinerAgent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Submariner Agent Suite")
}
