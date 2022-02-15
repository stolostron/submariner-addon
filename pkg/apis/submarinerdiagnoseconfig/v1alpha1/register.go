package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	GroupName     = "submarineraddon.open-cluster-management.io"
	GroupVersion  = schema.GroupVersion{Group: GroupName, Version: "v1alpha1"}
	schemeBuilder = runtime.NewSchemeBuilder(addKnownTypes)
	// Install is a function which adds this version to a scheme.
	Install = schemeBuilder.AddToScheme

	// Deprecated: generated code relies on SchemeGroupVersion.
	SchemeGroupVersion = GroupVersion
	// Deprecated: AddToScheme exists solely to keep the old generators creating valid code.
	AddToScheme = schemeBuilder.AddToScheme
)

// Deprecated: generated code relies on Resource being present, but it logically belongs to the group.
func Resource(resource string) schema.GroupResource {
	return schema.GroupResource{Group: GroupName, Resource: resource}
}

// Adds the list of known types to api.Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(GroupVersion,
		&SubmarinerDiagnoseConfig{},
		&SubmarinerDiagnoseConfigList{},
	)
	metav1.AddToGroupVersion(scheme, GroupVersion)

	return nil
}
