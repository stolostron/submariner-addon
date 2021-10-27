package submarinerbrokerinfo_test

import (
	"encoding/base64"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	configclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	fakeconfigclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned/fake"
	"github.com/open-cluster-management/submariner-addon/pkg/hub/submarinerbrokerinfo"
	apiconfigv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

const (
	brokerNamespace   = "broker-ns"
	clusterName       = "east"
	apiServerHost     = "api.test.dev04.red-chesterfield.com"
	apiServerHostPort = apiServerHost + ":6443"
	brokerToken       = "broker-token"
	brokerCA          = "broker-CA"
	ipsecPSk          = "test-psk"
)

func TestSubmarinerBrokerInfo(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Submariner Broker Info Suite")
}

var _ = Describe("Function Get", func() {
	var (
		configClient          configclient.Interface
		installationNamespace string
		submarinerConfig      *configv1alpha1.SubmarinerConfig
		infrastructure        *unstructured.Unstructured
		ipsecSecret           *corev1.Secret
		serviceAccount        *corev1.ServiceAccount
		serviceAccountSecret  *corev1.Secret
		kubeObjs              []runtime.Object
		dynamicObjs           []runtime.Object
		brokerInfo            *submarinerbrokerinfo.SubmarinerBrokerInfo
		err                   error
	)

	BeforeEach(func() {
		installationNamespace = ""

		infrastructure = &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": "config.openshift.io/v1",
				"kind":       "Infrastructure",
				"metadata": map[string]interface{}{
					"name": "cluster",
				},
				"status": map[string]interface{}{
					"apiServerURL": "https://" + apiServerHostPort,
				},
			},
		}

		ipsecSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "submariner-ipsec-psk",
				Namespace: brokerNamespace,
			},
			Data: map[string][]byte{
				"psk": []byte(ipsecPSk),
			},
		}

		serviceAccount = &corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterName,
				Namespace: brokerNamespace,
			},
			Secrets: []corev1.ObjectReference{{Name: "other"}, {Name: clusterName + "-token-5pw5c"}},
		}

		serviceAccountSecret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      clusterName + "-token-5pw5c",
				Namespace: brokerNamespace,
			},
			Data: map[string][]byte{
				"ca.crt": []byte(brokerCA),
				"token":  []byte(brokerToken),
			},
			Type: corev1.SecretTypeServiceAccountToken,
		}

		kubeObjs = []runtime.Object{ipsecSecret, serviceAccount, serviceAccountSecret}
		dynamicObjs = []runtime.Object{infrastructure}
	})

	JustBeforeEach(func() {
		if submarinerConfig != nil {
			configClient = fakeconfigclient.NewSimpleClientset(submarinerConfig)
		} else {
			configClient = fakeconfigclient.NewSimpleClientset()
		}

		brokerInfo, err = submarinerbrokerinfo.Get(
			kubefake.NewSimpleClientset(kubeObjs...),
			dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), dynamicObjs...),
			configClient,
			events.NewLoggingEventRecorder("test"),
			clusterName,
			brokerNamespace,
			submarinerConfig,
			installationNamespace,
		)
	})

	Context("on success", func() {
		JustBeforeEach(func() {
			Expect(err).To(Succeed())
		})

		It("should return the correct broker credentials", func() {
			Expect(brokerInfo.BrokerNamespace).To(Equal(brokerNamespace))
			Expect(brokerInfo.BrokerAPIServer).To(Equal(apiServerHostPort))
			Expect(brokerInfo.BrokerToken).To(Equal(brokerToken))
			Expect(brokerInfo.BrokerCA).To(Equal(base64.StdEncoding.EncodeToString([]byte(brokerCA))))
		})

		It("should return the correct IPSecPSK", func() {
			Expect(brokerInfo.IPSecPSK).To(Equal(base64.StdEncoding.EncodeToString([]byte(ipsecPSk))))
		})

		It("should return the correct ClusterName", func() {
			Expect(brokerInfo.ClusterName).To(Equal(clusterName))
		})

		When("no installation namespace is provided", func() {
			It("should return the default", func() {
				Expect(brokerInfo.InstallationNamespace).To(Equal("open-cluster-management-agent-addon"))
			})
		})

		When("an installation namespace is provided", func() {
			BeforeEach(func() {
				installationNamespace = "custom-install-ns"
			})

			It("should return it", func() {
				Expect(brokerInfo.InstallationNamespace).To(Equal(installationNamespace))
			})
		})

		When("no SubmarinerConfig is provided", func() {
			It("should return the defaults", func() {
				Expect(brokerInfo.CableDriver).To(Equal("libreswan"))
				Expect(brokerInfo.CatalogChannel).To(BeEmpty())
				Expect(brokerInfo.CatalogName).To(Equal("submariner"))
				Expect(brokerInfo.CatalogSource).To(Equal("redhat-operators"))
				Expect(brokerInfo.CatalogSourceNamespace).To(Equal("openshift-marketplace"))
				Expect(brokerInfo.CatalogStartingCSV).To(BeEmpty())
				Expect(brokerInfo.IPSecIKEPort).To(BeZero())
				Expect(brokerInfo.IPSecNATTPort).To(Equal(4500))
				Expect(brokerInfo.LighthouseAgentImage).To(BeEmpty())
				Expect(brokerInfo.LighthouseCoreDNSImage).To(BeEmpty())
				Expect(brokerInfo.NATEnabled).To(BeFalse())
				Expect(brokerInfo.LoadBalancerEnabled).To(BeFalse())
				Expect(brokerInfo.SubmarinerGatewayImage).To(BeEmpty())
				Expect(brokerInfo.SubmarinerRouteAgentImage).To(BeEmpty())
			})
		})

		When("a SubmarinerConfig is provided", func() {
			BeforeEach(func() {
				submarinerConfig = &configv1alpha1.SubmarinerConfig{
					Spec: configv1alpha1.SubmarinerConfigSpec{
						SubscriptionConfig: configv1alpha1.SubscriptionConfig{
							Channel:         "test-channel",
							Source:          "operatorhubio-catalog",
							SourceNamespace: "olm",
							StartingCSV:     "xyz",
						},
						ImagePullSpecs: configv1alpha1.SubmarinerImagePullSpecs{
							SubmarinerImagePullSpec:           "quay.io/submariner/submariner-gateway:10.0.1",
							LighthouseAgentImagePullSpec:      "quay.io/submariner/lighthouse-agent:10.0.1",
							LighthouseCoreDNSImagePullSpec:    "quay.io/submariner/lighthouse-coredns:10.0.1",
							SubmarinerRouteAgentImagePullSpec: "quay.io/submariner/submariner-route-agent:10.0.1",
						},
						CableDriver:        "wireguard",
						IPSecIKEPort:       1234,
						IPSecNATTPort:      5678,
						NATTEnable:         true,
						LoadBalancerEnable: true,
					},
				}
			})

			It("should return its data", func() {
				Expect(brokerInfo.CableDriver).To(Equal(submarinerConfig.Spec.CableDriver))
				Expect(brokerInfo.CatalogChannel).To(Equal(submarinerConfig.Spec.SubscriptionConfig.Channel))
				Expect(brokerInfo.CatalogName).To(Equal("submariner"))
				Expect(brokerInfo.CatalogSource).To(Equal(submarinerConfig.Spec.SubscriptionConfig.Source))
				Expect(brokerInfo.CatalogSourceNamespace).To(Equal(submarinerConfig.Spec.SubscriptionConfig.SourceNamespace))
				Expect(brokerInfo.CatalogStartingCSV).To(Equal(submarinerConfig.Spec.SubscriptionConfig.StartingCSV))
				Expect(brokerInfo.IPSecIKEPort).To(Equal(submarinerConfig.Spec.IPSecIKEPort))
				Expect(brokerInfo.IPSecNATTPort).To(Equal(submarinerConfig.Spec.IPSecNATTPort))
				Expect(brokerInfo.LighthouseAgentImage).To(Equal(submarinerConfig.Spec.ImagePullSpecs.LighthouseAgentImagePullSpec))
				Expect(brokerInfo.LighthouseCoreDNSImage).To(Equal(submarinerConfig.Spec.ImagePullSpecs.LighthouseCoreDNSImagePullSpec))
				Expect(brokerInfo.NATEnabled).To(Equal(submarinerConfig.Spec.NATTEnable))
				Expect(brokerInfo.LoadBalancerEnabled).To(Equal(submarinerConfig.Spec.LoadBalancerEnable))
				Expect(brokerInfo.SubmarinerGatewayImage).To(Equal(submarinerConfig.Spec.ImagePullSpecs.SubmarinerImagePullSpec))
				Expect(brokerInfo.SubmarinerRouteAgentImage).To(Equal(submarinerConfig.Spec.ImagePullSpecs.SubmarinerRouteAgentImagePullSpec))
			})

			Context("with some fields not set", func() {
				BeforeEach(func() {
					submarinerConfig.Spec.CableDriver = ""
					submarinerConfig.Spec.SubscriptionConfig.Source = ""
					submarinerConfig.Spec.SubscriptionConfig.SourceNamespace = ""
					submarinerConfig.Spec.IPSecNATTPort = 0
				})

				It("should return the defaults for the unset fields", func() {
					Expect(brokerInfo.CableDriver).To(Equal("libreswan"))
					Expect(brokerInfo.CatalogSource).To(Equal("redhat-operators"))
					Expect(brokerInfo.CatalogSourceNamespace).To(Equal("openshift-marketplace"))
					Expect(brokerInfo.IPSecNATTPort).To(Equal(4500))
				})
			})
		})

		When("an APIServer resource exists", func() {
			tlsData := []byte("tls-data")

			BeforeEach(func() {
				secretName := "apiserver-secret"
				apiServer := &apiconfigv1.APIServer{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster",
					},
					Spec: apiconfigv1.APIServerSpec{
						ServingCerts: apiconfigv1.APIServerServingCerts{
							NamedCertificates: []apiconfigv1.APIServerNamedServingCert{
								{
									Names:              []string{apiServerHost},
									ServingCertificate: apiconfigv1.SecretNameReference{Name: secretName},
								},
							},
						},
					},
				}

				Expect(apiconfigv1.Install(scheme.Scheme)).To(Succeed())
				obj := &unstructured.Unstructured{}
				Expect(scheme.Scheme.Convert(apiServer, obj, nil)).To(Succeed())

				dynamicObjs = append(dynamicObjs, obj)

				kubeObjs = append(kubeObjs, &corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      secretName,
						Namespace: "openshift-config",
					},
					Data: map[string][]byte{
						"tls.crt": tlsData,
					},
					Type: corev1.SecretTypeTLS,
				})
			})

			It("should return the correct BrokerCA", func() {
				Expect(brokerInfo.BrokerCA).To(Equal(base64.StdEncoding.EncodeToString(tlsData)))
			})
		})
	})

	When("the Infrastructure resource is missing", func() {
		BeforeEach(func() {
			dynamicObjs = []runtime.Object{}
		})

		It("should return an error", func() {
			Expect(err).ToNot(Succeed())
		})
	})

	When("the Infrastructure resource is missing the apiServerURL field", func() {
		BeforeEach(func() {
			Expect(unstructured.SetNestedMap(infrastructure.Object, map[string]interface{}{}, "status")).To(Succeed())
		})

		It("should return an error", func() {
			Expect(err).ToNot(Succeed())
		})
	})

	When("the IPSec PSK Secret resource is missing", func() {
		BeforeEach(func() {
			kubeObjs = []runtime.Object{serviceAccount, serviceAccountSecret}
		})

		It("should return an error", func() {
			Expect(err).ToNot(Succeed())
		})
	})

	When("the cluster ServiceAccount resource is missing", func() {
		BeforeEach(func() {
			kubeObjs = []runtime.Object{ipsecSecret, serviceAccountSecret}
		})

		It("should return an error", func() {
			Expect(err).ToNot(Succeed())
		})
	})

	When("the cluster ServiceAccount resource has no Secrets", func() {
		BeforeEach(func() {
			serviceAccount.Secrets = []corev1.ObjectReference{}
		})

		It("should return an error", func() {
			Expect(err).ToNot(Succeed())
		})
	})

	When("the cluster ServiceAccount Secret resource is missing", func() {
		BeforeEach(func() {
			kubeObjs = []runtime.Object{ipsecSecret, serviceAccount}
		})

		It("should return an error", func() {
			Expect(err).ToNot(Succeed())
		})
	})
})
