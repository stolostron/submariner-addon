package helpers

import (
	"context"
	"io/ioutil"
	"os"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"
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

func UpdateManagedClusterAddOnStatusFn(cond *metav1.Condition) UpdateManagedClusterAddOnStatusFunc {
	return func(oldStatus *addonv1alpha1.ManagedClusterAddOnStatus) error {
		meta.SetStatusCondition(&oldStatus.Conditions, *cond)

		return nil
	}
}

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
