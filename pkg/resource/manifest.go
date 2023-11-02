package resource

import (
	"context"
	"embed"
	"fmt"

	"github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcehelper"
	operatorhelpers "github.com/openshift/library-go/pkg/operator/v1helpers"
	"github.com/stolostron/submariner-addon/pkg/redact"
	"github.com/submariner-io/admiral/pkg/log"
	"github.com/submariner-io/admiral/pkg/resource"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var logger = log.Logger{Logger: logf.Log.WithName("Resource")}

var (
	genericScheme = runtime.NewScheme()
	genericCodec  = serializer.NewCodecFactory(genericScheme).UniversalDeserializer()
)

func init() {
	utilruntime.Must(appsv1.AddToScheme(genericScheme))
	utilruntime.Must(corev1.AddToScheme(genericScheme))
	utilruntime.Must(rbacv1.AddToScheme(genericScheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(genericScheme))
}

func ApplyManifests(ctx context.Context, kubeClient kubernetes.Interface, recorder events.Recorder,
	cache resourceapply.ResourceCache, assetFunc resourceapply.AssetFunc, files ...string,
) error {
	applyResults := resourceapply.ApplyDirectly(ctx, resourceapply.NewKubeClientHolder(kubeClient), recorder, cache,
		assetFunc, files...)

	errs := []error{}

	for _, result := range applyResults {
		if result.Error != nil {
			errs = append(errs, fmt.Errorf("error applying %q (%T): %w", result.File, result.Type, result.Error))
		} else if result.Changed {
			logger.Infof("%s from file %q created/updated: %s", result.Type, result.File, redact.JSON(resource.ToJSON(result.Result)))
		}
	}

	return operatorhelpers.NewMultiLineAggregate(errs)
}

func DeleteFromManifests(ctx context.Context, kubeClient kubernetes.Interface, recorder events.Recorder, assetFunc resourceapply.AssetFunc,
	files ...string,
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

		switch t := object.(type) {
		case *corev1.Namespace:
			err = kubeClient.CoreV1().Namespaces().Delete(ctx, t.Name, metav1.DeleteOptions{})
		case *rbacv1.Role:
			err = kubeClient.RbacV1().Roles(t.Namespace).Delete(ctx, t.Name, metav1.DeleteOptions{})
		case *rbacv1.RoleBinding:
			err = kubeClient.RbacV1().RoleBindings(t.Namespace).Delete(ctx, t.Name, metav1.DeleteOptions{})
		case *corev1.ServiceAccount:
			err = kubeClient.CoreV1().ServiceAccounts(t.Namespace).Delete(ctx, t.Name, metav1.DeleteOptions{})
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
		recorder.Eventf(fmt.Sprintf("Submariner%sDeleted", gvk.Kind), "Deleted %s",
			resourcehelper.FormatResourceForCLIWithNamespace(object))
		logger.Infof("Deleted %s %q", gvk.Kind, resource.MustToMeta(object).GetName())
	}

	return operatorhelpers.NewMultiLineAggregate(errs)
}

func AssetFromFile(manifestFiles embed.FS, config interface{}) resourceapply.AssetFunc {
	return func(name string) ([]byte, error) {
		template, err := manifestFiles.ReadFile(name)
		if err != nil {
			return nil, err
		}

		return assets.MustCreateAssetFromTemplate(name, template, config).Data, nil
	}
}
