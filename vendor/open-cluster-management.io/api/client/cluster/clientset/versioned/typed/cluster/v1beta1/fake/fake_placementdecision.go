// Code generated by client-gen. DO NOT EDIT.

package fake

import (
	"context"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	labels "k8s.io/apimachinery/pkg/labels"
	schema "k8s.io/apimachinery/pkg/runtime/schema"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	testing "k8s.io/client-go/testing"
	v1beta1 "open-cluster-management.io/api/cluster/v1beta1"
)

// FakePlacementDecisions implements PlacementDecisionInterface
type FakePlacementDecisions struct {
	Fake *FakeClusterV1beta1
	ns   string
}

var placementdecisionsResource = schema.GroupVersionResource{Group: "cluster.open-cluster-management.io", Version: "v1beta1", Resource: "placementdecisions"}

var placementdecisionsKind = schema.GroupVersionKind{Group: "cluster.open-cluster-management.io", Version: "v1beta1", Kind: "PlacementDecision"}

// Get takes name of the placementDecision, and returns the corresponding placementDecision object, and an error if there is any.
func (c *FakePlacementDecisions) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1beta1.PlacementDecision, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewGetAction(placementdecisionsResource, c.ns, name), &v1beta1.PlacementDecision{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.PlacementDecision), err
}

// List takes label and field selectors, and returns the list of PlacementDecisions that match those selectors.
func (c *FakePlacementDecisions) List(ctx context.Context, opts v1.ListOptions) (result *v1beta1.PlacementDecisionList, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewListAction(placementdecisionsResource, placementdecisionsKind, c.ns, opts), &v1beta1.PlacementDecisionList{})

	if obj == nil {
		return nil, err
	}

	label, _, _ := testing.ExtractFromListOptions(opts)
	if label == nil {
		label = labels.Everything()
	}
	list := &v1beta1.PlacementDecisionList{ListMeta: obj.(*v1beta1.PlacementDecisionList).ListMeta}
	for _, item := range obj.(*v1beta1.PlacementDecisionList).Items {
		if label.Matches(labels.Set(item.Labels)) {
			list.Items = append(list.Items, item)
		}
	}
	return list, err
}

// Watch returns a watch.Interface that watches the requested placementDecisions.
func (c *FakePlacementDecisions) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	return c.Fake.
		InvokesWatch(testing.NewWatchAction(placementdecisionsResource, c.ns, opts))

}

// Create takes the representation of a placementDecision and creates it.  Returns the server's representation of the placementDecision, and an error, if there is any.
func (c *FakePlacementDecisions) Create(ctx context.Context, placementDecision *v1beta1.PlacementDecision, opts v1.CreateOptions) (result *v1beta1.PlacementDecision, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewCreateAction(placementdecisionsResource, c.ns, placementDecision), &v1beta1.PlacementDecision{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.PlacementDecision), err
}

// Update takes the representation of a placementDecision and updates it. Returns the server's representation of the placementDecision, and an error, if there is any.
func (c *FakePlacementDecisions) Update(ctx context.Context, placementDecision *v1beta1.PlacementDecision, opts v1.UpdateOptions) (result *v1beta1.PlacementDecision, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateAction(placementdecisionsResource, c.ns, placementDecision), &v1beta1.PlacementDecision{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.PlacementDecision), err
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *FakePlacementDecisions) UpdateStatus(ctx context.Context, placementDecision *v1beta1.PlacementDecision, opts v1.UpdateOptions) (*v1beta1.PlacementDecision, error) {
	obj, err := c.Fake.
		Invokes(testing.NewUpdateSubresourceAction(placementdecisionsResource, "status", c.ns, placementDecision), &v1beta1.PlacementDecision{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.PlacementDecision), err
}

// Delete takes name of the placementDecision and deletes it. Returns an error if one occurs.
func (c *FakePlacementDecisions) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	_, err := c.Fake.
		Invokes(testing.NewDeleteAction(placementdecisionsResource, c.ns, name), &v1beta1.PlacementDecision{})

	return err
}

// DeleteCollection deletes a collection of objects.
func (c *FakePlacementDecisions) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	action := testing.NewDeleteCollectionAction(placementdecisionsResource, c.ns, listOpts)

	_, err := c.Fake.Invokes(action, &v1beta1.PlacementDecisionList{})
	return err
}

// Patch applies the patch and returns the patched placementDecision.
func (c *FakePlacementDecisions) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1beta1.PlacementDecision, err error) {
	obj, err := c.Fake.
		Invokes(testing.NewPatchSubresourceAction(placementdecisionsResource, c.ns, name, pt, data, subresources...), &v1beta1.PlacementDecision{})

	if obj == nil {
		return nil, err
	}
	return obj.(*v1beta1.PlacementDecision), err
}
