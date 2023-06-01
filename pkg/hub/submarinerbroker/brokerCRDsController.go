package submarinerbroker

import (
	"context"

	"github.com/aws/smithy-go/ptr"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/stolostron/submariner-addon/pkg/resource"
	"github.com/submariner-io/submariner-operator/pkg/embeddedyamls"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
)

const (
	ConfigCRDName = "submarinerconfigs.submarineraddon.open-cluster-management.io"
)

var staticCRDFiles = []string{
	embeddedyamls.Deploy_submariner_crds_submariner_io_clusters_yaml,
	embeddedyamls.Deploy_crds_submariner_io_brokers_yaml,
	embeddedyamls.Deploy_submariner_crds_submariner_io_endpoints_yaml,
	embeddedyamls.Deploy_submariner_crds_submariner_io_gateways_yaml,
	embeddedyamls.Deploy_mcsapi_crds_multicluster_x_k8s_io_serviceimports_yaml,
}

type submarinerBrokerCRDsController struct {
	crdClient     apiextensionsclientset.Interface
	eventRecorder events.Recorder
}

func NewCRDsController(
	crdClient apiextensionsclientset.Interface,
	crdInformer apiextensionsinformers.CustomResourceDefinitionInformer,
	recorder events.Recorder,
) factory.Controller {
	c := &submarinerBrokerCRDsController{
		crdClient:     crdClient,
		eventRecorder: recorder.WithComponentSuffix("submariner-broker-crds-controller"),
	}

	return factory.New().
		WithInformersQueueKeyFunc(func(obj runtime.Object) string {
			accessor, _ := meta.Accessor(obj)

			return accessor.GetName()
		}, crdInformer.Informer()).
		WithSync(c.sync).
		ToController("SubmarinerBrokerCRDsController", recorder)
}

func (c *submarinerBrokerCRDsController) sync(ctx context.Context, syncCtx factory.SyncContext) error {
	crdName := syncCtx.QueueKey()
	klog.V(4).Infof("Reconciling ConfigCRD %q", crdName)

	if crdName != ConfigCRDName {
		return nil
	}

	configCRD, err := c.crdClient.ApiextensionsV1().CustomResourceDefinitions().Get(ctx, crdName, v1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		return err
	}

	ownerRef := &v1.OwnerReference{
		APIVersion:         apiextensionsv1.SchemeGroupVersion.String(),
		Kind:               "CustomResourceDefinition",
		Name:               configCRD.GetName(),
		UID:                configCRD.GetUID(),
		Controller:         ptr.Bool(true),
		BlockOwnerDeletion: ptr.Bool(true),
	}

	return resource.ApplyCRDs(ctx, c.crdClient, syncCtx.Recorder(), ownerRef, func(yaml string) ([]byte, error) {
		return []byte(yaml), nil
	}, staticCRDFiles...)
}
