package submarineragent

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/ghodss/yaml"

	clientset "github.com/open-cluster-management/api/client/cluster/clientset/versioned"
	workv1client "github.com/open-cluster-management/api/client/work/clientset/versioned"
	workv1 "github.com/open-cluster-management/api/work/v1"
	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	configclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"
	hubbindata "github.com/open-cluster-management/submariner-addon/pkg/hub/bindata"
	"github.com/open-cluster-management/submariner-addon/pkg/hub/submarineragent/bindata"

	"github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/library-go/pkg/operator/events"
	operatorhelpers "github.com/openshift/library-go/pkg/operator/v1helpers"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

type wrapperInfo struct {
	name      string
	files     []string
	mustAsset func(name string) []byte
}

var (
	baseWrappers = []wrapperInfo{
		{
			name:      "submariner-agent-crds",
			mustAsset: hubbindata.MustAsset,
			files: []string{
				"manifests/crds/lighthouse.submariner.io_multiclusterservices_crd.yaml",
				"manifests/crds/lighthouse.submariner.io_serviceexports_crd.yaml",
				"manifests/crds/lighthouse.submariner.io_serviceimports_crd.yaml",
				"manifests/crds/submariner.io_clusters_crd.yaml",
				"manifests/crds/submariner.io_endpoints_crd.yaml",
				"manifests/crds/submariner.io_gateways_crd.yaml",
				"manifests/crds/submariner.io_servicediscoveries_crd.yaml",
				"manifests/crds/submariner.io_submariners_crd.yaml",
			},
		},
		{
			name:      "submariner-agent-rbac",
			mustAsset: bindata.MustAsset,
			files: []string{
				"manifests/agent/rbac/submariner-admin-aggeragate-clusterrole.yaml",
				"manifests/agent/rbac/submariner-lighthouse-clusterrole.yaml",
				"manifests/agent/rbac/submariner-lighthouse-clusterrolebinding.yaml",
				"manifests/agent/rbac/submariner-lighthouse-serviceaccount.yaml",
				"manifests/agent/rbac/submariner-operator-clusterrole.yaml",
				"manifests/agent/rbac/submariner-operator-clusterrolebinding.yaml",
				"manifests/agent/rbac/submariner-operator-namespace.yaml",
				"manifests/agent/rbac/submariner-operator-role.yaml",
				"manifests/agent/rbac/submariner-operator-rolebinding.yaml",
				"manifests/agent/rbac/submariner-operator-serviceaccount.yaml",
			},
		},
		{
			name:      "submariner-agent-operator",
			mustAsset: bindata.MustAsset,
			files: []string{
				"manifests/agent/operator/submariner-operator-deployment.yaml",
				"manifests/agent/operator/submariner.io-servicediscoveries-cr.yaml",
				"manifests/agent/operator/submariner.io-submariners-cr.yaml",
			},
		},
	}

	sccWrapper = []wrapperInfo{
		{
			name:      "submariner-scc",
			mustAsset: bindata.MustAsset,
			files: []string{
				"manifests/agent/rbac/submariner-scc.yaml",
			},
		},
		{
			name:      "submariner-scc-rbac",
			mustAsset: bindata.MustAsset,
			files: []string{
				"manifests/agent/rbac/submariner-scc-admin-aggeragate-clusterrole.yaml",
			},
		},
	}
)

type SubmarinerConfig struct {
	NATEnabled      bool
	BrokerAPIServer string
	BrokerNamespace string
	BrokerToken     string
	BrokerCA        string
	CableDriver     string
	IPSecIKEPort    int
	IPSecNATTPort   int
	IPSecPSK        string
	ClusterName     string
	ClusterCIDR     string
	ServiceCIDR     string
	Repository      string
	Version         string
}

func newSubmarinerConfig(
	client kubernetes.Interface,
	dynamicClient dynamic.Interface,
	clusterName, brokeNamespace string) (*SubmarinerConfig, error) {
	config := &SubmarinerConfig{
		Repository:      helpers.GetSubmarinerRepository(),
		Version:         helpers.GetSubmarinerVersion(),
		NATEnabled:      false,
		BrokerNamespace: brokeNamespace,
		ClusterName:     clusterName,
		CableDriver:     "strongswan", //TODO change to libreswan
		IPSecIKEPort:    500,
		IPSecNATTPort:   4500,
	}

	apiServer, err := helpers.GetBrokerAPIServer(dynamicClient)
	if err != nil {
		return config, err
	}
	config.BrokerAPIServer = apiServer

	IPSecPSK, err := helpers.GetIPSecPSK(client, brokeNamespace)
	if err != nil {
		return config, err
	}
	config.IPSecPSK = IPSecPSK

	token, ca, err := helpers.GetBrokerTokenAndCA(client, brokeNamespace, clusterName)
	if err != nil {
		return config, err
	}
	config.BrokerCA = ca
	config.BrokerToken = token

	return config, nil
}

