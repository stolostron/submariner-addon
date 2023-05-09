package submarineragent_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/operator/events"
	configv1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	configclient "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	fakeconfigclient "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/clientset/versioned/fake"
	configinformers "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/informers/externalversions"
	cloudFake "github.com/stolostron/submariner-addon/pkg/cloud/fake"
	"github.com/stolostron/submariner-addon/pkg/constants"
	"github.com/stolostron/submariner-addon/pkg/hub/submarineragent"
	"github.com/stolostron/submariner-addon/pkg/resource"
	coreresource "github.com/submariner-io/admiral/pkg/resource"
	"github.com/submariner-io/admiral/pkg/test"
	submarinerv1alpha1 "github.com/submariner-io/submariner-operator/api/v1alpha1"
	"github.com/submariner-io/submariner-operator/pkg/discovery/globalnet"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"
	addonfake "open-cluster-management.io/api/client/addon/clientset/versioned/fake"
	addoninformers "open-cluster-management.io/api/client/addon/informers/externalversions"
	clusterclient "open-cluster-management.io/api/client/cluster/clientset/versioned"
	fakeclusterclient "open-cluster-management.io/api/client/cluster/clientset/versioned/fake"
	clusterinformers "open-cluster-management.io/api/client/cluster/informers/externalversions"
	fakeworkclient "open-cluster-management.io/api/client/work/clientset/versioned/fake"
	workinformers "open-cluster-management.io/api/client/work/informers/externalversions"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	clusterv1beta2 "open-cluster-management.io/api/cluster/v1beta2"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

const (
	clusterName      = "east"
	clusterSetName   = "north-america"
	installNamespace = "install-ns"
	brokerNamespace  = "north-america-broker"
	ipsecPSK         = "test-psk"
	brokerToken      = "broker-token"
	brokerCA         = "broker-CA"
)

