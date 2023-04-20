package manifestwork

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/submariner-io/admiral/pkg/log"
	"github.com/submariner-io/admiral/pkg/resource"
	"github.com/submariner-io/admiral/pkg/util"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	workclient "open-cluster-management.io/api/client/work/clientset/versioned"
	workv1 "open-cluster-management.io/api/work/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var logger = log.Logger{Logger: logf.Log.WithName("ManifestWork")}

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
		logger.Infof("Created ManifestWork \"%s/%s\": %s", toApply.Namespace, toApply.Name, manifestsToString(toApply.Spec.Workload.Manifests))
	} else if result == util.OperationResultUpdated {
		recorder.Event("ManifestWorkApplied", fmt.Sprintf("manifestwork %s/%s was updated", toApply.Namespace, toApply.Name))
		logger.Infof("Updated ManifestWork \"%s/%s\"", toApply.Namespace, toApply.Name)
	}

	return err
}

func manifestsToString(manifests []workv1.Manifest) string {
	var out bytes.Buffer

	for i := range manifests {
		out.WriteByte('\n')
		_ = json.Indent(&out, manifests[i].Raw, "", "  ")
	}

	return out.String()
}
