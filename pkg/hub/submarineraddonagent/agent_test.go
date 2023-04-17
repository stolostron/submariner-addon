package submarineraddonagent_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/rand"
	"net"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/stolostron/submariner-addon/pkg/hub/submarineraddonagent"
	appsv1 "k8s.io/api/apps/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"open-cluster-management.io/addon-framework/pkg/agent"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"
	addonfake "open-cluster-management.io/api/client/addon/clientset/versioned/fake"
	clusterclient "open-cluster-management.io/api/client/cluster/clientset/versioned"
	fakeclusterclient "open-cluster-management.io/api/client/cluster/clientset/versioned/fake"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

func TestSubmarinerAddOnAgent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Submariner AddOn Agent Suite")
}

const defaultNamespace = "open-cluster-management-agent-addon"

var _ = Describe("Manifests", func() {
	t := newTestDriver()

	Context("using the default installation namespace", func() {
		It("should return the correct resources", func() {
			objs, err := t.addOnAgent.Manifests(&clusterv1.ManagedCluster{}, &addonapiv1alpha1.ManagedClusterAddOn{})
			Expect(err).To(Succeed())

			verifyManifestObjs(objs, defaultManifestObjStrings(defaultNamespace))
		})
	})

	Context("using a custom installation namespace", func() {
		It("should return the correct resources", func() {
			ns := "custom"
			objs, err := t.addOnAgent.Manifests(&clusterv1.ManagedCluster{}, &addonapiv1alpha1.ManagedClusterAddOn{
				Spec: addonapiv1alpha1.ManagedClusterAddOnSpec{
					InstallNamespace: ns,
				},
			})
			Expect(err).To(Succeed())

			verifyManifestObjs(objs, append(defaultManifestObjStrings(ns),
				toManifestObjString(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
					Name: ns,
				}})))
		})
	})

	Context("using only a default addonDeploymentConfig", func() {
		It("should succeed", func() {
			objs, err := t.addOnAgent.Manifests(&clusterv1.ManagedCluster{}, t.newManagedClusterAddOnWithADConfig(true))
			Expect(err).To(Succeed())

			verifyManifestObjs(objs, defaultManifestObjStrings(defaultNamespace))
		})
	})

	Context("using both default and cluster specific addonDeploymentConfig", func() {
		It("should succeed", func() {
			objs, err := t.addOnAgent.Manifests(&clusterv1.ManagedCluster{}, t.newManagedClusterAddOnWithADConfig(false))
			Expect(err).To(Succeed())

			verifyManifestObjs(objs, defaultManifestObjStrings(defaultNamespace))
		})
	})
})

