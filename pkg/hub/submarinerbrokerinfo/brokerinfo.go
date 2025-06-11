package submarinerbrokerinfo

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strings"

	apiconfigv1 "github.com/openshift/api/config/v1"
	"github.com/pkg/errors"
	configv1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	"github.com/stolostron/submariner-addon/pkg/constants"
	"github.com/submariner-io/admiral/pkg/log"
	"github.com/submariner-io/admiral/pkg/reporter"
	"github.com/submariner-io/submariner-operator/pkg/discovery/globalnet"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	controllerclient "sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	catalogName                   = "submariner"
	defaultCatalogSource          = "redhat-operators"
	defaultCatalogSourceNamespace = "openshift-marketplace"
	defaultCatalogChannel         = "stable-0.20"
	defaultCableDriver            = "libreswan"
	defaultInstallationNamespace  = "open-cluster-management-agent-addon"
	brokerAPIServer               = "BROKER_API_SERVER"
	ocpInfrastructureName         = "cluster"
	ocpAPIServerName              = "cluster"
	ocpConfigNamespace            = "openshift-config"
	brokerSuffix                  = "broker"
	namespaceMaxLength            = 63
)

var (
	logger = log.Logger{Logger: logf.Log.WithName("BrokerInfo")}

	infrastructureGVR = schema.GroupVersionResource{
		Group:    "config.openshift.io",
		Version:  "v1",
		Resource: "infrastructures",
	}

	apiServerGVR = schema.GroupVersionResource{
		Group:    "config.openshift.io",
		Version:  "v1",
		Resource: "apiservers",
	}

	loggedCatalogChannel string
)

type SubmarinerBrokerInfo struct {
	AirGappedDeployment       bool
	NATEnabled                bool
	LoadBalancerEnabled       bool
	InsecureBrokerConnection  bool
	IPSecDebug                bool
	Debug                     bool
	HaltOnCertificateError    bool
	ForceUDPEncaps            bool
	HostedCluster             bool
	IPSecNATTPort             int
	InstallationNamespace     string
	InstallPlanApproval       string
	BrokerAPIServer           string
	BrokerNamespace           string
	BrokerToken               string
	BrokerCA                  string
	IPSecPSK                  string
	CableDriver               string
	ClusterName               string
	ClusterCIDR               string
	GlobalCIDR                string
	ServiceCIDR               string
	CatalogChannel            string
	CatalogName               string
	CatalogSource             string
	CatalogSourceNamespace    string
	CatalogStartingCSV        string
	SubmarinerGatewayImage    string
	SubmarinerRouteAgentImage string
	SubmarinerGlobalnetImage  string
	LighthouseAgentImage      string
	LighthouseCoreDNSImage    string
	MetricsProxyImage         string
	NettestImage              string
	NodeSelector              map[string]string
	Tolerations               []corev1.Toleration
}

// Get retrieves submariner broker information consolidated with hub information.
func Get(
	ctx context.Context,
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	controllerClient controllerclient.Client,
	clusterName string,
	brokerNamespace string,
	submarinerConfig *configv1alpha1.SubmarinerConfig,
	installationNamespace string,
) (*SubmarinerBrokerInfo, error) {
	brokerInfo := &SubmarinerBrokerInfo{
		CableDriver:            defaultCableDriver,
		IPSecNATTPort:          constants.SubmarinerNatTPort,
		BrokerNamespace:        brokerNamespace,
		ClusterName:            clusterName,
		CatalogName:            catalogName,
		CatalogSource:          defaultCatalogSource,
		CatalogSourceNamespace: defaultCatalogSourceNamespace,
		CatalogChannel:         defaultCatalogChannel,
		InstallationNamespace:  defaultInstallationNamespace,
		InstallPlanApproval:    "Automatic",
		NodeSelector:           make(map[string]string),
		Tolerations:            make([]corev1.Toleration, 0),
		HaltOnCertificateError: true,
	}

	if installationNamespace != "" {
		brokerInfo.InstallationNamespace = installationNamespace
	}

	err := applyGlobalnetConfig(ctx, controllerClient, brokerNamespace, clusterName, brokerInfo, submarinerConfig)
	if err != nil {
		return nil, err
	}

	apiServer, err := getBrokerAPIServer(ctx, dynamicClient)
	if err != nil {
		return nil, err
	}

	brokerInfo.BrokerAPIServer = apiServer

	ipSecPSK, err := getIPSecPSK(ctx, kubeClient, brokerNamespace)
	if err != nil {
		return nil, err
	}

	brokerInfo.IPSecPSK = ipSecPSK

	token, ca, err := getBrokerTokenAndCA(ctx, kubeClient, dynamicClient, brokerNamespace, clusterName, apiServer)
	if err != nil {
		return nil, err
	}

	brokerInfo.BrokerCA = ca
	brokerInfo.BrokerToken = token

	applySubmarinerConfig(brokerInfo, submarinerConfig)

	return brokerInfo, nil
}

