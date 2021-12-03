package resource

import (
	"context"
	"fmt"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	operatorhelpers "github.com/openshift/library-go/pkg/operator/v1helpers"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsClient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
)

func ApplyCRDs(client apiextensionsClient.Interface, recorder events.Recorder, assetFunc resourceapply.AssetFunc, files ...string) error {
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
			_, _, err = resourceapply.ApplyCustomResourceDefinitionV1(context.TODO(), client.ApiextensionsV1(), recorder, t)
		default:
			err = fmt.Errorf("unhandled type %T", object)
		}

		if err != nil {
			errs = append(errs, err)
		}
	}

	return operatorhelpers.NewMultiLineAggregate(errs)
}
