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
	"slices"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/stolostron/submariner-addon/pkg/constants"
	"github.com/stolostron/submariner-addon/pkg/hub/submarineraddonagent"
	resourceutil "github.com/submariner-io/admiral/pkg/resource"
	appsv1 "k8s.io/api/apps/v1"
	certificatesv1 "k8s.io/api/certificates/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/utils/clock"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/agent"
	"open-cluster-management.io/addon-framework/pkg/utils"
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

func init() {
	utilruntime.Must(addonapiv1alpha1.Install(scheme.Scheme))
}

const (
	submarinerAddonName = "submariner-addon"
	agentImage          = "test-image"
	clusterName         = "east-cluster"
	localClusterURL     = "http://local"
)

var _ = Describe("Manifests", func() {
	t := newTestDriver()

	It("should return the Deployment with the correct fields", func() {
		objs, err := t.addOnAgent.Manifests(&clusterv1.ManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: clusterName,
			},
		}, &addonapiv1alpha1.ManagedClusterAddOn{})
		Expect(err).To(Succeed())

		deployment := getDeployment(objs)
		Expect(deployment.Spec.Template.Spec.Containers[0].Args).To(ContainElement(ContainSubstring(clusterName)))
		Expect(deployment.Spec.Template.Spec.Containers[0].Env).To(ContainElement(corev1.EnvVar{
			Name:  "HUB_HOST",
			Value: localClusterURL,
		}))
		Expect(deployment.Spec.Template.Spec.Volumes[0].Secret.SecretName).To(Equal(constants.SubmarinerAddOnName + "-hub-kubeconfig"))
		Expect(deployment.Spec.Template.Spec.NodeSelector).To(BeEmpty())
		Expect(deployment.Spec.Template.Spec.Tolerations).To(BeEmpty())
		Expect(deployment.Spec.Template.Spec.Containers[0].Resources).To(Equal(corev1.ResourceRequirements{}))
	})

	Context("using the default installation namespace", func() {
		It("should return the correct resources", func() {
			objs, err := t.addOnAgent.Manifests(&clusterv1.ManagedCluster{}, &addonapiv1alpha1.ManagedClusterAddOn{})
			Expect(err).To(Succeed())

			verifyManifestObjs(objs, expectedManifestObjStrings(addonfactory.AddonDefaultInstallNamespace, false))
		})
	})

	Context("using a custom installation namespace", func() {
		It("should return the correct resources", func() {
			ns := "submariner-operator"
			objs, err := t.addOnAgent.Manifests(&clusterv1.ManagedCluster{}, &addonapiv1alpha1.ManagedClusterAddOn{
				Spec: addonapiv1alpha1.ManagedClusterAddOnSpec{
					InstallNamespace: ns,
				},
			})
			Expect(err).To(Succeed())

			verifyManifestObjs(objs, expectedManifestObjStrings(ns, true))
		})
	})

	Context("using an AddonDeploymentConfig", func() {
		var adConfig *addonapiv1alpha1.AddOnDeploymentConfig

		BeforeEach(func() {
			adConfig = &addonapiv1alpha1.AddOnDeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "submariner-adconfig",
					Namespace: "foo",
				},
				Spec: addonapiv1alpha1.AddOnDeploymentConfigSpec{
					NodePlacement: &addonapiv1alpha1.NodePlacement{
						NodeSelector: map[string]string{"sel-key": "sel-value"},
						Tolerations: []corev1.Toleration{
							{
								Key:   "cluster-key",
								Value: "cluster-value",
							},
						},
					},
					ResourceRequirements: []addonapiv1alpha1.ContainerResourceRequirements{
						{
							Resources: corev1.ResourceRequirements{
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("150m"),
									corev1.ResourceMemory: resource.MustParse("256Mi"),
								},
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse("50m"),
									corev1.ResourceMemory: resource.MustParse("16Mi"),
								},
							},
						},
					},
				},
			}
		})

		Context("with matching ResourceRequirements ContainerID containing wildcards", func() {
			BeforeEach(func() {
				adConfig.Spec.ResourceRequirements[0].ContainerID = "deployments:*:*"
			})

			It("should return the Deployment with the correct fields", func() {
				t.testManifestsWithADConfig(adConfig)
			})
		})

		Context("with matching ResourceRequirements ContainerID containing exact names", func() {
			BeforeEach(func() {
				adConfig.Spec.ResourceRequirements[0].ContainerID = "deployments:submariner-addon:submariner-addon"
			})

			It("should return the Deployment with the correct fields", func() {
				t.testManifestsWithADConfig(adConfig)
			})
		})
	})
})

