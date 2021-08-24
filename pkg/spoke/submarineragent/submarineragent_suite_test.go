package submarineragent_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonFake "open-cluster-management.io/api/client/addon/clientset/versioned/fake"
)

const (
	clusterName = "test"
)

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

func newAddOn() *addonv1alpha1.ManagedClusterAddOn {
	return &addonv1alpha1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name:      helpers.SubmarinerAddOnName,
			Namespace: clusterName,
		},
	}
}
