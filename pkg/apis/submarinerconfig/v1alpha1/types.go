package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope="Namespaced"

// SubmarinerConfig represents the configuration for Submariner, the submariner-addon will use it
// to configure the Submariner.
type SubmarinerConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the configuration of the Submariner
	Spec SubmarinerConfigSpec `json:"spec"`
}

// SubmarinerConfigSpec describes the configuration of the Submariner
type SubmarinerConfigSpec struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SubmarinerConfigList is a collection of SubmarinerConfig.
type SubmarinerConfigList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is a list of SubmarinerConfig.
	Items []SubmarinerConfig `json:"items"`
}
