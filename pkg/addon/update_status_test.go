package addon_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/stolostron/submariner-addon/pkg/addon"
	"github.com/stolostron/submariner-addon/pkg/constants"
	"github.com/submariner-io/admiral/pkg/fake"
	"github.com/submariner-io/admiral/pkg/test"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonfake "open-cluster-management.io/api/client/addon/clientset/versioned/fake"
)

const (
	namespace     = "test-ns"
	conditionType = "TestType"
)

var _ = Describe("Update Status", func() {
	t := newUpdateStatusTestDriver()

	When("the Condition type doesn't exist", func() {
		It("should add it", func() {
			t.assertUpdateCondition(newDefaultCondition())
		})

		Context("and another Condition type exists", func() {
			It("should preserve the existing Condition", func() {
				t.assertUpdateCondition(&metav1.Condition{
					Type:    "OtherType",
					Status:  metav1.ConditionTrue,
					Reason:  "OtherReason",
					Message: "other message",
				})

				updatedStatus := t.assertUpdateCondition(&metav1.Condition{
					Type:    conditionType,
					Status:  metav1.ConditionTrue,
					Reason:  "TestReason",
					Message: "test message",
				})

				other := meta.FindStatusCondition(updatedStatus.Conditions, "OtherType")
				Expect(other).ToNot(BeNil())
			})
		})
	})

	When("the Condition type already exists", func() {
		Context("and the new Condition differs", func() {
			BeforeEach(func() {
				t.initialStatus = addonv1alpha1.ManagedClusterAddOnStatus{
					Conditions: []metav1.Condition{
						{
							Type:               conditionType,
							Status:             metav1.ConditionFalse,
							Reason:             "Reason1",
							Message:            "message1",
							LastTransitionTime: metav1.Time{Time: metav1.Now().Add(-10 * time.Second)},
						},
					},
				}
			})

			It("should update it", func() {
				t.assertUpdateCondition(&metav1.Condition{
					Type:    conditionType,
					Status:  metav1.ConditionTrue,
					Reason:  "Reason2",
					Message: "message2",
				})
			})
		})

		Context("and the new Condition is the same", func() {
			BeforeEach(func() {
				t.initialStatus = addonv1alpha1.ManagedClusterAddOnStatus{
					Conditions: []metav1.Condition{
						{
							Type:               conditionType,
							Status:             metav1.ConditionFalse,
							Reason:             "Reason1",
							Message:            "message1",
							LastTransitionTime: metav1.Time{Time: metav1.Now().Add(-10 * time.Second)},
						},
					},
				}
			})

			It("should not update it", func() {
				prevStatus := t.getStatus()

				_, updated, err := t.doUpdateStatus(addon.UpdateConditionFn(&prevStatus.Conditions[0]))
				Expect(err).To(Succeed())
				Expect(updated).To(BeFalse())

				currentStatus := t.getStatus()
				Expect(prevStatus).To(Equal(currentStatus))

				test.EnsureNoActionsForResource(&t.client.Fake, "managedclusteraddons", "update")
			})
		})
	})

	When("the ManagedClusterAddOn doesn't exist", func() {
		JustBeforeEach(func() {
			Expect(t.client.AddonV1alpha1().ManagedClusterAddOns(namespace).Delete(context.TODO(), constants.SubmarinerAddOnName,
				metav1.DeleteOptions{}))
		})

		It("should succeed and report not updated", func() {
			_, updated, err := t.doUpdateStatus(addon.UpdateConditionFn(newDefaultCondition()))
			Expect(err).To(Succeed())
			Expect(updated).To(BeFalse())
		})
	})

	When("the ManagedClusterAddOn retrieval fails", func() {
		JustBeforeEach(func() {
			fake.FailOnAction(&t.client.Fake, "managedclusteraddons", "get", nil, false)
		})

		It("should fail", func() {
			_, _, err := t.doUpdateStatus(addon.UpdateConditionFn(newDefaultCondition()))
			Expect(err).ToNot(Succeed())
		})
	})

	When("the ManagedClusterAddOn update fails", func() {
		JustBeforeEach(func() {
			fake.FailOnAction(&t.client.Fake, "managedclusteraddons", "update", nil, false)
		})

		It("should fail", func() {
			_, _, err := t.doUpdateStatus(addon.UpdateConditionFn(newDefaultCondition()))
			Expect(err).ToNot(Succeed())
		})
	})

	When("the ManagedClusterAddOn update initially fails with a conflict error", func() {
		JustBeforeEach(func() {
			fake.ConflictOnUpdateReactor(&t.client.Fake, "managedclusteraddons")
		})

		It("should eventually update it", func() {
			t.assertUpdateCondition(newDefaultCondition())
		})
	})
})

type updateStatusTestDriver struct {
	initialStatus addonv1alpha1.ManagedClusterAddOnStatus
	client        *addonfake.Clientset
}

func newUpdateStatusTestDriver() *updateStatusTestDriver {
	t := &updateStatusTestDriver{}

	JustBeforeEach(func() {
		t.client = addonfake.NewSimpleClientset(&addonv1alpha1.ManagedClusterAddOn{
			ObjectMeta: metav1.ObjectMeta{
				Name:      constants.SubmarinerAddOnName,
				Namespace: namespace,
			},

			Status: t.initialStatus,
		})
	})

	return t
}

func (t *updateStatusTestDriver) doUpdateStatus(updateFn addon.UpdateStatusFunc) (*addonv1alpha1.ManagedClusterAddOnStatus, bool, error) {
	return addon.UpdateStatus(context.TODO(), t.client, namespace, updateFn)
}

func (t *updateStatusTestDriver) assertUpdateCondition(newCond *metav1.Condition) *addonv1alpha1.ManagedClusterAddOnStatus {
	updatedStatus, updated, err := t.doUpdateStatus(addon.UpdateConditionFn(newCond))
	Expect(err).To(Succeed())
	Expect(updated).To(BeTrue())

	return t.assertStatusConditionUpdated(updatedStatus, newCond)
}

func (t *updateStatusTestDriver) assertStatusConditionUpdated(updatedStatus *addonv1alpha1.ManagedClusterAddOnStatus,
	expCond *metav1.Condition,
) *addonv1alpha1.ManagedClusterAddOnStatus {
	newStatus := t.getStatus()
	actual := meta.FindStatusCondition(newStatus.Conditions, expCond.Type)
	Expect(actual).ToNot(BeNil())

	Expect(actual.Status).To(Equal(expCond.Status))
	Expect(actual.Reason).To(Equal(expCond.Reason))
	Expect(actual.Message).To(Equal(expCond.Message))
	Expect(actual.LastTransitionTime).ToNot(BeNil())

	if !expCond.LastTransitionTime.IsZero() {
		Expect(actual.LastTransitionTime.After(expCond.LastTransitionTime.Time)).To(BeTrue())
	}

	Expect(updatedStatus).To(Equal(newStatus))

	return updatedStatus
}

func (t *updateStatusTestDriver) getStatus() *addonv1alpha1.ManagedClusterAddOnStatus {
	config, err := t.client.AddonV1alpha1().ManagedClusterAddOns(namespace).Get(context.TODO(), constants.SubmarinerAddOnName,
		metav1.GetOptions{})
	Expect(err).To(Succeed())

	return &config.Status
}

func newDefaultCondition() *metav1.Condition {
	return &metav1.Condition{
		Type:    conditionType,
		Status:  metav1.ConditionTrue,
		Reason:  "TestReason",
		Message: "test message",
	}
}