var _ = Describe("Controller", func() {
	t := newTestDriver()

	When("the ManagedClusterAddon is created", func() {
		Context("after the ManagedCluster and ManagedClusterSet", func() {
			JustBeforeEach(func() {
				t.createManagedClusterSet()
				t.createAddonDeploymentConfig(t.defaultADConfig)
				t.createClusterManagementAddon()
				t.ensureNoManifestWorks()

				t.createManagedCluster()
				t.ensureNoManifestWorks()

				t.createGlobalnetConfigMap()
				t.createAddon()
			})

			It("should deploy the ManifestWorks", func() {
				t.awaitManifestWorks()
			})

			It("should create RBAC resources for the managed cluster", func() {
				t.awaitClusterRBACResources()
			})

			t.testFinalizers()

			Context("and the SubmarinerConfig is present", func() {
				BeforeEach(func() {
					t.createSubmarinerConfig()

					t.managedCluster.Status.ClusterClaims = []clusterv1.ManagedClusterClaim{
						{
							Name:  "product.open-cluster-management.io",
							Value: constants.ProductOCP,
						},
						{
							Name:  "platform.open-cluster-management.io",
							Value: "AWS",
						},
						{
							Name:  "region.open-cluster-management.io",
							Value: "east",
						},
						{
							Name:  "version.openshift.io",
							Value: "1.0",
						},
					}
				})

				It("should deploy the ManifestWorks with the SubmarinerConfig overrides", func() {
					t.awaitManifestWorks()

					expCond := &metav1.Condition{
						Type:   configv1alpha1.SubmarinerConfigConditionApplied,
						Status: metav1.ConditionTrue,
						Reason: "SubmarinerConfigApplied",
					}

					test.AwaitStatusCondition(expCond, func() ([]metav1.Condition, error) {
						config, err := t.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(clusterName).Get(context.TODO(),
							constants.SubmarinerConfigName, metav1.GetOptions{})
						if err != nil {
							return nil, err
						}

						return config.Status.Conditions, nil
					})

					config, err := t.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(clusterName).Get(context.TODO(),
						constants.SubmarinerConfigName, metav1.GetOptions{})
					Expect(err).To(Succeed())
					Expect(config.Status.ManagedClusterInfo).To(Equal(configv1alpha1.ManagedClusterInfo{
						ClusterName:   t.managedCluster.Name,
						Vendor:        constants.ProductOCP,
						Platform:      "AWS",
						Region:        "east",
						VendorVersion: "1.0",
					}))
				})
			})

			Context("and the SubmarinerConfig is present but backup label on broker is missing", func() {
				BeforeEach(func() {
					t.createSubmarinerConfig()
					t.createSubmarinerBroker()
				})

				It("should add backup label to broker", func() {
					t.awaitBackupLabelOnBroker()
				})
			})

			Context("and the SubmarinerConfig is present but globalnet config missing", func() {
				BeforeEach(func() {
					t.createSubmarinerConfig()
				})

				JustBeforeEach(func() {
					t.deleteGlobalnetConfigMap()
				})

				It("should update the status of ManagedClusterAddOn about missing config", func() {
					expCond := &metav1.Condition{
						Type:   submarineragent.BrokerCfgApplied,
						Status: metav1.ConditionFalse,
						Reason: "BrokerConfigMissing",
					}

					test.AwaitStatusCondition(expCond, func() ([]metav1.Condition, error) {
						config, err := t.addOnClient.AddonV1alpha1().ManagedClusterAddOns(clusterName).Get(context.TODO(),
							constants.SubmarinerAddOnName, metav1.GetOptions{})
						if err != nil {
							return nil, err
						}

						return config.Status.Conditions, nil
					})
				})
			})

			Context("and the ManagedCluster product is Openshift", func() {
				BeforeEach(func() {
					t.managedCluster.Status.ClusterClaims = []clusterv1.ManagedClusterClaim{
						{
							Name:  "product.open-cluster-management.io",
							Value: constants.ProductOCP,
						},
					}
				})

				It("should deploy the operator ManifestWork with the Openshift security resources", func() {
					t.assertSCCManifestObjs(t.awaitOperatorManifestWork())
				})
			})
		})

		Context("before the ManagedCluster and ManagedClusterSet", func() {
			JustBeforeEach(func() {
				t.createAddon()
				t.ensureNoManifestWorks()

				t.createManagedCluster()
				t.ensureNoManifestWorks()

				t.createManagedClusterSet()
				t.createAddonDeploymentConfig(t.defaultADConfig)
				t.createClusterManagementAddon()
				t.createGlobalnetConfigMap()
			})

			It("should deploy the ManifestWorks", func() {
				t.awaitManifestWorks()
			})

			t.testFinalizers()
		})

		Context("with cluster specific AddonDeploymentConfig", func() {
			JustBeforeEach(func() {
				t.createAddonDeploymentConfigForCluster(t.managedCluster.Name)
				t.setClusterADConfig()
				t.createAddon()
				t.ensureNoManifestWorks()

				t.createManagedCluster()
				t.ensureNoManifestWorks()

				t.createManagedClusterSet()
				t.createAddonDeploymentConfig(t.defaultADConfig)
				t.createClusterManagementAddon()
				t.createGlobalnetConfigMap()
			})

			It("should deploy the ManifestWorks", func() {
				t.awaitManifestWorks()
			})

			t.testFinalizers()
		})
	})

	When("the SubmarinerConfig is created after the submariner ManifestWork is deployed", func() {
		JustBeforeEach(func() {
			t.initManifestWorks()
			t.createSubmarinerConfig()
		})

		It("should update the ManifestWorks", func() {
			t.assertOperatorManifestWork(test.AwaitUpdateAction(&t.manifestWorkClient.Fake, "manifestworks",
				submarineragent.OperatorManifestWorkName).(*workv1.ManifestWork))
			t.assertSubmarinerManifestWork(test.AwaitUpdateAction(&t.manifestWorkClient.Fake, "manifestworks",
				submarineragent.SubmarinerCRManifestWorkName).(*workv1.ManifestWork))
		})
	})

	When("the ManagedClusterAddon is being deleted", func() {
		JustBeforeEach(func() {
			t.initManifestWorks()
			test.SetDeleting(resource.ForAddon(t.addOnClient.AddonV1alpha1().ManagedClusterAddOns(clusterName)), t.addOn.Name)
		})

		t.testAgentCleanup()
	})

	When("the ManagedCluster is being deleted", func() {
		JustBeforeEach(func() {
			t.initManifestWorks()
			test.SetDeleting(resource.ForManagedCluster(t.clusterClient.ClusterV1().ManagedClusters()), t.managedCluster.Name)
		})

		t.testAgentCleanup()
	})

	When("the ManagedCluster is being deleted and the ManagedClusterAddon doesn't exist", func() {
		JustBeforeEach(func() {
			t.initManifestWorks()
			test.SetDeleting(resource.ForManagedCluster(t.clusterClient.ClusterV1().ManagedClusters()), t.managedCluster.Name)
			Expect(t.addOnClient.AddonV1alpha1().ManagedClusterAddOns(clusterName).Delete(
				context.Background(), t.addOn.Name, metav1.DeleteOptions{})).To(Succeed())
		})

		t.testAgentCleanup()
	})

	When("the ManagedCluster is removed from the ManagedClusterSet", func() {
		JustBeforeEach(func() {
			t.initManifestWorks()

			t.managedCluster.Labels = nil
			_, err := t.clusterClient.ClusterV1().ManagedClusters().Update(context.TODO(), t.managedCluster, metav1.UpdateOptions{})
			Expect(err).To(Succeed())
		})

		t.testAgentCleanup()
	})
})

