package submarinerbrokerinfo

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strings"

	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	configclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"
	apiconfigv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
)

const (
	catalogName                   = "submariner"
	defaultCatalogSource          = "redhat-operators"
	defaultCatalogSourceNamespace = "openshift-marketplace"
	defaultCableDriver            = "libreswan"
	defaultInstallationNamespace  = "open-cluster-management-agent-addon"
	brokerAPIServer               = "BROKER_API_SERVER"
	ocpInfrastructureName         = "cluster"
	ocpAPIServerName              = "cluster"
	ocpConfigNamespace            = "openshift-config"
)

var (
	// catalogSource represents submariner catalog source, this can be set with go build ldflags
	catalogSource string

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
)

type SubmarinerBrokerInfo struct {
	NATEnabled                bool
	IPSecIKEPort              int
	IPSecNATTPort             int
	InstallationNamespace     string
	BrokerAPIServer           string
	BrokerNamespace           string
	BrokerToken               string
	BrokerCA                  string
	IPSecPSK                  string
	CableDriver               string
	ClusterName               string
	ClusterCIDR               string
	ServiceCIDR               string
	CatalogChannel            string
	CatalogName               string
	CatalogSource             string
	CatalogSourceNamespace    string
	CatalogStartingCSV        string
	SubmarinerGatewayImage    string
	SubmarinerRouteAgentImage string
	LighthouseAgentImage      string
	LighthouseCoreDNSImage    string
}

// Get retrieves submariner broker information consolidated with hub information.
func Get(
	kubeClient kubernetes.Interface,
	dynamicClient dynamic.Interface,
	configClient configclient.Interface,
	recorder events.Recorder,
	clusterName string,
	brokeNamespace string,
	submarinerConfig *configv1alpha1.SubmarinerConfig,
	managedClusterAddOn *addonv1alpha1.ManagedClusterAddOn) (*SubmarinerBrokerInfo, error) {
	brokerInfo := &SubmarinerBrokerInfo{
		CableDriver:            defaultCableDriver,
		IPSecNATTPort:          helpers.SubmarinerNatTPort,
		BrokerNamespace:        brokeNamespace,
		ClusterName:            clusterName,
		CatalogName:            catalogName,
		CatalogSource:          defaultCatalogSource,
		CatalogSourceNamespace: defaultCatalogSourceNamespace,
		InstallationNamespace:  defaultInstallationNamespace,
	}

	if len(managedClusterAddOn.Spec.InstallNamespace) != 0 {
		brokerInfo.InstallationNamespace = managedClusterAddOn.Spec.InstallNamespace
	}

	apiServer, err := getBrokerAPIServer(dynamicClient)
	if err != nil {
		return nil, err
	}
	brokerInfo.BrokerAPIServer = apiServer

	ipSecPSK, err := getIPSecPSK(kubeClient, brokeNamespace)
	if err != nil {
		return nil, err
	}
	brokerInfo.IPSecPSK = ipSecPSK

	token, ca, err := getBrokerTokenAndCA(kubeClient, dynamicClient, brokeNamespace, clusterName)
	if err != nil {
		return nil, err
	}
	brokerInfo.BrokerCA = ca
	brokerInfo.BrokerToken = token

	if err := applySubmarinerConfig(configClient, recorder, brokerInfo, submarinerConfig); err != nil {
		return nil, err
	}

	return brokerInfo, nil
}

func applySubmarinerConfig(
	configClient configclient.Interface,
	recorder events.Recorder,
	brokerInfo *SubmarinerBrokerInfo, submarinerConfig *configv1alpha1.SubmarinerConfig) error {
	if submarinerConfig == nil {
		return nil
	}

	brokerInfo.NATEnabled = submarinerConfig.Spec.NATTEnable

	if submarinerConfig.Spec.CableDriver != "" {
		brokerInfo.CableDriver = submarinerConfig.Spec.CableDriver
	}

	if submarinerConfig.Spec.IPSecIKEPort != 0 {
		brokerInfo.IPSecIKEPort = submarinerConfig.Spec.IPSecIKEPort
	}

	if submarinerConfig.Spec.IPSecNATTPort != 0 {
		brokerInfo.IPSecNATTPort = submarinerConfig.Spec.IPSecNATTPort
	}

	if submarinerConfig.Spec.SubscriptionConfig.Channel != "" {
		brokerInfo.CatalogChannel = submarinerConfig.Spec.SubscriptionConfig.Channel
	}

	if submarinerConfig.Spec.SubscriptionConfig.Source != "" {
		brokerInfo.CatalogSource = submarinerConfig.Spec.SubscriptionConfig.Source
	}

	if submarinerConfig.Spec.SubscriptionConfig.SourceNamespace != "" {
		brokerInfo.CatalogSourceNamespace = submarinerConfig.Spec.SubscriptionConfig.SourceNamespace
	}

	if submarinerConfig.Spec.SubscriptionConfig.StartingCSV != "" {
		brokerInfo.CatalogStartingCSV = submarinerConfig.Spec.SubscriptionConfig.StartingCSV
	}

	if submarinerConfig.Spec.ImagePullSpecs.SubmarinerImagePullSpec != "" {
		brokerInfo.SubmarinerGatewayImage = submarinerConfig.Spec.ImagePullSpecs.SubmarinerImagePullSpec
	}

	if submarinerConfig.Spec.ImagePullSpecs.SubmarinerRouteAgentImagePullSpec != "" {
		brokerInfo.SubmarinerRouteAgentImage = submarinerConfig.Spec.ImagePullSpecs.SubmarinerRouteAgentImagePullSpec
	}

	if submarinerConfig.Spec.ImagePullSpecs.LighthouseCoreDNSImagePullSpec != "" {
		brokerInfo.LighthouseCoreDNSImage = submarinerConfig.Spec.ImagePullSpecs.LighthouseCoreDNSImagePullSpec
	}

	if submarinerConfig.Spec.ImagePullSpecs.LighthouseAgentImagePullSpec != "" {
		brokerInfo.LighthouseAgentImage = submarinerConfig.Spec.ImagePullSpecs.LighthouseAgentImagePullSpec
	}

	condition := metav1.Condition{
		Type:    configv1alpha1.SubmarinerConfigConditionApplied,
		Status:  metav1.ConditionTrue,
		Reason:  "SubmarinerConfigApplied",
		Message: "SubmarinerConfig was applied",
	}
	_, updated, err := helpers.UpdateSubmarinerConfigStatus(
		configClient,
		submarinerConfig.Namespace, submarinerConfig.Name,
		helpers.UpdateSubmarinerConfigConditionFn(condition),
	)
	if err != nil {
		return err
	}

	if updated {
		recorder.Eventf("SubmarinerConfigApplied", "SubmarinerConfig %s was applied for manged cluster %s", submarinerConfig.Name, submarinerConfig.Namespace)
	}

	return nil
}

