package submarinerbroker

import (
	"context"

	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/pkg/errors"
	"github.com/stolostron/submariner-addon/pkg/resource"
	submopcrds "github.com/submariner-io/submariner-operator/deploy/crds"
	submcrds "github.com/submariner-io/submariner/deploy/crds"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"k8s.io/utils/ptr"
	mcscrd "sigs.k8s.io/mcs-api/config/crd"
)

const (
	ConfigCRDName = "submarinerconfigs.submarineraddon.open-cluster-management.io"
)

var staticCRDFiles = []string{
	string(submcrds.ClustersCRD),
	string(submopcrds.BrokerCRD),
	string(submcrds.EndpointsCRD),
	string(submcrds.GatewaysCRD),
	string(mcscrd.ServiceImportCRD),
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
	if apierrors.IsNotFound(err) {
		return nil
	}

	if err != nil {
		return errors.Wrapf(err, "error retrieving CRD %q", crdName)
	}

	ownerRef := &v1.OwnerReference{
		APIVersion:         apiextensionsv1.SchemeGroupVersion.String(),
		Kind:               "CustomResourceDefinition",
		Name:               configCRD.GetName(),
		UID:                configCRD.GetUID(),
		Controller:         ptr.To(true),
		BlockOwnerDeletion: ptr.To(true),
	}

	//nolint:wrapcheck // No need to wrap here
	return resource.ApplyCRDs(ctx, c.crdClient, syncCtx.Recorder(), ownerRef, func(yaml string) ([]byte, error) {
		return []byte(yaml), nil
	}, staticCRDFiles...)
}
