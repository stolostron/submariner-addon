package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced"

// SubmarinerDiagnoseConfig represents the configuration to run SubmarinerDiagnose Job.
type SubmarinerDiagnoseConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the configuration of the Submariner
	Spec SubmarinerDiagnoseSpec `json:"spec"`

	// Status represents the current status of SubmarinerDiagnose
	// +optional
	Status SubmarinerDiagnoseStatus `json:"status,omitempty"`
}

// SubmarinerDiagnoseSpec defines the desired configuration to run SubmarinerDiagnose.
type SubmarinerDiagnoseSpec struct {
	All             bool            `json:"all,omitempty"`
	CNI             bool            `json:"CNI,omitempty"`
	Connections     bool            `json:"connections,omitempty"`
	Deployment      bool            `json:"deployment,omitempty"`
	Firewall        bool            `json:"firewall,omitempty"`
	GatherLogs      bool            `json:"gatherLogs,omitempty"`
	K8sVersion      bool            `json:"k8sVersion,omitempty"`
	KubeProxyMode   bool            `json:"kubeProxyMode,omitempty"`
	FirewallOptions FirewallOptions `json:"firewallOptions,omitempty"`

	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster.
	// Important: Run "make manifests" to regenerate code after modifying this file.
	// Add custom validation using kubebuilder tags: https://book-v1.book.kubebuilder.io/beyond_basics/generating_crd.html.
}

type FirewallOptions struct {
	InterCluster             bool   `json:"interCluster,omitempty"`
	IntraCluster             bool   `json:"intraCluster,omitempty"`
	Metrics                  bool   `json:"metrics,omitempty"`
	RemoteCluster            string `json:"remoteCluster,omitempty"`
	RemoteK8sAPIServer       string `json:"remoteK8sAPIServer,omitempty"`
	RemoteK8sAPIServerToken  string `json:"remoteK8sAPIServerToken,omitempty"`
	RemoteK8sCA              string `json:"remoteK8sCA,omitempty"`
	RemoteK8sSecret          string `json:"remoteK8sSecret,omitempty"`
	RemoteK8sRemoteNamespace string `json:"remoteK8sRemoteNamespace,omitempty"`
}

// SubmarinerDiagnoseStatus defines the observed result of SubmarinerDiagnose.
type SubmarinerDiagnoseStatus struct {
	Conditions     []metav1.Condition `json:"conditions"`
	K8sVersion     string             `json:"k8sVersion,omitempty"`
	CNIType        string             `json:"cniType,omitempty"`
	KubeProxyMode  bool               `json:"kubeProxyMode,omitempty"`
	FirewallStatus FirewallStatus     `json:"firewallStatus,omitempty"`
	// ConnectionsStatus available in Gateway status.
	// DeploymentStatus already captured in SubmarinerStatus, no need to duplicate information.
}

type FirewallPortStatus string

const (
	Allowed = "allowed"
	Blocked = "blocked"
	Unknown = "unknown"
)

type FirewallStatus struct {
	Metrics     FirewallPortStatus `json:"metricsStatus,omitempty"`
	VxlanTunnel FirewallPortStatus `json:"vxlanTunnel,omitempty"`
	IPSecTunnel FirewallPortStatus `json:"IPSecTunnel,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// SubmarinerDiagnoseConfigList is a collection of SubmarinerDiagnoseConfig.
type SubmarinerDiagnoseConfigList struct {
	metav1.TypeMeta `json:",inline"`
	// Standard list metadata.
	// More info: https://git.k8s.io/community/contributors/devel/api-conventions.md#types-kinds
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`

	// Items is a list of SubmarinerDiagnoseConfig.
	Items []SubmarinerDiagnoseConfig `json:"items"`
}
