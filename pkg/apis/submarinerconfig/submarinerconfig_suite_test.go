package submarinerconfig_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestSubmarinerConfig(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SubmarinerConfig Suite")
}
