package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced"

// SubmarinerConfig represents the configuration for Submariner, the submariner-addon will use it
// to configure the Submariner.
type SubmarinerConfig struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the configuration of the Submariner
	Spec SubmarinerConfigSpec `json:"spec"`

	// Status represents the current status of submariner configuration
	// +optional
	Status SubmarinerConfigStatus `json:"status,omitempty"`
}

// SubmarinerConfigSpec describes the configuration of the Submariner.
type SubmarinerConfigSpec struct {
	// CableDriver represents the submariner cable driver implementation.
	// Available options are libreswan (default) strongswan, wireguard, and vxlan.
	// +optional
	// +kubebuilder:default=libreswan
	CableDriver string `json:"cableDriver,omitempty"`

	// GlobalCIDR specifies the global CIDR used by the cluster.
	// +optional
	GlobalCIDR string `json:"globalCIDR,omitempty"`

	// IPSecIKEPort represents IPsec IKE port (default 500).
	// +optional
	// +kubebuilder:default=500
	IPSecIKEPort int `json:"IPSecIKEPort,omitempty"`

	// IPSecNATTPort represents IPsec NAT-T port (default 4500).
	// +optional
	// +kubebuilder:default=4500
	IPSecNATTPort int `json:"IPSecNATTPort,omitempty"`

	// NATTDiscoveryPort specifies the port used for NAT-T Discovery (default UDP/4900).
	// +optional
	// +kubebuilder:default=4900
	NATTDiscoveryPort int `json:"NATTDiscoveryPort,omitempty"`

	// NATTEnable represents IPsec NAT-T enabled (default true).
	// +optional
	// +kubebuilder:default=true
	NATTEnable bool `json:"NATTEnable"`

	// LoadBalancerEnable enables or disables load balancer mode. When enabled, a LoadBalancer is created in the
	// submariner-operator namespace (default false).
	// +optional
	// +kubebuilder:default=false
	LoadBalancerEnable bool `json:"loadBalancerEnable"`

	// InsecureBrokerConnection disables certificate validation when contacting the broker.
	// This is useful for scenarios where the certificate chain isn't the same everywhere, e.g. with self-signed
	// certificates with a different trust chain in each cluster.
	// +optional
	// +kubebuilder:default=false
	InsecureBrokerConnection bool `json:"insecureBrokerConnection"`

	// CredentialsSecret is a reference to the secret with a certain cloud platform
	// credentials, the supported platform includes AWS, GCP, Azure, ROKS and OSD.
	// The submariner-addon will use these credentials to prepare Submariner cluster
	// environment. If the submariner cluster environment requires submariner-addon
	// preparation, this field should be specified.
	// +optional
	CredentialsSecret *corev1.LocalObjectReference `json:"credentialsSecret,omitempty"`

	// SubscriptionConfig represents a Submariner subscription. SubscriptionConfig
	// can be used to customize the Submariner subscription.
	// +optional
	SubscriptionConfig `json:"subscriptionConfig,omitempty"`

	// ImagePullSpecs represents the desired images of submariner components installed on the managed cluster.
	// If not specified, the default submariner images that was defined by submariner operator will be used.
	// +optional
	ImagePullSpecs SubmarinerImagePullSpecs `json:"imagePullSpecs,omitempty"`

	// GatewayConfig represents the gateways configuration of the Submariner.
	// +optional
	GatewayConfig `json:"gatewayConfig,omitempty"`
}

// SubscriptionConfig contains configuration specified for a submariner subscription.
type SubscriptionConfig struct {
	// Source represents the catalog source of a submariner subscription.
	// The default value is redhat-operators
	// +optional
	// +kubebuilder:default=redhat-operators
	Source string `json:"source,omitempty"`

	// SourceNamespace represents the catalog source namespace of a submariner subscription.
	// The default value is openshift-marketplace
	// +optional
	// +kubebuilder:default=openshift-marketplace
	SourceNamespace string `json:"sourceNamespace,omitempty"`

	// Channel represents the channel of a submariner subscription.
	// +optional
	Channel string `json:"channel,omitempty"`

	// StartingCSV represents the startingCSV of a submariner subscription.
	// +optional
	StartingCSV string `json:"startingCSV,omitempty"`

	// InstallPlanApproval determines whether subscription installation plans are applied automatically.
	// +optional
	InstallPlanApproval string `json:"installPlanApproval,omitempty"`
}

