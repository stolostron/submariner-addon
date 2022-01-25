// Code generated by client-gen. DO NOT EDIT.

package v1alpha1

import (
	v1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	"github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/clientset/versioned/scheme"
	rest "k8s.io/client-go/rest"
)

type SubmarineraddonV1alpha1Interface interface {
	RESTClient() rest.Interface
	SubmarinerConfigsGetter
	SubmarinerDiagnoseConfigsGetter
}

// SubmarineraddonV1alpha1Client is used to interact with features provided by the submarineraddon.open-cluster-management.io group.
type SubmarineraddonV1alpha1Client struct {
	restClient rest.Interface
}

func (c *SubmarineraddonV1alpha1Client) SubmarinerConfigs(namespace string) SubmarinerConfigInterface {
	return newSubmarinerConfigs(c, namespace)
}

func (c *SubmarineraddonV1alpha1Client) SubmarinerDiagnoseConfigs(namespace string) SubmarinerDiagnoseConfigInterface {
	return newSubmarinerDiagnoseConfigs(c, namespace)
}

// NewForConfig creates a new SubmarineraddonV1alpha1Client for the given config.
func NewForConfig(c *rest.Config) (*SubmarineraddonV1alpha1Client, error) {
	config := *c
	if err := setConfigDefaults(&config); err != nil {
		return nil, err
	}
	client, err := rest.RESTClientFor(&config)
	if err != nil {
		return nil, err
	}
	return &SubmarineraddonV1alpha1Client{client}, nil
}

// NewForConfigOrDie creates a new SubmarineraddonV1alpha1Client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(c *rest.Config) *SubmarineraddonV1alpha1Client {
	client, err := NewForConfig(c)
	if err != nil {
		panic(err)
	}
	return client
}

// New creates a new SubmarineraddonV1alpha1Client for the given RESTClient.
func New(c rest.Interface) *SubmarineraddonV1alpha1Client {
	return &SubmarineraddonV1alpha1Client{c}
}

func setConfigDefaults(config *rest.Config) error {
	gv := v1alpha1.SchemeGroupVersion
	config.GroupVersion = &gv
	config.APIPath = "/apis"
	config.NegotiatedSerializer = scheme.Codecs.WithoutConversion()

	if config.UserAgent == "" {
		config.UserAgent = rest.DefaultKubernetesUserAgent()
	}

	return nil
}

// RESTClient returns a RESTClient that is used to communicate
// with API server by this client implementation.
func (c *SubmarineraddonV1alpha1Client) RESTClient() rest.Interface {
	if c == nil {
		return nil
	}
	return c.restClient
}