func applyGlobalnetConfig(ctx context.Context, controllerClient controllerclient.Client, brokerNamespace,
	clusterName string, brokerInfo *SubmarinerBrokerInfo, submarinerConfig *configv1alpha1.SubmarinerConfig,
) error {
	gnInfo, _, err := globalnet.GetGlobalNetworks(ctx, controllerClient, brokerNamespace)
	if err != nil && !apierrors.IsNotFound(err) {
		return errors.Wrapf(err, "error reading globalnet configmap from namespace %q", brokerNamespace)
	}

	if apierrors.IsNotFound(err) {
		logger.Warningf("globalnetConfigMap is missing in the broker namespace %q", brokerNamespace)
		return err
	}

	if gnInfo != nil && gnInfo.Enabled {
		netconfig := globalnet.Config{
			ClusterID: clusterName,
		}

		// If GlobalCIDR is manually specified by the user in the submarinerConfig
		if submarinerConfig != nil && submarinerConfig.Spec.GlobalCIDR != "" {
			netconfig.GlobalCIDR = submarinerConfig.Spec.GlobalCIDR
		}

		status := reporter.Silent()
		err = globalnet.AllocateAndUpdateGlobalCIDRConfigMap(ctx, controllerClient, brokerNamespace, &netconfig, status)
		if err != nil {
			logger.Errorf(err, "Unable to allocate globalCIDR to cluster %q", clusterName)
			return err
		}

		logger.V(log.DEBUG).Infof("Allocated globalCIDR %q for Cluster %q", netconfig.GlobalCIDR, clusterName)
		brokerInfo.GlobalCIDR = netconfig.GlobalCIDR
	}

	return nil
}

func applySubmarinerConfig(brokerInfo *SubmarinerBrokerInfo, submarinerConfig *configv1alpha1.SubmarinerConfig) {
	if submarinerConfig == nil {
		return
	}

	brokerInfo.AirGappedDeployment = submarinerConfig.Spec.AirGappedDeployment
	brokerInfo.NATEnabled = submarinerConfig.Spec.NATTEnable
	brokerInfo.LoadBalancerEnabled = submarinerConfig.Spec.LoadBalancerEnable
	brokerInfo.InsecureBrokerConnection = submarinerConfig.Spec.InsecureBrokerConnection
	brokerInfo.Debug = submarinerConfig.Spec.Debug
	brokerInfo.IPSecDebug = submarinerConfig.Spec.IPSecDebug
	brokerInfo.ForceUDPEncaps = submarinerConfig.Spec.ForceUDPEncaps
	brokerInfo.HaltOnCertificateError = submarinerConfig.Spec.HaltOnCertificateError
	brokerInfo.HostedCluster = submarinerConfig.Spec.HostedCluster

	setIfValueNotDefault(&brokerInfo.CableDriver, submarinerConfig.Spec.CableDriver)
	setIfValueNotDefault(&brokerInfo.IPSecNATTPort, submarinerConfig.Spec.IPSecNATTPort)
	setIfValueNotDefault(&brokerInfo.CatalogChannel, submarinerConfig.Spec.SubscriptionConfig.Channel)
	setIfValueNotDefault(&brokerInfo.CatalogSource, submarinerConfig.Spec.SubscriptionConfig.Source)
	setIfValueNotDefault(&brokerInfo.CatalogSourceNamespace, submarinerConfig.Spec.SubscriptionConfig.SourceNamespace)
	setIfValueNotDefault(&brokerInfo.CatalogStartingCSV, submarinerConfig.Spec.SubscriptionConfig.StartingCSV)
	setIfValueNotDefault(&brokerInfo.InstallPlanApproval, submarinerConfig.Spec.SubscriptionConfig.InstallPlanApproval)

	if brokerInfo.CatalogChannel != defaultCatalogChannel && brokerInfo.CatalogChannel != loggedCatalogChannel {
		logger.Infof("Tracking non-default catalog channel %q (default is %q)", brokerInfo.CatalogChannel, defaultCatalogChannel)
		loggedCatalogChannel = brokerInfo.CatalogChannel
	}

	applySubmarinerImageConfig(brokerInfo, submarinerConfig)
}

