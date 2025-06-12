package resource

import (
	"context"
	goerrors "errors"
	"fmt"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/pkg/errors"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsClient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func ApplyCRDs(ctx context.Context, client apiextensionsClient.Interface, recorder events.Recorder, ownerRef *metav1.OwnerReference,
	assetFunc resourceapply.AssetFunc, files ...string,
) error {
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

		// NOTE: Do not add CR resources into this switch otherwise the protobuf client can cause problems.
		switch t := object.(type) {
		case *apiextensionsv1.CustomResourceDefinition:
			if ownerRef != nil {
				t.OwnerReferences = []metav1.OwnerReference{*ownerRef}
			}

			_, _, err = resourceapply.ApplyCustomResourceDefinitionV1(ctx, client.ApiextensionsV1(), recorder, t)
		default:
			err = fmt.Errorf("unhandled type %T", object)
		}

		if err != nil {
			errs = append(errs, err)
		}
	}

	return errors.Wrap(goerrors.Join(errs...), "error apply CRDs")
}
