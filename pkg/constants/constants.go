package constants

const (
	SubmarinerAddOnName  = "submariner"
	SubmarinerConfigName = "submariner"

	ProductOCP        = "OpenShift"
	ProductROSA       = "ROSA"
	ProductARO        = "ARO"
	OCPVersionForOVNK = "4.11.0-rc"

	IPSecPSKSecretName = "submariner-ipsec-psk"

	SubmarinerNatTPort           = 4500
	SubmarinerNatTDiscoveryPort  = 4900
	SubmarinerRoutePort          = 4800
	SubmarinerGatewayMetricsPort = 8080

	// TODO: Currently we are configuring this Port unconditionally. This is an internal port, but can be
	// enabled only in Globalnet deployments.
	SubmarinerGlobalnetMetricsPort = 8081
)