func applySubmarinerImageConfig(brokerInfo *SubmarinerBrokerInfo, submarinerConfig *configv1alpha1.SubmarinerConfig) {
	setIfValueNotDefault(&brokerInfo.SubmarinerGatewayImage, submarinerConfig.Spec.ImagePullSpecs.SubmarinerImagePullSpec)
	setIfValueNotDefault(&brokerInfo.SubmarinerRouteAgentImage, submarinerConfig.Spec.ImagePullSpecs.SubmarinerRouteAgentImagePullSpec)
	setIfValueNotDefault(&brokerInfo.LighthouseCoreDNSImage, submarinerConfig.Spec.ImagePullSpecs.LighthouseCoreDNSImagePullSpec)
	setIfValueNotDefault(&brokerInfo.LighthouseAgentImage, submarinerConfig.Spec.ImagePullSpecs.LighthouseAgentImagePullSpec)
	setIfValueNotDefault(&brokerInfo.SubmarinerGlobalnetImage, submarinerConfig.Spec.ImagePullSpecs.SubmarinerGlobalnetImagePullSpec)
	setIfValueNotDefault(&brokerInfo.MetricsProxyImage, submarinerConfig.Spec.ImagePullSpecs.MetricsProxyImagePullSpec)
	setIfValueNotDefault(&brokerInfo.NettestImage, submarinerConfig.Spec.ImagePullSpecs.NettestImagePullSpec)
}

func setIfValueNotDefault[T comparable](target *T, value T) {
	var defvalue T
	if value != defvalue {
		*target = value
	}
}

func getIPSecPSK(ctx context.Context, client kubernetes.Interface, brokerNamespace string) (string, error) {
	secret, err := client.CoreV1().Secrets(brokerNamespace).Get(ctx, constants.IPSecPSKSecretName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get broker IPSEC PSK secret %v/%v: %w",
			brokerNamespace, constants.IPSecPSKSecretName, err)
	}

	return base64.StdEncoding.EncodeToString(secret.Data["psk"]), nil
}

func getBrokerAPIServer(ctx context.Context, dynamicClient dynamic.Interface) (string, error) {
	infrastructureConfig, err := dynamicClient.Resource(infrastructureGVR).
		Get(ctx, ocpInfrastructureName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			apiServer := os.Getenv(brokerAPIServer)
			if apiServer == "" {
				return "", fmt.Errorf("failed to get apiserver in env %v", brokerAPIServer)
			}

			return apiServer, nil
		}

		return "", errors.Wrap(err, "failed to get infrastructures cluster")
	}

	apiServer, found, err := unstructured.NestedString(infrastructureConfig.Object, "status", "apiServerURL")
	if err != nil {
		return "", errors.Wrap(err, "failed to get apiServerURL in infrastructures cluster")
	}

	if !found {
		return "", errors.New("apiServerURL not found in infrastructures cluster")
	}

	return strings.Trim(strings.Trim(apiServer, "hpst"), "/:"), nil
}

