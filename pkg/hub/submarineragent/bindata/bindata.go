// Code generated for package bindata by go-bindata DO NOT EDIT. (@generated)
// sources:
// manifests/agent/operator/submariner-operator-group.yaml
// manifests/agent/operator/submariner-operator-namespace.yaml
// manifests/agent/operator/submariner-operator-subscription.yaml
// manifests/agent/operator/submariner.io-submariners-cr.yaml
// manifests/agent/rbac/broker-cluster-rolebinding.yaml
// manifests/agent/rbac/broker-cluster-serviceaccount.yaml
// manifests/agent/rbac/operatorgroup-aggregate-clusterrole.yaml
// manifests/agent/rbac/scc-aggregate-clusterrole.yaml
// manifests/agent/rbac/submariner-agent-scc.yaml
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

var _manifestsAgentOperatorSubmarinerOperatorGroupYaml = []byte(`apiVersion: operators.coreos.com/v1
kind: OperatorGroup
metadata:
  name: submariner-operator
  namespace: submariner-operator
spec:
  targetNamespaces:
  - submariner-operator
`)

func manifestsAgentOperatorSubmarinerOperatorGroupYamlBytes() ([]byte, error) {
	return _manifestsAgentOperatorSubmarinerOperatorGroupYaml, nil
}

