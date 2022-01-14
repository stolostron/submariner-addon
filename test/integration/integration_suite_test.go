package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	"github.com/openshift/library-go/pkg/controller/controllercmd"

	addonclientset "github.com/open-cluster-management/api/client/addon/clientset/versioned"
	clusterclientset "github.com/open-cluster-management/api/client/cluster/clientset/versioned"
	workclientset "github.com/open-cluster-management/api/client/work/clientset/versioned"
	configclientset "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	"github.com/stolostron/submariner-addon/pkg/hub"
	"github.com/stolostron/submariner-addon/test/util"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestIntegration(t *testing.T) {
	gomega.RegisterFailHandler(ginkgo.Fail)
	ginkgo.RunSpecsWithDefaultAndCustomReporters(t, "Integration Suite", []ginkgo.Reporter{printer.NewlineReporter{}})
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

var kubeClient kubernetes.Interface
var clusterClient clusterclientset.Interface
var workClient workclientset.Interface
var configClinet configclientset.Interface
var addOnClient addonclientset.Interface
var dynamicClient dynamic.Interface

var _ = ginkgo.BeforeSuite(func(done ginkgo.Done) {
	logf.SetLogger(zap.New(zap.WriteTo(ginkgo.GinkgoWriter), zap.UseDevMode(true)))

	ginkgo.By("bootstrapping test environment")

	var err error

	// set api server env
	err = os.Setenv("BROKER_API_SERVER", "127.0.0.1")
	gomega.Expect(err).ToNot(gomega.HaveOccurred())

	// install cluster and work CRDs and start a local kube-apiserver
	testEnv = &envtest.Environment{
		ErrorIfCRDPathMissing: true,
		CRDDirectoryPaths: []string{
			filepath.Join(".", "vendor", "github.com", "open-cluster-management", "api", "cluster", "v1"),
			filepath.Join(".", "vendor", "github.com", "open-cluster-management", "api", "cluster", "v1alpha1", clustersetCRD),
			filepath.Join(".", "vendor", "github.com", "open-cluster-management", "api", "work", "v1", workCRD),
			filepath.Join(".", "vendor", "github.com", "open-cluster-management", "api", "addon", "v1alpha1"),
			filepath.Join(".", "pkg", "apis", "submarinerconfig", "v1alpha1"),
			filepath.Join(".", "test", "integration", "crds", "submariner"),
		},
	}

	cfg, err = testEnv.Start()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	gomega.Expect(cfg).ToNot(gomega.BeNil())

	// prepare clients
	kubeClient, err = kubernetes.NewForConfig(cfg)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	gomega.Expect(kubeClient).ToNot(gomega.BeNil())

	clusterClient, err = clusterclientset.NewForConfig(cfg)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	gomega.Expect(clusterClient).ToNot(gomega.BeNil())

	workClient, err = workclientset.NewForConfig(cfg)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	gomega.Expect(clusterClient).ToNot(gomega.BeNil())

	configClinet, err = configclientset.NewForConfig(cfg)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	gomega.Expect(configClinet).ToNot(gomega.BeNil())

	addOnClient, err = addonclientset.NewForConfig(cfg)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	gomega.Expect(addOnClient).ToNot(gomega.BeNil())

	dynamicClient, err = dynamic.NewForConfig(cfg)
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
	gomega.Expect(dynamicClient).ToNot(gomega.BeNil())

	// prepare open-cluster-management namespaces
	_, err = kubeClient.CoreV1().Namespaces().Create(context.Background(), util.NewManagedClusterNamespace("open-cluster-management"), metav1.CreateOptions{})
	gomega.Expect(err).NotTo(gomega.HaveOccurred())

	// start submariner broker and agent controller
	go func() {
		addOnOptions := hub.AddOnOptions{AgentImage: "test"}
		err := addOnOptions.RunControllerManager(context.Background(), &controllercmd.ControllerContext{
			KubeConfig:    cfg,
			EventRecorder: util.NewIntegrationTestEventRecorder("submariner-addon"),
		})
		gomega.Expect(err).NotTo(gomega.HaveOccurred())
	}()

	close(done)
}, 60)

var _ = ginkgo.AfterSuite(func() {
	ginkgo.By("tearing down the test environment")
	err := testEnv.Stop()
	gomega.Expect(err).ToNot(gomega.HaveOccurred())
})
