package reporter

import (
	"fmt"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/submariner-io/admiral/pkg/reporter"
)

type eventRecorderReporter struct {
	reason        string
	eventRecorder events.Recorder
}

// NewEventRecorderWrapper creates an event-recorder-based reporter.
func NewEventRecorderWrapper(reason string, recorder events.Recorder) reporter.Interface {
	return &reporter.Adapter{Basic: &eventRecorderReporter{
		reason:        reason,
		eventRecorder: recorder,
	}}
}

// Start will report that an operation started on the cloud.
func (g eventRecorderReporter) Start(message string, args ...interface{}) {
	g.eventRecorder.Eventf(g.reason, fmt.Sprintf(message, args...))
}

// Success will report that the last operation on the cloud has succeeded.
func (g eventRecorderReporter) Success(message string, args ...interface{}) {
	g.eventRecorder.Eventf(g.reason, message, args...)
}

// Failure will report that the last operation on the cloud has failed.
func (g eventRecorderReporter) Failure(message string, args ...interface{}) {
	g.eventRecorder.Warningf(g.reason, message, args...)
}

func (g eventRecorderReporter) End() {
	// Do nothing
}

func (g eventRecorderReporter) Warning(message string, args ...interface{}) {
	g.eventRecorder.Warningf(g.reason, message, args...)
}
