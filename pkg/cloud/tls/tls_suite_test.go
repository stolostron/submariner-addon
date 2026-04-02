package tls_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
)

var _ = BeforeSuite(func() {
	Expect(configv1.Install(k8sscheme.Scheme)).To(Succeed())
})

func TestTLS(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "TLS Suite")
}