func getKubeAPIServerCA(ctx context.Context, kubeAPIServer string, kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
) ([]byte, error) {
	kubeAPIServerURL, err := url.Parse("https://" + kubeAPIServer)
	if err != nil {
		return nil, err
	}

	unstructuredAPIServer, err := dynamicClient.Resource(apiServerGVR).Get(ctx, ocpAPIServerName, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	apiServer := &apiconfigv1.APIServer{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(
		unstructuredAPIServer.UnstructuredContent(), &apiServer); err != nil {
		return nil, err
	}

	for _, namedCert := range apiServer.Spec.ServingCerts.NamedCertificates {
		for _, name := range namedCert.Names {
			if !strings.EqualFold(name, kubeAPIServerURL.Hostname()) {
				continue
			}

			secretName := namedCert.ServingCertificate.Name
			secret, err := kubeClient.CoreV1().Secrets(ocpConfigNamespace).Get(ctx, secretName, metav1.GetOptions{})
			if err != nil {
				return nil, err
			}

			if secret.Type != corev1.SecretTypeTLS {
				return nil, fmt.Errorf("secret %s/%s should have type=kubernetes.io/tls", ocpConfigNamespace, secretName)
			}

			ca, ok := secret.Data["tls.crt"]
			if !ok {
				return nil, fmt.Errorf("failed to find data[tls.crt] in secret %s/%s", ocpConfigNamespace, secretName)
			}

			return ca, nil
		}
	}

	return nil, nil
}

func getBrokerTokenAndCA(ctx context.Context, kubeClient kubernetes.Interface, dynamicClient dynamic.Interface,
	brokerNS, clusterName, kubeAPIServer string,
) (string, string, error) {
	sa, err := kubeClient.CoreV1().ServiceAccounts(brokerNS).Get(ctx, clusterName, metav1.GetOptions{})
	if err != nil {
		return "", "", fmt.Errorf("failed to get agent ServiceAccount %v/%v: %w", brokerNS, clusterName, err)
	}

	tokenSecret, err := getTokenSecretForSA(ctx, kubeClient, sa)
	if err != nil {
		return "", "", err
	}

	return getTokenAndCAFromSecret(ctx, tokenSecret, kubeAPIServer, kubeClient, dynamicClient)
}

func getTokenSecretForSA(ctx context.Context, client kubernetes.Interface, sa *corev1.ServiceAccount,
) (*corev1.Secret, error) {
	var secret *corev1.Secret
	saSecrets, err := client.CoreV1().Secrets(sa.Namespace).List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("type", string(corev1.SecretTypeServiceAccountToken)).String(),
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get secrets of type service-account-token in %v", sa.Namespace)
	}

	for i := range saSecrets.Items {
		if saSecrets.Items[i].Annotations[corev1.ServiceAccountNameKey] == sa.Name {
			secret = &saSecrets.Items[i]
		}
	}

	// Secret not found, so create one and return.
	if secret == nil {
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:        GenerateBrokerName(sa.Name),
				Annotations: map[string]string{corev1.ServiceAccountNameKey: sa.Name},
			},
			Type: corev1.SecretTypeServiceAccountToken,
		}

		secret, err = client.CoreV1().Secrets(sa.Namespace).Create(ctx, secret, metav1.CreateOptions{})
		if err != nil {
			return nil, errors.Wrapf(err, "failed to create secret %s/%s of type service-account",
				sa.Name, sa.Namespace)
		}
	}

	return secret, nil
}

func getTokenAndCAFromSecret(ctx context.Context, tokenSecret *corev1.Secret, kubeAPIServer string,
	kubeClient kubernetes.Interface, dynamicClient dynamic.Interface,
) (string, string, error) {
	if len(tokenSecret.Data) == 0 || tokenSecret.Data["token"] == nil {
		return "", "", fmt.Errorf("token data not yet generated for secret %s/%s", tokenSecret.Namespace, tokenSecret.Name)
	}

	// try to get ca from apiserver secret firstly, if the ca cannot be found, get it from sa
	kubeAPIServerCA, err := getKubeAPIServerCA(ctx, kubeAPIServer, kubeClient, dynamicClient)
	if err != nil {
		return "", "", err
	}

	if kubeAPIServerCA == nil {
		return string(tokenSecret.Data["token"]), base64.StdEncoding.EncodeToString(tokenSecret.Data["ca.crt"]), nil
	}

	return string(tokenSecret.Data["token"]), base64.StdEncoding.EncodeToString(kubeAPIServerCA), nil
}

func GenerateBrokerName(name string) string {
	brokerName := fmt.Sprintf("%s-%s", name, brokerSuffix)
	if len(brokerName) > namespaceMaxLength {
		truncatedName := name[(len(brokerSuffix) - 1):]

		return fmt.Sprintf("%s-%s", truncatedName, brokerSuffix)
	}

	return brokerName
}