var _ = Describe("GetAgentAddonOptions", func() {
	clusterName := "test-managed-cluster"
	t := newTestDriver()

	var options agent.AgentAddonOptions

	JustBeforeEach(func() {
		options = t.addOnAgent.GetAgentAddonOptions()
	})

	Context("PermissionConfig", func() {
		It("should create the hub permission resources", func() {
			Expect(options.Registration.PermissionConfig(&clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
			}, &addonapiv1alpha1.ManagedClusterAddOn{})).To(Succeed())

			_, err := t.kubeClient.RbacV1().Roles(clusterName).Get(context.TODO(),
				"open-cluster-management:submariner-addon:agent", metav1.GetOptions{})
			Expect(err).To(Succeed())

			_, err = t.kubeClient.RbacV1().RoleBindings(clusterName).Get(context.TODO(),
				"open-cluster-management:submariner-addon:agent", metav1.GetOptions{})
			Expect(err).To(Succeed())
		})
	})

	Context("CSRApproveCheck", func() {
		var csr csrHolder
		var result bool
		const invalid string = "invalid"

		BeforeEach(func() {
			csr = csrHolder{
				SignerName:   certificatesv1.KubeAPIServerClientSignerName,
				ReqBlockType: "CERTIFICATE REQUEST",
				Orgs: []string{
					"system:authenticated",
					"system:open-cluster-management:addon:submariner",
					"system:open-cluster-management:cluster:" + clusterName + ":addon:submariner",
				},
				CN: "system:open-cluster-management:cluster:" + clusterName + ":addon:submariner:agent:submariner-addon-agent",
			}
		})

		JustBeforeEach(func() {
			result = options.Registration.CSRApproveCheck(&clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: clusterName,
				},
			}, &addonapiv1alpha1.ManagedClusterAddOn{}, newCSR(csr))
		})

		When("the CSR is valid", func() {
			It("should return true", func() {
				Expect(result).To(BeTrue())
			})
		})

		When("the signer name is invalid", func() {
			BeforeEach(func() {
				csr.SignerName = invalid
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("the block type is invalid", func() {
			BeforeEach(func() {
				csr.ReqBlockType = "RSA PRIVATE KEY"
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("the authenticated group is missing from the orgs", func() {
			BeforeEach(func() {
				csr.Orgs[0] = invalid
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("the addon group is missing from the orgs", func() {
			BeforeEach(func() {
				csr.Orgs[1] = invalid
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("the cluster addon group is missing from the orgs", func() {
			BeforeEach(func() {
				csr.Orgs[2] = invalid
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("an extra org is present", func() {
			BeforeEach(func() {
				csr.Orgs = append(csr.Orgs, "extra")
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})

		When("the common name is invalid", func() {
			BeforeEach(func() {
				csr.CN = invalid
			})

			It("should return false", func() {
				Expect(result).To(BeFalse())
			})
		})
	})

	Context("SupportedConfigGVRs", func() {
		It("should register addondeploymentconfig GVR", func() {
			Expect(options.SupportedConfigGVRs).To(Equal(
				[]schema.GroupVersionResource{
					{
						Group:   "addon.open-cluster-management.io",
						Version: "v1alpha1", Resource: "addondeploymentconfigs",
					},
				}))
		})
	})
})

type testDriver struct {
	addOnAgent    agent.AgentAddon
	kubeClient    kubernetes.Interface
	clusterClient clusterclient.Interface
	addOnClient   addonclient.Interface
}

func newTestDriver() *testDriver {
	t := &testDriver{}

	BeforeEach(func() {
		t.kubeClient = kubefake.NewSimpleClientset()
		t.clusterClient = fakeclusterclient.NewSimpleClientset()
		t.addOnClient = addonfake.NewSimpleClientset()
		t.addOnAgent = submarineraddonagent.NewAddOnAgent(t.kubeClient, t.clusterClient, t.addOnClient,
			events.NewLoggingEventRecorder("test"), "test")
	})

	return t
}

func verifyManifestObjs(actualObjs []runtime.Object, expected []string) {
	actual := map[string]bool{}
	for i := range actualObjs {
		actual[toManifestObjString(actualObjs[i])] = true
	}

	for i := range expected {
		_, found := actual[expected[i]]
		Expect(found).To(BeTrue(), "Missing expected manifest resource %q", expected[i])
		delete(actual, expected[i])
	}

	Expect(actual).To(BeEmpty(), "Received unexpected manifest resources")
}

func toManifestObjString(obj runtime.Object) string {
	m, err := meta.Accessor(obj)
	Expect(err).To(Succeed())

	return fmt.Sprintf("Type: %T, NS: %s, Name: %s", obj, m.GetNamespace(), m.GetName())
}

func defaultManifestObjStrings(ns string) []string {
	return []string{
		toManifestObjString(&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{
			Name: "open-cluster-management:submariner-addon:agent",
		}}),
		toManifestObjString(&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{
			Name: "open-cluster-management:submariner-addon:agent",
		}}),
		toManifestObjString(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
			Name:      "submariner-addon",
			Namespace: ns,
		}}),
		toManifestObjString(&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{
			Name:      "submariner-addon",
			Namespace: ns,
		}}),
		toManifestObjString(&rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{
			Name:      "submariner-addon",
			Namespace: ns,
		}}),
		toManifestObjString(&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
			Name:      "submariner-addon-sa",
			Namespace: ns,
		}}),
	}
}

type csrHolder struct {
	SignerName   string
	CN           string
	Orgs         []string
	ReqBlockType string
}

func newCSR(holder csrHolder) *certificatesv1.CertificateSigningRequest {
	//nolint:gosec // These are test credentials, using insecure rand vs crypto/rand is okay
	insecureRand := rand.New(rand.NewSource(0))
	pk, err := ecdsa.GenerateKey(elliptic.P256(), insecureRand)
	Expect(err).To(Succeed())

	csrb, err := x509.CreateCertificateRequest(insecureRand, &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   holder.CN,
			Organization: holder.Orgs,
		},
		DNSNames:       []string{},
		EmailAddresses: []string{},
		IPAddresses:    []net.IP{},
	}, pk)

	Expect(err).To(Succeed())

	return &certificatesv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:         "test-csr",
			GenerateName: "csr-",
		},
		Spec: certificatesv1.CertificateSigningRequestSpec{
			Username:   "test",
			Usages:     []certificatesv1.KeyUsage{},
			SignerName: holder.SignerName,
			Request:    pem.EncodeToMemory(&pem.Block{Type: holder.ReqBlockType, Bytes: csrb}),
		},
	}
}

func (t *testDriver) createAddonDeploymentConfig(config *addonapiv1alpha1.AddOnDeploymentConfig) {
	_, err := t.addOnClient.AddonV1alpha1().AddOnDeploymentConfigs(config.Namespace).Create(context.TODO(), config,
		metav1.CreateOptions{})
	Expect(err).To(Succeed())
}

func (t *testDriver) newManagedClusterAddOnWithADConfig(defaultOnly bool) *addonapiv1alpha1.ManagedClusterAddOn {
	clusterADConfig := &addonapiv1alpha1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "submariner-adconfig",
			Namespace: "foo",
		},
		Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
			NodePlacement: &addonapiv1alpha1.NodePlacement{
				NodeSelector: map[string]string{"foo": "bar"},
				Tolerations: []corev1.Toleration{
					{
						Key:   "cluster-key",
						Value: "cluster-value",
					},
				},
			},
		},
	}

	t.createAddonDeploymentConfig(clusterADConfig)

	addon := &addonapiv1alpha1.ManagedClusterAddOn{
		Spec: addonapiv1alpha1.ManagedClusterAddOnSpec{},
		Status: addonapiv1alpha1.ManagedClusterAddOnStatus{
			ConfigReferences: []addonapiv1alpha1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
						Group:    "addon.open-cluster-management.io",
						Resource: "addondeploymentconfigs",
					},
					ConfigReferent: addonapiv1alpha1.ConfigReferent{
						Name:      clusterADConfig.Name,
						Namespace: clusterADConfig.Namespace,
					},
				},
			},
		},
	}

	if !defaultOnly {
		clusterADConfig.Namespace = "cluster"
		addon.Spec.Configs = []addonapiv1alpha1.AddOnConfig{
			{
				ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
					Group:    "addon.open-cluster-management.io",
					Resource: "addondeploymentconfigs",
				},
				ConfigReferent: addonapiv1alpha1.ConfigReferent{
					Name:      clusterADConfig.Name,
					Namespace: clusterADConfig.Namespace,
				},
			},
		}

		t.createAddonDeploymentConfig(clusterADConfig)
	}

	return addon
}
