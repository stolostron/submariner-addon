package submarinerbroker

import (
	"context"
	"testing"
	"time"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	fakeapiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/fake"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"

	testinghelpers "github.com/stolostron/submariner-addon/pkg/helpers/testing"

	"github.com/openshift/library-go/pkg/operator/events/eventstesting"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
)

func newSubmarinerConfigCRD() *apiextensionsv1.CustomResourceDefinition {
	return &apiextensionsv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: configCRDName,
		},
	}
}

func TestNewSubmarinerBrokerCRDsControllerSync(t *testing.T) {
	cases := []struct {
		name            string
		crdName         string
		crds            []runtime.Object
		validateActions func(t *testing.T, crdActions []clienttesting.Action)
	}{
		{
			name:    "No submarinerConfig crd",
			crdName: "test",
			crds:    []runtime.Object{},
			validateActions: func(t *testing.T, crdActions []clienttesting.Action) {
				testinghelpers.AssertNoActions(t, crdActions)
			},
		},
		{
			name:    "has submarinerConfig CRD",
			crdName: configCRDName,
			crds:    []runtime.Object{newSubmarinerConfigCRD()},
			validateActions: func(t *testing.T, crdActions []clienttesting.Action) {
				testinghelpers.AssertActionResource(t, crdActions[2], "customresourcedefinitions")
				testinghelpers.AssertActions(t, crdActions, "get", "get", "create", "get", "create", "get", "create", "get", "create", "get", "create")
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeCRDClient := fakeapiextensionsclientset.NewSimpleClientset(c.crds...)
			apiExtensionsInformers := apiextensionsinformers.NewSharedInformerFactory(fakeCRDClient, 10*time.Minute)
			for _, crd := range c.crds {
				apiExtensionsInformers.Apiextensions().V1beta1().CustomResourceDefinitions().Informer().GetStore().Add(crd)
			}

			ctrl := &submarinerBrokerCRDsController{
				crdClient:     fakeCRDClient,
				eventRecorder: eventstesting.NewTestingEventRecorder(t),
			}

			err := ctrl.sync(context.TODO(), testinghelpers.NewFakeSyncContext(t, c.crdName))
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}

			c.validateActions(t, fakeCRDClient.Actions())
		})
	}
}
