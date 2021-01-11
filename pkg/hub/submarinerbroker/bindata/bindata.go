// Code generated for package bindata by go-bindata DO NOT EDIT. (@generated)
// sources:
// manifests/broker/broker-cluster-role.yaml
// manifests/broker/broker-namespace.yaml
// manifests/broker/submariner.io_clusters_crd.yaml
// manifests/broker/submariner.io_endpoints_crd.yaml
// manifests/broker/submariner.io_gateways_crd.yaml
// manifests/broker/submariner.io_lighthouse.serviceimports_crd.yaml
// manifests/broker/x-k8s.io_multicluster.serviceimports_crd.yaml
package bindata

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type asset struct {
	bytes []byte
	info  os.FileInfo
}

type bindataFileInfo struct {
	name    string
	size    int64
	mode    os.FileMode
	modTime time.Time
}

// Name return file name
func (fi bindataFileInfo) Name() string {
	return fi.name
}

// Size return file size
func (fi bindataFileInfo) Size() int64 {
	return fi.size
}

// Mode return file mode
func (fi bindataFileInfo) Mode() os.FileMode {
	return fi.mode
}

// Mode return file modify time
func (fi bindataFileInfo) ModTime() time.Time {
	return fi.modTime
}

// IsDir return file whether a directory
func (fi bindataFileInfo) IsDir() bool {
	return fi.mode&os.ModeDir != 0
}

// Sys return file is sys mode
func (fi bindataFileInfo) Sys() interface{} {
	return nil
}

var _manifestsBrokerBrokerClusterRoleYaml = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: submariner-k8s-broker-cluster
  namespace: {{ .SubmarinerNamespace }}
rules:
- apiGroups: ["submariner.io"]
  resources: ["clusters", "endpoints"]
  verbs: ["create", "get", "list", "watch", "patch", "update", "delete"]
- apiGroups: ["lighthouse.submariner.io"]
  resources: ["*"]
  verbs: ["create", "get", "list", "watch", "patch", "update", "delete"]
- apiGroups: ["multicluster.x-k8s.io"]
  resources: ["*"]
  verbs: ["create", "get", "list", "watch", "patch", "update", "delete"]
- apiGroups: ["discovery.k8s.io"]
  resources: ["endpointslices"]
  verbs: ["create", "get", "list", "watch","patch", "update", "delete"]
`)

func manifestsBrokerBrokerClusterRoleYamlBytes() ([]byte, error) {
	return _manifestsBrokerBrokerClusterRoleYaml, nil
}

func manifestsBrokerBrokerClusterRoleYaml() (*asset, error) {
	bytes, err := manifestsBrokerBrokerClusterRoleYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/broker/broker-cluster-role.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsBrokerBrokerNamespaceYaml = []byte(`apiVersion: v1
kind: Namespace
metadata:
  name: {{ .SubmarinerNamespace }}
`)

func manifestsBrokerBrokerNamespaceYamlBytes() ([]byte, error) {
	return _manifestsBrokerBrokerNamespaceYaml, nil
}

func manifestsBrokerBrokerNamespaceYaml() (*asset, error) {
	bytes, err := manifestsBrokerBrokerNamespaceYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/broker/broker-namespace.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsBrokerSubmarinerIo_clusters_crdYaml = []byte(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: clusters.submariner.io
  ownerReferences:
  - apiVersion: apiextensions.k8s.io/v1beta1
    controller: true
    blockOwnerDeletion: true
    kind: CustomResourceDefinition
    name: submarinerconfigs.submarineraddon.open-cluster-management.io
    uid: {{ .ConfigCRDUID }}
spec:
  group: submariner.io
  names:
    kind: Cluster
    listKind: ClusterList
    plural: clusters
    singular: cluster
  scope: Namespaced
  versions:
  - name: v1
    schema:
      openAPIV3Schema:
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
            type: string
          kind:
            description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          spec:
            properties:
              cluster_cidr:
                items:
                  type: string
                type: array
              cluster_id:
                type: string
              color_codes:
                items:
                  type: string
                type: array
              global_cidr:
                items:
                  type: string
                type: array
              service_cidr:
                items:
                  type: string
                type: array
            required:
            - cluster_cidr
            - cluster_id
            - color_codes
            - global_cidr
            - service_cidr
            type: object
        required:
        - spec
        type: object
    served: true
    storage: true
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: []
  storedVersions: []
`)

