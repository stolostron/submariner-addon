package submarinerconfig_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig"
	configv1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	fakeconfigclient "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/clientset/versioned/fake"
	"github.com/submariner-io/admiral/pkg/fake"
	"github.com/submariner-io/admiral/pkg/test"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	configName    = "test-config"
	namespace     = "testy-ns"
	conditionType = "TestType"
)

var _ = Describe("Update Status", func() {
	t := newUpdateStatusTestDriver()

	When("the Condition type doesn't exist", func() {
		It("should add it", func() {
			t.doUpdateCondition(newDefaultCondition())
		})

		Context("and another Condition type exists", func() {
			It("should preserve the existing Condition", func() {
				t.doUpdateCondition(&metav1.Condition{
					Type:    "OtherType",
					Status:  metav1.ConditionTrue,
					Reason:  "OtherReason",
					Message: "other message",
				})

				updatedStatus := t.doUpdateCondition(&metav1.Condition{
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
				t.initialStatus = configv1alpha1.SubmarinerConfigStatus{
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
				t.doUpdateCondition(&metav1.Condition{
					Type:    conditionType,
					Status:  metav1.ConditionTrue,
					Reason:  "Reason2",
					Message: "message2",
				})
			})
		})

		Context("and the new Condition is the same", func() {
			BeforeEach(func() {
				t.initialStatus = configv1alpha1.SubmarinerConfigStatus{
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

				_, updated, err := t.doUpdateStatus(submarinerconfig.UpdateConditionFn(&prevStatus.Conditions[0]))
				Expect(err).To(Succeed())
				Expect(updated).To(BeFalse())

				currentStatus := t.getStatus()
				Expect(prevStatus).To(Equal(currentStatus))

				test.EnsureNoActionsForResource(&t.client.Fake, "submarinerconfigs", "update")
			})
		})
	})

	When("the SubmarinerConfig doesn't exist", func() {
		JustBeforeEach(func() {
			Expect(t.client.SubmarineraddonV1alpha1().SubmarinerConfigs(namespace).Delete(context.TODO(), configName, metav1.DeleteOptions{})).
				To(Succeed())
		})

		It("should succeed and report not updated", func() {
			_, updated, err := t.doUpdateStatus(submarinerconfig.UpdateConditionFn(newDefaultCondition()))
			Expect(err).To(Succeed())
			Expect(updated).To(BeFalse())
		})
	})

	When("the SubmarinerConfig retrieval fails", func() {
		JustBeforeEach(func() {
			fake.FailOnAction(&t.client.Fake, "submarinerconfigs", "get", nil, false)
		})

		It("should fail", func() {
			_, _, err := t.doUpdateStatus(submarinerconfig.UpdateConditionFn(newDefaultCondition()))
			Expect(err).ToNot(Succeed())
		})
	})

	When("the SubmarinerConfig update fails", func() {
		JustBeforeEach(func() {
			fake.FailOnAction(&t.client.Fake, "submarinerconfigs", "update", nil, false)
		})

		It("should fail", func() {
			_, _, err := t.doUpdateStatus(submarinerconfig.UpdateConditionFn(newDefaultCondition()))
			Expect(err).ToNot(Succeed())
		})
	})

	When("the SubmarinerConfig update initially fails with a conflict error", func() {
		JustBeforeEach(func() {
			fake.ConflictOnUpdateReactor(&t.client.Fake, "submarinerconfigs")
		})

		It("should eventually update it", func() {
			t.doUpdateCondition(newDefaultCondition())
		})
	})

	When("ManagedClusterInfo is specified", func() {
		managedClusterInfo := &configv1alpha1.ManagedClusterInfo{
			ClusterName: "east",
			Vendor:      "test-vendor",
			Platform:    "AWS",
			Region:      "north-america",
			InfraID:     "test-infraID",
		}

		It("should update it", func() {
			updatedStatus, updated, err := t.doUpdateStatus(submarinerconfig.UpdateStatusFn(newDefaultCondition(), managedClusterInfo))
			Expect(err).To(Succeed())
			Expect(updated).To(BeTrue())

			t.assertManagedClusterInfoUpdated(updatedStatus, managedClusterInfo)
		})

		Context("along with a Condition", func() {
			It("should update both", func() {
				newCond := newDefaultCondition()
				updatedStatus, updated, err := t.doUpdateStatus(submarinerconfig.UpdateStatusFn(newCond, managedClusterInfo))
				Expect(err).To(Succeed())
				Expect(updated).To(BeTrue())

				t.assertManagedClusterInfoUpdated(updatedStatus, managedClusterInfo)
				t.assertStatusConditionUpdated(updatedStatus, newCond)
			})
		})
	})
})

type updateStatusTestDriver struct {
	initialStatus configv1alpha1.SubmarinerConfigStatus
	client        *fakeconfigclient.Clientset
}

func newUpdateStatusTestDriver() *updateStatusTestDriver {
	t := &updateStatusTestDriver{}

	JustBeforeEach(func() {
		t.client = fakeconfigclient.NewSimpleClientset(&configv1alpha1.SubmarinerConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      configName,
				Namespace: namespace,
			},

			Status: t.initialStatus,
		})
	})

	return t
}

func (t *updateStatusTestDriver) doUpdateStatus(
	updateFuncs submarinerconfig.UpdateStatusFunc,
) (*configv1alpha1.SubmarinerConfigStatus, bool, error) {
	return submarinerconfig.UpdateStatus(context.TODO(), t.client.SubmarineraddonV1alpha1().SubmarinerConfigs(namespace),
		configName, updateFuncs)
}

func (t *updateStatusTestDriver) doUpdateCondition(newCond *metav1.Condition) *configv1alpha1.SubmarinerConfigStatus {
	updatedStatus, updated, err := t.doUpdateStatus(submarinerconfig.UpdateConditionFn(newCond))
	Expect(err).To(Succeed())
	Expect(updated).To(BeTrue())

	return t.assertStatusConditionUpdated(updatedStatus, newCond)
}

func (t *updateStatusTestDriver) assertStatusConditionUpdated(updatedStatus *configv1alpha1.SubmarinerConfigStatus,
	expCond *metav1.Condition,
) *configv1alpha1.SubmarinerConfigStatus {
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

func (t *updateStatusTestDriver) getStatus() *configv1alpha1.SubmarinerConfigStatus {
	config, err := t.client.SubmarineraddonV1alpha1().SubmarinerConfigs(namespace).Get(context.TODO(), configName, metav1.GetOptions{})
	Expect(err).To(Succeed())

	return &config.Status
}

func (t *updateStatusTestDriver) assertManagedClusterInfoUpdated(updatedStatus *configv1alpha1.SubmarinerConfigStatus,
	expInfo *configv1alpha1.ManagedClusterInfo,
) {
	newStatus := t.getStatus()

	Expect(newStatus.ManagedClusterInfo.ClusterName).To(Equal(expInfo.ClusterName))
	Expect(newStatus.ManagedClusterInfo.Platform).To(Equal(expInfo.Platform))
	Expect(newStatus.ManagedClusterInfo.InfraID).To(Equal(expInfo.InfraID))
	Expect(newStatus.ManagedClusterInfo.Region).To(Equal(expInfo.Region))
	Expect(newStatus.ManagedClusterInfo.Vendor).To(Equal(expInfo.Vendor))

	Expect(updatedStatus).To(Equal(newStatus))
}

func newDefaultCondition() *metav1.Condition {
	return &metav1.Condition{
		Type:    conditionType,
		Status:  metav1.ConditionTrue,
		Reason:  "TestReason",
		Message: "test message",
	}
}
