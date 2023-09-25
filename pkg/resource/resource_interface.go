//nolint:dupl // Lines are similar but not duplicated and can't be refactored to avloid this error.
package resource

import (
	"context"

	configV1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	cfgv1a1clnt "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/clientset/versioned/typed/submarinerconfig/v1alpha1"
	"github.com/submariner-io/admiral/pkg/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	addonV1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonV1alpha1Client "open-cluster-management.io/api/client/addon/clientset/versioned/typed/addon/v1alpha1"
	clusterV1Client "open-cluster-management.io/api/client/cluster/clientset/versioned/typed/cluster/v1"
	clusterV1beta2Client "open-cluster-management.io/api/client/cluster/clientset/versioned/typed/cluster/v1beta2"
	workclient "open-cluster-management.io/api/client/work/clientset/versioned/typed/work/v1"
	clusterV1 "open-cluster-management.io/api/cluster/v1"
	clusterV1beta2 "open-cluster-management.io/api/cluster/v1beta2"
	workv1 "open-cluster-management.io/api/work/v1"
)

func ForManagedClusterSet(client clusterV1beta2Client.ManagedClusterSetInterface) resource.Interface {
	return &resource.InterfaceFuncs{
		GetFunc: func(ctx context.Context, name string, options metav1.GetOptions) (runtime.Object, error) {
			return client.Get(ctx, name, options)
		},
		CreateFunc: func(ctx context.Context, obj runtime.Object, options metav1.CreateOptions) (runtime.Object, error) {
			return client.Create(ctx, obj.(*clusterV1beta2.ManagedClusterSet), options)
		},
		UpdateFunc: func(ctx context.Context, obj runtime.Object, options metav1.UpdateOptions) (runtime.Object, error) {
			return client.Update(ctx, obj.(*clusterV1beta2.ManagedClusterSet), options)
		},
		DeleteFunc: func(ctx context.Context, name string, options metav1.DeleteOptions) error {
			return client.Delete(ctx, name, options)
		},
	}
}

func ForManagedCluster(client clusterV1Client.ManagedClusterInterface) resource.Interface {
	return &resource.InterfaceFuncs{
		GetFunc: func(ctx context.Context, name string, options metav1.GetOptions) (runtime.Object, error) {
			return client.Get(ctx, name, options)
		},
		CreateFunc: func(ctx context.Context, obj runtime.Object, options metav1.CreateOptions) (runtime.Object, error) {
			return client.Create(ctx, obj.(*clusterV1.ManagedCluster), options)
		},
		UpdateFunc: func(ctx context.Context, obj runtime.Object, options metav1.UpdateOptions) (runtime.Object, error) {
			return client.Update(ctx, obj.(*clusterV1.ManagedCluster), options)
		},
		DeleteFunc: func(ctx context.Context, name string, options metav1.DeleteOptions) error {
			return client.Delete(ctx, name, options)
		},
	}
}

func ForAddon(client addonV1alpha1Client.ManagedClusterAddOnInterface) resource.Interface {
	return &resource.InterfaceFuncs{
		GetFunc: func(ctx context.Context, name string, options metav1.GetOptions) (runtime.Object, error) {
			return client.Get(ctx, name, options)
		},
		CreateFunc: func(ctx context.Context, obj runtime.Object, options metav1.CreateOptions) (runtime.Object, error) {
			return client.Create(ctx, obj.(*addonV1alpha1.ManagedClusterAddOn), options)
		},
		UpdateFunc: func(ctx context.Context, obj runtime.Object, options metav1.UpdateOptions) (runtime.Object, error) {
			return client.Update(ctx, obj.(*addonV1alpha1.ManagedClusterAddOn), options)
		},
		DeleteFunc: func(ctx context.Context, name string, options metav1.DeleteOptions) error {
			return client.Delete(ctx, name, options)
		},
	}
}

func ForClusterAddon(client addonV1alpha1Client.ClusterManagementAddOnInterface) resource.Interface {
	return &resource.InterfaceFuncs{
		GetFunc: func(ctx context.Context, name string, options metav1.GetOptions) (runtime.Object, error) {
			return client.Get(ctx, name, options)
		},
		CreateFunc: func(ctx context.Context, obj runtime.Object, options metav1.CreateOptions) (runtime.Object, error) {
			return client.Create(ctx, obj.(*addonV1alpha1.ClusterManagementAddOn), options)
		},
		UpdateFunc: func(ctx context.Context, obj runtime.Object, options metav1.UpdateOptions) (runtime.Object, error) {
			return client.Update(ctx, obj.(*addonV1alpha1.ClusterManagementAddOn), options)
		},
		DeleteFunc: func(ctx context.Context, name string, options metav1.DeleteOptions) error {
			return client.Delete(ctx, name, options)
		},
	}
}

func ForSubmarinerConfig(client cfgv1a1clnt.SubmarinerConfigInterface) resource.Interface {
	return &resource.InterfaceFuncs{
		GetFunc: func(ctx context.Context, name string, options metav1.GetOptions) (runtime.Object, error) {
			return client.Get(ctx, name, options)
		},
		CreateFunc: func(ctx context.Context, obj runtime.Object, options metav1.CreateOptions) (runtime.Object, error) {
			return client.Create(ctx, obj.(*configV1alpha1.SubmarinerConfig), options)
		},
		UpdateFunc: func(ctx context.Context, obj runtime.Object, options metav1.UpdateOptions) (runtime.Object, error) {
			return client.Update(ctx, obj.(*configV1alpha1.SubmarinerConfig), options)
		},
		DeleteFunc: func(ctx context.Context, name string, options metav1.DeleteOptions) error {
			return client.Delete(ctx, name, options)
		},
	}
}

func ForManifestWork(client workclient.ManifestWorkInterface) resource.Interface {
	return &resource.InterfaceFuncs{
		GetFunc: func(ctx context.Context, name string, options metav1.GetOptions) (runtime.Object, error) {
			return client.Get(ctx, name, options)
		},
		CreateFunc: func(ctx context.Context, obj runtime.Object, options metav1.CreateOptions) (runtime.Object, error) {
			return client.Create(ctx, obj.(*workv1.ManifestWork), options)
		},
		UpdateFunc: func(ctx context.Context, obj runtime.Object, options metav1.UpdateOptions) (runtime.Object, error) {
			return client.Update(ctx, obj.(*workv1.ManifestWork), options)
		},
		DeleteFunc: func(ctx context.Context, name string, options metav1.DeleteOptions) error {
			return client.Delete(ctx, name, options)
		},
	}
}
