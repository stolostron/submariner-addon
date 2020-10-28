package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"

	"github.com/openshift/library-go/pkg/controller/controllercmd"

	clusterclientset "github.com/open-cluster-management/api/client/cluster/clientset/versioned"
	workclientset "github.com/open-cluster-management/api/client/work/clientset/versioned"
	"github.com/open-cluster-management/submariner-addon/pkg/hub"
	"github.com/open-cluster-management/submariner-addon/test/util"

	"k8s.io/client-go/kubernetes"

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

var testEnv *envtest.Environment

var kubeClient kubernetes.Interface
var clusterClient clusterclientset.Interface
var workClient workclientset.Interface

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
			filepath.Join(".", "vendor", "github.com", "open-cluster-management", "api", "cluster", "v1alpha1"),
			filepath.Join(".", "vendor", "github.com", "open-cluster-management", "api", "work", "v1"),
		},
	}

	cfg, err := testEnv.Start()
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

	// start submariner broker and agent controller
	go func() {
		err := hub.RunControllerManager(context.Background(), &controllercmd.ControllerContext{
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
