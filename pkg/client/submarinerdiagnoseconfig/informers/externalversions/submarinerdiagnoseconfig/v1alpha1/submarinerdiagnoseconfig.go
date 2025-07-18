// Code generated by informer-gen. DO NOT EDIT.

package v1alpha1

import (
	context "context"
	time "time"

	apissubmarinerdiagnoseconfigv1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerdiagnoseconfig/v1alpha1"
	versioned "github.com/stolostron/submariner-addon/pkg/client/submarinerdiagnoseconfig/clientset/versioned"
	internalinterfaces "github.com/stolostron/submariner-addon/pkg/client/submarinerdiagnoseconfig/informers/externalversions/internalinterfaces"
	submarinerdiagnoseconfigv1alpha1 "github.com/stolostron/submariner-addon/pkg/client/submarinerdiagnoseconfig/listers/submarinerdiagnoseconfig/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// SubmarinerDiagnoseConfigInformer provides access to a shared informer and lister for
// SubmarinerDiagnoseConfigs.
type SubmarinerDiagnoseConfigInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() submarinerdiagnoseconfigv1alpha1.SubmarinerDiagnoseConfigLister
}

type submarinerDiagnoseConfigInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewSubmarinerDiagnoseConfigInformer constructs a new informer for SubmarinerDiagnoseConfig type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewSubmarinerDiagnoseConfigInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredSubmarinerDiagnoseConfigInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredSubmarinerDiagnoseConfigInformer constructs a new informer for SubmarinerDiagnoseConfig type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredSubmarinerDiagnoseConfigInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.SubmarineraddonV1alpha1().SubmarinerDiagnoseConfigs(namespace).List(context.Background(), options)
			},
			WatchFunc: func(options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.SubmarineraddonV1alpha1().SubmarinerDiagnoseConfigs(namespace).Watch(context.Background(), options)
			},
			ListWithContextFunc: func(ctx context.Context, options v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.SubmarineraddonV1alpha1().SubmarinerDiagnoseConfigs(namespace).List(ctx, options)
			},
			WatchFuncWithContext: func(ctx context.Context, options v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.SubmarineraddonV1alpha1().SubmarinerDiagnoseConfigs(namespace).Watch(ctx, options)
			},
		},
		&apissubmarinerdiagnoseconfigv1alpha1.SubmarinerDiagnoseConfig{},
		resyncPeriod,
		indexers,
	)
}

func (f *submarinerDiagnoseConfigInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredSubmarinerDiagnoseConfigInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *submarinerDiagnoseConfigInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&apissubmarinerdiagnoseconfigv1alpha1.SubmarinerDiagnoseConfig{}, f.defaultInformer)
}

func (f *submarinerDiagnoseConfigInformer) Lister() submarinerdiagnoseconfigv1alpha1.SubmarinerDiagnoseConfigLister {
	return submarinerdiagnoseconfigv1alpha1.NewSubmarinerDiagnoseConfigLister(f.Informer().GetIndexer())
}