type testDriver struct {
	managedCluster     *clusterv1.ManagedCluster
	addOn              *addonv1alpha1.ManagedClusterAddOn
	clusterMgmtAddon   *addonv1alpha1.ClusterManagementAddOn
	defaultADConfig    *addonv1alpha1.AddOnDeploymentConfig
	clusterADConfig    *addonv1alpha1.AddOnDeploymentConfig
	submarinerConfig   *configv1alpha1.SubmarinerConfig
	broker             *unstructured.Unstructured
	kubeClient         kubernetes.Interface
	dynamicClient      *dynamicfake.FakeDynamicClient
	controllerClient   client.Client
	clusterClient      clusterclient.Interface
	manifestWorkClient *fakeworkclient.Clientset
	configClient       configclient.Interface
	addOnClient        addonclient.Interface
	stop               context.CancelFunc
	mockCtrl           *gomock.Controller
	cloudProvider      *cloudFake.MockProvider
}

func newTestDriver() *testDriver {
	t := &testDriver{}

	BeforeEach(func() {
		t.managedCluster = &clusterv1.ManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:   clusterName,
				Labels: map[string]string{clusterv1beta2.ClusterSetLabel: clusterSetName},
			},
		}

		t.addOn = &addonv1alpha1.ManagedClusterAddOn{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.SubmarinerAddOnName,
				Namespace: clusterName,
			},
			Spec: addonv1alpha1.ManagedClusterAddOnSpec{
				InstallNamespace: installNamespace,
			},
		}

		t.defaultADConfig = &addonv1alpha1.AddOnDeploymentConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "submariner-default",
				Namespace: "foobar",
			},
			Spec: addonv1alpha1.AddOnDeploymentConfigSpec{
				NodePlacement: &addonv1alpha1.NodePlacement{
					NodeSelector: map[string]string{"foo": "bar"},
					Tolerations: []corev1.Toleration{
						{
							Key:   "default-key",
							Value: "default-value",
						},
					},
				},
			},
		}

		t.clusterMgmtAddon = &addonv1alpha1.ClusterManagementAddOn{
			ObjectMeta: metav1.ObjectMeta{
				Name: constants.SubmarinerAddOnName,
			},
			Spec: addonv1alpha1.ClusterManagementAddOnSpec{
				AddOnMeta: addonv1alpha1.AddOnMeta{
					DisplayName: "Submariner Addon",
					Description: "Submariner Addon for MultiCluster connectivity",
				},
				SupportedConfigs: []addonv1alpha1.ConfigMeta{
					{
						ConfigGroupResource: addonv1alpha1.ConfigGroupResource{
							Group:    "addon.open-cluster-management.io",
							Resource: "addondeploymentconfigs",
						},
						DefaultConfig: &addonv1alpha1.ConfigReferent{
							Name:      t.defaultADConfig.Name,
							Namespace: t.defaultADConfig.Namespace,
						},
					},
				},
			},
		}

		t.submarinerConfig = nil
		t.broker = nil
		t.clusterADConfig = nil
		t.mockCtrl = gomock.NewController(GinkgoT())
		t.dynamicClient = dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
		t.clusterClient = fakeclusterclient.NewSimpleClientset()
		t.manifestWorkClient = fakeworkclient.NewSimpleClientset()
		t.configClient = fakeconfigclient.NewSimpleClientset()
		t.addOnClient = addonfake.NewSimpleClientset()
		t.cloudProvider = cloudFake.NewMockProvider(t.mockCtrl)
		t.controllerClient = fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()

		t.kubeClient = kubefake.NewSimpleClientset(
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "submariner-ipsec-psk",
					Namespace: brokerNamespace,
				},
				Data: map[string][]byte{
					"psk": []byte(ipsecPSK),
				},
			},
			&corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName,
					Namespace: brokerNamespace,
				},
				Secrets: []corev1.ObjectReference{{Name: clusterName + "-token-5pw5c"}},
			},
			&corev1.Secret{
				ObjectMeta: metav1.ObjectMeta{
					Name:      clusterName + "-token-5pw5c",
					Namespace: brokerNamespace,
				},
				Data: map[string][]byte{
					"ca.crt": []byte(brokerCA),
					"token":  []byte(brokerToken),
				},
				Type: corev1.SecretTypeServiceAccountToken,
			})
	})

	JustBeforeEach(func() {
		clusterInformerFactory := clusterinformers.NewSharedInformerFactory(t.clusterClient, 0)
		workInformerFactory := workinformers.NewSharedInformerFactory(t.manifestWorkClient, 0)
		configInformerFactory := configinformers.NewSharedInformerFactory(t.configClient, 0)
		addOnInformerFactory := addoninformers.NewSharedInformerFactory(t.addOnClient, 0)

		controller := submarineragent.NewSubmarinerAgentController(t.kubeClient, t.dynamicClient, t.controllerClient, t.clusterClient,
			t.manifestWorkClient, t.configClient, t.addOnClient,
			clusterInformerFactory.Cluster().V1().ManagedClusters(),
			clusterInformerFactory.Cluster().V1beta2().ManagedClusterSets(),
			workInformerFactory.Work().V1().ManifestWorks(),
			configInformerFactory.Submarineraddon().V1alpha1().SubmarinerConfigs(),
			addOnInformerFactory.Addon().V1alpha1().ClusterManagementAddOns(),
			addOnInformerFactory.Addon().V1alpha1().ManagedClusterAddOns(),
			addOnInformerFactory.Addon().V1alpha1().AddOnDeploymentConfigs(),
			events.NewLoggingEventRecorder("test"))

		var ctx context.Context

		ctx, t.stop = context.WithCancel(context.TODO())

		clusterInformerFactory.Start(ctx.Done())
		workInformerFactory.Start(ctx.Done())
		configInformerFactory.Start(ctx.Done())
		addOnInformerFactory.Start(ctx.Done())

		cache.WaitForCacheSync(ctx.Done(), clusterInformerFactory.Cluster().V1().ManagedClusters().Informer().HasSynced,
			workInformerFactory.Work().V1().ManifestWorks().Informer().HasSynced,
			configInformerFactory.Submarineraddon().V1alpha1().SubmarinerConfigs().Informer().HasSynced,
			addOnInformerFactory.Addon().V1alpha1().ManagedClusterAddOns().Informer().HasSynced)

		go controller.Run(ctx, 1)
	})

	AfterEach(func() {
		t.stop()
		t.mockCtrl.Finish()
	})

	return t
}

