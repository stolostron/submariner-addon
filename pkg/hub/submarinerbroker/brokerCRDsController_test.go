package submarinerbroker_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/submariner-addon/pkg/hub/submarinerbroker"
	"github.com/openshift/library-go/pkg/operator/events"
	fakereactor "github.com/submariner-io/admiral/pkg/fake"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	apiextensionsInformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/cache"
)

var _ = Describe("CRDs Controller", func() {
	t := newBrokerCRDsControllerTestDriver()

	When("the SubmarinerConfig CRD is created", func() {
		It("should deploy the Submariner CRDs", func() {
			t.awaitSubmarinerCRDs()
		})

		Context("and CRD creation initially fails", func() {
			BeforeEach(func() {
				t.justBeforeRun = func() {
					fakereactor.FailOnAction(&t.crdClient.Fake, "customresourcedefinitions", "create", nil, true)
				}
			})

			It("should eventually deploy the Submariner CRDs", func() {
				t.awaitSubmarinerCRDs()
			})
		})
	})
})

type brokerCRDsControllerTestDriver struct {
	crdClient     *fake.Clientset
	crds          []runtime.Object
	justBeforeRun func()
	stop          context.CancelFunc
}

func newBrokerCRDsControllerTestDriver() *brokerCRDsControllerTestDriver {
	t := &brokerCRDsControllerTestDriver{}

	BeforeEach(func() {
		t.crds = []runtime.Object{newSubmarinerConfigCRD()}
		t.justBeforeRun = func() {}
	})

	JustBeforeEach(func() {
		t.crdClient = fake.NewSimpleClientset(t.crds...)

		informerFactory := apiextensionsInformers.NewSharedInformerFactory(t.crdClient, 0)

		t.justBeforeRun()

		controller := submarinerbroker.NewCRDsController(t.crdClient,
			informerFactory.Apiextensions().V1().CustomResourceDefinitions(), events.NewLoggingEventRecorder("test"))

		var ctx context.Context

		ctx, t.stop = context.WithCancel(context.TODO())

		informerFactory.Start(ctx.Done())

		cache.WaitForCacheSync(ctx.Done(), informerFactory.Apiextensions().V1().CustomResourceDefinitions().Informer().HasSynced)

		go controller.Run(ctx, 1)
	})

	AfterEach(func() {
		t.stop()
	})

	return t
}

func (t *brokerCRDsControllerTestDriver) awaitSubmarinerCRDs() {
	t.awaitCRD("clusters.submariner.io")
	t.awaitCRD("endpoints.submariner.io")
	t.awaitCRD("gateways.submariner.io")
	t.awaitCRD("serviceimports.lighthouse.submariner.io")
	t.awaitCRD("serviceimports.multicluster.x-k8s.io")
}

func (t *brokerCRDsControllerTestDriver) awaitCRD(name string) {
	Eventually(func() error {
		_, err := t.crdClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), name, metav1.GetOptions{})

		return err
	}).Should(Succeed(), "CRD %q not found", name)
}

func newSubmarinerConfigCRD() *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "submarinerconfigs.submarineraddon.open-cluster-management.io",
		},
	}
}
