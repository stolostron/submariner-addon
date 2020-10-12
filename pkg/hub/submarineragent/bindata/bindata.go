// Code generated for package bindata by go-bindata DO NOT EDIT. (@generated)
// sources:
// manifests/agent/operator/submariner-operator-deployment.yaml
// manifests/agent/operator/submariner.io-servicediscoveries-cr.yaml
// manifests/agent/operator/submariner.io-submariners-cr.yaml
// manifests/agent/rbac/submariner-admin-aggeragate-clusterrole.yaml
// manifests/agent/rbac/submariner-cluster-rolebinding.yaml
// manifests/agent/rbac/submariner-cluster-serviceaccount.yaml
// manifests/agent/rbac/submariner-lighthouse-clusterrole.yaml
// manifests/agent/rbac/submariner-lighthouse-clusterrolebinding.yaml
// manifests/agent/rbac/submariner-lighthouse-serviceaccount.yaml
// manifests/agent/rbac/submariner-operator-clusterrole.yaml
// manifests/agent/rbac/submariner-operator-clusterrolebinding.yaml
// manifests/agent/rbac/submariner-operator-namespace.yaml
// manifests/agent/rbac/submariner-operator-role.yaml
// manifests/agent/rbac/submariner-operator-rolebinding.yaml
// manifests/agent/rbac/submariner-operator-serviceaccount.yaml
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

var _manifestsAgentOperatorSubmarinerOperatorDeploymentYaml = []byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: submariner-operator
  namespace: submariner-operator
spec:
  selector:
    matchLabels:
      name: submariner-operator
  template:
    metadata:
      labels:
        name: submariner-operator
    spec:
      containers:
      - command:
        - submariner-operator
        env:
        - name: WATCH_NAMESPACE
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.namespace
        - name: POD_NAME
          valueFrom:
            fieldRef:
              apiVersion: v1
              fieldPath: metadata.name
        - name: OPERATOR_NAME
          value: submariner-operator
        image: quay.io/submariner/submariner-operator:{{ .Version }}
        imagePullPolicy: Always
        name: submariner-operator
      serviceAccount: submariner-operator
      serviceAccountName: submariner-operator
`)

func manifestsAgentOperatorSubmarinerOperatorDeploymentYamlBytes() ([]byte, error) {
	return _manifestsAgentOperatorSubmarinerOperatorDeploymentYaml, nil
}

func manifestsAgentOperatorSubmarinerOperatorDeploymentYaml() (*asset, error) {
	bytes, err := manifestsAgentOperatorSubmarinerOperatorDeploymentYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/operator/submariner-operator-deployment.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsAgentOperatorSubmarinerIoServicediscoveriesCrYaml = []byte(`apiVersion: submariner.io/v1alpha1
kind: ServiceDiscovery
metadata:
  name: service-discovery
  namespace: submariner-operator
spec:
  brokerK8sApiServer: {{ .BrokerAPIServer }}
  brokerK8sApiServerToken: {{ .BrokerToken }}
  brokerK8sCA: {{ .BrokerCA }}
  brokerK8sRemoteNamespace: {{ .BrokerNamespace }}
  clusterID: {{ .ClusterName }}
  debug: false
  namespace: submariner-operator
  repository: quay.io/submariner
  version: {{ .Version }}
`)

func manifestsAgentOperatorSubmarinerIoServicediscoveriesCrYamlBytes() ([]byte, error) {
	return _manifestsAgentOperatorSubmarinerIoServicediscoveriesCrYaml, nil
}

func manifestsAgentOperatorSubmarinerIoServicediscoveriesCrYaml() (*asset, error) {
	bytes, err := manifestsAgentOperatorSubmarinerIoServicediscoveriesCrYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/operator/submariner.io-servicediscoveries-cr.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsAgentOperatorSubmarinerIoSubmarinersCrYaml = []byte(`apiVersion: submariner.io/v1alpha1
kind: Submariner
metadata:
  name: submariner
  namespace: submariner-operator