func (t *testDriver) initManifestWorks() {
	t.createManagedClusterSet()
	t.createAddonDeploymentConfig(t.defaultADConfig)
	t.createClusterManagementAddon()
	t.createManagedCluster()
	t.createAddon()
	t.createGlobalnetConfigMap()
	t.awaitManifestWorks()
	t.manifestWorkClient.Fake.ClearActions()
}

func (t *testDriver) testFinalizers() {
	It("should add finalizers to the ManagedClusterAddon and ManagedCluster", func() {
		test.AwaitFinalizer(resource.ForAddon(t.addOnClient.AddonV1alpha1().ManagedClusterAddOns(clusterName)),
			t.addOn.Name, submarineragent.AddOnFinalizer)
		test.AwaitFinalizer(resource.ForManagedCluster(t.clusterClient.ClusterV1().ManagedClusters()), clusterName,
			submarineragent.AgentFinalizer)
	})
}

func (t *testDriver) testAgentCleanup() {
	It("should delete the ManifestWorks", func() {
		t.awaitNoManifestWorks()
	})

	It("should delete the finalizers", func() {
		_, err := t.addOnClient.AddonV1alpha1().ManagedClusterAddOns(clusterName).Get(context.Background(), t.addOn.Name, metav1.GetOptions{})
		if !apierrors.IsNotFound(err) {
			test.AwaitNoFinalizer(resource.ForAddon(t.addOnClient.AddonV1alpha1().ManagedClusterAddOns(clusterName)),
				t.addOn.Name, submarineragent.AddOnFinalizer)
		}

		test.AwaitNoFinalizer(resource.ForManagedCluster(t.clusterClient.ClusterV1().ManagedClusters()), clusterName,
			submarineragent.AgentFinalizer)
	})

	It("should delete the RBAC resources for the managed cluster", func() {
		test.AwaitNoResource(coreresource.ForRoleBinding(t.kubeClient, brokerNamespace), "submariner-k8s-broker-cluster-"+clusterName)
		test.AwaitNoResource(coreresource.ForServiceAccount(t.kubeClient, brokerNamespace), clusterName)
	})
}

