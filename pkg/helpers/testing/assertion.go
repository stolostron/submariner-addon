package testing

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
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
