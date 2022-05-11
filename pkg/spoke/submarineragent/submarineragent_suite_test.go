package submarineragent_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	"github.com/stolostron/submariner-addon/pkg/constants"
	"github.com/submariner-io/admiral/pkg/test"
	submarinerv1alpha1 "github.com/submariner-io/submariner-operator/api/submariner/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	dynamicFake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/informers"
	k8sScheme "k8s.io/client-go/kubernetes/scheme"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonFake "open-cluster-management.io/api/client/addon/clientset/versioned/fake"
)

const (
	clusterName = "test"
)

var _ = BeforeSuite(func() {
	Expect(submarinerv1alpha1.AddToScheme(k8sScheme.Scheme)).To(Succeed())
	Expect(operatorsv1alpha1.AddToScheme(k8sScheme.Scheme)).To(Succeed())
})

func TestSubmarinerAgent(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Spoke Submariner Agent Suite")
}

type managedClusterAddOnTestBase struct {
	addOn       *addonv1alpha1.ManagedClusterAddOn
	addOnClient *addonFake.Clientset
}

func (t *managedClusterAddOnTestBase) init() {
	t.addOn = newAddOn()
	t.addOnClient = addonFake.NewSimpleClientset()
}

func (t *managedClusterAddOnTestBase) run() {
	if t.addOn != nil {
		_, err := t.addOnClient.AddonV1alpha1().ManagedClusterAddOns(t.addOn.Namespace).Create(context.TODO(), t.addOn,
			metav1.CreateOptions{})
		Expect(err).To(Succeed())
	}

	t.addOnClient.ClearActions()
}

func (t *managedClusterAddOnTestBase) awaitManagedClusterAddOnStatusCondition(expCond *metav1.Condition) {
	test.AwaitStatusCondition(expCond, func() ([]metav1.Condition, error) {
		config, err := t.addOnClient.AddonV1alpha1().ManagedClusterAddOns(clusterName).Get(context.TODO(),
			constants.SubmarinerAddOnName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		return config.Status.Conditions, nil
	})
}

func (t *managedClusterAddOnTestBase) awaitNoManagedClusterAddOnStatusCondition(condType string) {
	Consistently(func() *metav1.Condition {
		config, err := t.addOnClient.AddonV1alpha1().ManagedClusterAddOns(clusterName).Get(context.TODO(),
			constants.SubmarinerConfigName, metav1.GetOptions{})
		Expect(err).To(Succeed())

		return meta.FindStatusCondition(config.Status.Conditions, condType)
	}, 300*time.Millisecond).Should(BeNil())
}

func newAddOn() *addonv1alpha1.ManagedClusterAddOn {
	return &addonv1alpha1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name:      constants.SubmarinerAddOnName,
			Namespace: clusterName,
		},
	}
}

func newDynamicClientWithInformer(namespace string) (dynamic.ResourceInterface, dynamicinformer.DynamicSharedInformerFactory,
	informers.GenericInformer,
) {
	fakeGVR := schema.GroupVersionResource{
		Group:    "fake-dynamic-client-group",
		Version:  "v1",
		Resource: "unstructureds",
	}

	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: fakeGVR.Group, Version: fakeGVR.Version, Kind: "UnstructuredList"},
		&unstructured.UnstructuredList{})

	dynamicClient := dynamicFake.NewSimpleDynamicClient(scheme)
	dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)

	return dynamicClient.Resource(fakeGVR).Namespace(namespace), dynamicInformerFactory, dynamicInformerFactory.ForResource(fakeGVR)
}