func (t *testDriver) awaitManifestWorks() {
	t.awaitOperatorManifestWork()
	t.awaitSubmarinerManifestWork()
}

func (t *testDriver) awaitOperatorManifestWork() []*unstructured.Unstructured {
	return t.assertOperatorManifestWork(test.AwaitResource(resource.ForManifestWork(
		t.manifestWorkClient.WorkV1().ManifestWorks(clusterName)), submarineragent.OperatorManifestWorkName).(*workv1.ManifestWork))
}

func (t *testDriver) assertOperatorManifestWork(work *workv1.ManifestWork) []*unstructured.Unstructured {
	manifestObjs := unmarshallManifestObjs(work)

	assertNoManifestObj(manifestObjs, "Submariner", "")

	subscription := assertManifestObj(manifestObjs, "Subscription", "")
	assertNestedString(subscription, installNamespace, "metadata", "namespace")
	assertNestedString(subscription, "submariner", "spec", "name")

	if t.submarinerConfig != nil {
		assertNestedString(subscription, t.submarinerConfig.Spec.SubscriptionConfig.Channel, "spec", "channel")
		assertNestedString(subscription, t.submarinerConfig.Spec.SubscriptionConfig.Source, "spec", "source")
		assertNestedString(subscription, t.submarinerConfig.Spec.SubscriptionConfig.SourceNamespace, "spec", "sourceNamespace")
		assertNestedString(subscription, t.submarinerConfig.Spec.SubscriptionConfig.StartingCSV, "spec", "startingCSV")
	}

	clusterRole := &rbacv1.ClusterRole{}
	Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(
		assertManifestObj(manifestObjs, "ClusterRole", "operatorgroups").Object, clusterRole)).To(Succeed())
	Expect(clusterRole.Rules).To(HaveLen(1))
	Expect(clusterRole.Rules[0].APIGroups).To(ContainElement("operators.coreos.com"))
	Expect(clusterRole.Rules[0].Resources).To(ContainElement("operatorgroups"))

	assertManifestObj(manifestObjs, "OperatorGroup", "")

	return manifestObjs
}

