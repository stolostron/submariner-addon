package addon

import (
	"context"

	"github.com/stolostron/submariner-addon/pkg/constants"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"
)

type UpdateStatusFunc func(status *addonv1alpha1.ManagedClusterAddOnStatus) error

func UpdateStatus(ctx context.Context, client addonclient.Interface, addOnNamespace string,
	updateFuncs ...UpdateStatusFunc,
) (*addonv1alpha1.ManagedClusterAddOnStatus, bool, error) {
	updated := false
	var updatedAddOnStatus *addonv1alpha1.ManagedClusterAddOnStatus

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		addOn, err := client.AddonV1alpha1().ManagedClusterAddOns(addOnNamespace).Get(ctx, constants.SubmarinerAddOnName,
			metav1.GetOptions{})
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

func UpdateConditionFn(cond *metav1.Condition) UpdateStatusFunc {
	return func(oldStatus *addonv1alpha1.ManagedClusterAddOnStatus) error {
		meta.SetStatusCondition(&oldStatus.Conditions, *cond)

		return nil
	}
}
