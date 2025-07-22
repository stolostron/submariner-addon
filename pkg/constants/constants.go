package constants

const (
	SubmarinerAddOnName      = "submariner"
	SubmarinerConfigName     = "submariner"
	SubmarinerAddOnFinalizer = "submarineraddon.open-cluster-management.io/submariner-addon-cleanup"

	ProductOCP        = "OpenShift"
	ProductROSA       = "ROSA"
	ProductARO        = "ARO"
	ProductROKS       = "ROKS"
	ProductOSD        = "OpenShiftDedicated"
	OCPVersionForOVNK = "4.11.0-rc"

	IPSecPSKSecretName = "submariner-ipsec-psk"

	SubmarinerNatTPort          = 4500
	SubmarinerNatTDiscoveryPort = 4900
	SubmarinerRoutePort         = 4800
)