func (t *testDriver) awaitSubmarinerManifestWork() {
	t.assertSubmarinerManifestWork(test.AwaitResource(resource.ForManifestWork(
		t.manifestWorkClient.WorkV1().ManifestWorks(clusterName)), submarineragent.SubmarinerCRManifestWorkName).(*workv1.ManifestWork))
}

func (t *testDriver) assertSubmarinerManifestWork(work *workv1.ManifestWork) {
	manifestObjs := unmarshallManifestObjs(work)

	submariner := &submarinerv1alpha1.Submariner{}
	Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(
		assertManifestObj(manifestObjs, "Submariner", "").Object, submariner)).To(Succeed())
	Expect(submariner.Namespace).To(Equal(installNamespace))
	Expect(submariner.Spec.BrokerK8sApiServer).To(Equal("127.0.0.1"))
	Expect(submariner.Spec.BrokerK8sApiServerToken).To(Equal(brokerToken))
	Expect(submariner.Spec.BrokerK8sCA).To(Equal(base64.StdEncoding.EncodeToString([]byte(brokerCA))))
	Expect(submariner.Spec.BrokerK8sRemoteNamespace).To(Equal(brokerNamespace))
	Expect(submariner.Spec.CeIPSecPSK).To(Equal(base64.StdEncoding.EncodeToString([]byte(ipsecPSK))))
	Expect(submariner.Spec.ClusterID).To(Equal(clusterName))
	Expect(submariner.Spec.Namespace).To(Equal(installNamespace))

	adConfig := t.clusterADConfig

	if adConfig == nil {
		adConfig = t.defaultADConfig
	}

	Expect(submariner.Spec.NodeSelector).To(Equal(adConfig.Spec.NodePlacement.NodeSelector))
	Expect(submariner.Spec.Tolerations).To(Equal(adConfig.Spec.NodePlacement.Tolerations))

	if t.submarinerConfig != nil {
		Expect(submariner.Spec.CableDriver).To(Equal(t.submarinerConfig.Spec.CableDriver))
		Expect(submariner.Spec.CeIPSecNATTPort).To(Equal(t.submarinerConfig.Spec.IPSecNATTPort))
		Expect(submariner.Spec.NatEnabled).To(Equal(t.submarinerConfig.Spec.NATTEnable))
	}
}

func (t *testDriver) ensureNoManifestWorks() {
	Consistently(func() []workv1.ManifestWork {
		list, err := t.manifestWorkClient.WorkV1().ManifestWorks(clusterName).List(context.TODO(), metav1.ListOptions{})
		Expect(err).To(Succeed())

		return list.Items
	}).Should(BeEmpty(), "Found unexpected ManifestWorks")
}

func (t *testDriver) awaitNoManifestWorks() {
	t.awaitNoManifestWork(submarineragent.SubmarinerCRManifestWorkName)
	t.awaitNoManifestWork(submarineragent.OperatorManifestWorkName)

	deletedNames := []string{}
	last := ""

	actions := t.manifestWorkClient.Fake.Actions()
	for i := range actions {
		if actions[i].GetVerb() == "delete" && actions[i].GetResource().Resource == "manifestworks" &&
			actions[i].(testing.DeleteAction).GetName() != last {
			last = actions[i].(testing.DeleteAction).GetName()
			deletedNames = append(deletedNames, actions[i].(testing.DeleteAction).GetName())
		}
	}

	Expect(deletedNames).To(Equal([]string{submarineragent.SubmarinerCRManifestWorkName, submarineragent.OperatorManifestWorkName}))
}

func (t *testDriver) awaitNoManifestWork(name string) {
	Eventually(func() bool {
		_, err := t.manifestWorkClient.WorkV1().ManifestWorks(clusterName).Get(context.TODO(), name,
			metav1.GetOptions{})

		return apierrors.IsNotFound(err)
	}, 3).Should(BeTrue(), "Found unexpected ManifestWork")
}

