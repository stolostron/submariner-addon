// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	"time"

	v1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	scheme "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// SubmarinerConfigsGetter has a method to return a SubmarinerConfigInterface.
// A group's client should implement this interface.
type SubmarinerConfigsGetter interface {
	SubmarinerConfigs(namespace string) SubmarinerConfigInterface
}

// SubmarinerConfigInterface has methods to work with SubmarinerConfig resources.
type SubmarinerConfigInterface interface {
	Create(ctx context.Context, submarinerConfig *v1alpha1.SubmarinerConfig, opts v1.CreateOptions) (*v1alpha1.SubmarinerConfig, error)
	Update(ctx context.Context, submarinerConfig *v1alpha1.SubmarinerConfig, opts v1.UpdateOptions) (*v1alpha1.SubmarinerConfig, error)
	UpdateStatus(ctx context.Context, submarinerConfig *v1alpha1.SubmarinerConfig, opts v1.UpdateOptions) (*v1alpha1.SubmarinerConfig, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.SubmarinerConfig, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.SubmarinerConfigList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.SubmarinerConfig, err error)
	SubmarinerConfigExpansion
}

// submarinerConfigs implements SubmarinerConfigInterface
type submarinerConfigs struct {
	client rest.Interface
	ns     string
}

// newSubmarinerConfigs returns a SubmarinerConfigs
func newSubmarinerConfigs(c *SubmarineraddonV1alpha1Client, namespace string) *submarinerConfigs {
	return &submarinerConfigs{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the submarinerConfig, and returns the corresponding submarinerConfig object, and an error if there is any.
func (c *submarinerConfigs) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.SubmarinerConfig, err error) {
	result = &v1alpha1.SubmarinerConfig{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("submarinerconfigs").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of SubmarinerConfigs that match those selectors.
func (c *submarinerConfigs) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.SubmarinerConfigList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.SubmarinerConfigList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("submarinerconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested submarinerConfigs.
func (c *submarinerConfigs) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("submarinerconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a submarinerConfig and creates it.  Returns the server's representation of the submarinerConfig, and an error, if there is any.
func (c *submarinerConfigs) Create(ctx context.Context, submarinerConfig *v1alpha1.SubmarinerConfig, opts v1.CreateOptions) (result *v1alpha1.SubmarinerConfig, err error) {
	result = &v1alpha1.SubmarinerConfig{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("submarinerconfigs").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(submarinerConfig).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a submarinerConfig and updates it. Returns the server's representation of the submarinerConfig, and an error, if there is any.
func (c *submarinerConfigs) Update(ctx context.Context, submarinerConfig *v1alpha1.SubmarinerConfig, opts v1.UpdateOptions) (result *v1alpha1.SubmarinerConfig, err error) {
	result = &v1alpha1.SubmarinerConfig{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("submarinerconfigs").
		Name(submarinerConfig.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(submarinerConfig).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *submarinerConfigs) UpdateStatus(ctx context.Context, submarinerConfig *v1alpha1.SubmarinerConfig, opts v1.UpdateOptions) (result *v1alpha1.SubmarinerConfig, err error) {
	result = &v1alpha1.SubmarinerConfig{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("submarinerconfigs").
		Name(submarinerConfig.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(submarinerConfig).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the submarinerConfig and deletes it. Returns an error if one occurs.
func (c *submarinerConfigs) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("submarinerconfigs").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *submarinerConfigs) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("submarinerconfigs").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched submarinerConfig.
func (c *submarinerConfigs) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.SubmarinerConfig, err error) {
	result = &v1alpha1.SubmarinerConfig{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("submarinerconfigs").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
