//nolint:testpackage // These tests will either be removed or converted to Gingko.
package helpers

import (
	"context"
	"os"
	"testing"
	"time"

	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	fakeconfigclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned/fake"
	testinghelpers "github.com/open-cluster-management/submariner-addon/pkg/helpers/testing"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonfake "open-cluster-management.io/api/client/addon/clientset/versioned/fake"
)

//nolint:dupl // These tests will either be removed or converted to Gingko.
func TestUpdateStatusCondition(t *testing.T) {
	nowish := metav1.Now()
	beforeish := metav1.Time{Time: nowish.Add(-10 * time.Second)}
	afterish := metav1.Time{Time: nowish.Add(10 * time.Second)}

	cases := []struct {
		name               string
		startingConditions []metav1.Condition
		newCondition       metav1.Condition
		expextedUpdated    bool
		expectedConditions []metav1.Condition
	}{
		{
			name:               "add to empty",
			startingConditions: []metav1.Condition{},
			newCondition:       testinghelpers.NewSubmarinerConfigCondition("test", "True", "my-reason", "my-message", nil),
			expextedUpdated:    true,
			expectedConditions: []metav1.Condition{testinghelpers.NewSubmarinerConfigCondition("test", "True", "my-reason", "my-message", nil)},
		},
		{
			name: "add to non-conflicting",
			startingConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
			},
			newCondition:    testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", nil),
			expextedUpdated: true,
			expectedConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
				testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", nil),
			},
		},
		{
			name: "change existing status",
			startingConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
				testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", nil),
			},
			newCondition:    testinghelpers.NewSubmarinerConfigCondition("one", "False", "my-different-reason", "my-othermessage", nil),
			expextedUpdated: true,
			expectedConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
				testinghelpers.NewSubmarinerConfigCondition("one", "False", "my-different-reason", "my-othermessage", nil),
			},
		},
		{
			name: "leave existing transition time",
			startingConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
				testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", &beforeish),
			},
			newCondition:    testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", &afterish),
			expextedUpdated: false,
			expectedConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
				testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", &beforeish),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeClusterClient := fakeconfigclient.NewSimpleClientset(&configv1alpha1.SubmarinerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testconfig",
					Namespace: "cluster1",
				},
				Status: configv1alpha1.SubmarinerConfigStatus{
					Conditions: c.startingConditions,
				},
			})

			status, updated, err := UpdateSubmarinerConfigStatus(
				fakeClusterClient,
				"cluster1", "testconfig",
				UpdateSubmarinerConfigConditionFn(&c.newCondition),
			)
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
			if updated != c.expextedUpdated {
				t.Errorf("expected %t, but %t", c.expextedUpdated, updated)
			}
			for i := range c.expectedConditions {
				expected := c.expectedConditions[i]
				actual := status.Conditions[i]
				if expected.LastTransitionTime == (metav1.Time{}) {
					actual.LastTransitionTime = metav1.Time{}
				}
				if !equality.Semantic.DeepEqual(expected, actual) {
					t.Errorf(diff.ObjectDiff(expected, actual))
				}
			}
		})
	}
}

//nolint:dupl // These tests will either be removed or converted to Gingko.
func TestUpdateManagedClusterAddOnStatus(t *testing.T) {
	nowish := metav1.Now()
	beforeish := metav1.Time{Time: nowish.Add(-10 * time.Second)}
	afterish := metav1.Time{Time: nowish.Add(10 * time.Second)}

	cases := []struct {
		name               string
		startingConditions []metav1.Condition
		newCondition       metav1.Condition
		expextedUpdated    bool
		expectedConditions []metav1.Condition
	}{
		{
			name:               "add to empty",
			startingConditions: []metav1.Condition{},
			newCondition:       testinghelpers.NewSubmarinerConfigCondition("test", "True", "my-reason", "my-message", nil),
			expextedUpdated:    true,
			expectedConditions: []metav1.Condition{testinghelpers.NewSubmarinerConfigCondition("test", "True", "my-reason", "my-message", nil)},
		},
		{
			name: "add to non-conflicting",
			startingConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
			},
			newCondition:    testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", nil),
			expextedUpdated: true,
			expectedConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
				testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", nil),
			},
		},
		{
			name: "change existing status",
			startingConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
				testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", nil),
			},
			newCondition:    testinghelpers.NewSubmarinerConfigCondition("one", "False", "my-different-reason", "my-othermessage", nil),
			expextedUpdated: true,
			expectedConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
				testinghelpers.NewSubmarinerConfigCondition("one", "False", "my-different-reason", "my-othermessage", nil),
			},
		},
		{
			name: "leave existing transition time",
			startingConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
				testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", &beforeish),
			},
			newCondition:    testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", &afterish),
			expextedUpdated: false,
			expectedConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
				testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", &beforeish),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeAddOnClient := addonfake.NewSimpleClientset(&addonv1alpha1.ManagedClusterAddOn{
				ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: SubmarinerAddOnName},
				Status: addonv1alpha1.ManagedClusterAddOnStatus{
					Conditions: c.startingConditions,
				},
			})

			status, updated, err := UpdateManagedClusterAddOnStatus(context.TODO(), fakeAddOnClient, "test",
				UpdateManagedClusterAddOnStatusFn(&c.newCondition))
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
			if updated != c.expextedUpdated {
				t.Errorf("expected %t, but %t", c.expextedUpdated, updated)
			}
			for i := range c.expectedConditions {
				expected := c.expectedConditions[i]
				actual := status.Conditions[i]
				if expected.LastTransitionTime == (metav1.Time{}) {
					actual.LastTransitionTime = metav1.Time{}
				}
				if !equality.Semantic.DeepEqual(expected, actual) {
					t.Errorf(diff.ObjectDiff(expected, actual))
				}
			}
		})
	}
}

func TestGetEnv(t *testing.T) {
	os.Setenv("test_env", "test_val")
	defer os.Unsetenv("test_env")

	cases := []struct {
		name          string
		envKey        string
		defaultValue  string
		expectedValue string
	}{
		{
			name:          "env exists",
			envKey:        "test_env",
			expectedValue: "test_val",
		},
		{
			name:          "env does not exist",
			envKey:        "nonexistent",
			defaultValue:  "default_val",
			expectedValue: "default_val",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			value := GetEnv(c.envKey, c.defaultValue)
			if value != c.expectedValue {
				t.Errorf("expect %v, but got: %v", c.expectedValue, value)
			}
		})
	}
}
