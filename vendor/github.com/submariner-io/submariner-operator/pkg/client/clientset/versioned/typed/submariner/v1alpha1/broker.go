/*
Copyright The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	"context"
	"time"

	v1alpha1 "github.com/submariner-io/submariner-operator/api/submariner/v1alpha1"
	scheme "github.com/submariner-io/submariner-operator/pkg/client/clientset/versioned/scheme"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// BrokersGetter has a method to return a BrokerInterface.
// A group's client should implement this interface.
type BrokersGetter interface {
	Brokers(namespace string) BrokerInterface
}

// BrokerInterface has methods to work with Broker resources.
type BrokerInterface interface {
	Create(ctx context.Context, broker *v1alpha1.Broker, opts v1.CreateOptions) (*v1alpha1.Broker, error)
	Update(ctx context.Context, broker *v1alpha1.Broker, opts v1.UpdateOptions) (*v1alpha1.Broker, error)
	UpdateStatus(ctx context.Context, broker *v1alpha1.Broker, opts v1.UpdateOptions) (*v1alpha1.Broker, error)
	Delete(ctx context.Context, name string, opts v1.DeleteOptions) error
	DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error
	Get(ctx context.Context, name string, opts v1.GetOptions) (*v1alpha1.Broker, error)
	List(ctx context.Context, opts v1.ListOptions) (*v1alpha1.BrokerList, error)
	Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error)
	Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.Broker, err error)
	BrokerExpansion
}

// brokers implements BrokerInterface
type brokers struct {
	client rest.Interface
	ns     string
}

// newBrokers returns a Brokers
func newBrokers(c *SubmarinerV1alpha1Client, namespace string) *brokers {
	return &brokers{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the broker, and returns the corresponding broker object, and an error if there is any.
func (c *brokers) Get(ctx context.Context, name string, options v1.GetOptions) (result *v1alpha1.Broker, err error) {
	result = &v1alpha1.Broker{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("brokers").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do(ctx).
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of Brokers that match those selectors.
func (c *brokers) List(ctx context.Context, opts v1.ListOptions) (result *v1alpha1.BrokerList, err error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	result = &v1alpha1.BrokerList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("brokers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Do(ctx).
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested brokers.
func (c *brokers) Watch(ctx context.Context, opts v1.ListOptions) (watch.Interface, error) {
	var timeout time.Duration
	if opts.TimeoutSeconds != nil {
		timeout = time.Duration(*opts.TimeoutSeconds) * time.Second
	}
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("brokers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Timeout(timeout).
		Watch(ctx)
}

// Create takes the representation of a broker and creates it.  Returns the server's representation of the broker, and an error, if there is any.
func (c *brokers) Create(ctx context.Context, broker *v1alpha1.Broker, opts v1.CreateOptions) (result *v1alpha1.Broker, err error) {
	result = &v1alpha1.Broker{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("brokers").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(broker).
		Do(ctx).
		Into(result)
	return
}

// Update takes the representation of a broker and updates it. Returns the server's representation of the broker, and an error, if there is any.
func (c *brokers) Update(ctx context.Context, broker *v1alpha1.Broker, opts v1.UpdateOptions) (result *v1alpha1.Broker, err error) {
	result = &v1alpha1.Broker{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("brokers").
		Name(broker.Name).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(broker).
		Do(ctx).
		Into(result)
	return
}

// UpdateStatus was generated because the type contains a Status member.
// Add a +genclient:noStatus comment above the type to avoid generating UpdateStatus().
func (c *brokers) UpdateStatus(ctx context.Context, broker *v1alpha1.Broker, opts v1.UpdateOptions) (result *v1alpha1.Broker, err error) {
	result = &v1alpha1.Broker{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("brokers").
		Name(broker.Name).
		SubResource("status").
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(broker).
		Do(ctx).
		Into(result)
	return
}

// Delete takes name of the broker and deletes it. Returns an error if one occurs.
func (c *brokers) Delete(ctx context.Context, name string, opts v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("brokers").
		Name(name).
		Body(&opts).
		Do(ctx).
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *brokers) DeleteCollection(ctx context.Context, opts v1.DeleteOptions, listOpts v1.ListOptions) error {
	var timeout time.Duration
	if listOpts.TimeoutSeconds != nil {
		timeout = time.Duration(*listOpts.TimeoutSeconds) * time.Second
	}
	return c.client.Delete().
		Namespace(c.ns).
		Resource("brokers").
		VersionedParams(&listOpts, scheme.ParameterCodec).
		Timeout(timeout).
		Body(&opts).
		Do(ctx).
		Error()
}

// Patch applies the patch and returns the patched broker.
func (c *brokers) Patch(ctx context.Context, name string, pt types.PatchType, data []byte, opts v1.PatchOptions, subresources ...string) (result *v1alpha1.Broker, err error) {
	result = &v1alpha1.Broker{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("brokers").
		Name(name).
		SubResource(subresources...).
		VersionedParams(&opts, scheme.ParameterCodec).
		Body(data).
		Do(ctx).
		Into(result)
	return
}
