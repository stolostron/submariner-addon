// Code generated for package bindata by go-bindata DO NOT EDIT. (@generated)
// sources:
// manifests/crds/lighthouse.submariner.io_multiclusterservices_crd.yaml
// manifests/crds/lighthouse.submariner.io_serviceexports_crd.yaml
// manifests/crds/lighthouse.submariner.io_serviceimports_crd.yaml
// manifests/crds/submariner.io_clusters_crd.yaml
// manifests/crds/submariner.io_endpoints_crd.yaml
// manifests/crds/submariner.io_gateways_crd.yaml
// manifests/crds/submariner.io_servicediscoveries_crd.yaml
// manifests/crds/submariner.io_submariners_crd.yaml
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

var _manifestsCrdsLighthouseSubmarinerIo_multiclusterservices_crdYaml = []byte(`---
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: multiclusterservices.lighthouse.submariner.io
spec:
  group: lighthouse.submariner.io
  version: v1
  names:
    kind: MultiClusterService
    plural: multiclusterservices
    singular: multiclusterservice
  scope: Namespaced
  validation:
    openAPIV3Schema:
      properties:
        spec:
          properties:
            clusterServiceInfo:
              properties:
                clusterID:
                  type: "string"
                  minimum: 1
                clusterDomain:
                  type: "string"
                  minimum: 0
                serviceIP:
                  type: "string"
                  minimum: 1
                port:
                  type: "integer"
                  minimum: 0
`)

func manifestsCrdsLighthouseSubmarinerIo_multiclusterservices_crdYamlBytes() ([]byte, error) {
	return _manifestsCrdsLighthouseSubmarinerIo_multiclusterservices_crdYaml, nil
}

