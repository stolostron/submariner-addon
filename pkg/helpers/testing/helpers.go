package testing

import (
	"testing"

	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/events/eventstesting"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/util/workqueue"
)

type fakeSyncContext struct {
	key      string
	queue    workqueue.RateLimitingInterface
	recorder events.Recorder
}

func (f fakeSyncContext) Queue() workqueue.RateLimitingInterface { return f.queue }
func (f fakeSyncContext) QueueKey() string                       { return f.key }
func (f fakeSyncContext) Recorder() events.Recorder              { return f.recorder }

func NewFakeSyncContext(t *testing.T, key string) *fakeSyncContext {
	t.Helper()

	return &fakeSyncContext{
		key:      key,
		queue:    workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter()),
		recorder: eventstesting.NewTestingEventRecorder(t),
	}
}

func NewSubmarinerConfigCondition(name, status, reason, message string, lastTransition *metav1.Time) metav1.Condition {
	ret := metav1.Condition{
		Type:    name,
		Status:  metav1.ConditionStatus(status),
		Reason:  reason,
		Message: message,
	}
	if lastTransition != nil {
		ret.LastTransitionTime = *lastTransition
	}

	return ret
}

func ToUnstructured(obj runtime.Object) *unstructured.Unstructured {
	m, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	Expect(err).To(Succeed())

	return &unstructured.Unstructured{Object: m}
}