spec:
  broker: k8s
  brokerK8sApiServer: {{ .BrokerAPIServer }}
  brokerK8sApiServerToken: {{ .BrokerToken }}
  brokerK8sCA: {{ .BrokerCA }}
  brokerK8sRemoteNamespace: {{ .BrokerNamespace }}
  ceIPSecDebug: false
  ceIPSecIKEPort: 500
  ceIPSecNATTPort: 4500
  ceIPSecPSK: {{ .IPSecPSK }}
  clusterCIDR: ""
  clusterID: {{ .ClusterName }}
  colorCodes: blue
  debug: false
  namespace: submariner-operator
  natEnabled: {{ .NATEnabled }}
  repository: quay.io/submariner
  serviceCIDR: ""
  serviceDiscoveryEnabled: true
  version: {{ .Version }}
`)

func manifestsAgentOperatorSubmarinerIoSubmarinersCrYamlBytes() ([]byte, error) {
	return _manifestsAgentOperatorSubmarinerIoSubmarinersCrYaml, nil
}

func manifestsAgentOperatorSubmarinerIoSubmarinersCrYaml() (*asset, error) {
	bytes, err := manifestsAgentOperatorSubmarinerIoSubmarinersCrYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/operator/submariner.io-submariners-cr.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsAgentRbacSubmarinerAdminAggeragateClusterroleYaml = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: open-cluster-management:submariner-admin-aggregate-clusterrole
  labels:
    rbac.authorization.k8s.io/aggregate-to-admin: "true"
rules:
  - apiGroups: ["submariner.io"]
    resources: ["submariners","servicediscoveries"]
    verbs: ["get", "list", "watch", "create", "update", "patch", "delete"]
`)

func manifestsAgentRbacSubmarinerAdminAggeragateClusterroleYamlBytes() ([]byte, error) {
	return _manifestsAgentRbacSubmarinerAdminAggeragateClusterroleYaml, nil
}

func manifestsAgentRbacSubmarinerAdminAggeragateClusterroleYaml() (*asset, error) {
	bytes, err := manifestsAgentRbacSubmarinerAdminAggeragateClusterroleYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/rbac/submariner-admin-aggeragate-clusterrole.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsAgentRbacSubmarinerClusterRolebindingYaml = []byte(`kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: submariner-k8s-broker-cluster-{{ .ManagedClusterName }}
  namespace: {{ .SubmarinerBrokerNamespace }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: submariner-k8s-broker-cluster
subjects:
  - kind: ServiceAccount
    name: {{ .ManagedClusterName }}
    namespace: {{ .SubmarinerBrokerNamespace }}
`)

func manifestsAgentRbacSubmarinerClusterRolebindingYamlBytes() ([]byte, error) {
	return _manifestsAgentRbacSubmarinerClusterRolebindingYaml, nil
}

func manifestsAgentRbacSubmarinerClusterRolebindingYaml() (*asset, error) {
	bytes, err := manifestsAgentRbacSubmarinerClusterRolebindingYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/rbac/submariner-cluster-rolebinding.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsAgentRbacSubmarinerClusterServiceaccountYaml = []byte(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .ManagedClusterName }}
  namespace: {{ .SubmarinerBrokerNamespace }}
  labels:
    cluster.open-cluster-management.io/submariner-cluster-sa: {{ .ManagedClusterName }}
`)

func manifestsAgentRbacSubmarinerClusterServiceaccountYamlBytes() ([]byte, error) {
	return _manifestsAgentRbacSubmarinerClusterServiceaccountYaml, nil
}

func manifestsAgentRbacSubmarinerClusterServiceaccountYaml() (*asset, error) {
	bytes, err := manifestsAgentRbacSubmarinerClusterServiceaccountYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/rbac/submariner-cluster-serviceaccount.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsAgentRbacSubmarinerLighthouseClusterroleYaml = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: submariner-lighthouse
rules:
  - apiGroups:
      - ""
    resources:
      - services
      - namespaces
      - endpoints
    verbs:
      - get
      - list
      - watch
      - update
  - apiGroups:
      - discovery.k8s.io
    resources:
      - endpointslices
    verbs:
      - create
      - get
      - list
      - watch
      - update
      - delete
      - deletecollection
  - apiGroups:
      - lighthouse.submariner.io
    resources:
      - "*"
    verbs:
      - create
      - get
      - list
      - watch
      - update
      - delete
  - apiGroups:
      - submariner.io
    resources:
      - "gateways"
    verbs:
      - get
      - list
      - watch
`)