func getIPSecPSK(client kubernetes.Interface, brokerNamespace string) (string, error) {
	secret, err := client.CoreV1().Secrets(brokerNamespace).Get(context.TODO(), helpers.IPSecPSKSecretName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get broker IPSEC PSK secret %v/%v: %v", brokerNamespace, helpers.IPSecPSKSecretName, err)
	}

	return base64.StdEncoding.EncodeToString(secret.Data["psk"]), nil
}

func getBrokerAPIServer(dynamicClient dynamic.Interface) (string, error) {
	infrastructureConfig, err := dynamicClient.Resource(infrastructureGVR).Get(context.TODO(), ocpInfrastructureName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			apiServer := os.Getenv(brokerAPIServer)
			if apiServer == "" {
				return "", fmt.Errorf("failed to get apiserver in env %v", brokerAPIServer)
			}
			return apiServer, nil
		}
		return "", fmt.Errorf("failed to get infrastructures cluster: %v", err)
	}

	apiServer, found, err := unstructured.NestedString(infrastructureConfig.Object, "status", "apiServerURL")
	if err != nil || !found {
		return "", fmt.Errorf("failed to get apiServerURL in infrastructures cluster: %v,%v", found, err)
	}

	return strings.Trim(apiServer, "https://"), nil
}

func getKubeAPIServerCA(kubeClient kubernetes.Interface, dynamicClient dynamic.Interface) ([]byte, error) {
	kubeAPIServer, err := getBrokerAPIServer(dynamicClient)
	if err != nil {
		return nil, err
	}

	kubeAPIServerURL, err := url.Parse(fmt.Sprintf("https://%s", kubeAPIServer))
	if err != nil {
		return nil, err
	}

	unstructuredAPIServer, err := dynamicClient.Resource(apiServerGVR).Get(context.TODO(), ocpAPIServerName, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	apiServer := &apiconfigv1.APIServer{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(unstructuredAPIServer.UnstructuredContent(), &apiServer); err != nil {
		return nil, err
	}

	for _, namedCert := range apiServer.Spec.ServingCerts.NamedCertificates {
		for _, name := range namedCert.Names {
			if strings.EqualFold(name, kubeAPIServerURL.Hostname()) {
				secretName := namedCert.ServingCertificate.Name
				secret, err := kubeClient.CoreV1().Secrets(ocpConfigNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
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
	}

	return nil, nil
}

func getBrokerTokenAndCA(kubeClient kubernetes.Interface, dynamicClient dynamic.Interface, brokerNS, clusterName string) (token, ca string, err error) {
	sa, err := kubeClient.CoreV1().ServiceAccounts(brokerNS).Get(context.TODO(), clusterName, metav1.GetOptions{})
	if err != nil {
		return "", "", fmt.Errorf("failed to get agent ServiceAccount %v/%v: %v", brokerNS, clusterName, err)
	}
	if len(sa.Secrets) < 1 {
		return "", "", fmt.Errorf("ServiceAccount %v does not have any secret", sa.Name)
	}

	brokerTokenPrefix := fmt.Sprintf("%s-token-", clusterName)
	for _, secret := range sa.Secrets {
		if strings.HasPrefix(secret.Name, brokerTokenPrefix) {
			tokenSecret, err := kubeClient.CoreV1().Secrets(brokerNS).Get(context.TODO(), secret.Name, metav1.GetOptions{})
			if err != nil {
				return "", "", fmt.Errorf("failed to get secret %v of agent ServiceAccount %v/%v: %v", secret.Name, brokerNS, clusterName, err)
			}

			if tokenSecret.Type == corev1.SecretTypeServiceAccountToken {
				// try to get ca from apiserver secret firstly, if the ca cannot be found, get it from sa
				kubeAPIServerCA, err := getKubeAPIServerCA(kubeClient, dynamicClient)
				if err != nil {
					return "", "", err
				}

				if kubeAPIServerCA == nil {
					return string(tokenSecret.Data["token"]), base64.StdEncoding.EncodeToString(tokenSecret.Data["ca.crt"]), nil
				}

				return string(tokenSecret.Data["token"]), base64.StdEncoding.EncodeToString(kubeAPIServerCA), nil
			}
		}
	}

	return "", "", fmt.Errorf("ServiceAccount %v/%v does not have a secret of type token", brokerNS, clusterName)

}
