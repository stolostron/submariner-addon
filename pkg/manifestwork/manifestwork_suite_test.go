package manifestwork_test

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestManifestwork(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Manifestwork Suite")
}
