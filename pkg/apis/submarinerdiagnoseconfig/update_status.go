package submarinerdiagnoseconfig

import (
	"context"

	diagnosev1alpha1 "github.com/stolostron/submariner-addon/pkg/apis/submarinerdiagnoseconfig/v1alpha1"
	diagnoseclient "github.com/stolostron/submariner-addon/pkg/client/submarinerdiagnoseconfig/clientset/versioned/typed/submarinerdiagnoseconfig/v1alpha1" //nolint
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/retry"
)

type UpdateStatusFunc func(status *diagnosev1alpha1.SubmarinerDiagnoseStatus)

func UpdateStatus(ctx context.Context, client diagnoseclient.SubmarinerDiagnoseConfigInterface, name string,
	updateFuncs ...UpdateStatusFunc,
) (*diagnosev1alpha1.SubmarinerDiagnoseStatus, bool, error) {
	updated := false
	var updatedStatus *diagnosev1alpha1.SubmarinerDiagnoseStatus

	err := retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		config, err := client.Get(ctx, name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			return nil
		}

		if err != nil {
			return err
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
			return err
		}

		updatedStatus = &updatedConfig.Status
		updated = err == nil

		return err
	})

	return updatedStatus, updated, err
}

func UpdateConditionFn(cond *metav1.Condition) UpdateStatusFunc {
	return func(oldStatus *diagnosev1alpha1.SubmarinerDiagnoseStatus) {
		meta.SetStatusCondition(&oldStatus.Conditions, *cond)
	}
}
