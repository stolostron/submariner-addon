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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
)

var (
	genericScheme = runtime.NewScheme()
	genericCodec  = serializer.NewCodecFactory(genericScheme).UniversalDeserializer()
)

func init() {
	utilruntime.Must(appsv1.AddToScheme(genericScheme))
	utilruntime.Must(corev1.AddToScheme(genericScheme))
	utilruntime.Must(rbacv1.AddToScheme(genericScheme))
	utilruntime.Must(apiextensionsv1beta1.AddToScheme(genericScheme))
	utilruntime.Must(apiextensionsv1.AddToScheme(genericScheme))
}

func ApplyManifests(kubeClient kubernetes.Interface, recorder events.Recorder, assetFunc resourceapply.AssetFunc, files ...string) error {
	applyResults := resourceapply.ApplyDirectly(resourceapply.NewKubeClientHolder(kubeClient), recorder, assetFunc, files...)

	errs := []error{}

	for _, result := range applyResults {
		if result.Error != nil {
			errs = append(errs, fmt.Errorf("error applying %q (%T): %v", result.File, result.Type, result.Error))
		}
	}

	return operatorhelpers.NewMultiLineAggregate(errs)
}

func DeleteFromManifests(kubeClient kubernetes.Interface, recorder events.Recorder, assetFunc resourceapply.AssetFunc,
	files ...string) error {
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
			err = kubeClient.CoreV1().Namespaces().Delete(context.TODO(), t.Name, metav1.DeleteOptions{})
		case *rbacv1.Role:
			err = kubeClient.RbacV1().Roles(t.Namespace).Delete(context.TODO(), t.Name, metav1.DeleteOptions{})
		case *rbacv1.RoleBinding:
			err = kubeClient.RbacV1().RoleBindings(t.Namespace).Delete(context.TODO(), t.Name, metav1.DeleteOptions{})
		case *corev1.ServiceAccount:
			err = kubeClient.CoreV1().ServiceAccounts(t.Namespace).Delete(context.TODO(), t.Name, metav1.DeleteOptions{})
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
