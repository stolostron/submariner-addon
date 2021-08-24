package testing

import (
	"reflect"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/wait"
	clienttesting "k8s.io/client-go/testing"
)

// AssertActions asserts the actual actions have the expected action verb
func AssertActions(t *testing.T, actualActions []clienttesting.Action, expectedVerbs ...string) {
	if len(actualActions) != len(expectedVerbs) {
		t.Errorf("expected %d call but got: %#v", len(expectedVerbs), actualActions)
	}
	for i, expected := range expectedVerbs {
		if actualActions[i].GetVerb() != expected {
			t.Errorf("expected %s action but got: %#v", expected, actualActions[i])
		}
	}
}

// AssertNoActions asserts no actions are happened
func AssertNoActions(t *testing.T, actualActions []clienttesting.Action) {
	AssertActions(t, actualActions)
}

// AssertFinalizers asserts the given runtime object has the expected finalizers
func AssertFinalizers(t *testing.T, obj runtime.Object, finalizers []string) {
	accessor, _ := meta.Accessor(obj)
	actual := accessor.GetFinalizers()
	if len(actual) == 0 && len(finalizers) == 0 {
		return
	}
	if !reflect.DeepEqual(actual, finalizers) {
		t.Fatal(diff.ObjectDiff(actual, finalizers))
	}
}

func AssertActionResource(t *testing.T, action clienttesting.Action, expectedResource string) {
	if action == nil {
		t.Fatal("action is nil")
	}
	if action.GetResource().Resource != expectedResource {
		t.Errorf("expected action resource %s but got: %#v", expectedResource, action)
	}
}

func EnsureNoActionsForResource(f *clienttesting.Fake, resource string, expectedVerbs ...string) {
	Consistently(func() []string {
		expSet := sets.NewString(expectedVerbs...)
		verbs := []string{}

		actualActions := f.Actions()
		for i := range actualActions {
			if actualActions[i].GetResource().Resource == resource && expSet.Has(actualActions[i].GetVerb()) {
				verbs = append(verbs, actualActions[i].GetVerb())
			}
		}

		return verbs
	}).Should(BeEmpty())
}

func AwaitStatusCondition(expCond metav1.Condition, get func() ([]metav1.Condition, error)) {
	var found *metav1.Condition

	err := wait.PollImmediate(50*time.Millisecond, 5*time.Second, func() (bool, error) {
		conditions, err := get()
		if err != nil {
			return false, err
		}

		found = meta.FindStatusCondition(conditions, expCond.Type)
		if found == nil {
			return false, nil
		}

		return found.Status == expCond.Status && found.Reason == expCond.Reason, nil
	})

	if err == wait.ErrWaitTimeout {
		Expect(found).ToNot(BeNil(), "Status condition not found")
		Expect(found.Type).To(Equal(expCond.Type))
		Expect(found.Status).To(Equal(expCond.Status))
		Expect(found.LastTransitionTime).To(Not(BeNil()))
		Expect(found.Message).To(Not(BeEmpty()))
		Expect(found.Reason).To(Equal(expCond.Reason))
	} else {
		Expect(err).To(Succeed())
	}
}