func wrapManifestWorks(config *SubmarinerConfig, wrappers []wrapperInfo) ([]*workv1.ManifestWork, error) {
	var manifestWorks []*workv1.ManifestWork
	for _, w := range wrappers {
		work := &workv1.ManifestWork{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      w.name,
				Namespace: config.ClusterName,
			},
			Spec: workv1.ManifestWorkSpec{},
		}

		var manifests []workv1.Manifest
		for _, file := range w.files {
			yamlData := assets.MustCreateAssetFromTemplate(file, w.mustAsset(filepath.Join("", file)), config).Data
			jsonData, err := yaml.YAMLToJSON(yamlData)
			if err != nil {
				return manifestWorks, err
			}
			manifest := workv1.Manifest{RawExtension: runtime.RawExtension{Raw: jsonData}}
			manifests = append(manifests, manifest)
		}
		work.Spec.Workload.Manifests = manifests

		manifestWorks = append(manifestWorks, work)
	}

	return manifestWorks, nil
}

func ApplySubmarinerManifestWorks(
	ctx context.Context,
	client kubernetes.Interface,
	dynamicClient dynamic.Interface,
	workClient workv1client.Interface,
	clusterClient clientset.Interface,
	configClient configclient.Interface,
	recorder events.Recorder,
	clusterName, brokeNamespace string,
	submarinerConfig *configv1alpha1.SubmarinerConfig) error {
	config, err := newSubmarinerConfig(client, dynamicClient, clusterName, brokeNamespace)
	if err != nil {
		return fmt.Errorf("failed to create submariner config of cluster %v : %v", clusterName, err)
	}

	// If there has SubmarinerConfig in the cluster namespace, we use the config to configure the submariner borker info
	if submarinerConfig != nil {
		if submarinerConfig.Spec.CableDriver != "" {
			config.CableDriver = submarinerConfig.Spec.CableDriver
		}
		if submarinerConfig.Spec.IPSecIKEPort != 0 {
			config.IPSecIKEPort = submarinerConfig.Spec.IPSecIKEPort
		}
		if submarinerConfig.Spec.IPSecNATTPort != 0 {
			config.IPSecNATTPort = submarinerConfig.Spec.IPSecNATTPort
		}
		condition := metav1.Condition{
			Type:    configv1alpha1.SubmarinerConfigConditionApplied,
			Status:  metav1.ConditionTrue,
			Reason:  "SubmarinerConfigApplied",
			Message: "SubmarinerConfig was applied",
		}
		_, updated, err := helpers.UpdateSubmarinerConfigStatus(
			configClient,
			submarinerConfig.Namespace, submarinerConfig.Name,
			helpers.UpdateSubmarinerConfigConditionFn(condition),
		)
		if err != nil {
			return err
		}
		if updated {
			recorder.Eventf("SubmarinerConfigApplied", "SubmarinerConfig %s was applied for manged cluster %s", submarinerConfig.Name, submarinerConfig.Namespace)
		}
	}

	managedCluster, err := clusterClient.ClusterV1().ManagedClusters().Get(context.TODO(), config.ClusterName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get managedcluster %v: %v", clusterName, err)
	}

	var wrappers []wrapperInfo = baseWrappers
	switch helpers.GetClusterType(managedCluster) {
	case helpers.ClusterTypeOCP:
		config.NATEnabled = true
		wrappers = append(wrappers, sccWrapper...)

	}

	manifestWorks, err := wrapManifestWorks(config, wrappers)
	if err != nil {
		return fmt.Errorf("failed to wrap mainfestWorks:%+v", err)
	}

	var errs []error
	for _, work := range manifestWorks {
		err := ApplyManifestWork(work, workClient, ctx)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to apply manifestWork %+v: %+v", work.Name, err))
			continue
		}
	}

	return operatorhelpers.NewMultiLineAggregate(errs)
}

func RemoveSubmarinerManifestWorks(
	ctx context.Context,
	clusterClient clientset.Interface,
	workClient workv1client.Interface,
	recorder events.Recorder,
	namespace string) error {
	var errs []error
	var wrappers []wrapperInfo = baseWrappers

	managedCluster, err := clusterClient.ClusterV1().ManagedClusters().Get(context.TODO(), namespace, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get managedcluster %v: %v", namespace, err)
	}
	switch helpers.GetClusterType(managedCluster) {
	case helpers.ClusterTypeOCP:
		wrappers = append(wrappers, sccWrapper...)
	}

	for _, w := range wrappers {
		err := workClient.WorkV1().ManifestWorks(namespace).Delete(ctx, w.name, metav1.DeleteOptions{})
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		recorder.Eventf("SubmarinerManifestWorksDeleted", "Deleted manifestwork %q", fmt.Sprintf("%s/%s", namespace, w.name))
	}
	return operatorhelpers.NewMultiLineAggregate(errs)
}

func ApplyManifestWork(required *workv1.ManifestWork, client workv1client.Interface, ctx context.Context) error {
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		existing, err := client.WorkV1().ManifestWorks(required.Namespace).
			Get(ctx, required.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				_, err = client.WorkV1().ManifestWorks(required.Namespace).
					Create(ctx, required, metav1.CreateOptions{})
			}
			return err
		}

		if !equality.Semantic.DeepEqual(existing.Spec, required.Spec) {
			_, err = client.WorkV1().ManifestWorks(required.Namespace).
				Update(ctx, required, metav1.UpdateOptions{})
		}

		return err
	})

	return err
}