type SubmarinerImagePullSpecs struct {
	// SubmarinerImagePullSpec represents the desired image of submariner.
	// +optional
	SubmarinerImagePullSpec string `json:"submarinerImagePullSpec,omitempty"`

	// LighthouseAgentImagePullSpec represents the desired image of the lighthouse agent.
	// +optional
	LighthouseAgentImagePullSpec string `json:"lighthouseAgentImagePullSpec,omitempty"`

	// LighthouseCoreDNSImagePullSpec represents the desired image of lighthouse coredns.
	// +optional
	LighthouseCoreDNSImagePullSpec string `json:"lighthouseCoreDNSImagePullSpec,omitempty"`

	// SubmarinerRouteAgentImagePullSpec represents the desired image of the submariner route agent.
	// +optional
	SubmarinerRouteAgentImagePullSpec string `json:"submarinerRouteAgentImagePullSpec,omitempty"`

	// SubmarinerGlobalnetImagePullSpec represents the desired image of the submariner globalnet.
	// +optional
	SubmarinerGlobalnetImagePullSpec string `json:"submarinerGlobalnetImagePullSpec,omitempty"`

	// SubmarinerNetworkPluginSyncerImagePullSpec represents the desired image of the submariner networkplugin syncer.
	// +optional
	SubmarinerNetworkPluginSyncerImagePullSpec string `json:"submarinerNetworkPluginSyncerImagePullSpec,omitempty"`

	// MetricsProxyImagePullSpec represents the desired image of the metrics proxy.
	// +optional
	MetricsProxyImagePullSpec string `json:"metricsProxyImagePullSpec,omitempty"`

	// NettestImagePullSpec represents the desired image of nettest.
	// +optional
	NettestImagePullSpec string `json:"nettestImagePullSpec,omitempty"`
}

type GatewayConfig struct {
	// AWS represents the configuration for Amazon Web Services.
	// If the platform of managed cluster is not Amazon Web Services, this field will be ignored.
	// +optional
	AWS `json:"aws,omitempty"`

	// GCP represents the configuration for Google Cloud Platform.
	// If the platform of managed cluster is not Google Cloud Platform, this field will be ignored.
	// +optional
	GCP `json:"gcp,omitempty"`

	// Azure represents the configuration for Azure Cloud Platform.
	// If the platform of managed cluster is not Azure Cloud Platform, this field will be ignored.
	// +optional
	Azure `json:"azure,omitempty"`

	// RHOS represents the configuration for Redhat Openstack Platform.
	// If the platform of managed cluster is not Redhat Openstack Platform, this field will be ignored.
	// +optional
	RHOS `json:"rhos,omitempty"`

	// Gateways represents the count of worker nodes that will be used to deploy the Submariner gateway
	// component on the managed cluster. The default value is 1, if the value is greater than 1, the
	// Submariner gateway HA will be enabled automatically.
	// +optional
	// +kubebuilder:default=1
	Gateways int `json:"gateways,omitempty"`
}

type AWS struct {
	// InstanceType represents the Amazon Web Services EC2 instance type of the gateway node that will be
	// created on the managed cluster.
	// The default value is `m5n.large`.
	// +optional
	// +kubebuilder:default=m5n.large
	InstanceType string `json:"instanceType,omitempty"`
}

type GCP struct {
	// InstanceType represents the Google Cloud Platform instance type of the gateway node that will be
	// created on the managed cluster.
	// The default value is `n1-standard-4`.
	// +optional
	// +kubebuilder:default=n1-standard-4
	InstanceType string `json:"instanceType,omitempty"`
}

type RHOS struct {
	// InstanceType represents the Redhat Openstack instance type of the gateway node that will be
	// created on the managed cluster.
	// The default value is `PnTAE.CPU_4_Memory_8192_Disk_50`.
	// +optional
	// +kubebuilder:default=PnTAE.CPU_4_Memory_8192_Disk_50
	InstanceType string `json:"instanceType,omitempty"`
}

type Azure struct {
	// InstanceType represents the Azure Cloud Platform instance type of the gateway node that will be
	// created on the managed cluster.
	// The default value is `Standard_F4s_v2`.
	// +optional
	// +kubebuilder:default=Standard_F4s_v2
	InstanceType string `json:"instanceType,omitempty"`
}

const (
	// SubmarinerConfigConditionApplied means the configuration has successfully
	// applied.
	SubmarinerConfigConditionApplied string = "SubmarinerConfigApplied"

	// SubmarinerConfigConditionEnvPrepared means the submariner cluster environment
	// is prepared on a specfied cloud platform with the given cloud platform credentials.
	SubmarinerConfigConditionEnvPrepared string = "SubmarinerClusterEnvironmentPrepared"
)

// SubmarinerConfigStatus represents the current status of submariner configuration.
type SubmarinerConfigStatus struct {
	// Conditions contain the different condition statuses for this configuration.
	Conditions []metav1.Condition `json:"conditions"`
	// ManagedClusterInfo represents the information of a managed cluster.
	// +optional
	ManagedClusterInfo ManagedClusterInfo `json:"managedClusterInfo,omitempty"`
}

type ManagedClusterInfo struct {
	// ClusterName represents the name of the managed cluster.
	// +optional
	ClusterName string `json:"clusterName,omitempty"`
	// Vendor represents the kubernetes vendor of the managed cluster.
	// +optional
	Vendor string `json:"vendor,omitempty"`
	// Platform represents the cloud provider of the managed cluster.
	// +optional
	Platform string `json:"platform,omitempty"`
	// Region represents the cloud region of the managed cluster.
	// +optional
	Region string `json:"region,omitempty"`
	// InfraId represents the infrastructure id of the managed cluster.
	// +optional
	InfraID string `json:"infraId,omitempty"`
	// VendorVersion represents k8s vendor version of the managed cluster.
	// +optional
	VendorVersion string `json:"vendorVersion,omitempty"`
	// NetworkType represents the network type (cni) of the managed cluster.
	// +optional
	NetworkType string `json:"networkType,omitempty"`
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
