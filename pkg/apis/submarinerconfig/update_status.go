package submarinerconfig

import (
	"context"

	"github.com/pkg/errors"
	configv1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	configclient "github.com/stolostron/submariner-addon/pkg/client/submarinerconfig/clientset/versioned/typed/submarinerconfig/v1alpha1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

type UpdateStatusFunc func(status *configv1alpha1.SubmarinerConfigStatus)

func UpdateStatus(ctx context.Context, client configclient.SubmarinerConfigInterface, name string,
	updateFuncs ...UpdateStatusFunc,
) (*configv1alpha1.SubmarinerConfigStatus, bool, error) {
	updated := false
	var updatedStatus *configv1alpha1.SubmarinerConfigStatus

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		config, err := client.Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}

		if err != nil {
			return errors.Wrapf(err, "error retrieving SubmarinerConfig %q", name)
		}

		oldStatus := &config.Status

		newStatus := oldStatus.DeepCopy()
		for _, update := range updateFuncs {
			update(newStatus)
		}

		if equality.Semantic.DeepEqual(oldStatus, newStatus) {
			// We return the newStatus which is a deep copy of oldStatus but with all update funcs applied.
			updatedStatus = newStatus

			return nil
		}

		config.Status = *newStatus
		updatedConfig, err := client.UpdateStatus(ctx, config, metav1.UpdateOptions{})
		if err != nil {
			return errors.Wrapf(err, "error updating status SubmarinerConfig %q", name)
		}

		updatedStatus = &updatedConfig.Status
		updated = true

		return nil
	})

	return updatedStatus, updated, err //nolint:wrapcheck // No need to wrap here
}

func UpdateConditionFn(cond *metav1.Condition) UpdateStatusFunc {
	return func(oldStatus *configv1alpha1.SubmarinerConfigStatus) {
		meta.SetStatusCondition(&oldStatus.Conditions, *cond)
	}
}

func UpdateStatusFn(cond *metav1.Condition,
	managedClusterInfo *configv1alpha1.ManagedClusterInfo,
) UpdateStatusFunc {
	return func(oldStatus *configv1alpha1.SubmarinerConfigStatus) {
		oldStatus.ManagedClusterInfo = *managedClusterInfo

		if cond != nil {
			meta.SetStatusCondition(&oldStatus.Conditions, *cond)
		}
	}
}
