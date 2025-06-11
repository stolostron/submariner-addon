package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	configclientset "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	"github.com/stolostron/submariner-addon/pkg/constants"
	"github.com/stolostron/submariner-addon/pkg/hub"
	"github.com/stolostron/submariner-addon/pkg/resource"
	"github.com/stolostron/submariner-addon/test/util"
	"github.com/submariner-io/admiral/pkg/log/kzerolog"
	admutil "github.com/submariner-io/admiral/pkg/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonclientset "open-cluster-management.io/api/client/addon/clientset/versioned"
	clusterclientset "open-cluster-management.io/api/client/cluster/clientset/versioned"
	workclientset "open-cluster-management.io/api/client/work/clientset/versioned"
	clusterV1 "open-cluster-management.io/api/cluster/v1"
	clusterv1beta2 "open-cluster-management.io/api/cluster/v1beta2"
	workv1 "open-cluster-management.io/api/work/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Integration Suite")
}

const (
	eventuallyTimeout  = 30 // seconds
	eventuallyInterval = 1  // seconds
)

const (
	// fixed a specified crd to avoid conflict error when set up test env.
	clustersetCRD = "0000_00_clusters.open-cluster-management.io_managedclustersets.crd.yaml"
	workCRD       = "0000_00_work.open-cluster-management.io_manifestworks.crd.yaml"
)

var (
	testEnv          *envtest.Environment
	cfg              *rest.Config
	kubeClient       kubernetes.Interface
	clusterClient    clusterclientset.Interface
	controllerClient client.Client
	workClient       workclientset.Interface
	configClinet     configclientset.Interface
	addOnClient      addonclientset.Interface
	dynamicClient    dynamic.Interface
)

var _ = BeforeSuite(func() {
	kzerolog.InitK8sLogging()

	By("bootstrapping test environment")

	var err error

	// set api server env
	err = os.Setenv("BROKER_API_SERVER", "127.0.0.1")
	Expect(err).ToNot(HaveOccurred())

	// install cluster and work CRDs and start a local kube-apiserver
	testEnv = &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths: []string{
			filepath.Join(".", "vendor", "open-cluster-management.io", "api", "cluster", "v1"),
			filepath.Join(".", "vendor", "open-cluster-management.io", "api", "cluster", "v1beta2", clustersetCRD),
			filepath.Join(".", "vendor", "open-cluster-management.io", "api", "work", "v1", workCRD),
			filepath.Join(".", "vendor", "open-cluster-management.io", "api", "addon", "v1alpha1"),
			filepath.Join(".", "pkg", "apis", "submarinerconfig", "v1alpha1"),
			filepath.Join(".", "test", "integration", "crds", "submariner"),
		},
	}

	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	// prepare clients
	kubeClient, err = kubernetes.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred())
	Expect(kubeClient).ToNot(BeNil())

	clusterClient, err = clusterclientset.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred())
	Expect(clusterClient).ToNot(BeNil())

	workClient, err = workclientset.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred())
	Expect(clusterClient).ToNot(BeNil())

	configClinet, err = configclientset.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred())
	Expect(configClinet).ToNot(BeNil())

	addOnClient, err = addonclientset.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred())
	Expect(addOnClient).ToNot(BeNil())

	dynamicClient, err = dynamic.NewForConfig(cfg)
	Expect(err).ToNot(HaveOccurred())
	Expect(dynamicClient).ToNot(BeNil())

	controllerClient, err = client.New(cfg, client.Options{})
	Expect(err).ToNot(HaveOccurred())
	Expect(controllerClient).ToNot(BeNil())

	// prepare open-cluster-management namespaces
	_, err = kubeClient.CoreV1().Namespaces().Create(context.Background(), util.NewNamespace("open-cluster-management"),
		metav1.CreateOptions{})
	if !apierrors.IsAlreadyExists(err) {
		Expect(err).NotTo(HaveOccurred())
	}

	// create local cluster with URL in ClientConfig
	localCluster := util.NewManagedCluster("cluster-local", map[string]string{"local-cluster": "true"})
	localCluster.Spec.ManagedClusterClientConfigs = []clusterV1.ClientConfig{
		{
			URL: "https://dummy:443",
		},
	}
	_, err = clusterClient.ClusterV1().ManagedClusters().Create(context.Background(), localCluster, metav1.CreateOptions{})
	if !apierrors.IsAlreadyExists(err) {
		Expect(err).NotTo(HaveOccurred())
	}
})

var _ = AfterSuite(func() {
	By("Tearing down the test environment")

	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})

