package reporter

import (
	"fmt"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/submariner-io/cloud-prepare/pkg/api"
)

type eventRecorderReporter struct {
	reason        string
	eventRecorder events.Recorder
}

// NewEventRecorder creates an event-recorder-based reporter
func NewEventRecorderWrapper(reason string, recorder events.Recorder) api.Reporter {
	return eventRecorderReporter{
		reason:        reason,
		eventRecorder: recorder,
	}
}

// Started will report that an operation started on the cloud
func (g eventRecorderReporter) Started(message string, args ...interface{}) {
	g.eventRecorder.Eventf(g.reason, fmt.Sprintf(message, args...))
}

// Succeeded will report that the last operation on the cloud has succeeded
func (g eventRecorderReporter) Succeeded(message string, args ...interface{}) {
	g.eventRecorder.Eventf(g.reason, message, args...)
}

// Failed will report that the last operation on the cloud has failed
func (g eventRecorderReporter) Failed(errs ...error) {
	message := "Failed"
	errMessages := []string{}
	for i := range errs {
		message += "\n%s"
		errMessages = append(errMessages, errs[i].Error())
	}
	g.eventRecorder.Warningf(g.reason, message, errMessages)
}
