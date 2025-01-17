// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	v1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	submarinerconfigv1alpha1 "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/clientset/versioned/typed/submarinerconfig/v1alpha1"
	gentype "k8s.io/client-go/gentype"
)

// fakeSubmarinerConfigs implements SubmarinerConfigInterface
type fakeSubmarinerConfigs struct {
	*gentype.FakeClientWithList[*v1alpha1.SubmarinerConfig, *v1alpha1.SubmarinerConfigList]
	Fake *FakeSubmarineraddonV1alpha1
}

func newFakeSubmarinerConfigs(fake *FakeSubmarineraddonV1alpha1, namespace string) submarinerconfigv1alpha1.SubmarinerConfigInterface {
	return &fakeSubmarinerConfigs{
		gentype.NewFakeClientWithList[*v1alpha1.SubmarinerConfig, *v1alpha1.SubmarinerConfigList](
			fake.Fake,
			namespace,
			v1alpha1.SchemeGroupVersion.WithResource("submarinerconfigs"),
			v1alpha1.SchemeGroupVersion.WithKind("SubmarinerConfig"),
			func() *v1alpha1.SubmarinerConfig { return &v1alpha1.SubmarinerConfig{} },
			func() *v1alpha1.SubmarinerConfigList { return &v1alpha1.SubmarinerConfigList{} },
			func(dst, src *v1alpha1.SubmarinerConfigList) { dst.ListMeta = src.ListMeta },
			func(list *v1alpha1.SubmarinerConfigList) []*v1alpha1.SubmarinerConfig {
				return gentype.ToPointerSlice(list.Items)
			},
			func(list *v1alpha1.SubmarinerConfigList, items []*v1alpha1.SubmarinerConfig) {
				list.Items = gentype.FromPointerSlice(items)
			},
		),
		fake,
	}
}