func manifestsBrokerSubmarinerIo_clusters_crdYamlBytes() ([]byte, error) {
	return _manifestsBrokerSubmarinerIo_clusters_crdYaml, nil
}

func manifestsBrokerSubmarinerIo_clusters_crdYaml() (*asset, error) {
	bytes, err := manifestsBrokerSubmarinerIo_clusters_crdYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/broker/submariner.io_clusters_crd.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsBrokerSubmarinerIo_endpoints_crdYaml = []byte(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: endpoints.submariner.io
  ownerReferences:
  - apiVersion: apiextensions.k8s.io/v1beta1
    controller: true
    blockOwnerDeletion: true
    kind: CustomResourceDefinition
    name: submarinerconfigs.submarineraddon.open-cluster-management.io
    uid: {{ .ConfigCRDUID }}
spec:
  group: submariner.io
  names:
    kind: Endpoint
    listKind: EndpointList
    plural: endpoints
    singular: endpoint
  scope: Namespaced
  versions:
  - name: v1
    schema:
      openAPIV3Schema:
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
            type: string
          kind:
            description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          spec:
            properties:
              backend:
                type: string
              backend_config:
                additionalProperties:
                  type: string
                type: object
              cable_name:
                type: string
              cluster_id:
                type: string
              healthCheckIP:
                type: string
              hostname:
                type: string
              nat_enabled:
                type: boolean
              private_ip:
                type: string
              public_ip:
                type: string
              subnets:
                items:
                  type: string
                type: array
            required:
            - backend
            - cable_name
            - cluster_id
            - hostname
            - nat_enabled
            - private_ip
            - public_ip
            - subnets
            type: object
        required:
        - spec
        type: object
    served: true
    storage: true
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: []
  storedVersions: []
`)

func manifestsBrokerSubmarinerIo_endpoints_crdYamlBytes() ([]byte, error) {
	return _manifestsBrokerSubmarinerIo_endpoints_crdYaml, nil
}

func manifestsBrokerSubmarinerIo_endpoints_crdYaml() (*asset, error) {
	bytes, err := manifestsBrokerSubmarinerIo_endpoints_crdYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/broker/submariner.io_endpoints_crd.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsBrokerSubmarinerIo_gateways_crdYaml = []byte(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: gateways.submariner.io
  ownerReferences:
  - apiVersion: apiextensions.k8s.io/v1beta1
    controller: true
    blockOwnerDeletion: true
    kind: CustomResourceDefinition
    name: submarinerconfigs.submarineraddon.open-cluster-management.io
    uid: {{ .ConfigCRDUID }}
spec:
  group: submariner.io
  names:
    kind: Gateway
    listKind: GatewayList
    plural: gateways
    singular: gateway
  scope: Namespaced
  versions:
  - additionalPrinterColumns:
    - description: High availability status of the Gateway
      jsonPath: .status.haStatus
      name: HA Status
      type: string
    name: v1
    schema:
      openAPIV3Schema:
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
            type: string
          kind:
            description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          status:
            properties:
              connections:
                items:
                  properties:
                    endpoint:
                      properties:
                        backend:
                          type: string
                        backend_config:
                          additionalProperties:
                            type: string
                          type: object
                        cable_name:
                          type: string
                        cluster_id:
                          type: string
                        healthCheckIP:
                          type: string
                        hostname:
                          type: string
                        nat_enabled:
                          type: boolean
                        private_ip:
                          type: string
                        public_ip:
                          type: string
                        subnets:
                          items:
                            type: string
                          type: array
                      required:
                      - backend
                      - cable_name
                      - cluster_id
                      - hostname
                      - nat_enabled
                      - private_ip
                      - public_ip
                      - subnets
                      type: object
                    latency:
                      description: LatencySpec describes the round trip time information
                        in nanoseconds for a packet between the gateway pods of two
                        clusters.
                      properties:
                        averageRTT:
                          format: int64
                          type: integer
                        lastRTT:
                          description: TODO This shall be deleted once the operator
                            is using the latest. Using Optional to avoid validation
                            errors when this field is not used.
                          format: int64
                          type: integer
                        maxRTT:
                          format: int64
                          type: integer
                        minRTT:
                          format: int64
                          type: integer
                        stddevRTT:
                          format: int64
                          type: integer
                      type: object
                    latencyRTT:
                      description: LatencySpec describes the round trip time information
                        for a packet between the gateway pods of two clusters.
                      properties:
                        average:
                          type: string
                        last:
                          type: string
                        max:
                          type: string
                        min:
                          type: string
                        stdDev:
                          type: string
                      type: object
                    status:
                      type: string
                    statusMessage:
                      type: string
                  required:
                  - endpoint
                  - status
                  - statusMessage
                  type: object
                type: array
              haStatus:
                type: string
              localEndpoint:
                properties:
                  backend:
                    type: string
                  backend_config:
                    additionalProperties:
                      type: string
                    type: object
                  cable_name:
                    type: string
                  cluster_id:
                    type: string
                  healthCheckIP:
                    type: string
                  hostname:
                    type: string
                  nat_enabled:
                    type: boolean
                  private_ip:
                    type: string
                  public_ip:
                    type: string
                  subnets:
                    items:
                      type: string
                    type: array
                required:
                - backend
                - cable_name
                - cluster_id
                - hostname
                - nat_enabled
                - private_ip
                - public_ip
                - subnets
                type: object
              statusFailure:
                type: string
              version:
                type: string
            required:
            - connections
            - haStatus
            - localEndpoint
            - statusFailure
            - version
            type: object
        required:
        - status
        type: object
    served: true
    storage: true
    subresources: {}
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: []
  storedVersions: []
`)

func manifestsBrokerSubmarinerIo_gateways_crdYamlBytes() ([]byte, error) {
	return _manifestsBrokerSubmarinerIo_gateways_crdYaml, nil
}

func manifestsBrokerSubmarinerIo_gateways_crdYaml() (*asset, error) {
	bytes, err := manifestsBrokerSubmarinerIo_gateways_crdYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/broker/submariner.io_gateways_crd.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsBrokerSubmarinerIo_lighthouseServiceimports_crdYaml = []byte(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: serviceimports.lighthouse.submariner.io
  ownerReferences:
  - apiVersion: apiextensions.k8s.io/v1beta1
    controller: true
    blockOwnerDeletion: true
    kind: CustomResourceDefinition
    name: submarinerconfigs.submarineraddon.open-cluster-management.io
    uid: {{ .ConfigCRDUID }}
spec:
  group: lighthouse.submariner.io
  names:
    kind: ServiceImport
    listKind: ServiceImportList
    plural: serviceimports
    singular: serviceimport
  scope: Namespaced
  versions:
  - name: v2alpha1
    schema:
      openAPIV3Schema:
        description: ServiceImport describes a service imported from clusters in a
          clusterset.
        properties:
          apiVersion:
            description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
            type: string
          kind:
            description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
            type: string
          metadata:
            type: object
          spec:
            description: ServiceImportSpec describes an imported service and the information
              necessary to consume it.
            properties:
              ip:
                type: string
              ports:
                items:
                  description: ServicePort represents the port on which the service
                    is exposed
                  properties:
                    appProtocol:
                      description: The application protocol for this port. This field
                        follows standard Kubernetes label syntax. Un-prefixed names
                        are reserved for IANA standard service names (as per RFC-6335
                        and http://www.iana.org/assignments/service-names). Non-standard
                        protocols should use prefixed names such as mycompany.com/my-custom-protocol.
                        Field can be enabled with ServiceAppProtocol feature gate.
                      type: string
                    name:
                      description: The name of this port within the service. This
                        must be a DNS_LABEL. All ports within a ServiceSpec must have
                        unique names. When considering the endpoints for a Service,
                        this must match the 'name' field in the EndpointPort. Optional
                        if only one ServicePort is defined on this service.
                      type: string
                    port:
                      description: The port that will be exposed by this service.
                      format: int32
                      type: integer
                    protocol:
                      description: The IP protocol for this port. Supports "TCP",
                        "UDP", and "SCTP". Default is TCP.
                      type: string
                  required:
                  - port
                  type: object
                type: array
                x-kubernetes-list-type: atomic
              sessionAffinity:
                description: Session Affinity Type string
                type: string
              sessionAffinityConfig:
                description: SessionAffinityConfig represents the configurations of
                  session affinity.
                properties:
                  clientIP:
                    description: clientIP contains the configurations of Client IP
                      based session affinity.
                    properties:
                      timeoutSeconds:
                        description: timeoutSeconds specifies the seconds of ClientIP
                          type session sticky time. The value must be >0 && <=86400(for
                          1 day) if ServiceAffinity == "ClientIP". Default value is
                          10800(for 3 hours).
                        format: int32
                        type: integer
                    type: object
                type: object
              type:
                description: ServiceImportType designates the type of a ServiceImport
                type: string
            required:
            - ports
            type: object
          status:
            description: ServiceImportStatus describes derived state of an imported
              service.
            properties:
              clusters:
                items:
                  description: ClusterStatus contains service configuration mapped
                    to a specific source cluster
                  properties:
                    cluster:
                      type: string
                    ips:
                      description: The IP(s) of the service running in the cluster.  In
                        the case of a headless service, it is the list of pod IPs
                        that back the service.
                      items:
                        type: string
                      type: array
                  required:
                  - cluster
                  type: object
                type: array
                x-kubernetes-list-map-keys:
                - cluster
                x-kubernetes-list-type: map
            type: object
        type: object
    served: true
    storage: true
status:
  acceptedNames:
    kind: ""
    plural: ""
  conditions: []
  storedVersions: []
`)

func manifestsBrokerSubmarinerIo_lighthouseServiceimports_crdYamlBytes() ([]byte, error) {
	return _manifestsBrokerSubmarinerIo_lighthouseServiceimports_crdYaml, nil
}

func manifestsBrokerSubmarinerIo_lighthouseServiceimports_crdYaml() (*asset, error) {
	bytes, err := manifestsBrokerSubmarinerIo_lighthouseServiceimports_crdYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/broker/submariner.io_lighthouse.serviceimports_crd.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsBrokerXK8sIo_multiclusterServiceimports_crdYaml = []byte(`apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: serviceimports.multicluster.x-k8s.io
  ownerReferences:
  - apiVersion: apiextensions.k8s.io/v1beta1
    controller: true
    blockOwnerDeletion: true
    kind: CustomResourceDefinition
    name: submarinerconfigs.submarineraddon.open-cluster-management.io
    uid: {{ .ConfigCRDUID }}
spec:
  group: multicluster.x-k8s.io
  scope: Namespaced
  names:
    plural: serviceimports
    singular: serviceimport
    kind: ServiceImport
    shortNames:
      - svcim
  versions:
    - name: v1alpha1
      served: true
      storage: true
      subresources:
        status: {}
      additionalPrinterColumns:
        - name: Type
          type: string
          description: The type of this ServiceImport
          jsonPath: .spec.type
        - name: IP
          type: string
          description: The VIP for this ServiceImport
          jsonPath: .spec.ips
        - name: Age
          type: date
          jsonPath: .metadata.creationTimestamp
      schema:
        openAPIV3Schema:
          description: ServiceImport describes a service imported from clusters in a
            ClusterSet.
          type: object
          properties:
            apiVersion:
              description: 'APIVersion defines the versioned schema of this representation
              of an object. Servers should convert recognized schemas to the latest
              internal value, and may reject unrecognized values. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources'
              type: string
            kind:
              description: 'Kind is a string value representing the REST resource this
              object represents. Servers may infer this from the endpoint the client
              submits requests to. Cannot be updated. In CamelCase. More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds'
              type: string
            metadata:
              type: object
            spec:
              description: spec defines the behavior of a ServiceImport.
              type: object
              required:
                - ports
                - type
              properties:
                ips:
                  description: ip will be used as the VIP for this service when type
                    is ClusterSetIP.
                  type: array
                  maxItems: 1
                  items:
                    type: string
                ports:
                  type: array
                  items:
                    description: ServicePort represents the port on which the service
                      is exposed
                    type: object
                    required:
                      - port
                    properties:
                      appProtocol:
                        description: The application protocol for this port. This field
                          follows standard Kubernetes label syntax. Un-prefixed names
                          are reserved for IANA standard service names (as per RFC-6335
                          and http://www.iana.org/assignments/service-names). Non-standard
                          protocols should use prefixed names such as mycompany.com/my-custom-protocol.
                          Field can be enabled with ServiceAppProtocol feature gate.
                        type: string
                      name:
                        description: The name of this port within the service. This
                          must be a DNS_LABEL. All ports within a ServiceSpec must have
                          unique names. When considering the endpoints for a Service,
                          this must match the 'name' field in the EndpointPort. Optional
                          if only one ServicePort is defined on this service.
                        type: string
                      port:
                        description: The port that will be exposed by this service.
                        type: integer
                        format: int32
                      protocol:
                        description: The IP protocol for this port. Supports "TCP",
                          "UDP", and "SCTP". Default is TCP.
                        type: string
                  x-kubernetes-list-type: atomic
                sessionAffinity:
                  description: 'Supports "ClientIP" and "None". Used to maintain session
                  affinity. Enable client IP based session affinity. Must be ClientIP
                  or None. Defaults to None. Ignored when type is Headless More info:
                  https://kubernetes.io/docs/concepts/services-networking/service/#virtual-ips-and-service-proxies'
                  type: string
                sessionAffinityConfig:
                  description: sessionAffinityConfig contains session affinity configuration.
                  type: object
                  properties:
                    clientIP:
                      description: clientIP contains the configurations of Client IP
                        based session affinity.
                      type: object
                      properties:
                        timeoutSeconds:
                          description: timeoutSeconds specifies the seconds of ClientIP
                            type session sticky time. The value must be >0 && <=86400(for
                            1 day) if ServiceAffinity == "ClientIP". Default value is
                            10800(for 3 hours).
                          type: integer
                          format: int32
                type:
                  description: type defines the type of this service. Must be ClusterSetIP
                    or Headless.
                  type: string
                  enum:
                    - ClusterSetIP
                    - Headless
            status:
              description: status contains information about the exported services that
                form the multi-cluster service referenced by this ServiceImport.
              type: object
              properties:
                clusters:
                  description: clusters is the list of exporting clusters from which
                    this service was derived.
                  type: array
                  items:
                    description: ClusterStatus contains service configuration mapped
                      to a specific source cluster
                    type: object
                    required:
                      - cluster
                    properties:
                      cluster:
                        description: cluster is the name of the exporting cluster. Must
                          be a valid RFC-1123 DNS label.
                        type: string
                  x-kubernetes-list-map-keys:
                    - cluster
                  x-kubernetes-list-type: map
`)

func manifestsBrokerXK8sIo_multiclusterServiceimports_crdYamlBytes() ([]byte, error) {
	return _manifestsBrokerXK8sIo_multiclusterServiceimports_crdYaml, nil
}

func manifestsBrokerXK8sIo_multiclusterServiceimports_crdYaml() (*asset, error) {
	bytes, err := manifestsBrokerXK8sIo_multiclusterServiceimports_crdYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/broker/x-k8s.io_multicluster.serviceimports_crd.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

// Asset loads and returns the asset for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func Asset(name string) ([]byte, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("Asset %s can't read by error: %v", name, err)
		}
		return a.bytes, nil
	}
	return nil, fmt.Errorf("Asset %s not found", name)
}

// MustAsset is like Asset but panics when Asset would return an error.
// It simplifies safe initialization of global variables.
func MustAsset(name string) []byte {
	a, err := Asset(name)
	if err != nil {
		panic("asset: Asset(" + name + "): " + err.Error())
	}

	return a
}

// AssetInfo loads and returns the asset info for the given name.
// It returns an error if the asset could not be found or
// could not be loaded.
func AssetInfo(name string) (os.FileInfo, error) {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	if f, ok := _bindata[cannonicalName]; ok {
		a, err := f()
		if err != nil {
			return nil, fmt.Errorf("AssetInfo %s can't read by error: %v", name, err)
		}
		return a.info, nil
	}
	return nil, fmt.Errorf("AssetInfo %s not found", name)
}

// AssetNames returns the names of the assets.
func AssetNames() []string {
	names := make([]string, 0, len(_bindata))
	for name := range _bindata {
		names = append(names, name)
	}
	return names
}

// _bindata is a table, holding each asset generator, mapped to its name.
var _bindata = map[string]func() (*asset, error){
	"manifests/broker/broker-cluster-role.yaml":                         manifestsBrokerBrokerClusterRoleYaml,
	"manifests/broker/broker-namespace.yaml":                            manifestsBrokerBrokerNamespaceYaml,
	"manifests/broker/submariner.io_clusters_crd.yaml":                  manifestsBrokerSubmarinerIo_clusters_crdYaml,
	"manifests/broker/submariner.io_endpoints_crd.yaml":                 manifestsBrokerSubmarinerIo_endpoints_crdYaml,
	"manifests/broker/submariner.io_gateways_crd.yaml":                  manifestsBrokerSubmarinerIo_gateways_crdYaml,
	"manifests/broker/submariner.io_lighthouse.serviceimports_crd.yaml": manifestsBrokerSubmarinerIo_lighthouseServiceimports_crdYaml,
	"manifests/broker/x-k8s.io_multicluster.serviceimports_crd.yaml":    manifestsBrokerXK8sIo_multiclusterServiceimports_crdYaml,
}

// AssetDir returns the file names below a certain
// directory embedded in the file by go-bindata.
// For example if you run go-bindata on data/... and data contains the
// following hierarchy:
//     data/
//       foo.txt
//       img/
//         a.png
//         b.png
// then AssetDir("data") would return []string{"foo.txt", "img"}
// AssetDir("data/img") would return []string{"a.png", "b.png"}
// AssetDir("foo.txt") and AssetDir("notexist") would return an error
// AssetDir("") will return []string{"data"}.
func AssetDir(name string) ([]string, error) {
	node := _bintree
	if len(name) != 0 {
		cannonicalName := strings.Replace(name, "\\", "/", -1)
		pathList := strings.Split(cannonicalName, "/")
		for _, p := range pathList {
			node = node.Children[p]
			if node == nil {
				return nil, fmt.Errorf("Asset %s not found", name)
			}
		}
	}
	if node.Func != nil {
		return nil, fmt.Errorf("Asset %s not found", name)
	}
	rv := make([]string, 0, len(node.Children))
	for childName := range node.Children {
		rv = append(rv, childName)
	}
	return rv, nil
}

type bintree struct {
	Func     func() (*asset, error)
	Children map[string]*bintree
}

var _bintree = &bintree{nil, map[string]*bintree{
	"manifests": {nil, map[string]*bintree{
		"broker": {nil, map[string]*bintree{
			"broker-cluster-role.yaml":                         {manifestsBrokerBrokerClusterRoleYaml, map[string]*bintree{}},
			"broker-namespace.yaml":                            {manifestsBrokerBrokerNamespaceYaml, map[string]*bintree{}},
			"submariner.io_clusters_crd.yaml":                  {manifestsBrokerSubmarinerIo_clusters_crdYaml, map[string]*bintree{}},
			"submariner.io_endpoints_crd.yaml":                 {manifestsBrokerSubmarinerIo_endpoints_crdYaml, map[string]*bintree{}},
			"submariner.io_gateways_crd.yaml":                  {manifestsBrokerSubmarinerIo_gateways_crdYaml, map[string]*bintree{}},
			"submariner.io_lighthouse.serviceimports_crd.yaml": {manifestsBrokerSubmarinerIo_lighthouseServiceimports_crdYaml, map[string]*bintree{}},
			"x-k8s.io_multicluster.serviceimports_crd.yaml":    {manifestsBrokerXK8sIo_multiclusterServiceimports_crdYaml, map[string]*bintree{}},
		}},
	}},
}}

// RestoreAsset restores an asset under the given directory
func RestoreAsset(dir, name string) error {
	data, err := Asset(name)
	if err != nil {
		return err
	}
	info, err := AssetInfo(name)
	if err != nil {
		return err
	}
	err = os.MkdirAll(_filePath(dir, filepath.Dir(name)), os.FileMode(0755))
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(_filePath(dir, name), data, info.Mode())
	if err != nil {
		return err
	}
	err = os.Chtimes(_filePath(dir, name), info.ModTime(), info.ModTime())
	if err != nil {
		return err
	}
	return nil
}

// RestoreAssets restores an asset under the given directory recursively
func RestoreAssets(dir, name string) error {
	children, err := AssetDir(name)
	// File
	if err != nil {
		return RestoreAsset(dir, name)
	}
	// Dir
	for _, child := range children {
		err = RestoreAssets(dir, filepath.Join(name, child))
		if err != nil {
			return err
		}
	}
	return nil
}

func _filePath(dir, name string) string {
	cannonicalName := strings.Replace(name, "\\", "/", -1)
	return filepath.Join(append([]string{dir}, strings.Split(cannonicalName, "/")...)...)
}