func (t *testDriver) awaitClusterRBACResources() {
	roleBinding := test.AwaitResource(coreresource.ForRoleBinding(t.kubeClient, brokerNamespace),
		"submariner-k8s-broker-cluster-"+clusterName).(*rbacv1.RoleBinding)
	Expect(roleBinding.Subjects).To(HaveLen(1))
	Expect(roleBinding.Subjects[0].Kind).To(Equal("ServiceAccount"))
	Expect(roleBinding.Subjects[0].Name).To(Equal(clusterName))
	Expect(roleBinding.Subjects[0].Namespace).To(Equal(brokerNamespace))

	test.AwaitResource(coreresource.ForServiceAccount(t.kubeClient, brokerNamespace), clusterName)
}

func (t *testDriver) assertSCCManifestObjs(objs []*unstructured.Unstructured) {
	clusterRole := &rbacv1.ClusterRole{}
	Expect(runtime.DefaultUnstructuredConverter.FromUnstructured(
		assertManifestObj(objs, "ClusterRole", "scc").Object, clusterRole)).To(Succeed())
	Expect(clusterRole.Rules).To(HaveLen(1))
	Expect(clusterRole.Rules[0].APIGroups).To(ContainElement("security.openshift.io"))
	Expect(clusterRole.Rules[0].Resources).To(ContainElement("securitycontextconstraints"))
}

func (t *testDriver) awaitBackupLabelOnBroker() {
	Eventually(func() bool {
		broker, err := t.dynamicClient.Resource(submarineragent.BrokerGVR).Namespace(brokerNamespace).Get(context.TODO(),
			submarineragent.BrokerObjectName, metav1.GetOptions{})
		if err != nil {
			return false
		}

		labels := broker.GetLabels()
		if labels == nil {
			return false
		}
		_, ok := labels[submarineragent.BackupLabelKey]

		return ok
	}).Should(BeTrue(), "Backup label missing on submariner-broker")
}

func (t *testDriver) createSubmarinerConfig() {
	t.submarinerConfig = &configv1alpha1.SubmarinerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: constants.SubmarinerConfigName,
		},
		Spec: configv1alpha1.SubmarinerConfigSpec{
			CableDriver:   "vxlan",
			IPSecNATTPort: 202,
			NATTEnable:    true,
			SubscriptionConfig: configv1alpha1.SubscriptionConfig{
				Source:          "test-source",
				SourceNamespace: "test-source-ns",
				Channel:         "test-channel",
				StartingCSV:     "test-starting-CSV",
			},
		},
	}

	_, err := t.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(clusterName).Create(context.TODO(),
		t.submarinerConfig, metav1.CreateOptions{})
	Expect(err).To(Succeed())
}

func (t *testDriver) createManagedCluster() {
	_, err := t.clusterClient.ClusterV1().ManagedClusters().Create(context.TODO(), t.managedCluster, metav1.CreateOptions{})
	Expect(err).To(Succeed())
}

func (t *testDriver) createAddonDeploymentConfig(config *addonv1alpha1.AddOnDeploymentConfig) {
	_, err := t.addOnClient.AddonV1alpha1().AddOnDeploymentConfigs(config.Namespace).Create(context.TODO(), config, metav1.CreateOptions{})
	Expect(err).To(Succeed())
}

func (t *testDriver) createAddonDeploymentConfigForCluster(cluster string) {
	t.clusterADConfig = &addonv1alpha1.AddOnDeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "submariner-adconfig",
			Namespace: cluster,
		},
		Spec: addonv1alpha1.AddOnDeploymentConfigSpec{
			NodePlacement: &addonv1alpha1.NodePlacement{
				NodeSelector: map[string]string{"foo": cluster},
				Tolerations: []corev1.Toleration{
					{
						Key:   "cluster-key",
						Value: "cluster-value",
					},
				},
			},
		},
	}

	t.createAddonDeploymentConfig(t.clusterADConfig)
}

