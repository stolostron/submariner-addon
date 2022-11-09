package integration_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	configclientset "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	"github.com/stolostron/submariner-addon/pkg/hub"
	"github.com/stolostron/submariner-addon/test/util"
	suboperatorapi "github.com/submariner-io/submariner-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	addonclientset "open-cluster-management.io/api/client/addon/clientset/versioned"
	clusterclientset "open-cluster-management.io/api/client/cluster/clientset/versioned"
	workclientset "open-cluster-management.io/api/client/work/clientset/versioned"
	clusterV1 "open-cluster-management.io/api/cluster/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
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

var testEnv *envtest.Environment

var cfg *rest.Config

var (
	kubeClient       kubernetes.Interface
	clusterClient    clusterclientset.Interface
	controllerClient client.Client
	workClient       workclientset.Interface
	configClinet     configclientset.Interface
	addOnClient      addonclientset.Interface
	dynamicClient    dynamic.Interface
)

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

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
			filepath.Join(".", "vendor", "open-cluster-management.io", "api", "cluster", "v1beta1", clustersetCRD),
			filepath.Join(".", "vendor", "open-cluster-management.io", "api", "work", "v1", workCRD),
			filepath.Join(".", "vendor", "open-cluster-management.io", "api", "addon", "v1alpha1"),
			filepath.Join(".", "pkg", "apis", "submarinerconfig", "v1alpha1"),
			filepath.Join(".", "test", "integration", "crds", "submariner"),
		},
	}

	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	Expect(suboperatorapi.AddToScheme(scheme.Scheme)).To(Succeed())

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
	_, err = kubeClient.CoreV1().Namespaces().Create(context.Background(), util.NewManagedClusterNamespace("open-cluster-management"),
		metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	// create local cluster with URL in ClientConfig
	localCluster := util.NewManagedCluster("local-cluster", map[string]string{})
	localCluster.Spec.ManagedClusterClientConfigs = []clusterV1.ClientConfig{
		{
			URL: "https://dummy:443",
		},
	}
	_, err = clusterClient.ClusterV1().ManagedClusters().Create(context.Background(), localCluster, metav1.CreateOptions{})
	Expect(err).NotTo(HaveOccurred())

	// start submariner broker and agent controller
	go func() {
		addOnOptions := hub.AddOnOptions{AgentImage: "test"}
		err := addOnOptions.RunControllerManager(context.Background(), &controllercmd.ControllerContext{
			KubeConfig:    cfg,
			EventRecorder: util.NewIntegrationTestEventRecorder("submariner-addon"),
		})
		Expect(err).NotTo(HaveOccurred())
	}()
})

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
