package helpers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"

	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	configclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"
	workclient "open-cluster-management.io/api/client/work/clientset/versioned"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"

	"github.com/openshift/api"
	apicofigv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcehelper"
	errorhelpers "github.com/openshift/library-go/pkg/operator/v1helpers"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

const (
	SubmarinerAddOnName  = "submariner"
	SubmarinerConfigName = "submariner"
)

const (
	ProductOCP = "OpenShift"
)

const (
	IPSecPSKSecretLength = 48
	IPSecPSKSecretName   = "submariner-ipsec-psk"
	BrokerAPIServer      = "BROKER_API_SERVER"
)

const (
	SubmarinerIKEPort           = 500
	SubmarinerNatTPort          = 4500
	SubmarinerNatTDiscoveryPort = 4900
	SubmarinerRoutePort         = 4800
	SubmarinerMetricsPort       = 8080
)

const (
	ocpInfrastructureName = "cluster"
	ocpAPIServerName      = "cluster"
	ocpConfigNamespace    = "openshift-config"
)

const (
	brokerSuffix            = "broker"
	namespaceMaxLength      = 63
	clusterSetNameMaxLength = namespaceMaxLength - len(brokerSuffix) - 1
)

var (
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

var (
	genericScheme = runtime.NewScheme()
	genericCodecs = serializer.NewCodecFactory(genericScheme)
	genericCodec  = genericCodecs.UniversalDeserializer()
)

func init() {
	utilruntime.Must(api.InstallKube(genericScheme))
	utilruntime.Must(apiextensionsv1beta1.AddToScheme(genericScheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(genericScheme))
}

type UpdateSubmarinerConfigStatusFunc func(status *configv1alpha1.SubmarinerConfigStatus) error

func UpdateSubmarinerConfigStatus(
	client configclient.Interface,
	namespace, name string,
	updateFuncs ...UpdateSubmarinerConfigStatusFunc) (*configv1alpha1.SubmarinerConfigStatus, bool, error) {
	updated := false
	var updatedStatus *configv1alpha1.SubmarinerConfigStatus

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		config, err := client.SubmarineraddonV1alpha1().SubmarinerConfigs(namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return err
		}
		oldStatus := &config.Status

		newStatus := oldStatus.DeepCopy()
		for _, update := range updateFuncs {
			if err := update(newStatus); err != nil {
				return err
			}
		}
		if equality.Semantic.DeepEqual(oldStatus, newStatus) {
			// We return the newStatus which is a deep copy of oldStatus but with all update funcs applied.
			updatedStatus = newStatus
			return nil
		}

		config.Status = *newStatus
		updatedConfig, err := client.SubmarineraddonV1alpha1().SubmarinerConfigs(namespace).UpdateStatus(context.TODO(), config, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		updatedStatus = &updatedConfig.Status
		updated = err == nil
		return err
	})

	return updatedStatus, updated, err
}

func UpdateSubmarinerConfigConditionFn(cond metav1.Condition) UpdateSubmarinerConfigStatusFunc {
	return func(oldStatus *configv1alpha1.SubmarinerConfigStatus) error {
		meta.SetStatusCondition(&oldStatus.Conditions, cond)
		return nil
	}
}

func UpdateSubmarinerConfigStatusFn(cond metav1.Condition, managedClusterInfo configv1alpha1.ManagedClusterInfo) UpdateSubmarinerConfigStatusFunc {
	return func(oldStatus *configv1alpha1.SubmarinerConfigStatus) error {
		oldStatus.ManagedClusterInfo = managedClusterInfo
		meta.SetStatusCondition(&oldStatus.Conditions, cond)
		return nil
	}
}

type UpdateManagedClusterAddOnStatusFunc func(status *addonv1alpha1.ManagedClusterAddOnStatus) error

func UpdateManagedClusterAddOnStatus(ctx context.Context, client addonclient.Interface, addOnNamespace string,
	updateFuncs ...UpdateManagedClusterAddOnStatusFunc) (*addonv1alpha1.ManagedClusterAddOnStatus, bool, error) {
	updated := false
	var updatedAddOnStatus *addonv1alpha1.ManagedClusterAddOnStatus

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		addOn, err := client.AddonV1alpha1().ManagedClusterAddOns(addOnNamespace).Get(ctx, SubmarinerAddOnName, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return nil
		}

		if err != nil {
			return err
		}
		oldStatus := &addOn.Status

		newStatus := oldStatus.DeepCopy()
		for _, update := range updateFuncs {
			if err := update(newStatus); err != nil {
				return err
			}
		}
		if equality.Semantic.DeepEqual(oldStatus, newStatus) {
			// We return the newStatus which is a deep copy of oldStatus but with all update funcs applied.
			updatedAddOnStatus = newStatus
			return nil
		}

		addOn.Status = *newStatus
		updatedAddOn, err := client.AddonV1alpha1().ManagedClusterAddOns(addOnNamespace).UpdateStatus(ctx, addOn, metav1.UpdateOptions{})
		if err != nil {
			return err
		}
		updatedAddOnStatus = &updatedAddOn.Status
		updated = err == nil
		return err
	})

	return updatedAddOnStatus, updated, err
}

