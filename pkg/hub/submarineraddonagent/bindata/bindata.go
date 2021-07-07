// Code generated for package bindata by go-bindata DO NOT EDIT. (@generated)
// sources:
// pkg/hub/submarineraddonagent/manifests/clusterrole.yaml
// pkg/hub/submarineraddonagent/manifests/clusterrolebinding.yaml
// pkg/hub/submarineraddonagent/manifests/deployment.yaml
// pkg/hub/submarineraddonagent/manifests/hub_role.yaml
// pkg/hub/submarineraddonagent/manifests/hub_rolebinding.yaml
// pkg/hub/submarineraddonagent/manifests/namespace.yaml
// pkg/hub/submarineraddonagent/manifests/role.yaml
// pkg/hub/submarineraddonagent/manifests/rolebinding.yaml
// pkg/hub/submarineraddonagent/manifests/serviceaccount.yaml
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

var _pkgHubSubmarineraddonagentManifestsClusterroleYaml = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: open-cluster-management:submariner-addon:agent
rules:
# Allow submariner-addon agent to run with openshift library-go
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "watch"]
# Allow submariner-addon agent to get cloud credations
- apiGroups: [""]
  resources: ["secrets"]
  verbs: ["get"]
# Allow submariner-addon agent to get/update nodes
- apiGroups: [""]
  resources: ["nodes"]
  verbs: ["get", "list", "watch", "update", "patch"]
`)

func pkgHubSubmarineraddonagentManifestsClusterroleYamlBytes() ([]byte, error) {
	return _pkgHubSubmarineraddonagentManifestsClusterroleYaml, nil
}

func pkgHubSubmarineraddonagentManifestsClusterroleYaml() (*asset, error) {
	bytes, err := pkgHubSubmarineraddonagentManifestsClusterroleYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "pkg/hub/submarineraddonagent/manifests/clusterrole.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _pkgHubSubmarineraddonagentManifestsClusterrolebindingYaml = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: open-cluster-management:submariner-addon:agent
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: open-cluster-management:submariner-addon:agent
subjects:
  - kind: ServiceAccount
    name: submariner-addon-sa
    namespace: {{ .AddonInstallNamespace }}
`)

func pkgHubSubmarineraddonagentManifestsClusterrolebindingYamlBytes() ([]byte, error) {
	return _pkgHubSubmarineraddonagentManifestsClusterrolebindingYaml, nil
}

