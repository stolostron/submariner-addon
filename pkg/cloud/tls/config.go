package tls

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/http"

	configv1 "github.com/openshift/api/config/v1"
	openshifttls "github.com/openshift/controller-runtime-common/pkg/tls"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/pkg/errors"
	submreporter "github.com/submariner-io/admiral/pkg/reporter"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

// GetConfigFromAPIServer fetches the TLS security profile from apiserver.config.openshift.io/cluster
// and converts it to a tls.Config for HTTP clients.
//
// This implements OCP 4.22 TLS profile compliance by fetching the cluster's TLS profile
// and applying it using the official controller-runtime-common/pkg/tls package.
//
// The insecureSkipVerify parameter controls certificate verification, but TLS version
// and cipher restrictions from the cluster profile are always applied.
func GetConfigFromAPIServer(ctx context.Context, dynamicClient dynamic.Interface, reporter submreporter.Interface, insecureSkipVerify bool,
) (*tls.Config, error) {
	// Fetch the API Server configuration to get the TLS security profile
	apiServerGVR := configv1.GroupVersion.WithResource("apiservers")

	unstructuredAPIServer, err := dynamicClient.Resource(apiServerGVR).Get(ctx, openshifttls.APIServerName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch apiserver.config.openshift.io/%s: %w", openshifttls.APIServerName, err)
	}

	// Convert unstructured to typed APIServer object
	apiServer := &configv1.APIServer{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredAPIServer.Object, apiServer); err != nil {
		return nil, fmt.Errorf("failed to convert APIServer from unstructured: %w", err)
	}

	// Get TLS profile spec from API Server configuration using official OpenShift package
	profileSpec, err := openshifttls.GetTLSProfileSpec(apiServer.Spec.TLSSecurityProfile)
	if err != nil {
		return nil, fmt.Errorf("failed to get TLS profile spec: %w", err)
	}

	// Convert profile spec to tls.Config using official OpenShift package
	// This handles TLS 1.3 correctly (only sets CipherSuites when MinVersion != TLS 1.3)
	tlsConfigFunc, unsupportedCiphers := openshifttls.NewTLSConfigFromProfile(profileSpec)

	// Log warnings for any unsupported ciphers
	for _, cipher := range unsupportedCiphers {
		reporter.Warning("Cipher suite %q not available in this Go version, skipping", cipher)
	}

	// Create and configure tls.Config
	tlsConfig := &tls.Config{
		InsecureSkipVerify: insecureSkipVerify, //nolint:gosec // InsecureSkipVerify is set from clouds.yaml verify setting
	}
	tlsConfigFunc(tlsConfig)

	// Validate that cipher filtering didn't leave us with an empty list for pre-TLS 1.3.
	// An empty CipherSuites with TLS 1.0-1.2 causes Go to fall back to its defaults,
	// which would bypass the cluster's TLS security profile.
	if tlsConfig.MinVersion < tls.VersionTLS13 {
		profileCipherCount := len(profileSpec.Ciphers)
		actualCipherCount := len(tlsConfig.CipherSuites)

		if profileCipherCount > 0 && actualCipherCount == 0 {
			return nil, fmt.Errorf("API Server TLS profile specified %d cipher suites but none are supported by this Go version",
				profileCipherCount)
		}
	}

	reporter.Success("Applied TLS profile from API Server: MinVersion=%s, CipherSuites=%d, InsecureSkipVerify=%t",
		crypto.TLSVersionToNameOrDie(tlsConfig.MinVersion), len(tlsConfig.CipherSuites), insecureSkipVerify)

	return tlsConfig, nil
}

func GetConfiguredHTTPClient(ctx context.Context, dynamicClient dynamic.Interface, reporter submreporter.Interface,
	insecureSkipVerify bool,
) (*http.Client, error) {
	// Fetch TLS configuration from the API Server to ensure compliance with cluster TLS profile
	// This implements OCP TLS profile compliance requirement by using the API Server
	// configuration as the authoritative source for TLS settings.
	tlsConfig, err := GetConfigFromAPIServer(ctx, dynamicClient, reporter, insecureSkipVerify)
	if err != nil {
		return nil, errors.Wrap(err, "error fetching TLS profile from API Server")
	}

	// Apply the TLS configuration to the HTTP client transport
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = tlsConfig

	return &http.Client{Transport: transport}, nil
}