func manifestsCrdsLighthouseSubmarinerIo_multiclusterservices_crdYaml() (*asset, error) {
	bytes, err := manifestsCrdsLighthouseSubmarinerIo_multiclusterservices_crdYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/crds/lighthouse.submariner.io_multiclusterservices_crd.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsCrdsLighthouseSubmarinerIo_serviceexports_crdYaml = []byte(`apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: serviceexports.lighthouse.submariner.io
spec:
  group: lighthouse.submariner.io
  version: v2alpha1
  names:
    kind: ServiceExport
    plural: serviceexports
    singular: serviceexport
  scope: Namespaced
`)

func manifestsCrdsLighthouseSubmarinerIo_serviceexports_crdYamlBytes() ([]byte, error) {
	return _manifestsCrdsLighthouseSubmarinerIo_serviceexports_crdYaml, nil
}

func manifestsCrdsLighthouseSubmarinerIo_serviceexports_crdYaml() (*asset, error) {
	bytes, err := manifestsCrdsLighthouseSubmarinerIo_serviceexports_crdYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/crds/lighthouse.submariner.io_serviceexports_crd.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsCrdsLighthouseSubmarinerIo_serviceimports_crdYaml = []byte(`apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: serviceimports.lighthouse.submariner.io
spec:
  group: lighthouse.submariner.io
  version: v2alpha1
  names:
    kind: ServiceImport
    plural: serviceimports
    singular: serviceimport
  scope: Namespaced
`)

func manifestsCrdsLighthouseSubmarinerIo_serviceimports_crdYamlBytes() ([]byte, error) {
	return _manifestsCrdsLighthouseSubmarinerIo_serviceimports_crdYaml, nil
}

func manifestsCrdsLighthouseSubmarinerIo_serviceimports_crdYaml() (*asset, error) {
	bytes, err := manifestsCrdsLighthouseSubmarinerIo_serviceimports_crdYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/crds/lighthouse.submariner.io_serviceimports_crd.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsCrdsSubmarinerIo_clusters_crdYaml = []byte(`apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: clusters.submariner.io
spec:
  group: submariner.io
  version: v1
  names:
    kind: Cluster
    listKind: ClusterList
    plural: clusters
    singular: cluster
  scope: Namespaced
`)

func manifestsCrdsSubmarinerIo_clusters_crdYamlBytes() ([]byte, error) {
	return _manifestsCrdsSubmarinerIo_clusters_crdYaml, nil
}

func manifestsCrdsSubmarinerIo_clusters_crdYaml() (*asset, error) {
	bytes, err := manifestsCrdsSubmarinerIo_clusters_crdYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/crds/submariner.io_clusters_crd.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsCrdsSubmarinerIo_endpoints_crdYaml = []byte(`apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: endpoints.submariner.io
spec:
  group: submariner.io
  version: v1
  names:
    kind: Endpoint
    listKind: EndpointList
    plural: endpoints
    singular: endpoint
  scope: Namespaced
`)

func manifestsCrdsSubmarinerIo_endpoints_crdYamlBytes() ([]byte, error) {
	return _manifestsCrdsSubmarinerIo_endpoints_crdYaml, nil
}

func manifestsCrdsSubmarinerIo_endpoints_crdYaml() (*asset, error) {
	bytes, err := manifestsCrdsSubmarinerIo_endpoints_crdYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/crds/submariner.io_endpoints_crd.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsCrdsSubmarinerIo_gateways_crdYaml = []byte(`apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: gateways.submariner.io
spec:
  group: submariner.io
  version: v1
  names:
    kind: Gateway
    listKind: GatewayList
    plural: gateways
    singular: gateway
  scope: Namespaced
  additionalPrinterColumns:
    - name: ha-status
      type: string
      description: High Availability Status of the Gateway
      JSONPath: .status.haStatus
`)

func manifestsCrdsSubmarinerIo_gateways_crdYamlBytes() ([]byte, error) {
	return _manifestsCrdsSubmarinerIo_gateways_crdYaml, nil
}

func manifestsCrdsSubmarinerIo_gateways_crdYaml() (*asset, error) {
	bytes, err := manifestsCrdsSubmarinerIo_gateways_crdYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/crds/submariner.io_gateways_crd.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsCrdsSubmarinerIo_servicediscoveries_crdYaml = []byte(`apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: servicediscoveries.submariner.io
spec:
  group: submariner.io
  names:
    kind: ServiceDiscovery
    listKind: ServiceDiscoveryList
    plural: servicediscoveries
    singular: servicediscovery
  scope: Namespaced
  subresources:
    status: {}
  validation:
    openAPIV3Schema:
      description: ServiceDiscovery is the Schema for the servicediscoveries API
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
          description: ServiceDiscoverySpec defines the desired state of ServiceDiscovery
          properties:
            brokerK8sApiServer:
              type: string
            brokerK8sApiServerToken:
              type: string
            brokerK8sCA:
              type: string
            brokerK8sRemoteNamespace:
              type: string
            clusterID:
              type: string
            debug:
              type: boolean
            globalnetEnabled:
              type: boolean
            namespace:
              type: string
            repository:
              type: string
            version:
              type: string
          required:
          - brokerK8sApiServer
          - brokerK8sApiServerToken
          - brokerK8sCA
          - brokerK8sRemoteNamespace
          - clusterID
          - debug
          - namespace
          type: object
        status:
          description: ServiceDiscoveryStatus defines the observed state of ServiceDiscovery
          type: object
      type: object
  version: v1alpha1
  versions:
  - name: v1alpha1
    served: true
    storage: true
`)

func manifestsCrdsSubmarinerIo_servicediscoveries_crdYamlBytes() ([]byte, error) {
	return _manifestsCrdsSubmarinerIo_servicediscoveries_crdYaml, nil
}

func manifestsCrdsSubmarinerIo_servicediscoveries_crdYaml() (*asset, error) {
	bytes, err := manifestsCrdsSubmarinerIo_servicediscoveries_crdYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/crds/submariner.io_servicediscoveries_crd.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsCrdsSubmarinerIo_submariners_crdYaml = []byte(`apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: submariners.submariner.io
spec:
  group: submariner.io
  names:
    kind: Submariner
    listKind: SubmarinerList
    plural: submariners
    singular: submariner
  scope: Namespaced
  subresources:
    status: {}
  validation:
    openAPIV3Schema:
      description: Submariner is the Schema for the submariners API
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
          description: SubmarinerSpec defines the desired state of Submariner
          properties:
            broker:
              type: string
            brokerK8sApiServer:
              type: string
            brokerK8sApiServerToken:
              type: string
            brokerK8sCA:
              type: string
            brokerK8sRemoteNamespace:
              type: string
            cableDriver:
              type: string
            ceIPSecDebug:
              type: boolean
            ceIPSecIKEPort:
              type: integer
            ceIPSecNATTPort:
              type: integer
            ceIPSecPSK:
              type: string
            clusterCIDR:
              type: string
            clusterID:
              type: string
            colorCodes:
              type: string
            debug:
              type: boolean
            globalCIDR:
              type: string
            namespace:
              type: string
            natEnabled:
              type: boolean
            repository:
              type: string
            serviceCIDR:
              type: string
            serviceDiscoveryEnabled:
              type: boolean
            version:
              type: string
          required:
          - broker
          - brokerK8sApiServer
          - brokerK8sApiServerToken
          - brokerK8sCA
          - brokerK8sRemoteNamespace
          - ceIPSecDebug
          - ceIPSecPSK
          - clusterCIDR
          - clusterID
          - debug
          - namespace
          - natEnabled
          - serviceCIDR
          type: object
        status:
          description: SubmarinerStatus defines the observed state of Submariner
          properties:
            clusterCIDR:
              type: string
            clusterID:
              type: string
            colorCodes:
              type: string
            engineDaemonSetStatus:
              description: DaemonSetStatus represents the current status of a daemon
                set.
              properties:
                collisionCount:
                  description: Count of hash collisions for the DaemonSet. The DaemonSet
                    controller uses this field as a collision avoidance mechanism
                    when it needs to create the name for the newest ControllerRevision.
                  format: int32
                  type: integer
                conditions:
                  description: Represents the latest available observations of a DaemonSet's
                    current state.
                  items:
                    description: DaemonSetCondition describes the state of a DaemonSet
                      at a certain point.
                    properties:
                      lastTransitionTime:
                        description: Last time the condition transitioned from one
                          status to another.
                        format: date-time
                        type: string
                      message:
                        description: A human readable message indicating details about
                          the transition.
                        type: string
                      reason:
                        description: The reason for the condition's last transition.
                        type: string
                      status:
                        description: Status of the condition, one of True, False,
                          Unknown.
                        type: string
                      type:
                        description: Type of DaemonSet condition.
                        type: string
                    required:
                    - status
                    - type
                    type: object
                  type: array
                currentNumberScheduled:
                  description: 'The number of nodes that are running at least 1 daemon
                    pod and are supposed to run the daemon pod. More info: https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/'
                  format: int32
                  type: integer
                desiredNumberScheduled:
                  description: 'The total number of nodes that should be running the
                    daemon pod (including nodes correctly running the daemon pod).
                    More info: https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/'
                  format: int32
                  type: integer
                numberAvailable:
                  description: The number of nodes that should be running the daemon
                    pod and have one or more of the daemon pod running and available
                    (ready for at least spec.minReadySeconds)
                  format: int32
                  type: integer
                numberMisscheduled:
                  description: 'The number of nodes that are running the daemon pod,
                    but are not supposed to run the daemon pod. More info: https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/'
                  format: int32
                  type: integer
                numberReady:
                  description: The number of nodes that should be running the daemon
                    pod and have one or more of the daemon pod running and ready.
                  format: int32
                  type: integer
                numberUnavailable:
                  description: The number of nodes that should be running the daemon
                    pod and have none of the daemon pod running and available (ready
                    for at least spec.minReadySeconds)
                  format: int32
                  type: integer
                observedGeneration:
                  description: The most recent generation observed by the daemon set
                    controller.
                  format: int64
                  type: integer
                updatedNumberScheduled:
                  description: The total number of nodes that are running updated
                    daemon pod
                  format: int32
                  type: integer
              required:
              - currentNumberScheduled
              - desiredNumberScheduled
              - numberMisscheduled
              - numberReady
              type: object
            gateways:
              items:
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
              type: array
            globalCIDR:
              type: string
            globalnetDaemonSetStatus:
              description: DaemonSetStatus represents the current status of a daemon
                set.
              properties:
                collisionCount:
                  description: Count of hash collisions for the DaemonSet. The DaemonSet
                    controller uses this field as a collision avoidance mechanism
                    when it needs to create the name for the newest ControllerRevision.
                  format: int32
                  type: integer
                conditions:
                  description: Represents the latest available observations of a DaemonSet's
                    current state.
                  items:
                    description: DaemonSetCondition describes the state of a DaemonSet
                      at a certain point.
                    properties:
                      lastTransitionTime:
                        description: Last time the condition transitioned from one
                          status to another.
                        format: date-time
                        type: string
                      message:
                        description: A human readable message indicating details about
                          the transition.
                        type: string
                      reason:
                        description: The reason for the condition's last transition.
                        type: string
                      status:
                        description: Status of the condition, one of True, False,
                          Unknown.
                        type: string
                      type:
                        description: Type of DaemonSet condition.
                        type: string
                    required:
                    - status
                    - type
                    type: object
                  type: array
                currentNumberScheduled:
                  description: 'The number of nodes that are running at least 1 daemon
                    pod and are supposed to run the daemon pod. More info: https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/'
                  format: int32
                  type: integer
                desiredNumberScheduled:
                  description: 'The total number of nodes that should be running the
                    daemon pod (including nodes correctly running the daemon pod).
                    More info: https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/'
                  format: int32
                  type: integer
                numberAvailable:
                  description: The number of nodes that should be running the daemon
                    pod and have one or more of the daemon pod running and available
                    (ready for at least spec.minReadySeconds)
                  format: int32
                  type: integer
                numberMisscheduled:
                  description: 'The number of nodes that are running the daemon pod,
                    but are not supposed to run the daemon pod. More info: https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/'
                  format: int32
                  type: integer
                numberReady:
                  description: The number of nodes that should be running the daemon
                    pod and have one or more of the daemon pod running and ready.
                  format: int32
                  type: integer
                numberUnavailable:
                  description: The number of nodes that should be running the daemon
                    pod and have none of the daemon pod running and available (ready
                    for at least spec.minReadySeconds)
                  format: int32
                  type: integer
                observedGeneration:
                  description: The most recent generation observed by the daemon set
                    controller.
                  format: int64
                  type: integer
                updatedNumberScheduled:
                  description: The total number of nodes that are running updated
                    daemon pod
                  format: int32
                  type: integer
              required:
              - currentNumberScheduled
              - desiredNumberScheduled
              - numberMisscheduled
              - numberReady
              type: object
            natEnabled:
              type: boolean
            routeAgentDaemonSetStatus:
              description: DaemonSetStatus represents the current status of a daemon
                set.
              properties:
                collisionCount:
                  description: Count of hash collisions for the DaemonSet. The DaemonSet
                    controller uses this field as a collision avoidance mechanism
                    when it needs to create the name for the newest ControllerRevision.
                  format: int32
                  type: integer
                conditions:
                  description: Represents the latest available observations of a DaemonSet's
                    current state.
                  items:
                    description: DaemonSetCondition describes the state of a DaemonSet
                      at a certain point.
                    properties:
                      lastTransitionTime:
                        description: Last time the condition transitioned from one
                          status to another.
                        format: date-time
                        type: string
                      message:
                        description: A human readable message indicating details about
                          the transition.
                        type: string
                      reason:
                        description: The reason for the condition's last transition.
                        type: string
                      status:
                        description: Status of the condition, one of True, False,
                          Unknown.
                        type: string
                      type:
                        description: Type of DaemonSet condition.
                        type: string
                    required:
                    - status
                    - type
                    type: object
                  type: array
                currentNumberScheduled:
                  description: 'The number of nodes that are running at least 1 daemon
                    pod and are supposed to run the daemon pod. More info: https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/'
                  format: int32
                  type: integer
                desiredNumberScheduled:
                  description: 'The total number of nodes that should be running the
                    daemon pod (including nodes correctly running the daemon pod).
                    More info: https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/'
                  format: int32
                  type: integer
                numberAvailable:
                  description: The number of nodes that should be running the daemon
                    pod and have one or more of the daemon pod running and available
                    (ready for at least spec.minReadySeconds)
                  format: int32
                  type: integer
                numberMisscheduled:
                  description: 'The number of nodes that are running the daemon pod,
                    but are not supposed to run the daemon pod. More info: https://kubernetes.io/docs/concepts/workloads/controllers/daemonset/'
                  format: int32
                  type: integer
                numberReady:
                  description: The number of nodes that should be running the daemon
                    pod and have one or more of the daemon pod running and ready.
                  format: int32
                  type: integer
                numberUnavailable:
                  description: The number of nodes that should be running the daemon
                    pod and have none of the daemon pod running and available (ready
                    for at least spec.minReadySeconds)
                  format: int32
                  type: integer
                observedGeneration:
                  description: The most recent generation observed by the daemon set
                    controller.
                  format: int64
                  type: integer
                updatedNumberScheduled:
                  description: The total number of nodes that are running updated
                    daemon pod
                  format: int32
                  type: integer
              required:
              - currentNumberScheduled
              - desiredNumberScheduled
              - numberMisscheduled
              - numberReady
              type: object
            serviceCIDR:
              type: string
          required:
          - clusterID
          - natEnabled
          type: object
      type: object
  version: v1alpha1
  versions:
  - name: v1alpha1
    served: true
    storage: true
`)

func manifestsCrdsSubmarinerIo_submariners_crdYamlBytes() ([]byte, error) {
	return _manifestsCrdsSubmarinerIo_submariners_crdYaml, nil
}

func manifestsCrdsSubmarinerIo_submariners_crdYaml() (*asset, error) {
	bytes, err := manifestsCrdsSubmarinerIo_submariners_crdYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/crds/submariner.io_submariners_crd.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
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
	"manifests/crds/lighthouse.submariner.io_multiclusterservices_crd.yaml": manifestsCrdsLighthouseSubmarinerIo_multiclusterservices_crdYaml,
	"manifests/crds/lighthouse.submariner.io_serviceexports_crd.yaml":       manifestsCrdsLighthouseSubmarinerIo_serviceexports_crdYaml,
	"manifests/crds/lighthouse.submariner.io_serviceimports_crd.yaml":       manifestsCrdsLighthouseSubmarinerIo_serviceimports_crdYaml,
	"manifests/crds/submariner.io_clusters_crd.yaml":                        manifestsCrdsSubmarinerIo_clusters_crdYaml,
	"manifests/crds/submariner.io_endpoints_crd.yaml":                       manifestsCrdsSubmarinerIo_endpoints_crdYaml,
	"manifests/crds/submariner.io_gateways_crd.yaml":                        manifestsCrdsSubmarinerIo_gateways_crdYaml,
	"manifests/crds/submariner.io_servicediscoveries_crd.yaml":              manifestsCrdsSubmarinerIo_servicediscoveries_crdYaml,
	"manifests/crds/submariner.io_submariners_crd.yaml":                     manifestsCrdsSubmarinerIo_submariners_crdYaml,
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
		"crds": {nil, map[string]*bintree{
			"lighthouse.submariner.io_multiclusterservices_crd.yaml": {manifestsCrdsLighthouseSubmarinerIo_multiclusterservices_crdYaml, map[string]*bintree{}},
			"lighthouse.submariner.io_serviceexports_crd.yaml":       {manifestsCrdsLighthouseSubmarinerIo_serviceexports_crdYaml, map[string]*bintree{}},
			"lighthouse.submariner.io_serviceimports_crd.yaml":       {manifestsCrdsLighthouseSubmarinerIo_serviceimports_crdYaml, map[string]*bintree{}},
			"submariner.io_clusters_crd.yaml":                        {manifestsCrdsSubmarinerIo_clusters_crdYaml, map[string]*bintree{}},
			"submariner.io_endpoints_crd.yaml":                       {manifestsCrdsSubmarinerIo_endpoints_crdYaml, map[string]*bintree{}},
			"submariner.io_gateways_crd.yaml":                        {manifestsCrdsSubmarinerIo_gateways_crdYaml, map[string]*bintree{}},
			"submariner.io_servicediscoveries_crd.yaml":              {manifestsCrdsSubmarinerIo_servicediscoveries_crdYaml, map[string]*bintree{}},
			"submariner.io_submariners_crd.yaml":                     {manifestsCrdsSubmarinerIo_submariners_crdYaml, map[string]*bintree{}},
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
