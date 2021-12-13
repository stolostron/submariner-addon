package helpers

import (
	"io/ioutil"
	"os"
)

const (
	SubmarinerAddOnName  = "submariner"
	SubmarinerConfigName = "submariner"
)

const (
	ProductOCP = "OpenShift"
)

const (
	IPSecPSKSecretName = "submariner-ipsec-psk"
)

const (
	SubmarinerNatTPort          = 4500
	SubmarinerNatTDiscoveryPort = 4900
	SubmarinerRoutePort         = 4800
	SubmarinerMetricsPort       = 8080
)

func GetEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	return value
}

// GetCurrentNamespace returns the current namesapce from file system,
// if the namespace is not found, it returns the defaultNamespace.
func GetCurrentNamespace(defaultNamespace string) string {
	nsBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return defaultNamespace
	}

	return string(nsBytes)
}
