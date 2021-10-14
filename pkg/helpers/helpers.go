package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	configclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcehelper"
	errorhelpers "github.com/openshift/library-go/pkg/operator/v1helpers"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"
	clusterv1 "open-cluster-management.io/api/cluster/v1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
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
	IPSecPSKSecretName = "submariner-ipsec-psk"
)

const (
	SubmarinerNatTPort          = 4500
	SubmarinerNatTDiscoveryPort = 4900
	SubmarinerRoutePort         = 4800
	SubmarinerMetricsPort       = 8080
)

const (
	brokerSuffix       = "broker"
	namespaceMaxLength = 63
)

var (
	genericScheme = runtime.NewScheme()
	genericCodecs = serializer.NewCodecFactory(genericScheme)
	genericCodec  = genericCodecs.UniversalDeserializer()
)

func init() {
	utilruntime.Must(appsv1.AddToScheme(genericScheme))
	utilruntime.Must(corev1.AddToScheme(genericScheme))
	utilruntime.Must(rbacv1.AddToScheme(genericScheme))
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
		updatedConfig, err := client.SubmarineraddonV1alpha1().SubmarinerConfigs(namespace).UpdateStatus(context.TODO(),
			config, metav1.UpdateOptions{})
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

func UpdateSubmarinerConfigStatusFn(cond *metav1.Condition,
	managedClusterInfo configv1alpha1.ManagedClusterInfo) UpdateSubmarinerConfigStatusFunc {
	return func(oldStatus *configv1alpha1.SubmarinerConfigStatus) error {
		oldStatus.ManagedClusterInfo = managedClusterInfo

		if cond != nil {
			meta.SetStatusCondition(&oldStatus.Conditions, *cond)
		}

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

// GetCurrentNamespace returns the current namesapce from file system,
// if the namespace is not found, it returns the defaultNamespace
func GetCurrentNamespace(defaultNamespace string) string {
	nsBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		return defaultNamespace
	}

	return string(nsBytes)
}

func GenerateBrokerName(clusterSetName string) string {
	name := fmt.Sprintf("%s-%s", clusterSetName, brokerSuffix)
	if len(name) > namespaceMaxLength {
		truncatedClusterSetName := clusterSetName[(len(brokerSuffix) - 1):]

		return fmt.Sprintf("%s-%s", truncatedClusterSetName, brokerSuffix)
	}

	return name
}

func RemoveConfigFinalizer(ctx context.Context, configClient configclient.Interface, config *configv1alpha1.SubmarinerConfig,
	finalizer string) error {
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
func RemoveAddOnFinalizer(ctx context.Context, addOnClient addonclient.Interface, addOn *addonv1alpha1.ManagedClusterAddOn,
	finalizer string) error {
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
