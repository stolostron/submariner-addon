package submarinerbroker_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSubmarinerbroker(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Submariner Broker Suite")
}