func UpdateManagedClusterAddOnStatusFn(cond metav1.Condition) UpdateManagedClusterAddOnStatusFunc {
	return func(oldStatus *addonv1alpha1.ManagedClusterAddOnStatus) error {
		meta.SetStatusCondition(&oldStatus.Conditions, cond)
		return nil
	}
}

type CRDClientHolder struct {
	apiExtensionsClient apiextensionsclient.Interface
}

func NewCRDClientHolder() *CRDClientHolder {
	return &CRDClientHolder{}
}
func (c *CRDClientHolder) WithAPIExtensionsClient(client apiextensionsclient.Interface) *CRDClientHolder {
	c.apiExtensionsClient = client
	return c
}

// ApplyCRDDirectly is used to apply CRD v1 and v1beta1 since resourceapply in library-go cannot apply CRD v1 with error
func ApplyCRDDirectly(
	clients *CRDClientHolder,
	recorder events.Recorder,
	manifests resourceapply.AssetFunc,
	files ...string) []resourceapply.ApplyResult {
	ret := []resourceapply.ApplyResult{}

	for _, file := range files {
		result := resourceapply.ApplyResult{File: file}
		objBytes, err := manifests(file)
		if err != nil {
			result.Error = fmt.Errorf("missing %q: %v", file, err)
			ret = append(ret, result)
			continue
		}
		requiredObj, _, err := genericCodec.Decode(objBytes, nil, nil)
		if err != nil {
			result.Error = fmt.Errorf("cannot decode %q: %v", file, err)
			ret = append(ret, result)
			continue
		}
		result.Type = fmt.Sprintf("%T", requiredObj)

		// NOTE: Do not add CR resources into this switch otherwise the protobuf client can cause problems.
		switch t := requiredObj.(type) {
		case *apiextensionsv1beta1.CustomResourceDefinition:
			if clients.apiExtensionsClient == nil {
				result.Error = fmt.Errorf("missing apiExtensionsClient")
			}
			result.Result, result.Changed, result.Error = resourceapply.ApplyCustomResourceDefinitionV1Beta1(clients.apiExtensionsClient.ApiextensionsV1beta1(), recorder, t)
		case *apiextensionsv1.CustomResourceDefinition:
			if clients.apiExtensionsClient == nil {
				result.Error = fmt.Errorf("missing apiExtensionsClient")
			}
			result.Result, result.Changed, result.Error = resourceapply.ApplyCustomResourceDefinitionV1(clients.apiExtensionsClient.ApiextensionsV1(), recorder, t)

		default:
			result.Error = fmt.Errorf("unhandled type %T", requiredObj)
		}

		ret = append(ret, result)
	}

	return ret
}

