package provider

import (
	"github.com/openshift/library-go/pkg/operator/events"
	configv1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
)

type Info struct {
	RestMapper        meta.RESTMapper
	KubeClient        kubernetes.Interface
	DynamicClient     dynamic.Interface
	EventRecorder     events.Recorder
	CredentialsSecret *corev1.Secret
	configv1alpha1.SubmarinerConfigSpec
	configv1alpha1.ManagedClusterInfo
	SubmarinerConfigAnnotations map[string]string
}
