package submarinerbroker_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/stolostron/submariner-addon/pkg/hub/submarinerbroker"
	fakereactor "github.com/submariner-io/admiral/pkg/fake"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	apiextensionsInformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"
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
	crd           *apiextensionsv1.CustomResourceDefinition
	justBeforeRun func()
	stop          context.CancelFunc
}

func newBrokerCRDsControllerTestDriver() *brokerCRDsControllerTestDriver {
	t := &brokerCRDsControllerTestDriver{}

	BeforeEach(func() {
		t.crd = newSubmarinerConfigCRD()
		t.justBeforeRun = func() {}
	})

	JustBeforeEach(func() {
		t.crdClient = fake.NewSimpleClientset(t.crd)

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
	t.awaitCRD("serviceimports.multicluster.x-k8s.io")
	t.awaitCRD("brokers.submariner.io")
}

func (t *brokerCRDsControllerTestDriver) awaitCRD(name string) {
	var crd *apiextensionsv1.CustomResourceDefinition

	Eventually(func() error {
		var err error
		crd, err = t.crdClient.ApiextensionsV1().CustomResourceDefinitions().Get(context.TODO(), name, metav1.GetOptions{})

		return err
	}).Should(Succeed(), "CRD %q not found", name)

	Expect(crd.OwnerReferences).To(HaveLen(1))
	Expect(crd.OwnerReferences[0].APIVersion).To(Equal(apiextensionsv1.SchemeGroupVersion.String()))
	Expect(crd.OwnerReferences[0].Kind).To(Equal("CustomResourceDefinition"))
	Expect(crd.OwnerReferences[0].Name).To(Equal(t.crd.Name))
	Expect(crd.OwnerReferences[0].Controller).ToNot(BeNil())
	Expect(*crd.OwnerReferences[0].Controller).To(BeTrue())
	Expect(crd.OwnerReferences[0].BlockOwnerDeletion).ToNot(BeNil())
	Expect(*crd.OwnerReferences[0].BlockOwnerDeletion).To(BeTrue())
	Expect(crd.OwnerReferences[0].UID).To(Equal(t.crd.UID))
}

func newSubmarinerConfigCRD() *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: submarinerbroker.ConfigCRDName,
			UID:  uuid.NewUUID(),
		},
	}
}
