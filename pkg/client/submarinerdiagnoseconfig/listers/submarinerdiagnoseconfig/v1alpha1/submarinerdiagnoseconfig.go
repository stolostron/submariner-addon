// Code generated by lister-gen. DO NOT EDIT.

package v1alpha1

import (
	submarinerdiagnoseconfigv1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerdiagnoseconfig/v1alpha1"
	labels "k8s.io/apimachinery/pkg/labels"
	listers "k8s.io/client-go/listers"
	cache "k8s.io/client-go/tools/cache"
)

// SubmarinerDiagnoseConfigLister helps list SubmarinerDiagnoseConfigs.
// All objects returned here must be treated as read-only.
type SubmarinerDiagnoseConfigLister interface {
	// List lists all SubmarinerDiagnoseConfigs in the indexer.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*submarinerdiagnoseconfigv1alpha1.SubmarinerDiagnoseConfig, err error)
	// SubmarinerDiagnoseConfigs returns an object that can list and get SubmarinerDiagnoseConfigs.
	SubmarinerDiagnoseConfigs(namespace string) SubmarinerDiagnoseConfigNamespaceLister
	SubmarinerDiagnoseConfigListerExpansion
}

// submarinerDiagnoseConfigLister implements the SubmarinerDiagnoseConfigLister interface.
type submarinerDiagnoseConfigLister struct {
	listers.ResourceIndexer[*submarinerdiagnoseconfigv1alpha1.SubmarinerDiagnoseConfig]
}

// NewSubmarinerDiagnoseConfigLister returns a new SubmarinerDiagnoseConfigLister.
func NewSubmarinerDiagnoseConfigLister(indexer cache.Indexer) SubmarinerDiagnoseConfigLister {
	return &submarinerDiagnoseConfigLister{listers.New[*submarinerdiagnoseconfigv1alpha1.SubmarinerDiagnoseConfig](indexer, submarinerdiagnoseconfigv1alpha1.Resource("submarinerdiagnoseconfig"))}
}

// SubmarinerDiagnoseConfigs returns an object that can list and get SubmarinerDiagnoseConfigs.
func (s *submarinerDiagnoseConfigLister) SubmarinerDiagnoseConfigs(namespace string) SubmarinerDiagnoseConfigNamespaceLister {
	return submarinerDiagnoseConfigNamespaceLister{listers.NewNamespaced[*submarinerdiagnoseconfigv1alpha1.SubmarinerDiagnoseConfig](s.ResourceIndexer, namespace)}
}

// SubmarinerDiagnoseConfigNamespaceLister helps list and get SubmarinerDiagnoseConfigs.
// All objects returned here must be treated as read-only.
type SubmarinerDiagnoseConfigNamespaceLister interface {
	// List lists all SubmarinerDiagnoseConfigs in the indexer for a given namespace.
	// Objects returned here must be treated as read-only.
	List(selector labels.Selector) (ret []*submarinerdiagnoseconfigv1alpha1.SubmarinerDiagnoseConfig, err error)
	// Get retrieves the SubmarinerDiagnoseConfig from the indexer for a given namespace and name.
	// Objects returned here must be treated as read-only.
	Get(name string) (*submarinerdiagnoseconfigv1alpha1.SubmarinerDiagnoseConfig, error)
	SubmarinerDiagnoseConfigNamespaceListerExpansion
}

// submarinerDiagnoseConfigNamespaceLister implements the SubmarinerDiagnoseConfigNamespaceLister
// interface.
type submarinerDiagnoseConfigNamespaceLister struct {
	listers.ResourceIndexer[*submarinerdiagnoseconfigv1alpha1.SubmarinerDiagnoseConfig]
}