func pkgHubSubmarineraddonagentManifestsClusterrolebindingYaml() (*asset, error) {
	bytes, err := pkgHubSubmarineraddonagentManifestsClusterrolebindingYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "pkg/hub/submarineraddonagent/manifests/clusterrolebinding.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _pkgHubSubmarineraddonagentManifestsDeploymentYaml = []byte(`kind: Deployment
apiVersion: apps/v1
metadata:
  name: submariner-addon
  namespace: {{ .AddonInstallNamespace }}
  labels:
    app: submariner-addon
spec:
  replicas: 1
  selector:
    matchLabels:
      app: submariner-addon
  template:
    metadata:
      labels:
        app: submariner-addon
    spec:
      serviceAccountName: submariner-addon-sa
      containers:
      - name: submariner-addon
        image: {{ .Image }}
        args:
          - "/submariner"
          - "agent"
          - "--hub-kubeconfig=/var/run/hub/kubeconfig"
          - "--cluster-name={{ .ClusterName }}"
        volumeMounts:
          - name: hub-config
            mountPath: /var/run/hub
      volumes:
      - name: hub-config
        secret:
          secretName: {{ .KubeConfigSecret }}
`)

func pkgHubSubmarineraddonagentManifestsDeploymentYamlBytes() ([]byte, error) {
	return _pkgHubSubmarineraddonagentManifestsDeploymentYaml, nil
}

func pkgHubSubmarineraddonagentManifestsDeploymentYaml() (*asset, error) {
	bytes, err := pkgHubSubmarineraddonagentManifestsDeploymentYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "pkg/hub/submarineraddonagent/manifests/deployment.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _pkgHubSubmarineraddonagentManifestsHub_roleYaml = []byte(`kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: open-cluster-management:submariner-addon:agent
  namespace: {{ .ClusterName }}
rules:
# Allow submariner-addon agent to operator managedclusteraddons on the hub cluster
- apiGroups: ["addon.open-cluster-management.io"]
  resources: ["managedclusteraddons"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: ["addon.open-cluster-management.io"]
  resources: ["managedclusteraddons/finalizers"]
  verbs: ["update"]
- apiGroups: ["addon.open-cluster-management.io"]
  resources: ["managedclusteraddons/status"]
  verbs: ["patch", "update"]
# Allow submariner-addon agent to operator submarinerconfigs on the hub cluster
- apiGroups: ["submarineraddon.open-cluster-management.io"]
  resources: ["submarinerconfigs"]
  verbs: ["get", "list", "watch", "update", "patch"]
- apiGroups: ["submarineraddon.open-cluster-management.io"]
  resources: ["submarinerconfigs/finalizers"]
  verbs: ["update"]
- apiGroups: ["submarineraddon.open-cluster-management.io"]
  resources: ["submarinerconfigs/status"]
  verbs: ["patch", "update"]
`)

func pkgHubSubmarineraddonagentManifestsHub_roleYamlBytes() ([]byte, error) {
	return _pkgHubSubmarineraddonagentManifestsHub_roleYaml, nil
}

func pkgHubSubmarineraddonagentManifestsHub_roleYaml() (*asset, error) {
	bytes, err := pkgHubSubmarineraddonagentManifestsHub_roleYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "pkg/hub/submarineraddonagent/manifests/hub_role.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _pkgHubSubmarineraddonagentManifestsHub_rolebindingYaml = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: open-cluster-management:submariner-addon:agent
  namespace: {{ .ClusterName }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: open-cluster-management:submariner-addon:agent
subjects:
  - kind: Group
    apiGroup: rbac.authorization.k8s.io
    name: {{ .Group }}
`)

func pkgHubSubmarineraddonagentManifestsHub_rolebindingYamlBytes() ([]byte, error) {
	return _pkgHubSubmarineraddonagentManifestsHub_rolebindingYaml, nil
}

func pkgHubSubmarineraddonagentManifestsHub_rolebindingYaml() (*asset, error) {
	bytes, err := pkgHubSubmarineraddonagentManifestsHub_rolebindingYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "pkg/hub/submarineraddonagent/manifests/hub_rolebinding.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _pkgHubSubmarineraddonagentManifestsNamespaceYaml = []byte(`apiVersion: v1
kind: Namespace
metadata:
  name: {{ .AddonInstallNamespace }}
`)

func pkgHubSubmarineraddonagentManifestsNamespaceYamlBytes() ([]byte, error) {
	return _pkgHubSubmarineraddonagentManifestsNamespaceYaml, nil
}

func pkgHubSubmarineraddonagentManifestsNamespaceYaml() (*asset, error) {
	bytes, err := pkgHubSubmarineraddonagentManifestsNamespaceYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "pkg/hub/submarineraddonagent/manifests/namespace.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _pkgHubSubmarineraddonagentManifestsRoleYaml = []byte(`kind: Role
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: submariner-addon
  namespace: {{ .AddonInstallNamespace }}
rules:
# Allow submariner-addon agent to run with openshift library-go
- apiGroups: [""]
  resources: ["configmaps"]
  verbs: ["get", "list", "watch", "create", "delete", "update", "patch"]
- apiGroups: [""]
  resources: ["pods"]
  verbs: ["get", "list"]
- apiGroups: ["apps"]
  resources: ["replicasets"]
  verbs: ["get"]
- apiGroups: ["", "events.k8s.io"]
  resources: ["events"]
  verbs: ["create", "patch", "update"]
# Allow submariner-addon agent to run with addon-framwork
- apiGroups: ["coordination.k8s.io"]
  resources: ["leases"]
  verbs: ["get", "list", "watch", "create", "update", "delete"]
# Allow submariner-addon agent to monitor submariner deployment status
- apiGroups: ["operators.coreos.com"]
  resources: ["subscriptions"]
  verbs: ["get", "list", "watch"]
- apiGroups: ["apps"]
  resources: ["daemonsets", "deployments"]
  verbs: ["get", "list", "watch"]
# Allow submariner-addon agent to get submariners
- apiGroups: ["submariner.io"]
  resources: ["submariners"]
  verbs: ["get", "list", "watch"]
`)

func pkgHubSubmarineraddonagentManifestsRoleYamlBytes() ([]byte, error) {
	return _pkgHubSubmarineraddonagentManifestsRoleYaml, nil
}

func pkgHubSubmarineraddonagentManifestsRoleYaml() (*asset, error) {
	bytes, err := pkgHubSubmarineraddonagentManifestsRoleYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "pkg/hub/submarineraddonagent/manifests/role.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _pkgHubSubmarineraddonagentManifestsRolebindingYaml = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: RoleBinding
metadata:
  name: submariner-addon
  namespace: {{ .AddonInstallNamespace }}
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: Role
  name: submariner-addon
subjects:
  - kind: ServiceAccount
    name: submariner-addon-sa
`)

func pkgHubSubmarineraddonagentManifestsRolebindingYamlBytes() ([]byte, error) {
	return _pkgHubSubmarineraddonagentManifestsRolebindingYaml, nil
}

func pkgHubSubmarineraddonagentManifestsRolebindingYaml() (*asset, error) {
	bytes, err := pkgHubSubmarineraddonagentManifestsRolebindingYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "pkg/hub/submarineraddonagent/manifests/rolebinding.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
	a := &asset{bytes: bytes, info: info}
	return a, nil
}

var _pkgHubSubmarineraddonagentManifestsServiceaccountYaml = []byte(`kind: ServiceAccount
apiVersion: v1
metadata:
  name: submariner-addon-sa
  namespace: {{ .AddonInstallNamespace }}
`)

func pkgHubSubmarineraddonagentManifestsServiceaccountYamlBytes() ([]byte, error) {
	return _pkgHubSubmarineraddonagentManifestsServiceaccountYaml, nil
}

func pkgHubSubmarineraddonagentManifestsServiceaccountYaml() (*asset, error) {
	bytes, err := pkgHubSubmarineraddonagentManifestsServiceaccountYamlBytes()
	if err != nil {
		return nil, err
	}

	info := bindataFileInfo{name: "pkg/hub/submarineraddonagent/manifests/serviceaccount.yaml", size: 0, mode: os.FileMode(0), modTime: time.Unix(0, 0)}
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
	"pkg/hub/submarineraddonagent/manifests/clusterrole.yaml":        pkgHubSubmarineraddonagentManifestsClusterroleYaml,
	"pkg/hub/submarineraddonagent/manifests/clusterrolebinding.yaml": pkgHubSubmarineraddonagentManifestsClusterrolebindingYaml,
	"pkg/hub/submarineraddonagent/manifests/deployment.yaml":         pkgHubSubmarineraddonagentManifestsDeploymentYaml,
	"pkg/hub/submarineraddonagent/manifests/hub_role.yaml":           pkgHubSubmarineraddonagentManifestsHub_roleYaml,
	"pkg/hub/submarineraddonagent/manifests/hub_rolebinding.yaml":    pkgHubSubmarineraddonagentManifestsHub_rolebindingYaml,
	"pkg/hub/submarineraddonagent/manifests/namespace.yaml":          pkgHubSubmarineraddonagentManifestsNamespaceYaml,
	"pkg/hub/submarineraddonagent/manifests/role.yaml":               pkgHubSubmarineraddonagentManifestsRoleYaml,
	"pkg/hub/submarineraddonagent/manifests/rolebinding.yaml":        pkgHubSubmarineraddonagentManifestsRolebindingYaml,
	"pkg/hub/submarineraddonagent/manifests/serviceaccount.yaml":     pkgHubSubmarineraddonagentManifestsServiceaccountYaml,
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
	"pkg": {nil, map[string]*bintree{
		"hub": {nil, map[string]*bintree{
			"submarineraddonagent": {nil, map[string]*bintree{
				"manifests": {nil, map[string]*bintree{
					"clusterrole.yaml":        {pkgHubSubmarineraddonagentManifestsClusterroleYaml, map[string]*bintree{}},
					"clusterrolebinding.yaml": {pkgHubSubmarineraddonagentManifestsClusterrolebindingYaml, map[string]*bintree{}},
					"deployment.yaml":         {pkgHubSubmarineraddonagentManifestsDeploymentYaml, map[string]*bintree{}},
					"hub_role.yaml":           {pkgHubSubmarineraddonagentManifestsHub_roleYaml, map[string]*bintree{}},
					"hub_rolebinding.yaml":    {pkgHubSubmarineraddonagentManifestsHub_rolebindingYaml, map[string]*bintree{}},
					"namespace.yaml":          {pkgHubSubmarineraddonagentManifestsNamespaceYaml, map[string]*bintree{}},
					"role.yaml":               {pkgHubSubmarineraddonagentManifestsRoleYaml, map[string]*bintree{}},
					"rolebinding.yaml":        {pkgHubSubmarineraddonagentManifestsRolebindingYaml, map[string]*bintree{}},
					"serviceaccount.yaml":     {pkgHubSubmarineraddonagentManifestsServiceaccountYaml, map[string]*bintree{}},
				}},
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
