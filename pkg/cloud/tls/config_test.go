package tls_test

import (
	"context"
	"crypto/tls"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	configv1 "github.com/openshift/api/config/v1"
	openshifttls "github.com/openshift/controller-runtime-common/pkg/tls"
	tlspkg "github.com/stolostron/submariner-addon/pkg/cloud/tls"
	submreporter "github.com/submariner-io/admiral/pkg/reporter"
	"github.com/submariner-io/admiral/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8sscheme "k8s.io/client-go/kubernetes/scheme"
)

var createOptions = metav1.CreateOptions{}

var _ = Describe("GetConfigFromAPIServer", func() {
	var (
		dynamicClient *dynamicfake.FakeDynamicClient
		apiServer     *configv1.APIServer
		reporter      submreporter.Interface
	)

	BeforeEach(func() {
		apiServer = &configv1.APIServer{
			ObjectMeta: metav1.ObjectMeta{
				Name: openshifttls.APIServerName,
			},
			Spec: configv1.APIServerSpec{
				TLSSecurityProfile: &configv1.TLSSecurityProfile{},
			},
		}

		dynamicClient = dynamicfake.NewSimpleDynamicClient(k8sscheme.Scheme)
		reporter = submreporter.Stdout()
	})

	JustBeforeEach(func(ctx context.Context) {
		if apiServer != nil {
			_, err := dynamicClient.Resource(configv1.GroupVersion.WithResource("apiservers")).
				Create(ctx, resource.MustToUnstructured(apiServer), createOptions)
			Expect(err).NotTo(HaveOccurred())
		}
	})

	Context("with Intermediate TLS profile", func() {
		BeforeEach(func() {
			apiServer.Spec.TLSSecurityProfile.Type = configv1.TLSProfileIntermediateType
		})

		It("should return a valid tls.Config with MinVersion TLS 1.2", func(ctx context.Context) {
			tlsConfig, err := tlspkg.GetConfigFromAPIServer(ctx, dynamicClient, reporter, false)

			Expect(err).NotTo(HaveOccurred())
			Expect(tlsConfig).NotTo(BeNil())
			Expect(tlsConfig.MinVersion).To(Equal(uint16(tls.VersionTLS12)))
			Expect(tlsConfig.CipherSuites).NotTo(BeEmpty())
			Expect(tlsConfig.InsecureSkipVerify).To(BeFalse())
		})

		It("should set InsecureSkipVerify when requested", func(ctx context.Context) {
			tlsConfig, err := tlspkg.GetConfigFromAPIServer(ctx, dynamicClient, reporter, true)

			Expect(err).NotTo(HaveOccurred())
			Expect(tlsConfig.InsecureSkipVerify).To(BeTrue())
			// TLS version and ciphers should still be set
			Expect(tlsConfig.MinVersion).To(Equal(uint16(tls.VersionTLS12)))
			Expect(tlsConfig.CipherSuites).NotTo(BeEmpty())
		})
	})

	Context("with Modern TLS profile (TLS 1.3)", func() {
		BeforeEach(func() {
			apiServer.Spec.TLSSecurityProfile.Type = configv1.TLSProfileModernType
		})

		It("should return a valid tls.Config with MinVersion TLS 1.3 and no CipherSuites", func(ctx context.Context) {
			tlsConfig, err := tlspkg.GetConfigFromAPIServer(ctx, dynamicClient, reporter, false)

			Expect(err).NotTo(HaveOccurred())
			Expect(tlsConfig).NotTo(BeNil())
			Expect(tlsConfig.MinVersion).To(Equal(uint16(tls.VersionTLS13)))
			Expect(tlsConfig.CipherSuites).To(BeEmpty())
		})
	})

	Context("with Custom TLS profile", func() {
		BeforeEach(func() {
			apiServer.Spec.TLSSecurityProfile = &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						MinTLSVersion: configv1.VersionTLS12,
						Ciphers: []string{
							"ECDHE-ECDSA-AES128-GCM-SHA256",
							"ECDHE-RSA-AES128-GCM-SHA256",
							"Unknown",
						},
					},
				},
			}
		})

		It("should return a valid tls.Config with the custom settings", func(ctx context.Context) {
			tlsConfig, err := tlspkg.GetConfigFromAPIServer(ctx, dynamicClient, reporter, false)

			Expect(err).NotTo(HaveOccurred())
			Expect(tlsConfig).NotTo(BeNil())
			Expect(tlsConfig.MinVersion).To(Equal(uint16(tls.VersionTLS12)))
			Expect(tlsConfig.CipherSuites).To(HaveLen(2))
		})
	})

	Context("with nil TLS profile", func() {
		BeforeEach(func() {
			apiServer.Spec.TLSSecurityProfile = nil
		})

		It("should default to Intermediate profile", func(ctx context.Context) {
			tlsConfig, err := tlspkg.GetConfigFromAPIServer(ctx, dynamicClient, reporter, false)

			Expect(err).NotTo(HaveOccurred())
			Expect(tlsConfig).NotTo(BeNil())
			Expect(tlsConfig.MinVersion).To(Equal(uint16(tls.VersionTLS12)))
			Expect(tlsConfig.CipherSuites).NotTo(BeEmpty())
		})
	})

	Context("when the APIServer resource does not exist", func() {
		BeforeEach(func() {
			apiServer = nil
		})

		It("should return an error", func(ctx context.Context) {
			_, err := tlspkg.GetConfigFromAPIServer(ctx, dynamicClient, reporter, false)

			Expect(err).To(HaveOccurred())
		})
	})

	Context("with Custom TLS profile containing only unsupported ciphers for TLS 1.2", func() {
		BeforeEach(func() {
			apiServer.Spec.TLSSecurityProfile = &configv1.TLSSecurityProfile{
				Type: configv1.TLSProfileCustomType,
				Custom: &configv1.CustomTLSProfile{
					TLSProfileSpec: configv1.TLSProfileSpec{
						MinTLSVersion: configv1.VersionTLS12,
						Ciphers: []string{
							"FAKE-UNSUPPORTED-CIPHER-1",
							"FAKE-UNSUPPORTED-CIPHER-2",
						},
					},
				},
			}
		})

		It("should return an error preventing silent fallback to defaults", func(ctx context.Context) {
			_, err := tlspkg.GetConfigFromAPIServer(ctx, dynamicClient, reporter, false)
			Expect(err).To(HaveOccurred())
		})
	})
})
