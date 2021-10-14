package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	configclientset "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	"github.com/open-cluster-management/submariner-addon/pkg/hub"
	"github.com/open-cluster-management/submariner-addon/test/util"
	"github.com/openshift/library-go/pkg/controller/controllercmd"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	addonclientset "open-cluster-management.io/api/client/addon/clientset/versioned"
	clusterclientset "open-cluster-management.io/api/client/cluster/clientset/versioned"
	workclientset "open-cluster-management.io/api/client/work/clientset/versioned"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestIntegration(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecsWithDefaultAndCustomReporters(t, "Integration Suite", []Reporter{printer.NewlineReporter{}})
}

const (
	eventuallyTimeout  = 30 // seconds
	eventuallyInterval = 1  // seconds
)

const (
	// fixed a specified crd to avoid conflict error when set up test env
	clustersetCRD = "0000_00_clusters.open-cluster-management.io_managedclustersets.crd.yaml"
	workCRD       = "0000_00_work.open-cluster-management.io_manifestworks.crd.yaml"
)

var testEnv *envtest.Environment

var cfg *rest.Config

var (
	kubeClient    kubernetes.Interface
	clusterClient clusterclientset.Interface
	workClient    workclientset.Interface
	configClinet  configclientset.Interface
	addOnClient   addonclientset.Interface
	dynamicClient dynamic.Interface
)

var _ = BeforeSuite(func(done Done) {
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

	// prepare open-cluster-management namespaces
	_, err = kubeClient.CoreV1().Namespaces().Create(context.Background(), util.NewManagedClusterNamespace("open-cluster-management"),
		metav1.CreateOptions{})
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

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