func (t *testDriver) createClusterManagementAddon() {
	_, err := t.addOnClient.AddonV1alpha1().ClusterManagementAddOns().Create(context.TODO(), t.clusterMgmtAddon, metav1.CreateOptions{})
	Expect(err).To(Succeed())
}

func (t *testDriver) createManagedClusterSet() {
	mcs := &clusterv1beta2.ManagedClusterSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterSetName,
		},
	}

	_, err := t.clusterClient.ClusterV1beta2().ManagedClusterSets().Create(context.TODO(), mcs, metav1.CreateOptions{})
	Expect(err).To(Succeed())
}

func (t *testDriver) createAddon() {
	_, err := t.addOnClient.AddonV1alpha1().ManagedClusterAddOns(clusterName).Create(context.TODO(), t.addOn, metav1.CreateOptions{})
	Expect(err).To(Succeed())
}

func (t *testDriver) createGlobalnetConfigMap() {
	err := globalnet.CreateConfigMap(context.TODO(), t.controllerClient, false, "", 0, brokerNamespace)
	Expect(err).To(Succeed())
}

func (t *testDriver) createSubmarinerBroker() {
	t.broker = &unstructured.Unstructured{}
	t.broker.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "submariner.io",
		Version: "v1alpha1",
		Kind:    "Broker",
	})
	t.broker.SetNamespace(brokerNamespace)
	t.broker.SetName(submarineragent.BrokerObjectName)
	err := unstructured.SetNestedField(t.broker.Object, "false", "spec", "globalnetEnabled")

	Expect(err).To(Succeed())
	_, err = t.dynamicClient.Resource(submarineragent.BrokerGVR).Namespace(brokerNamespace).Create(context.TODO(),
		t.broker, metav1.CreateOptions{})
	Expect(err).To(Succeed())
}

func (t *testDriver) deleteGlobalnetConfigMap() {
	err := globalnet.DeleteConfigMap(context.TODO(), t.controllerClient, brokerNamespace)
	Expect(err).To(Succeed())
}

func (t *testDriver) setClusterADConfig() {
	t.addOn.Spec.Configs = []addonv1alpha1.AddOnConfig{
		{
			ConfigGroupResource: addonv1alpha1.ConfigGroupResource{
				Group:    "addon.open-cluster-management.io",
				Resource: "addondeploymentconfigs",
			},
			ConfigReferent: addonv1alpha1.ConfigReferent{
				Name:      t.clusterADConfig.Name,
				Namespace: t.clusterADConfig.Namespace,
			},
		},
	}
}

func unmarshallManifestObjs(work *workv1.ManifestWork) []*unstructured.Unstructured {
	manifestObjs := make([]*unstructured.Unstructured, len(work.Spec.Workload.Manifests))

	for i := range work.Spec.Workload.Manifests {
		obj := &unstructured.Unstructured{}
		err := obj.UnmarshalJSON(work.Spec.Workload.Manifests[i].Raw)
		Expect(err).To(Succeed())

		manifestObjs[i] = obj
	}

	return manifestObjs
}

func assertManifestObj(objs []*unstructured.Unstructured, kind, name string) *unstructured.Unstructured {
	for _, o := range objs {
		if o.GetKind() == kind && (name == "" || strings.Contains(o.GetName(), name)) {
			return o
		}
	}

	Fail(fmt.Sprintf("Expected manifest resource with kind %q and name %q", kind, name))

	return nil
}

func assertNoManifestObj(objs []*unstructured.Unstructured, kind, name string) {
	for _, o := range objs {
		if o.GetKind() == kind && (name == "" || strings.Contains(o.GetName(), name)) {
			Fail(fmt.Sprintf("Unexpected manifest resource with kind %q and name %q", kind, name))
		}
	}
}

func assertNestedString(obj *unstructured.Unstructured, expected string, fields ...string) {
	actual, ok, err := unstructured.NestedString(obj.Object, fields...)
	Expect(err).To(Succeed())
	Expect(ok).To(BeTrue(), "Nested field %q not found", strings.Join(fields, "/"))
	Expect(actual).To(Equal(expected), "Unexpected value for nested field %q", strings.Join(fields, "/"))
}
