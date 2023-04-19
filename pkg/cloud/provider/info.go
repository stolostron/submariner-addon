package provider

import (
	"github.com/openshift/library-go/pkg/operator/events"
	configv1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	workclient "open-cluster-management.io/api/client/work/clientset/versioned"
)

type Info struct {
	RestMapper    meta.RESTMapper
	KubeClient    kubernetes.Interface
	DynamicClient dynamic.Interface
	HubKubeClient kubernetes.Interface
	WorkClient    workclient.Interface
	EventRecorder events.Recorder
	configv1alpha1.SubmarinerConfigSpec
	configv1alpha1.ManagedClusterInfo
}