var _ = Describe("GetAgentAddonOptions", func() {
	t := newTestDriver()

	Context("PermissionConfig", func() {
		It("should create the hub permission resources", func() {
			Expect(t.addOnAgent.GetAgentAddonOptions().Registration.PermissionConfig(&clusterv1.ManagedCluster{
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
			result = t.addOnAgent.GetAgentAddonOptions().Registration.CSRApproveCheck(&clusterv1.ManagedCluster{
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
			Expect(t.addOnAgent.GetAgentAddonOptions().SupportedConfigGVRs).To(Equal(
				[]schema.GroupVersionResource{
					addonapiv1alpha1.GroupVersion.WithResource("addondeploymentconfigs"),
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
		t.kubeClient = kubefake.NewClientset()
		t.clusterClient = fakeclusterclient.NewSimpleClientset(&clusterv1.ManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name: "",
				Labels: map[string]string{
					"local-cluster": "true",
				},
			},
			Spec: clusterv1.ManagedClusterSpec{
				ManagedClusterClientConfigs: []clusterv1.ClientConfig{
					{
						URL: localClusterURL,
					},
				},
			},
		})
		t.addOnClient = addonfake.NewSimpleClientset()

		var err error

		t.addOnAgent, err = submarineraddonagent.NewAddOnAgent(t.kubeClient, t.clusterClient, t.addOnClient,
			events.NewLoggingEventRecorder("test", clock.RealClock{}), agentImage)
		Expect(err).NotTo(HaveOccurred())
	})

	return t
}

func (t *testDriver) createAddonDeploymentConfig(config *addonapiv1alpha1.AddOnDeploymentConfig) {
	_, err := t.addOnClient.AddonV1alpha1().AddOnDeploymentConfigs(config.Namespace).Create(context.TODO(), config,
		metav1.CreateOptions{})
	Expect(err).To(Succeed())
}

func (t *testDriver) newManagedClusterAddOn(adConfig *addonapiv1alpha1.AddOnDeploymentConfig) *addonapiv1alpha1.ManagedClusterAddOn {
	t.createAddonDeploymentConfig(adConfig)

	specHash, err := utils.GetSpecHash(resourceutil.MustToUnstructured(adConfig))
	Expect(err).NotTo(HaveOccurred())

	addon := &addonapiv1alpha1.ManagedClusterAddOn{
		Spec: addonapiv1alpha1.ManagedClusterAddOnSpec{},
		Status: addonapiv1alpha1.ManagedClusterAddOnStatus{
			ConfigReferences: []addonapiv1alpha1.ConfigReference{
				{
					ConfigGroupResource: addonapiv1alpha1.ConfigGroupResource{
						Group:    "addon.open-cluster-management.io",
						Resource: "addondeploymentconfigs",
					},
					DesiredConfig: &addonapiv1alpha1.ConfigSpecHash{
						ConfigReferent: addonapiv1alpha1.ConfigReferent{
							Name:      adConfig.Name,
							Namespace: adConfig.Namespace,
						},
						SpecHash: specHash,
					},
				},
			},
		},
	}

	return addon
}

func (t *testDriver) testManifestsWithADConfig(adConfig *addonapiv1alpha1.AddOnDeploymentConfig) {
	objs, err := t.addOnAgent.Manifests(&clusterv1.ManagedCluster{}, t.newManagedClusterAddOn(adConfig))
	Expect(err).To(Succeed())

	deployment := getDeployment(objs)
	Expect(deployment.Spec.Template.Spec.NodeSelector).To(Equal(adConfig.Spec.NodePlacement.NodeSelector))
	Expect(deployment.Spec.Template.Spec.Tolerations).To(Equal(adConfig.Spec.NodePlacement.Tolerations))
	Expect(deployment.Spec.Template.Spec.Containers[0].Resources).To(Equal(adConfig.Spec.ResourceRequirements[0].Resources))
}

func getDeployment(objs []runtime.Object) *appsv1.Deployment {
	index := slices.IndexFunc(objs, func(obj runtime.Object) bool {
		_, ok := obj.(*appsv1.Deployment)
		return ok
	})
	Expect(index).To(BeNumerically(">=", 0), "Deployment resource not found")

	return objs[index].(*appsv1.Deployment)
}

func verifyManifestObjs(actualObjs []runtime.Object, expected []string) {
	actual := sets.New[string]()
	for i := range actualObjs {
		actual.Insert(toManifestObjString(actualObjs[i]))
	}

	for i := range expected {
		Expect(actual.Has(expected[i])).To(BeTrue(), "Missing expected manifest resource %q", expected[i])
		actual.Delete(expected[i])
	}

	Expect(actual).To(BeEmpty(), "Received unexpected manifest resources")
}

func toManifestObjString(obj runtime.Object) string {
	m, err := meta.Accessor(obj)
	Expect(err).To(Succeed())

	return fmt.Sprintf("Type: %T, NS: %s, Name: %s", obj, m.GetNamespace(), m.GetName())
}

func expectedManifestObjStrings(ns string, expNamespace bool) []string {
	manifests := []string{
		toManifestObjString(&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{
			Name: "open-cluster-management:submariner-addon:agent",
		}}),
		toManifestObjString(&rbacv1.ClusterRoleBinding{ObjectMeta: metav1.ObjectMeta{
			Name: "open-cluster-management:submariner-addon:agent",
		}}),
		toManifestObjString(&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{
			Name:      submarinerAddonName,
			Namespace: ns,
		}}),
		toManifestObjString(&rbacv1.Role{ObjectMeta: metav1.ObjectMeta{
			Name:      submarinerAddonName,
			Namespace: ns,
		}}),
		toManifestObjString(&rbacv1.RoleBinding{ObjectMeta: metav1.ObjectMeta{
			Name:      submarinerAddonName,
			Namespace: ns,
		}}),
		toManifestObjString(&corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{
			Name:      "submariner-addon-sa",
			Namespace: ns,
		}}),
	}

	if expNamespace {
		manifests = append(manifests, toManifestObjString(&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name: ns,
		}}))
	}

	return manifests
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