func manifestsAgentOperatorSubmarinerOperatorGroupYaml() (*asset, error) {
	bytes, err := manifestsAgentOperatorSubmarinerOperatorGroupYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/operator/submariner-operator-group.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsAgentOperatorSubmarinerOperatorNamespaceYaml = []byte(`apiVersion: v1
kind: Namespace
metadata:
  name: submariner-operator
`)

func manifestsAgentOperatorSubmarinerOperatorNamespaceYamlBytes() ([]byte, error) {
	return _manifestsAgentOperatorSubmarinerOperatorNamespaceYaml, nil
}

func manifestsAgentOperatorSubmarinerOperatorNamespaceYaml() (*asset, error) {
	bytes, err := manifestsAgentOperatorSubmarinerOperatorNamespaceYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/operator/submariner-operator-namespace.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsAgentOperatorSubmarinerOperatorSubscriptionYaml = []byte(`apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: submariner
  namespace: submariner-operator
spec:
  channel: {{ .CatalogChannel }}
  installPlanApproval: Automatic
  name: {{ .CatalogName }}
  source: {{ .CatalogSource }}
  sourceNamespace: {{ .CatalogSourceNamespace }}
  startingCSV: {{ .CatalogStartingCSV }}
`)

func manifestsAgentOperatorSubmarinerOperatorSubscriptionYamlBytes() ([]byte, error) {
	return _manifestsAgentOperatorSubmarinerOperatorSubscriptionYaml, nil
}

func manifestsAgentOperatorSubmarinerOperatorSubscriptionYaml() (*asset, error) {
	bytes, err := manifestsAgentOperatorSubmarinerOperatorSubscriptionYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/operator/submariner-operator-subscription.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
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
  cableDriver: {{ .CableDriver }}
  ceIPSecDebug: false
  ceIPSecIKEPort: {{ .IPSecIKEPort }}
  ceIPSecNATTPort: {{ .IPSecNATTPort }}
  ceIPSecPSK: {{ .IPSecPSK }}
  clusterCIDR: ""
  clusterID: {{ .ClusterName }}
  colorCodes: blue
  debug: false
  namespace: submariner-operator
  natEnabled: {{ .NATEnabled }}
  serviceCIDR: ""
  serviceDiscoveryEnabled: true
{{- if or .SubmarinerGatewayImage .SubmarinerRouteAgentImage .LighthouseAgentImage .LighthouseCoreDNSImage }}
  imageOverrides:
    {{- if .SubmarinerGatewayImage }}
    submariner: {{ .SubmarinerGatewayImage }}
    {{- end}}
    {{- if .SubmarinerRouteAgentImage }}
    submariner-route-agent: {{ .SubmarinerRouteAgentImage }}
    {{- end}}
    {{- if .LighthouseAgentImage }}
    lighthouse-agent: {{ .LighthouseAgentImage }}
    {{- end}}
    {{- if .LighthouseCoreDNSImage }}
    lighthouse-coredns: {{ .LighthouseCoreDNSImage }}
    {{- end}}
{{- end}}
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

var _manifestsAgentRbacBrokerClusterRolebindingYaml = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
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

func manifestsAgentRbacBrokerClusterRolebindingYamlBytes() ([]byte, error) {
	return _manifestsAgentRbacBrokerClusterRolebindingYaml, nil
}

func manifestsAgentRbacBrokerClusterRolebindingYaml() (*asset, error) {
	bytes, err := manifestsAgentRbacBrokerClusterRolebindingYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/rbac/broker-cluster-rolebinding.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsAgentRbacBrokerClusterServiceaccountYaml = []byte(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: {{ .ManagedClusterName }}
  namespace: {{ .SubmarinerBrokerNamespace }}
  labels:
    cluster.open-cluster-management.io/submariner-cluster-sa: {{ .ManagedClusterName }}
`)

func manifestsAgentRbacBrokerClusterServiceaccountYamlBytes() ([]byte, error) {
	return _manifestsAgentRbacBrokerClusterServiceaccountYaml, nil
}

func manifestsAgentRbacBrokerClusterServiceaccountYaml() (*asset, error) {
	bytes, err := manifestsAgentRbacBrokerClusterServiceaccountYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/rbac/broker-cluster-serviceaccount.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsAgentRbacOperatorgroupAggregateClusterroleYaml = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: open-cluster-management:submariner-addon-operatorgroups-aggregate-clusterrole
  labels:
    rbac.authorization.k8s.io/aggregate-to-admin: "true"
rules:
  - apiGroups: ["operators.coreos.com"]
    resources: ["operatorgroups"]
    verbs: ["get", "create", "delete", "update"]
`)

func manifestsAgentRbacOperatorgroupAggregateClusterroleYamlBytes() ([]byte, error) {
	return _manifestsAgentRbacOperatorgroupAggregateClusterroleYaml, nil
}

func manifestsAgentRbacOperatorgroupAggregateClusterroleYaml() (*asset, error) {
	bytes, err := manifestsAgentRbacOperatorgroupAggregateClusterroleYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/rbac/operatorgroup-aggregate-clusterrole.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsAgentRbacSccAggregateClusterroleYaml = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: open-cluster-management:submariner-scc-admin-aggregate-clusterrole
  labels:
    rbac.authorization.k8s.io/aggregate-to-admin: "true"
rules:
  - apiGroups: ["security.openshift.io"]
    resources: ["securitycontextconstraints"]
    verbs: ["get", "create", "delete"]
`)

func manifestsAgentRbacSccAggregateClusterroleYamlBytes() ([]byte, error) {
	return _manifestsAgentRbacSccAggregateClusterroleYaml, nil
}

func manifestsAgentRbacSccAggregateClusterroleYaml() (*asset, error) {
	bytes, err := manifestsAgentRbacSccAggregateClusterroleYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/rbac/scc-aggregate-clusterrole.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _manifestsAgentRbacSubmarinerAgentSccYaml = []byte(`apiVersion: security.openshift.io/v1
kind: SecurityContextConstraints
metadata:
  name: submariner-agent-scc
allowHostDirVolumePlugin: true
allowHostIPC: true
allowHostNetwork: true
allowHostPID: true
allowHostPorts: true
allowPrivilegeEscalation: true
allowPrivilegedContainer: true
allowedCapabilities:
- '*'
allowedUnsafeSysctls:
- '*'
defaultAddCapabilities: null
fsGroup:
  type: RunAsAny
groups:
- system:cluster-admins
- system:nodes
- system:masters
priority: 1
readOnlyRootFilesystem: false
requiredDropCapabilities: []
runAsUser:
  type: RunAsAny
seLinuxContext:
  type: RunAsAny
seccompProfiles:
- '*'
supplementalGroups:
  type: RunAsAny
users:
- system:serviceaccount:submariner-operator:submariner-operator
- system:serviceaccount:submariner-operator:submariner-lighthouse
volumes:
- '*'`)

func manifestsAgentRbacSubmarinerAgentSccYamlBytes() ([]byte, error) {
	return _manifestsAgentRbacSubmarinerAgentSccYaml, nil
}

func manifestsAgentRbacSubmarinerAgentSccYaml() (*asset, error) {
	bytes, err := manifestsAgentRbacSubmarinerAgentSccYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "manifests/agent/rbac/submariner-agent-scc.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
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
	"manifests/agent/operator/submariner-operator-group.yaml":        manifestsAgentOperatorSubmarinerOperatorGroupYaml,
	"manifests/agent/operator/submariner-operator-namespace.yaml":    manifestsAgentOperatorSubmarinerOperatorNamespaceYaml,
	"manifests/agent/operator/submariner-operator-subscription.yaml": manifestsAgentOperatorSubmarinerOperatorSubscriptionYaml,
	"manifests/agent/operator/submariner.io-submariners-cr.yaml":     manifestsAgentOperatorSubmarinerIoSubmarinersCrYaml,
	"manifests/agent/rbac/broker-cluster-rolebinding.yaml":           manifestsAgentRbacBrokerClusterRolebindingYaml,
	"manifests/agent/rbac/broker-cluster-serviceaccount.yaml":        manifestsAgentRbacBrokerClusterServiceaccountYaml,
	"manifests/agent/rbac/operatorgroup-aggregate-clusterrole.yaml":  manifestsAgentRbacOperatorgroupAggregateClusterroleYaml,
	"manifests/agent/rbac/scc-aggregate-clusterrole.yaml":            manifestsAgentRbacSccAggregateClusterroleYaml,
	"manifests/agent/rbac/submariner-agent-scc.yaml":                 manifestsAgentRbacSubmarinerAgentSccYaml,
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
				"submariner-operator-group.yaml":        {manifestsAgentOperatorSubmarinerOperatorGroupYaml, map[string]*bintree{}},
				"submariner-operator-namespace.yaml":    {manifestsAgentOperatorSubmarinerOperatorNamespaceYaml, map[string]*bintree{}},
				"submariner-operator-subscription.yaml": {manifestsAgentOperatorSubmarinerOperatorSubscriptionYaml, map[string]*bintree{}},
				"submariner.io-submariners-cr.yaml":     {manifestsAgentOperatorSubmarinerIoSubmarinersCrYaml, map[string]*bintree{}},
			}},
			"rbac": {nil, map[string]*bintree{
				"broker-cluster-rolebinding.yaml":          {manifestsAgentRbacBrokerClusterRolebindingYaml, map[string]*bintree{}},
				"broker-cluster-serviceaccount.yaml":       {manifestsAgentRbacBrokerClusterServiceaccountYaml, map[string]*bintree{}},
				"operatorgroup-aggregate-clusterrole.yaml": {manifestsAgentRbacOperatorgroupAggregateClusterroleYaml, map[string]*bintree{}},
				"scc-aggregate-clusterrole.yaml":           {manifestsAgentRbacSccAggregateClusterroleYaml, map[string]*bintree{}},
				"submariner-agent-scc.yaml":                {manifestsAgentRbacSubmarinerAgentSccYaml, map[string]*bintree{}},
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
