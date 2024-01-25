//nolint:dupl // Lines are similar but not duplicated and can't be refactored to avloid this error.
package resource

import (
	"context"

	configV1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	cfgv1a1clnt "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/clientset/versioned/typed/submarinerconfig/v1alpha1"
	"github.com/submariner-io/admiral/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	addonV1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonV1alpha1Client "open-cluster-management.io/api/client/addon/clientset/versioned/typed/addon/v1alpha1"
	clusterV1Client "open-cluster-management.io/api/client/cluster/clientset/versioned/typed/cluster/v1"
	clusterV1beta2Client "open-cluster-management.io/api/client/cluster/clientset/versioned/typed/cluster/v1beta2"
	workclient "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	clusterV1 "open-cluster-management.io/api/cluster/v1"
	clusterV1beta2 "open-cluster-management.io/api/cluster/v1beta2"
	workv1 "open-cluster-management.io/api/work/v1"
)

func ForManagedClusterSet(client clusterV1beta2Client.ManagedClusterSetInterface) resource.Interface[*clusterV1beta2.ManagedClusterSet] {
	return &resource.InterfaceFuncs[*clusterV1beta2.ManagedClusterSet]{
		GetFunc: func(ctx context.Context, name string, options metav1.GetOptions) (*clusterV1beta2.ManagedClusterSet, error) {
			return client.Get(ctx, name, options)
		},
		CreateFunc: func(ctx context.Context, obj *clusterV1beta2.ManagedClusterSet, options metav1.CreateOptions,
		) (*clusterV1beta2.ManagedClusterSet, error) {
			return client.Create(ctx, obj, options)
		},
		UpdateFunc: func(ctx context.Context, obj *clusterV1beta2.ManagedClusterSet, options metav1.UpdateOptions,
		) (*clusterV1beta2.ManagedClusterSet, error) {
			return client.Update(ctx, obj, options)
		},
		DeleteFunc: func(ctx context.Context, name string, options metav1.DeleteOptions) error {
			return client.Delete(ctx, name, options)
		},
	}
}

func ForManagedCluster(client clusterV1Client.ManagedClusterInterface) resource.Interface[*clusterV1.ManagedCluster] {
	return &resource.InterfaceFuncs[*clusterV1.ManagedCluster]{
		GetFunc: func(ctx context.Context, name string, options metav1.GetOptions) (*clusterV1.ManagedCluster, error) {
			return client.Get(ctx, name, options)
		},
		CreateFunc: func(ctx context.Context, obj *clusterV1.ManagedCluster, options metav1.CreateOptions) (*clusterV1.ManagedCluster, error) {
			return client.Create(ctx, obj, options)
		},
		UpdateFunc: func(ctx context.Context, obj *clusterV1.ManagedCluster, options metav1.UpdateOptions) (*clusterV1.ManagedCluster, error) {
			return client.Update(ctx, obj, options)
		},
		DeleteFunc: func(ctx context.Context, name string, options metav1.DeleteOptions) error {
			return client.Delete(ctx, name, options)
		},
	}
}

func ForAddon(client addonV1alpha1Client.ManagedClusterAddOnInterface) resource.Interface[*addonV1alpha1.ManagedClusterAddOn] {
	return &resource.InterfaceFuncs[*addonV1alpha1.ManagedClusterAddOn]{
		GetFunc: func(ctx context.Context, name string, options metav1.GetOptions) (*addonV1alpha1.ManagedClusterAddOn, error) {
			return client.Get(ctx, name, options)
		},
		CreateFunc: func(ctx context.Context, obj *addonV1alpha1.ManagedClusterAddOn, options metav1.CreateOptions,
		) (*addonV1alpha1.ManagedClusterAddOn, error) {
			return client.Create(ctx, obj, options)
		},
		UpdateFunc: func(ctx context.Context, obj *addonV1alpha1.ManagedClusterAddOn, options metav1.UpdateOptions,
		) (*addonV1alpha1.ManagedClusterAddOn, error) {
			return client.Update(ctx, obj, options)
		},
		DeleteFunc: func(ctx context.Context, name string, options metav1.DeleteOptions) error {
			return client.Delete(ctx, name, options)
		},
	}
}

func ForClusterAddon(client addonV1alpha1Client.ClusterManagementAddOnInterface) resource.Interface[*addonV1alpha1.ClusterManagementAddOn] {
	return &resource.InterfaceFuncs[*addonV1alpha1.ClusterManagementAddOn]{
		GetFunc: func(ctx context.Context, name string, options metav1.GetOptions) (*addonV1alpha1.ClusterManagementAddOn, error) {
			return client.Get(ctx, name, options)
		},
		CreateFunc: func(ctx context.Context, obj *addonV1alpha1.ClusterManagementAddOn, options metav1.CreateOptions,
		) (*addonV1alpha1.ClusterManagementAddOn, error) {
			return client.Create(ctx, obj, options)
		},
		UpdateFunc: func(ctx context.Context, obj *addonV1alpha1.ClusterManagementAddOn, options metav1.UpdateOptions,
		) (*addonV1alpha1.ClusterManagementAddOn, error) {
			return client.Update(ctx, obj, options)
		},
		DeleteFunc: func(ctx context.Context, name string, options metav1.DeleteOptions) error {
			return client.Delete(ctx, name, options)
		},
	}
}

func ForSubmarinerConfig(client cfgv1a1clnt.SubmarinerConfigInterface) resource.Interface[*configV1alpha1.SubmarinerConfig] {
	return &resource.InterfaceFuncs[*configV1alpha1.SubmarinerConfig]{
		GetFunc: func(ctx context.Context, name string, options metav1.GetOptions) (*configV1alpha1.SubmarinerConfig, error) {
			return client.Get(ctx, name, options)
		},
		CreateFunc: func(ctx context.Context, obj *configV1alpha1.SubmarinerConfig, options metav1.CreateOptions,
		) (*configV1alpha1.SubmarinerConfig, error) {
			return client.Create(ctx, obj, options)
		},
		UpdateFunc: func(ctx context.Context, obj *configV1alpha1.SubmarinerConfig, options metav1.UpdateOptions,
		) (*configV1alpha1.SubmarinerConfig, error) {
			return client.Update(ctx, obj, options)
		},
		DeleteFunc: func(ctx context.Context, name string, options metav1.DeleteOptions) error {
			return client.Delete(ctx, name, options)
		},
	}
}

func ForManifestWork(client workclient.ManifestWorkInterface) resource.Interface[*workv1.ManifestWork] {
	return &resource.InterfaceFuncs[*workv1.ManifestWork]{
		GetFunc: func(ctx context.Context, name string, options metav1.GetOptions) (*workv1.ManifestWork, error) {
			return client.Get(ctx, name, options)
		},
		CreateFunc: func(ctx context.Context, obj *workv1.ManifestWork, options metav1.CreateOptions) (*workv1.ManifestWork, error) {
			return client.Create(ctx, obj, options)
		},
		UpdateFunc: func(ctx context.Context, obj *workv1.ManifestWork, options metav1.UpdateOptions) (*workv1.ManifestWork, error) {
			return client.Update(ctx, obj, options)
		},
		DeleteFunc: func(ctx context.Context, name string, options metav1.DeleteOptions) error {
			return client.Delete(ctx, name, options)
		},
	}
}
