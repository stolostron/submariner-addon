package manifestwork_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stolostron/submariner-addon/pkg/cloud/manifestwork"
)

var _ = Describe("MachineSet functions", func() {
	testMsd := manifestwork.NewMachineSetDeployer(nil, "test", "cluster", nil)

	When("calling List", func() {
		It("should panic", func() {
			Expect(func() { _, _ = testMsd.List() }).To(Panic())
		})
	})

	When("calling GetWorkerNodeImage", func() {
		It("should succeed", func() {
			_, err := testMsd.GetWorkerNodeImage(nil, "infra")
			Expect(err).To(Succeed())
		})
	})
})
