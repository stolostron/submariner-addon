package manifestwork_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestManifestWork(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "ManifestWork Suite")
}