// CleanUpSubmarinerManifests clean up submariner resources from its manifest files
func CleanUpSubmarinerManifests(
	ctx context.Context,
	client kubernetes.Interface,
	recorder events.Recorder,
	assetFunc resourceapply.AssetFunc,
	files ...string) error {
	errs := []error{}
	for _, file := range files {
		objectRaw, err := assetFunc(file)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		object, _, err := genericCodec.Decode(objectRaw, nil, nil)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		switch t := object.(type) {
		case *corev1.Namespace:
			err = client.CoreV1().Namespaces().Delete(ctx, t.Name, metav1.DeleteOptions{})
		case *rbacv1.Role:
			err = client.RbacV1().Roles(t.Namespace).Delete(ctx, t.Name, metav1.DeleteOptions{})
		case *rbacv1.RoleBinding:
			err = client.RbacV1().RoleBindings(t.Namespace).Delete(ctx, t.Name, metav1.DeleteOptions{})
		case *corev1.ServiceAccount:
			err = client.CoreV1().ServiceAccounts(t.Namespace).Delete(ctx, t.Name, metav1.DeleteOptions{})
		default:
			err = fmt.Errorf("unhandled type %T", object)
		}
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}

		gvk := resourcehelper.GuessObjectGroupVersionKind(object)
		recorder.Eventf(fmt.Sprintf("Submariner%sDeleted", gvk.Kind), "Deleted %s", resourcehelper.FormatResourceForCLIWithNamespace(object))
	}
	return errorhelpers.NewMultiLineAggregate(errs)
}

func GenerateIPSecPSKSecret(client kubernetes.Interface, brokerNamespace string) error {
	_, err := client.CoreV1().Secrets(brokerNamespace).Get(context.TODO(), IPSecPSKSecretName, metav1.GetOptions{})
	switch {
	case errors.IsNotFound(err):
		psk := make([]byte, IPSecPSKSecretLength)
		if _, err := rand.Read(psk); err != nil {
			return err
		}
		pskSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: IPSecPSKSecretName,
			},
			Data: map[string][]byte{
				"psk": psk,
			},
		}
		_, err := client.CoreV1().Secrets(brokerNamespace).Create(context.TODO(), pskSecret, metav1.CreateOptions{})
		return err
	case err != nil:
		return err
	}
	return nil
}

func GetIPSecPSK(client kubernetes.Interface, brokerNamespace string) (string, error) {
	secret, err := client.CoreV1().Secrets(brokerNamespace).Get(context.TODO(), IPSecPSKSecretName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get broker IPSECPSK secret %v/%v: %v", brokerNamespace, IPSecPSKSecretName, err)
	}

	return base64.StdEncoding.EncodeToString(secret.Data["psk"]), nil
}

func GetBrokerAPIServer(dynamicClient dynamic.Interface) (string, error) {
	infrastructureConfig, err := dynamicClient.Resource(infrastructureGVR).Get(context.TODO(), ocpInfrastructureName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			apiServer := os.Getenv(BrokerAPIServer)
			if apiServer == "" {
				return "", fmt.Errorf("failed to get apiserver in env %v", BrokerAPIServer)
			}
			return apiServer, nil
		}
		return "", fmt.Errorf("failed to get infrastructures cluster: %v", err)
	}

	apiServer, found, err := unstructured.NestedString(infrastructureConfig.Object, "status", "apiServerURL")
	if err != nil || !found {
		return "", fmt.Errorf("failed to get apiServerURL in infrastructures cluster:%v,%v", found, err)
	}

	return strings.Trim(apiServer, "https://"), nil
}

func GetBrokerTokenAndCA(kubeClient kubernetes.Interface, dynamicClient dynamic.Interface, brokerNS, clusterName string) (token, ca string, err error) {
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

func GetClusterProduct(managedCluster *clusterv1.ManagedCluster) string {
	for _, claim := range managedCluster.Status.ClusterClaims {
		if claim.Name == "product.open-cluster-management.io" {
			return claim.Value
		}
	}

	return ""
}

func GetEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

func ApplyManifestWork(ctx context.Context, client workclient.Interface, required *workv1.ManifestWork, recorder events.Recorder) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		existing, err := client.WorkV1().ManifestWorks(required.Namespace).Get(ctx, required.Name, metav1.GetOptions{})
		switch {
		case errors.IsNotFound(err):
			_, err := client.WorkV1().ManifestWorks(required.Namespace).Create(ctx, required, metav1.CreateOptions{})
			if err != nil {
				return err
			}

			recorder.Event("ManifestWorkApplied", fmt.Sprintf("manifestwork %s/%s was created", required.Namespace, required.Name))
			return nil
		case err != nil:
			return err
		}

		if manifestsEqual(existing.Spec.Workload.Manifests, required.Spec.Workload.Manifests) {
			return nil
		}

		existingCopy := existing.DeepCopy()
		existingCopy.Spec = required.Spec
		_, err = client.WorkV1().ManifestWorks(required.Namespace).Update(ctx, existingCopy, metav1.UpdateOptions{})
		if err != nil {
			return err
		}

		recorder.Event("ManifestWorkApplied", fmt.Sprintf("manifestwork %s/%s was updated", required.Namespace, required.Name))
		return nil
	})
}

