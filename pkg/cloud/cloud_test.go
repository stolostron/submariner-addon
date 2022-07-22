package cloud_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/operator/events"
	configv1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	"github.com/stolostron/submariner-addon/pkg/cloud"
	"github.com/stolostron/submariner-addon/pkg/constants"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubeFake "k8s.io/client-go/kubernetes/fake"
	workfake "open-cluster-management.io/api/client/work/clientset/versioned/fake"
)

var _ = Describe("ProviderFactory Get", func() {
	var (
		providerFactory    cloud.ProviderFactory
		managedClusterInfo *configv1alpha1.ManagedClusterInfo
	)

	BeforeEach(func() {
		providerFactory = cloud.NewProviderFactory(nil, kubeFake.NewSimpleClientset(), workfake.NewSimpleClientset(),
			dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()), kubeFake.NewSimpleClientset())

		managedClusterInfo = &configv1alpha1.ManagedClusterInfo{
			ClusterName: "east",
			Vendor:      constants.ProductOCP,
			Platform:    "no-provider-available",
			Region:      "test-region",
			InfraID:     "test-infraID",
		}
	})

	When("the ManagedClusterInfo Platform has no provider implementation", func() {
		It("should return false", func() {
			provider, found, err := providerFactory.Get(managedClusterInfo, &configv1alpha1.SubmarinerConfig{},
				events.NewLoggingEventRecorder("test"))
			Expect(err).To(Succeed())
			Expect(found).To(BeFalse())
			Expect(provider).To(BeNil())
		})
	})

	When("the ManagedClusterInfo Vendor is not supported", func() {
		BeforeEach(func() {
			managedClusterInfo.Vendor = "not-supported"
		})

		It("should return an error", func() {
			_, _, err := providerFactory.Get(managedClusterInfo, &configv1alpha1.SubmarinerConfig{},
				events.NewLoggingEventRecorder("test"))
			Expect(err).To(HaveOccurred())
		})
	})
})
