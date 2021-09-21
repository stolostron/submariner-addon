package finalizer

import (
	"context"

	"github.com/submariner-io/admiral/pkg/resource"
	"github.com/submariner-io/admiral/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func Add(ctx context.Context, client resource.Interface, obj runtime.Object, finalizerName string) (bool, error) {
	objMeta := resource.ToMeta(obj)
	if !objMeta.GetDeletionTimestamp().IsZero() {
		return false, nil
	}

	if find(objMeta, finalizerName) {
		return false, nil
	}

	err := util.Update(ctx, client, obj, func(existing runtime.Object) (runtime.Object, error) {
		objMeta := resource.ToMeta(existing)
		objMeta.SetFinalizers(append(objMeta.GetFinalizers(), finalizerName))

		return existing, nil
	})

	return err == nil, err
}

func Remove(ctx context.Context, client resource.Interface, obj runtime.Object, finalizerName string) error {
	if !find(resource.ToMeta(obj), finalizerName) {
		return nil
	}

	return util.Update(ctx, client, obj, func(existing runtime.Object) (runtime.Object, error) {
		objMeta := resource.ToMeta(existing)

		newFinalizers := []string{}
		for _, f := range objMeta.GetFinalizers() {
			if f == finalizerName {
				continue
			}

			newFinalizers = append(newFinalizers, f)
		}

		objMeta.SetFinalizers(newFinalizers)
		return existing, nil
	})
}

func find(objMeta metav1.Object, finalizerName string) bool {
	for _, f := range objMeta.GetFinalizers() {
		if f == finalizerName {
			return true
		}
	}

	return false
}