func GetManagedClusterInfo(managedCluster *clusterv1.ManagedCluster) configv1alpha1.ManagedClusterInfo {
	clusterInfo := configv1alpha1.ManagedClusterInfo{
		ClusterName: managedCluster.Name,
	}
	for _, claim := range managedCluster.Status.ClusterClaims {
		if claim.Name == "product.open-cluster-management.io" {
			clusterInfo.Vendor = claim.Value
		}
		if claim.Name == "platform.open-cluster-management.io" {
			clusterInfo.Platform = claim.Value
		}
		if claim.Name == "region.open-cluster-management.io" {
			clusterInfo.Region = claim.Value
		}
		if claim.Name == "infrastructure.openshift.io" {
			var infraInfo map[string]interface{}
			if err := json.Unmarshal([]byte(claim.Value), &infraInfo); err == nil {
				clusterInfo.InfraId = fmt.Sprintf("%v", infraInfo["infraName"])
			}
		}
	}
	return clusterInfo
}

func manifestsEqual(new, old []workv1.Manifest) bool {
	if len(new) != len(old) {
		return false
	}

	for i := range new {
		if !equality.Semantic.DeepEqual(new[i].Raw, old[i].Raw) {
			return false
		}
	}
	return true
}

func getKubeAPIServerCA(kubeClient kubernetes.Interface, dynamicClient dynamic.Interface) ([]byte, error) {
	kubeAPIServer, err := GetBrokerAPIServer(dynamicClient)
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

	apiServer := &apicofigv1.APIServer{}
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

// GetCurrentNamespace returns the current namesapce from file system,
// if the namespace is not found, it returns the defaultNamespace
func GetCurrentNamespace(defaultNamespace string) string {
	nsBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return defaultNamespace
	}
	return string(nsBytes)
}

func GernerateBrokerName(clusterSetName string) string {
	name := fmt.Sprintf("%s-%s", clusterSetName, brokerSuffix)
	if len(name) > namespaceMaxLength {
		truncatedClusterSetName := clusterSetName[(len(brokerSuffix) - 1):]
		return fmt.Sprintf("%s-%s", truncatedClusterSetName, brokerSuffix)
	}
	return name
}

func RemoveConfigFinalizer(ctx context.Context, configClient configclient.Interface, config *configv1alpha1.SubmarinerConfig, finalizer string) error {
	copiedFinalizers := []string{}
	for i := range config.Finalizers {
		if config.Finalizers[i] == finalizer {
			continue
		}
		copiedFinalizers = append(copiedFinalizers, config.Finalizers[i])
	}

	if len(config.Finalizers) != len(copiedFinalizers) {
		copied := config.DeepCopy()
		copied.Finalizers = copiedFinalizers
		_, err := configClient.SubmarineraddonV1alpha1().SubmarinerConfigs(copied.Namespace).Update(ctx, copied, metav1.UpdateOptions{})
		return err
	}

	return nil
}

// RemoveAddOnFinalizer removes the addon finalizer from a submariner-addon
func RemoveAddOnFinalizer(ctx context.Context, addOnClient addonclient.Interface, addOn *addonv1alpha1.ManagedClusterAddOn, finalizer string) error {
	copiedFinalizers := []string{}
	for i := range addOn.Finalizers {
		if addOn.Finalizers[i] == finalizer {
			continue
		}
		copiedFinalizers = append(copiedFinalizers, addOn.Finalizers[i])
	}

	if len(addOn.Finalizers) != len(copiedFinalizers) {
		copied := addOn.DeepCopy()
		copied.Finalizers = copiedFinalizers
		_, err := addOnClient.AddonV1alpha1().ManagedClusterAddOns(copied.Namespace).Update(ctx, copied, metav1.UpdateOptions{})
		return err
	}

	return nil
}