func deployManagedClusterSet() (string, string) {
	managedClusterSetName := "set-" + rand.String(6)
	brokerNamespace := managedClusterSetName + "-broker"

	By("Create a ManagedClusterSet")

	_, err := clusterClient.ClusterV1beta2().ManagedClusterSets().Create(context.Background(),
		util.NewManagedClusterSet(managedClusterSetName), metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	By("Check if the submariner broker is deployed")

	Eventually(func() bool {
		return util.CheckBrokerResources(kubeClient, brokerNamespace, true)
	}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())

	return managedClusterSetName, brokerNamespace
}

func deployManagedClusterWithAddOn(managedClusterSetName, managedClusterName, brokerNamespace string) {
	By("Create a ManagedCluster")

	managedCluster := util.NewManagedCluster(managedClusterName, map[string]string{
		clusterv1beta2.ClusterSetLabel: managedClusterSetName,
	})
	_, err := clusterClient.ClusterV1().ManagedClusters().Create(context.Background(), managedCluster, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	By("Create the ManagedCluster's namespace")

	_, err = kubeClient.CoreV1().Namespaces().Create(context.Background(), util.NewNamespace(managedClusterName),
		metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	By("Create the brokers.submariner.io config in the broker namespace")

	createBrokerConfiguration(brokerNamespace)

	By("Create a submariner ManagedClusterAddOn")

	cma, err := addOnClient.AddonV1alpha1().ClusterManagementAddOns().Get(context.Background(), constants.SubmarinerAddOnName,
		metav1.GetOptions{})
	Expect(err).NotTo(HaveOccurred())

	addOn := util.NewManagedClusterAddOn(managedClusterName)
	addOn.OwnerReferences = []metav1.OwnerReference{
		*metav1.NewControllerRef(cma, addonv1alpha1.GroupVersion.WithKind("ClusterManagementAddOn")),
	}

	_, err = addOnClient.AddonV1alpha1().ManagedClusterAddOns(managedClusterName).Create(context.Background(), addOn, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	By("Setup the service account")

	err = util.SetupServiceAccount(kubeClient, brokerNamespace, managedClusterName)
	Expect(err).NotTo(HaveOccurred())
}

func startControllerManager() func() {
	ctx, stop := context.WithCancel(context.Background())

	go func() {
		defer GinkgoRecover()

		createClusterManagementAddOn(ctx)

		addOnOptions := hub.AddOnOptions{AgentImage: "test"}
		err := addOnOptions.RunControllerManager(ctx, &controllercmd.ControllerContext{
			KubeConfig:    cfg,
			EventRecorder: util.NewIntegrationTestEventRecorder("submariner-addon"),
		})
		Expect(err).NotTo(HaveOccurred())
	}()

	return func() {
		stop()

		err := admutil.Update(context.Background(), resource.ForClusterAddon(addOnClient.AddonV1alpha1().ClusterManagementAddOns()),
			&addonv1alpha1.ClusterManagementAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name: constants.SubmarinerAddOnName,
				},
			}, func(existing *addonv1alpha1.ClusterManagementAddOn) (*addonv1alpha1.ClusterManagementAddOn, error) {
				existing.Finalizers = nil
				return existing, nil
			})
		if !apierrors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred())
		}

		err = addOnClient.AddonV1alpha1().ClusterManagementAddOns().Delete(context.Background(), constants.SubmarinerAddOnName,
			metav1.DeleteOptions{})
		if !apierrors.IsNotFound(err) {
			Expect(err).NotTo(HaveOccurred())
		}
	}
}

func createClusterManagementAddOn(ctx context.Context) {
	data, err := os.ReadFile(filepath.Join(".", "deploy", "olm-catalog", "manifests", "submarineraddon.cluster-management-addon.yaml"))
	Expect(err).To(Succeed())

	cma := &addonv1alpha1.ClusterManagementAddOn{}
	err = yaml.Unmarshal(data, cma)

	// Adding installStrategy for testing purposes.
	cma.Spec.InstallStrategy = addonv1alpha1.InstallStrategy{
		Type: "Manual",
	}

	Expect(err).To(Succeed())

	_, err = addOnClient.AddonV1alpha1().ClusterManagementAddOns().Create(ctx, cma, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())
}

func awaitManifestWorks(workClient workclientset.Interface, managedClusterName string, works ...string,
) []*workv1.ManifestWork {
	var (
		actual []*workv1.ManifestWork
		ok     bool
	)

	Eventually(func() bool {
		ok, actual = util.CheckManifestWorks(workClient, managedClusterName, true, works...)
		return ok
	}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())

	return actual
}

func awaitNoManifestWorks(workClient workclientset.Interface, managedClusterName string, works ...string) {
	Eventually(func() bool {
		ok, _ := util.CheckManifestWorks(workClient, managedClusterName, false, works...)
		return ok
	}, eventuallyTimeout, eventuallyInterval).Should(BeTrue())
}
