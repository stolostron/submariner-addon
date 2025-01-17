package cloud_test

import (
	"context"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/operator/events"
	configv1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	"github.com/stolostron/submariner-addon/pkg/cloud"
	"github.com/stolostron/submariner-addon/pkg/cloud/fake"
	"github.com/stolostron/submariner-addon/pkg/cloud/provider"
	"github.com/stolostron/submariner-addon/pkg/constants"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	kubeFake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/utils/clock"
)

var _ = Describe("ProviderFactory Get", func() {
	clusterName := "east"

	var (
		providerFactory  cloud.ProviderFactory
		submarinerConfig *configv1alpha1.SubmarinerConfig
		hubKubeClient    kubernetes.Interface
	)

	BeforeEach(func() {
		hubKubeClient = kubeFake.NewSimpleClientset()

		providerFactory = cloud.NewProviderFactory(nil, kubeFake.NewSimpleClientset(),
			dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()), hubKubeClient)

		submarinerConfig = &configv1alpha1.SubmarinerConfig{
			Status: configv1alpha1.SubmarinerConfigStatus{
				ManagedClusterInfo: configv1alpha1.ManagedClusterInfo{
					ClusterName: clusterName,
					Vendor:      constants.ProductOCP,
					Platform:    "no-provider-available",
					Region:      "test-region",
					InfraID:     "test-infraID",
				},
			},
		}
	})

	When("the ManagedClusterInfo Platform has no provider implementation", func() {
		It("should return false", func() {
			provider, found, err := providerFactory.Get(submarinerConfig, events.NewLoggingEventRecorder("test", clock.RealClock{}))
			Expect(err).To(Succeed())
			Expect(found).To(BeFalse())
			Expect(provider).To(BeNil())
		})
	})

	When("the ManagedClusterInfo Vendor is not supported", func() {
		BeforeEach(func() {
			submarinerConfig.Status.ManagedClusterInfo.Vendor = "not-supported"
		})

		It("should return an error", func() {
			_, _, err := providerFactory.Get(submarinerConfig, events.NewLoggingEventRecorder("test", clock.RealClock{}))
			Expect(err).To(HaveOccurred())
		})
	})

	When("the ManagedClusterInfo Vendor is ROSA", func() {
		BeforeEach(func() {
			submarinerConfig.Status.ManagedClusterInfo.Vendor = constants.ProductROSA
		})

		It("should return false", func() {
			provider, found, err := providerFactory.Get(submarinerConfig, events.NewLoggingEventRecorder("test", clock.RealClock{}))
			Expect(err).To(Succeed())
			Expect(found).To(BeFalse())
			Expect(provider).To(BeNil())
		})
	})

	When("a provider implementation is registered", func() {
		mockProvider := &fake.MockProvider{}
		credentialsSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-secret",
				Namespace: clusterName,
			},
		}

		BeforeEach(func() {
			submarinerConfig.Status.ManagedClusterInfo.Platform = "FOO"
			cloud.RegisterProvider(submarinerConfig.Status.ManagedClusterInfo.Platform, func(info *provider.Info) (cloud.Provider, error) {
				Expect(info.IPSecNATTPort).To(Equal(constants.SubmarinerNatTPort))
				Expect(info.NATTDiscoveryPort).To(Equal(constants.SubmarinerNatTDiscoveryPort))
				Expect(info.CredentialsSecret).To(Equal(credentialsSecret))

				return mockProvider, nil
			})
		})

		Context("and the credentials Secret exists", func() {
			BeforeEach(func() {
				submarinerConfig.Spec.CredentialsSecret = &corev1.LocalObjectReference{Name: credentialsSecret.Name}

				_, err := hubKubeClient.CoreV1().Secrets(submarinerConfig.Status.ManagedClusterInfo.ClusterName).Create(context.TODO(),
					credentialsSecret, metav1.CreateOptions{})
				Expect(err).To(Succeed())
			})

			It("should return an instance", func() {
				provider, found, err := providerFactory.Get(submarinerConfig, events.NewLoggingEventRecorder("test", clock.RealClock{}))

				Expect(err).To(Succeed())
				Expect(found).To(BeTrue())
				Expect(provider).To(Equal(mockProvider))
			})
		})

		Context("and the credentials Secret reference isn't provided", func() {
			It("should return an error", func() {
				_, _, err := providerFactory.Get(submarinerConfig, events.NewLoggingEventRecorder("test", clock.RealClock{}))
				Expect(err).To(HaveOccurred())
			})
		})

		Context("and the credentials Secret doesn't exist", func() {
			BeforeEach(func() {
				submarinerConfig.Spec.CredentialsSecret = &corev1.LocalObjectReference{Name: "test-secret"}
			})

			It("should return an error", func() {
				_, _, err := providerFactory.Get(submarinerConfig, events.NewLoggingEventRecorder("test", clock.RealClock{}))
				Expect(err).To(HaveOccurred())
			})
		})
	})

	When("skip prepare is enabled", func() {
		BeforeEach(func() {
			submarinerConfig.Annotations = map[string]string{"submariner.io/skip-cloud-prepare": strconv.FormatBool(true)}
		})

		It("should return false", func() {
			provider, found, err := providerFactory.Get(submarinerConfig, events.NewLoggingEventRecorder("test", clock.RealClock{}))
			Expect(err).To(Succeed())
			Expect(found).To(BeFalse())
			Expect(provider).To(BeNil())
		})
	})
})
