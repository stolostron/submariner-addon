package reporter

import (
	operatorhelpers "github.com/openshift/library-go/pkg/operator/v1helpers"
	"github.com/submariner-io/cloud-prepare/pkg/api"
	"k8s.io/klog/v2"
)

type cloudPrepareReporter struct{}

func NewCloudPrepareReporter() api.Reporter {
	return &cloudPrepareReporter{}
}

func (r *cloudPrepareReporter) Started(message string, args ...interface{}) {
	klog.Infof(message, args...)
}

func (r *cloudPrepareReporter) Succeeded(message string, args ...interface{}) {
	klog.Infof(message, args...)
}

func (r *cloudPrepareReporter) Failed(err ...error) {
	klog.Errorf(operatorhelpers.NewMultiLineAggregate(err).Error())
}