func manifestsAgentRbacSubmarinerLighthouseClusterroleYamlBytes() ([]byte, error) {
	return _manifestsAgentRbacSubmarinerLighthouseClusterroleYaml, nil
}

func manifestsAgentRbacSubmarinerLighthouseClusterroleYaml() (*asset, error) {
	bytes, err := manifestsAgentRbacSubmarinerLighthouseClusterroleYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/rbac/submariner-lighthouse-clusterrole.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsAgentRbacSubmarinerLighthouseClusterrolebindingYaml = []byte(`kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: submariner-lighthouse
subjects:
  - kind: ServiceAccount
    name: submariner-lighthouse
    namespace: submariner-operator
roleRef:
  kind: ClusterRole
  name: submariner-lighthouse
  apiGroup: rbac.authorization.k8s.io
`)

func manifestsAgentRbacSubmarinerLighthouseClusterrolebindingYamlBytes() ([]byte, error) {
	return _manifestsAgentRbacSubmarinerLighthouseClusterrolebindingYaml, nil
}

func manifestsAgentRbacSubmarinerLighthouseClusterrolebindingYaml() (*asset, error) {
	bytes, err := manifestsAgentRbacSubmarinerLighthouseClusterrolebindingYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/rbac/submariner-lighthouse-clusterrolebinding.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsAgentRbacSubmarinerLighthouseServiceaccountYaml = []byte(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: submariner-lighthouse
  namespace: submariner-operator
`)

func manifestsAgentRbacSubmarinerLighthouseServiceaccountYamlBytes() ([]byte, error) {
	return _manifestsAgentRbacSubmarinerLighthouseServiceaccountYaml, nil
}

func manifestsAgentRbacSubmarinerLighthouseServiceaccountYaml() (*asset, error) {
	bytes, err := manifestsAgentRbacSubmarinerLighthouseServiceaccountYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/rbac/submariner-lighthouse-serviceaccount.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsAgentRbacSubmarinerOperatorClusterroleYaml = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: submariner-operator
rules:
  - apiGroups:
      - ""
    resources:
      - configmaps
    verbs:
      - get
      - list
      - watch
      - update
  - apiGroups:
      - ""
    resources:
      - pods
      - services
    verbs:
      - get
      - list
      - watch
  - apiGroups:
      - operator.openshift.io
    resources:
      - dnses
    verbs:
      - get
      - list
      - watch
      - update
  - apiGroups:
      - config.openshift.io
    resources:
      - networks
    verbs:
      - get
      - list
  - apiGroups:
      - network.openshift.io
    resources:
      - clusternetworks
    verbs:
      - get
      - list
  - apiGroups:
      - ""
    resources:
      - nodes
    verbs:
      - get
      - list
      - watch
      - update
`)

func manifestsAgentRbacSubmarinerOperatorClusterroleYamlBytes() ([]byte, error) {
	return _manifestsAgentRbacSubmarinerOperatorClusterroleYaml, nil
}

func manifestsAgentRbacSubmarinerOperatorClusterroleYaml() (*asset, error) {
	bytes, err := manifestsAgentRbacSubmarinerOperatorClusterroleYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/rbac/submariner-operator-clusterrole.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsAgentRbacSubmarinerOperatorClusterrolebindingYaml = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: submariner-operator
subjects:
  - kind: ServiceAccount
    name: submariner-operator
    namespace: submariner-operator
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: submariner-operator
`)

func manifestsAgentRbacSubmarinerOperatorClusterrolebindingYamlBytes() ([]byte, error) {
	return _manifestsAgentRbacSubmarinerOperatorClusterrolebindingYaml, nil
}

func manifestsAgentRbacSubmarinerOperatorClusterrolebindingYaml() (*asset, error) {
	bytes, err := manifestsAgentRbacSubmarinerOperatorClusterrolebindingYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/rbac/submariner-operator-clusterrolebinding.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsAgentRbacSubmarinerOperatorNamespaceYaml = []byte(`apiVersion: v1
kind: Namespace
metadata:
  name: submariner-operator
`)

func manifestsAgentRbacSubmarinerOperatorNamespaceYamlBytes() ([]byte, error) {
	return _manifestsAgentRbacSubmarinerOperatorNamespaceYaml, nil
}

func manifestsAgentRbacSubmarinerOperatorNamespaceYaml() (*asset, error) {
	bytes, err := manifestsAgentRbacSubmarinerOperatorNamespaceYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/rbac/submariner-operator-namespace.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsAgentRbacSubmarinerOperatorRoleYaml = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: Role
metadata:
  name: submariner-operator
  namespace: submariner-operator
rules:
  - apiGroups:
      - ""
    resources:
      - pods
      - services
      - services/finalizers
      - endpoints
      - persistentvolumeclaims
      - events
      - configmaps
      - secrets
    verbs:
      - '*'
  - apiGroups:
      - apps
    resources:
      - deployments
      - daemonsets
      - replicasets
      - statefulsets
    verbs:
      - '*'
  - apiGroups:
      - monitoring.coreos.com
    resources:
      - servicemonitors
    verbs:
      - get
      - create
  - apiGroups:
      - apps
    resourceNames:
      - submariner-operator
    resources:
      - deployments/finalizers
    verbs:
      - update
  - apiGroups:
      - ""
    resources:
      - pods
    verbs:
      - get
  - apiGroups:
      - apps
    resources:
      - replicasets
    verbs:
      - get
  - apiGroups:
      - submariner.io
    resources:
      - '*'
      - servicediscoveries
    verbs:
      - '*'
`)

func manifestsAgentRbacSubmarinerOperatorRoleYamlBytes() ([]byte, error) {
	return _manifestsAgentRbacSubmarinerOperatorRoleYaml, nil
}

func manifestsAgentRbacSubmarinerOperatorRoleYaml() (*asset, error) {
	bytes, err := manifestsAgentRbacSubmarinerOperatorRoleYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/rbac/submariner-operator-role.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsAgentRbacSubmarinerOperatorRolebindingYaml = []byte(`kind: RoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: submariner-operator
  namespace: submariner-operator
subjects:
  - kind: ServiceAccount
    name: submariner-operator
roleRef:
  kind: Role
  name: submariner-operator
  apiGroup: rbac.authorization.k8s.io
`)

func manifestsAgentRbacSubmarinerOperatorRolebindingYamlBytes() ([]byte, error) {
	return _manifestsAgentRbacSubmarinerOperatorRolebindingYaml, nil
}

func manifestsAgentRbacSubmarinerOperatorRolebindingYaml() (*asset, error) {
	bytes, err := manifestsAgentRbacSubmarinerOperatorRolebindingYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/rbac/submariner-operator-rolebinding.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsAgentRbacSubmarinerOperatorServiceaccountYaml = []byte(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: submariner-operator
  namespace: submariner-operator
`)

func manifestsAgentRbacSubmarinerOperatorServiceaccountYamlBytes() ([]byte, error) {
	return _manifestsAgentRbacSubmarinerOperatorServiceaccountYaml, nil
}

func manifestsAgentRbacSubmarinerOperatorServiceaccountYaml() (*asset, error) {
	bytes, err := manifestsAgentRbacSubmarinerOperatorServiceaccountYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/rbac/submariner-operator-serviceaccount.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
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
	"manifests/agent/operator/submariner-operator-deployment.yaml":       manifestsAgentOperatorSubmarinerOperatorDeploymentYaml,
	"manifests/agent/operator/submariner.io-servicediscoveries-cr.yaml":  manifestsAgentOperatorSubmarinerIoServicediscoveriesCrYaml,
	"manifests/agent/operator/submariner.io-submariners-cr.yaml":         manifestsAgentOperatorSubmarinerIoSubmarinersCrYaml,
	"manifests/agent/rbac/submariner-admin-aggeragate-clusterrole.yaml":  manifestsAgentRbacSubmarinerAdminAggeragateClusterroleYaml,
	"manifests/agent/rbac/submariner-cluster-rolebinding.yaml":           manifestsAgentRbacSubmarinerClusterRolebindingYaml,
	"manifests/agent/rbac/submariner-cluster-serviceaccount.yaml":        manifestsAgentRbacSubmarinerClusterServiceaccountYaml,
	"manifests/agent/rbac/submariner-lighthouse-clusterrole.yaml":        manifestsAgentRbacSubmarinerLighthouseClusterroleYaml,
	"manifests/agent/rbac/submariner-lighthouse-clusterrolebinding.yaml": manifestsAgentRbacSubmarinerLighthouseClusterrolebindingYaml,
	"manifests/agent/rbac/submariner-lighthouse-serviceaccount.yaml":     manifestsAgentRbacSubmarinerLighthouseServiceaccountYaml,
	"manifests/agent/rbac/submariner-operator-clusterrole.yaml":          manifestsAgentRbacSubmarinerOperatorClusterroleYaml,
	"manifests/agent/rbac/submariner-operator-clusterrolebinding.yaml":   manifestsAgentRbacSubmarinerOperatorClusterrolebindingYaml,
	"manifests/agent/rbac/submariner-operator-namespace.yaml":            manifestsAgentRbacSubmarinerOperatorNamespaceYaml,
	"manifests/agent/rbac/submariner-operator-role.yaml":                 manifestsAgentRbacSubmarinerOperatorRoleYaml,
	"manifests/agent/rbac/submariner-operator-rolebinding.yaml":          manifestsAgentRbacSubmarinerOperatorRolebindingYaml,
	"manifests/agent/rbac/submariner-operator-serviceaccount.yaml":       manifestsAgentRbacSubmarinerOperatorServiceaccountYaml,
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
		"agent": {nil, map[string]*bintree{
			"operator": {nil, map[string]*bintree{
				"submariner-operator-deployment.yaml":      {manifestsAgentOperatorSubmarinerOperatorDeploymentYaml, map[string]*bintree{}},
				"submariner.io-servicediscoveries-cr.yaml": {manifestsAgentOperatorSubmarinerIoServicediscoveriesCrYaml, map[string]*bintree{}},
				"submariner.io-submariners-cr.yaml":        {manifestsAgentOperatorSubmarinerIoSubmarinersCrYaml, map[string]*bintree{}},
			}},
			"rbac": {nil, map[string]*bintree{
				"submariner-admin-aggeragate-clusterrole.yaml":  {manifestsAgentRbacSubmarinerAdminAggeragateClusterroleYaml, map[string]*bintree{}},
				"submariner-cluster-rolebinding.yaml":           {manifestsAgentRbacSubmarinerClusterRolebindingYaml, map[string]*bintree{}},
				"submariner-cluster-serviceaccount.yaml":        {manifestsAgentRbacSubmarinerClusterServiceaccountYaml, map[string]*bintree{}},
				"submariner-lighthouse-clusterrole.yaml":        {manifestsAgentRbacSubmarinerLighthouseClusterroleYaml, map[string]*bintree{}},
				"submariner-lighthouse-clusterrolebinding.yaml": {manifestsAgentRbacSubmarinerLighthouseClusterrolebindingYaml, map[string]*bintree{}},
				"submariner-lighthouse-serviceaccount.yaml":     {manifestsAgentRbacSubmarinerLighthouseServiceaccountYaml, map[string]*bintree{}},
				"submariner-operator-clusterrole.yaml":          {manifestsAgentRbacSubmarinerOperatorClusterroleYaml, map[string]*bintree{}},
				"submariner-operator-clusterrolebinding.yaml":   {manifestsAgentRbacSubmarinerOperatorClusterrolebindingYaml, map[string]*bintree{}},
				"submariner-operator-namespace.yaml":            {manifestsAgentRbacSubmarinerOperatorNamespaceYaml, map[string]*bintree{}},
				"submariner-operator-role.yaml":                 {manifestsAgentRbacSubmarinerOperatorRoleYaml, map[string]*bintree{}},
				"submariner-operator-rolebinding.yaml":          {manifestsAgentRbacSubmarinerOperatorRolebindingYaml, map[string]*bintree{}},
				"submariner-operator-serviceaccount.yaml":       {manifestsAgentRbacSubmarinerOperatorServiceaccountYaml, map[string]*bintree{}},
			}},
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
