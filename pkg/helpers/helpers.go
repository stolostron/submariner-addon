package helpers

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"

	workclient "github.com/open-cluster-management/api/client/work/clientset/versioned"
	clusterv1 "github.com/open-cluster-management/api/cluster/v1"
	workv1 "github.com/open-cluster-management/api/work/v1"
	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	configclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"

	"github.com/openshift/api"
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
	ClusterTypeOCP       = "OCP"
	IPSecPSKSecretLength = 48
	IPSecPSKSecretName   = "submariner-ipsec-psk"
	BrokerAPIServer      = "BROKER_API_SERVER"
)

const (
	SubmarinerIKEPort     = 500
	SubmarinerNatTPort    = 4500
	SubmarinerRoutePort   = 4800
	SubmarinerMetricsPort = 8080
)

const ocpInfrastructureName = "cluster"

var infrastructureGVR schema.GroupVersionResource = schema.GroupVersionResource{
	Group:    "config.openshift.io",
	Version:  "v1",
	Resource: "infrastructures",
}

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

func GetBrokerTokenAndCA(client kubernetes.Interface, brokerNS, clusterName string) (token, ca string, err error) {
	sa, err := client.CoreV1().ServiceAccounts(brokerNS).Get(context.TODO(), clusterName, metav1.GetOptions{})
	if err != nil {
		return "", "", fmt.Errorf("failed to get agent ServiceAccount %v/%v: %v", brokerNS, clusterName, err)
	}
	if len(sa.Secrets) < 1 {
		return "", "", fmt.Errorf("ServiceAccount %v does not have any secret", sa.Name)
	}

	brokerTokenPrefix := fmt.Sprintf("%s-token-", clusterName)
	for _, secret := range sa.Secrets {
		if strings.HasPrefix(secret.Name, brokerTokenPrefix) {
			tokenSecret, err := client.CoreV1().Secrets(brokerNS).Get(context.TODO(), secret.Name, metav1.GetOptions{})
			if err != nil {
				return "", "", fmt.Errorf("failed to get secret %v of aget ServiceAccount %v/%v: %v", secret.Name, brokerNS, clusterName, err)
			}

			if tokenSecret.Type == corev1.SecretTypeServiceAccountToken {
				ca := base64.StdEncoding.EncodeToString(tokenSecret.Data["ca.crt"])
				return string(tokenSecret.Data["token"]), ca, nil
			}
		}
	}

	return "", "", fmt.Errorf("ServiceAccount %v/%v does not have a secret of type token", brokerNS, clusterName)

}

func GetClusterType(managedCluster *clusterv1.ManagedCluster) string {
	if clusterType, found := managedCluster.GetLabels()["vendor"]; found {
		switch clusterType {
		case "OCP", "OpenShift":
			return ClusterTypeOCP
		default:
			return clusterType
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

func ApplyManifestWork(ctx context.Context, client workclient.Interface, required *workv1.ManifestWork) error {
	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		existing, err := client.WorkV1().ManifestWorks(required.Namespace).Get(ctx, required.Name, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				_, err = client.WorkV1().ManifestWorks(required.Namespace).Create(ctx, required, metav1.CreateOptions{})
			}
			return err
		}

		if !manifestsEqual(existing.Spec.Workload.Manifests, required.Spec.Workload.Manifests) {
			existingCopy := existing.DeepCopy()
			existingCopy.Spec = required.Spec
			_, err = client.WorkV1().ManifestWorks(required.Namespace).Update(ctx, existingCopy, metav1.UpdateOptions{})
		}

		return err
	})

	return err
}

func GetManagedClusterInfo(managedCluster *clusterv1.ManagedCluster) configv1alpha1.ManagedClusterInfo {
	clusterInfo := configv1alpha1.ManagedClusterInfo{
		ClusterName: managedCluster.Name,
		Vendor:      GetClusterType(managedCluster),
	}
	for _, claim := range managedCluster.Status.ClusterClaims {
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
