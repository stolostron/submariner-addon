package manifestwork_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestManifestwork(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Manifestwork Suite")
}
