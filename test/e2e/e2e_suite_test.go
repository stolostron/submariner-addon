package e2e

import (
	"flag"
	. "github.com/onsi/ginkgo"
	"github.com/onsi/ginkgo/reporters"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/submariner-addon/test/utils"

	clusterclient "github.com/open-cluster-management/api/client/cluster/clientset/versioned"
	workclient "github.com/open-cluster-management/api/client/work/clientset/versioned"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"os"
	"testing"
)

const (
	eventuallyTimeout  = 30 // seconds
	eventuallyInterval = 1  // seconds
)

var (
	kubeConfig           string
	reportFile           string
	testOptions          utils.TestOptions
	testOptionsContainer utils.TestOptionsContainer
	optionsFile          string
	hubNamespace         string
	managedClusterName   string
	kubeClient           kubernetes.Interface
	clusterClient        clusterclient.Interface
	workClient           workclient.Interface
)

func init() {
	klog.SetOutput(GinkgoWriter)
	klog.InitFlags(nil)

	flag.StringVar(&reportFile, "report-file", "results.xml", "Provide the path to where the junit results will be printed.")
	flag.StringVar(&kubeConfig, "kubeconfig", "", "Location of the kubeconfig to use; defaults to KUBECONFIG if not set")
	flag.StringVar(&optionsFile, "options", "", "Location of an \"options.yaml\" file to provide input for various tests")
}

func TestSubmarinerAddonE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	junitReporter := reporters.NewJUnitReporter(reportFile)
	RunSpecsWithDefaultAndCustomReporters(t, "submariner-addon E2E Suite", []Reporter{junitReporter})
}

var _ = BeforeSuite(func() {
	initVars()

	var err error
	managedClusterName, err = utils.GetAManagedCluster(clusterClient)
	Expect(err).NotTo(HaveOccurred())
})

var _ = AfterSuite(func() {})

func initVars() {
	if optionsFile == "" {
		optionsFile = os.Getenv("OPTIONS")
		if optionsFile == "" {
			optionsFile = "resources/options.yaml"
		}
	}

	klog.V(1).Infof("options filename=%s", optionsFile)

	data, err := ioutil.ReadFile(optionsFile)
	if err != nil {
		klog.Errorf("--options error: %v", err)
	}
	Expect(err).NotTo(HaveOccurred())

	klog.Infof("file preview: %s \n", string(optionsFile))

	err = yaml.Unmarshal([]byte(data), &testOptionsContainer)
	if err != nil {
		klog.Errorf("--options error: %v", err)
	}
	Expect(err).NotTo(HaveOccurred())

	testOptions = testOptionsContainer.Options

	if testOptions.KubeConfig == "" {
		if kubeConfig == "" {
			kubeConfig = os.Getenv("KUBECONFIG")
		}
		testOptions.KubeConfig = kubeConfig
	}

	// allow developers and testers to use a non-default namespace for ACM deployment
	if testOptions.HubCluster.Namespace != "" {
		hubNamespace = testOptions.HubCluster.Namespace
	} else {
		hubNamespace = "open-cluster-management"
	}

	// prepare clients
	clusterCfg, err := clientcmd.BuildConfigFromFlags("", testOptions.KubeConfig)
	if err != nil {
		klog.Errorf("failed to get ClusterCfg from path %v . %v", testOptions.KubeConfig, err)
		Expect(err).NotTo(HaveOccurred())
	}

	if kubeClient, err = kubernetes.NewForConfig(clusterCfg); err != nil {
		klog.Errorf("failed to get kubeClient. %v", err)
		Expect(err).NotTo(HaveOccurred())
	}

	if clusterClient, err = clusterclient.NewForConfig(clusterCfg); err != nil {
		klog.Errorf("failed to get clusterClient. %v", err)
		Expect(err).NotTo(HaveOccurred())
	}
	if workClient, err = workclient.NewForConfig(clusterCfg); err != nil {
		klog.Errorf("failed to get workClient. %v", err)
		Expect(err).NotTo(HaveOccurred())
	}
}
