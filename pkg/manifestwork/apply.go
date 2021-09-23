package manifestwork

import (
	"context"
	"fmt"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/submariner-io/admiral/pkg/resource"
	"github.com/submariner-io/admiral/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	workclient "open-cluster-management.io/api/client/work/clientset/versioned"
	workv1 "open-cluster-management.io/api/work/v1"
)

func Apply(ctx context.Context, client workclient.Interface, toApply *workv1.ManifestWork, recorder events.Recorder) error {
	resourceInterface := &resource.InterfaceFuncs{
		GetFunc: func(ctx context.Context, name string, options metav1.GetOptions) (runtime.Object, error) {
			return client.WorkV1().ManifestWorks(toApply.Namespace).Get(ctx, toApply.Name, options)
		},
		CreateFunc: func(ctx context.Context, obj runtime.Object, options metav1.CreateOptions) (runtime.Object, error) {
			return client.WorkV1().ManifestWorks(toApply.Namespace).Create(ctx, obj.(*workv1.ManifestWork), options)
		},
		UpdateFunc: func(ctx context.Context, obj runtime.Object, options metav1.UpdateOptions) (runtime.Object, error) {
			return client.WorkV1().ManifestWorks(toApply.Namespace).Update(ctx, obj.(*workv1.ManifestWork), options)
		},
	}

	result, err := util.CreateOrUpdate(ctx, resourceInterface, toApply, func(existing runtime.Object) (runtime.Object, error) {
		existing.(*workv1.ManifestWork).Spec = toApply.Spec
		return existing, nil
	})

	if result == util.OperationResultCreated {
		recorder.Event("ManifestWorkApplied", fmt.Sprintf("manifestwork %s/%s was created", toApply.Namespace, toApply.Name))
	} else if result == util.OperationResultUpdated {
		recorder.Event("ManifestWorkApplied", fmt.Sprintf("manifestwork %s/%s was updated", toApply.Namespace, toApply.Name))
	}

	return err
}
