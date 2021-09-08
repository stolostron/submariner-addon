package submarineragent_test

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"
	testingHelpers "github.com/open-cluster-management/submariner-addon/pkg/helpers/testing"
	"k8s.io/apimachinery/pkg/api/meta"
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

func (t *managedClusterAddOnTestBase) awaitManagedClusterAddOnStatusCondition(expCond metav1.Condition) {
	testingHelpers.AwaitStatusCondition(expCond, func() ([]metav1.Condition, error) {
		config, err := t.addOnClient.AddonV1alpha1().ManagedClusterAddOns(clusterName).Get(context.TODO(),
			helpers.SubmarinerAddOnName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}

		return config.Status.Conditions, nil
	})
}

func (t *managedClusterAddOnTestBase) awaitNoManagedClusterAddOnStatusCondition(condType string) {
	Consistently(func() *metav1.Condition {
		config, err := t.addOnClient.AddonV1alpha1().ManagedClusterAddOns(clusterName).Get(context.TODO(),
			helpers.SubmarinerConfigName, metav1.GetOptions{})
		Expect(err).To(Succeed())

		return meta.FindStatusCondition(config.Status.Conditions, condType)
	}, 300*time.Millisecond).Should(BeNil())
}

func newAddOn() *addonv1alpha1.ManagedClusterAddOn {
	return &addonv1alpha1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name:      helpers.SubmarinerAddOnName,
			Namespace: clusterName,
		},
	}
}
