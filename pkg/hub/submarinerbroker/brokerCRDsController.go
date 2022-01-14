package submarinerbroker

import (
	"context"
	"fmt"
	"path/filepath"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/library-go/pkg/controller/factory"
	"github.com/openshift/library-go/pkg/operator/events"
	operatorhelpers "github.com/openshift/library-go/pkg/operator/v1helpers"
	"github.com/stolostron/submariner-addon/pkg/helpers"
	"github.com/stolostron/submariner-addon/pkg/hub/submarinerbroker/bindata"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsinformers "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions/apiextensions/v1beta1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
)

const (
	configCRDName = "submarinerconfigs.submarineraddon.open-cluster-management.io"
)

var staticCRDFiles = []string{
	"manifests/broker/submariner.io_clusters_crd.yaml",
	"manifests/broker/submariner.io_endpoints_crd.yaml",
	"manifests/broker/submariner.io_gateways_crd.yaml",
	"manifests/broker/submariner.io_lighthouse.serviceimports_crd.yaml",
	"manifests/broker/x-k8s.io_multicluster.serviceimports_crd.yaml",
}

type brokerCRDsConfig struct {
	ConfigCRDUID types.UID
}

type submarinerBrokerCRDsController struct {
	crdClient     apiextensionsclientset.Interface
	eventRecorder events.Recorder
}

func NewSubmarinerBrokerCRDsController(
	crdClient apiextensionsclientset.Interface,
	crdInformer apiextensionsinformers.CustomResourceDefinitionInformer,
	recorder events.Recorder) factory.Controller {
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

	if crdName != configCRDName {
		return nil
	}

	configCRD, err := c.crdClient.ApiextensionsV1beta1().CustomResourceDefinitions().Get(ctx, crdName, v1.GetOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	if err != nil {
		return err
	}

	crdsConfig := brokerCRDsConfig{
		ConfigCRDUID: configCRD.GetUID(),
	}

	// Apply CRDs
	clientHolder := helpers.NewCRDClientHolder().WithAPIExtensionsClient(c.crdClient)
	applyResults := helpers.ApplyCRDDirectly(
		clientHolder,
		syncCtx.Recorder(),
		func(name string) ([]byte, error) {
			return assets.MustCreateAssetFromTemplate(name, bindata.MustAsset(filepath.Join("", name)), crdsConfig).Data, nil
		},
		staticCRDFiles...,
	)

	errs := []error{}
	for _, result := range applyResults {
		if result.Error != nil {
			errs = append(errs, fmt.Errorf("%q (%T): %v", result.File, result.Type, result.Error))
		}
	}

	return operatorhelpers.NewMultiLineAggregate(errs)
}
