package helpers

import (
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
