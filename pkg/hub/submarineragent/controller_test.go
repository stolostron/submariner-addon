package submarineragent_test

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/golang/mock/gomock"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	configclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	fakeconfigclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned/fake"
	configinformers "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/informers/externalversions"
	cloudFake "github.com/open-cluster-management/submariner-addon/pkg/cloud/fake"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"
	"github.com/open-cluster-management/submariner-addon/pkg/hub/submarineragent"
	"github.com/open-cluster-management/submariner-addon/pkg/resource"
	"github.com/openshift/library-go/pkg/operator/events"
	coreresource "github.com/submariner-io/admiral/pkg/resource"
	"github.com/submariner-io/admiral/pkg/test"
	submarinerv1alpha1 "github.com/submariner-io/submariner-operator/api/submariner/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	kubefake "k8s.io/client-go/kubernetes/fake"
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
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	workv1 "open-cluster-management.io/api/work/v1"
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
				t.ensureNoManifestWork()

				t.createManagedCluster()
				t.ensureNoManifestWork()

				t.createAddon()
			})

			It("should deploy the submariner ManifestWork", func() {
				t.awaitManifestWork()
			})

			It("should create RBAC resources for the managed cluster", func() {
				t.awaitClusterRBACResources()
			})

			t.testFinalizers()

			Context("and the SubmarinerConfig is present", func() {
				BeforeEach(func() {
					t.createSubmarinerConfig()
				})

				It("should deploy the submariner ManifestWork with the SubmarinerConfig overrides", func() {
					t.awaitManifestWork()

					expCond := &metav1.Condition{
						Type:   configv1alpha1.SubmarinerConfigConditionApplied,
						Status: metav1.ConditionTrue,
						Reason: "SubmarinerConfigApplied",
					}

					test.AwaitStatusCondition(expCond, func() ([]metav1.Condition, error) {
						config, err := t.configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(clusterName).Get(context.TODO(),
							helpers.SubmarinerConfigName, metav1.GetOptions{})
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
							Value: helpers.ProductOCP,
						},
					}
				})

				It("should deploy the submariner ManifestWork with the Openshift security resources", func() {
					t.assertSCCManifestObjs(t.awaitManifestWork())
				})
			})
		})

		Context("before the ManagedCluster and ManagedClusterSet", func() {
			JustBeforeEach(func() {
				t.createAddon()
				t.ensureNoManifestWork()

				t.createManagedCluster()
				t.ensureNoManifestWork()

				t.createManagedClusterSet()
			})

			It("should deploy the submariner ManifestWork", func() {
				t.awaitManifestWork()
			})

			t.testFinalizers()
		})
	})

	When("the SubmarinerConfig is created after the submariner ManifestWork is deployed", func() {
		JustBeforeEach(func() {
			t.initManifestWork()
			t.createSubmarinerConfig()
		})

		It("should update the ManifestWork", func() {
			t.assertManifestWork(test.AwaitUpdateAction(&t.manifestWorkClient.Fake, "manifestworks",
				submarineragent.ManifestWorkName).(*workv1.ManifestWork))
		})
	})

	When("the ManagedClusterAddon is being deleted", func() {
		JustBeforeEach(func() {
			t.initManifestWork()
			test.SetDeleting(resource.ForAddon(t.addOnClient.AddonV1alpha1().ManagedClusterAddOns(clusterName)), t.addOn.Name)
		})

		t.testAgentCleanup()
	})

	When("the ManagedCluster is being deleted", func() {
		JustBeforeEach(func() {
			t.initManifestWork()
			test.SetDeleting(resource.ForManagedCluster(t.clusterClient.ClusterV1().ManagedClusters()), t.managedCluster.Name)
		})

		t.testAgentCleanup()
	})

	When("the ManagedCluster is removed from the ManagedClusterSet", func() {
		JustBeforeEach(func() {
			t.initManifestWork()

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
	submarinerConfig   *configv1alpha1.SubmarinerConfig
	kubeClient         kubernetes.Interface
	dynamicClient      dynamic.Interface
	clusterClient      clusterclient.Interface
	manifestWorkClient *fakeworkclient.Clientset
	configClient       configclient.Interface
	addOnClient        addonclient.Interface
	stop               context.CancelFunc
	mockCtrl           *gomock.Controller
}

func newTestDriver() *testDriver {
	t := &testDriver{}

	BeforeEach(func() {
		t.managedCluster = &clusterv1.ManagedCluster{
			ObjectMeta: metav1.ObjectMeta{
				Name:   clusterName,
				Labels: map[string]string{submarineragent.ClusterSetLabel: clusterSetName},
			},
		}

		t.addOn = &addonv1alpha1.ManagedClusterAddOn{
			ObjectMeta: metav1.ObjectMeta{
				Name:      helpers.SubmarinerAddOnName,
				Namespace: clusterName,
			},
			Spec: addonv1alpha1.ManagedClusterAddOnSpec{
				InstallNamespace: installNamespace,
			},
		}

		t.submarinerConfig = nil
		t.mockCtrl = gomock.NewController(GinkgoT())
		t.dynamicClient = dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
		t.clusterClient = fakeclusterclient.NewSimpleClientset()
		t.manifestWorkClient = fakeworkclient.NewSimpleClientset()
		t.configClient = fakeconfigclient.NewSimpleClientset()
		t.addOnClient = addonfake.NewSimpleClientset()

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

		cloudProvider := cloudFake.NewMockProvider(t.mockCtrl)
		cloudProvider.EXPECT().PrepareSubmarinerClusterEnv().Return(nil).AnyTimes()
		cloudProvider.EXPECT().CleanUpSubmarinerClusterEnv().Return(nil).AnyTimes()

		providerFactory := cloudFake.NewMockProviderFactory(t.mockCtrl)
		providerFactory.EXPECT().Get(gomock.Any(), gomock.Any(), gomock.Any()).Return(cloudProvider, nil).AnyTimes()

		controller := submarineragent.NewSubmarinerAgentController(t.kubeClient, t.dynamicClient, t.clusterClient,
			t.manifestWorkClient, t.configClient, t.addOnClient,
			clusterInformerFactory.Cluster().V1().ManagedClusters(),
			clusterInformerFactory.Cluster().V1beta1().ManagedClusterSets(),
			workInformerFactory.Work().V1().ManifestWorks(),
			configInformerFactory.Submarineraddon().V1alpha1().SubmarinerConfigs(),
			addOnInformerFactory.Addon().V1alpha1().ManagedClusterAddOns(),
			providerFactory, events.NewLoggingEventRecorder("test"))

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

func (t *testDriver) initManifestWork() {
	t.createManagedClusterSet()
	t.createManagedCluster()
	t.createAddon()
	t.awaitManifestWork()
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
	It("should delete the submariner ManifestWork", func() {
		t.awaitNoManifestWork()
	})

	It("should delete the finalizers", func() {
		test.AwaitNoFinalizer(resource.ForAddon(t.addOnClient.AddonV1alpha1().ManagedClusterAddOns(clusterName)),
			t.addOn.Name, submarineragent.AddOnFinalizer)
		test.AwaitNoFinalizer(resource.ForManagedCluster(t.clusterClient.ClusterV1().ManagedClusters()), clusterName,
			submarineragent.AgentFinalizer)
	})

	It("should delete the RBAC resources for the managed cluster", func() {
		test.AwaitNoResource(coreresource.ForRoleBinding(t.kubeClient, brokerNamespace), "submariner-k8s-broker-cluster-"+clusterName)
		test.AwaitNoResource(coreresource.ForServiceAccount(t.kubeClient, brokerNamespace), clusterName)
	})
}

func (t *testDriver) awaitManifestWork() []*unstructured.Unstructured {
	return t.assertManifestWork(test.AwaitResource(resource.ForManifestWork(t.manifestWorkClient.WorkV1().ManifestWorks(clusterName)),
		submarineragent.ManifestWorkName).(*workv1.ManifestWork))
}

func (t *testDriver) assertManifestWork(work *workv1.ManifestWork) []*unstructured.Unstructured {
	manifestObjs := make([]*unstructured.Unstructured, len(work.Spec.Workload.Manifests))

	for i := range work.Spec.Workload.Manifests {
		obj := &unstructured.Unstructured{}
		err := obj.UnmarshalJSON(work.Spec.Workload.Manifests[i].Raw)
		Expect(err).To(Succeed())

		manifestObjs[i] = obj
	}

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

	if t.submarinerConfig != nil {
		Expect(submariner.Spec.CableDriver).To(Equal(t.submarinerConfig.Spec.CableDriver))
		Expect(submariner.Spec.CeIPSecIKEPort).To(Equal(t.submarinerConfig.Spec.IPSecIKEPort))
		Expect(submariner.Spec.CeIPSecNATTPort).To(Equal(t.submarinerConfig.Spec.IPSecNATTPort))
		Expect(submariner.Spec.NatEnabled).To(Equal(t.submarinerConfig.Spec.NATTEnable))
	}

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

func (t *testDriver) ensureNoManifestWork() {
	Consistently(func() bool {
		_, err := t.manifestWorkClient.WorkV1().ManifestWorks(clusterName).Get(context.TODO(), submarineragent.ManifestWorkName,
			metav1.GetOptions{})

		return apierrors.IsNotFound(err)
	}).Should(BeTrue(), "Found unexpected ManifestWork")
}

func (t *testDriver) awaitNoManifestWork() {
	Eventually(func() bool {
		_, err := t.manifestWorkClient.WorkV1().ManifestWorks(clusterName).Get(context.TODO(), submarineragent.ManifestWorkName,
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

	assertManifestObj(objs, "SecurityContextConstraints", "")
}

func (t *testDriver) createSubmarinerConfig() {
	t.submarinerConfig = &configv1alpha1.SubmarinerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name: helpers.SubmarinerConfigName,
		},
		Spec: configv1alpha1.SubmarinerConfigSpec{
			CableDriver:   "vxlan",
			IPSecIKEPort:  101,
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

func (t *testDriver) createManagedClusterSet() {
	mcs := &clusterv1beta1.ManagedClusterSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: clusterSetName,
		},
	}

	_, err := t.clusterClient.ClusterV1beta1().ManagedClusterSets().Create(context.TODO(), mcs, metav1.CreateOptions{})
	Expect(err).To(Succeed())
}

func (t *testDriver) createAddon() {
	_, err := t.addOnClient.AddonV1alpha1().ManagedClusterAddOns(clusterName).Create(context.TODO(), t.addOn, metav1.CreateOptions{})
	Expect(err).To(Succeed())
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

func assertNestedString(obj *unstructured.Unstructured, expected string, fields ...string) {
	actual, ok, err := unstructured.NestedString(obj.Object, fields...)
	Expect(err).To(Succeed())
	Expect(ok).To(BeTrue(), "Nested field %q not found", strings.Join(fields, "/"))
	Expect(actual).To(Equal(expected), "Unexpected value for nested field %q", strings.Join(fields, "/"))
}
