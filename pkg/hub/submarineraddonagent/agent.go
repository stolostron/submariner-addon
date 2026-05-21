package submarineraddonagent

import (
	"context"
	"embed"
	goerrors "errors"
	"fmt"
	"os"

	"github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/pkg/errors"
	"github.com/stolostron/submariner-addon/pkg/constants"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	"open-cluster-management.io/addon-framework/pkg/addonfactory"
	"open-cluster-management.io/addon-framework/pkg/agent"
	"open-cluster-management.io/addon-framework/pkg/utils"
	addonapiv1beta1 "open-cluster-management.io/api/addon/v1beta1"
	addonclient "open-cluster-management.io/api/client/addon/clientset/versioned"
	clusterclient "open-cluster-management.io/api/client/cluster/clientset/versioned"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
)

const (
	agentName                  = "submariner-addon-agent"
	selfManagedClusterLabelKey = "local-cluster"
	clusterAddOnGroup          = "system:open-cluster-management:cluster:%s:addon:submariner"
)

var agentHubPermissionFiles = []string{
	"manifests/hub/role.yaml",
	"manifests/hub/rolebinding.yaml",
}

//go:embed manifests
var manifestFiles embed.FS

// addOnAgent monitors the Submariner agent status and configure Submariner cluster environment on the managed cluster.
type addOnAgent struct {
	agent.AgentAddon
	kubeClient    kubernetes.Interface
	clusterClient clusterclient.Interface
	recorder      events.Recorder
	agentImage    string
	hubHost       string
	resourceCache resourceapply.ResourceCache
}

// NewAddOnAgent returns an instance of addOnAgent.
func NewAddOnAgent(kubeClient kubernetes.Interface, clusterClient clusterclient.Interface,
	addonClient addonclient.Interface, recorder events.Recorder, agentImage string,
) (agent.AgentAddon, error) {
	a := &addOnAgent{
		kubeClient:    kubeClient,
		clusterClient: clusterClient,
		recorder:      recorder,
		agentImage:    agentImage,
		resourceCache: resourceapply.NewResourceCache(),
	}

	registrationOption := &agent.RegistrationOption{
		Configurations:   agent.KubeClientSignerConfigurations(constants.SubmarinerAddOnName, agentName),
		CSRApproveCheck:  utils.DefaultCSRApprover(agentName),
		PermissionConfig: a.permissionConfig,
	}

	var err error

	a.AgentAddon, err = addonfactory.NewAgentAddonFactory(constants.SubmarinerAddOnName, manifestFiles, "manifests/spoke").
		WithConfigGVRs(utils.AddOnDeploymentConfigGVR).
		WithGetValuesFuncs(
			a.getValues,
			addonfactory.GetAddOnDeploymentConfigValues(
				utils.NewAddOnDeploymentConfigGetter(addonClient),
				addonfactory.ToAddOnDeploymentConfigValues,
			),
		).
		WithAgentRegistrationOption(registrationOption).
		WithAgentInstallNamespace(
			utils.AgentInstallNamespaceFromDeploymentConfigFunc(
				utils.NewAddOnDeploymentConfigGetter(addonClient),
			),
		).
		BuildTemplateAgentAddon()

	return a, errors.Wrap(err, "error building AgentAddon")
}

func (a *addOnAgent) Manifests(ctx context.Context, cluster *clusterv1.ManagedCluster, addon *addonapiv1beta1.ManagedClusterAddOn,
) ([]runtime.Object, error) {
	objs, err := a.AgentAddon.Manifests(ctx, cluster, addon)
	if err != nil {
		return nil, err //nolint:wrapcheck // No need to wrap
	}

	// Find the add-on agent deployment and check its installation namespace. If it's the default add-on namespace then
	// we will not maintain (create/delete) it, because other ACM add-ons will be installed in this namespace. Otherwise,
	// specifically add a Namespace object to the returned resources that has the "deletion-orphan" annotation set so the
	// ManifestWorks doesn't delete the Namespace on uninstall to avoid a race condition where the Submariner operator
	// pod is deleted before it is able to run cleanup and remove its finalizer from the Submariner resource.
	for _, o := range objs {
		deployment, ok := o.(*appsv1.Deployment)
		if !ok {
			continue
		}

		if deployment.Namespace != addonfactory.AddonDefaultInstallNamespace {
			objs = append(objs, &corev1.Namespace{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Namespace",
					APIVersion: corev1.SchemeGroupVersion.String(),
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:        deployment.Namespace,
					Annotations: map[string]string{addonapiv1beta1.DeletionOrphanAnnotationKey: "true"},
				},
			})
		}

		break
	}

	return objs, nil
}

func (a *addOnAgent) getValues(cluster *clusterv1.ManagedCluster, _ *addonapiv1beta1.ManagedClusterAddOn) (addonfactory.Values, error) {
	manifestConfig := struct {
		Image                string
		HubHost              string
		OpenShiftProfile     string
		OpenShiftProfileHost string
		OpenShiftProfilePort string
	}{
		Image:                a.agentImage,
		HubHost:              a.getHubHost(),
		OpenShiftProfile:     os.Getenv("OPENSHIFT_PROFILE"),
		OpenShiftProfileHost: os.Getenv("OPENSHIFT_PROFILE_HOST"),
		OpenShiftProfilePort: os.Getenv("OPENSHIFT_PROFILE_PORT"),
	}

	return addonfactory.StructToValues(manifestConfig), nil
}

// Generates manifestworks to deploy the required roles of submariner-addon agent.
func (a *addOnAgent) permissionConfig(ctx context.Context, cluster *clusterv1.ManagedCluster, _ *addonapiv1beta1.ManagedClusterAddOn,
) error {
	config := struct {
		ClusterName string
		Group       string
	}{
		ClusterName: cluster.Name,
		Group:       fmt.Sprintf(clusterAddOnGroup, cluster.Name),
	}

	results := resourceapply.ApplyDirectly(
		ctx,
		resourceapply.NewKubeClientHolder(a.kubeClient),
		a.recorder,
		a.resourceCache,
		func(name string) ([]byte, error) {
			template, err := manifestFiles.ReadFile(name)
			if err != nil {
				return nil, errors.Wrapf(err, "error reading manifest file %q", name)
			}

			return assets.MustCreateAssetFromTemplate(name, template, config).Data, nil
		},
		agentHubPermissionFiles...,
	)

	errs := []error{}

	for _, result := range results {
		if result.Error != nil {
			errs = append(errs, result.Error)
		}
	}

	return goerrors.Join(errs...)
}

// This retrieves the Hub's API url by getting it from the local-cluster. The
// local-cluster will be missing in kind deployments. In ACM deployments there is a race
// condition between local-cluster and submariner-addon pod creation. So we check for it right
// before we use it for manifests. This code gets called repeatedly from syncer, so even if
// local-cluster was missing at startup, by the time we hit this code it should be available.
func (a *addOnAgent) getHubHost() string {
	if a.hubHost == "" {
		localClusters, err := a.clusterClient.ClusterV1().ManagedClusters().List(context.TODO(), metav1.ListOptions{
			LabelSelector: selfManagedClusterLabelKey + "=true",
		})
		if err != nil {
			klog.Errorf("Unable to determine hub host - error listing ManagedClusters: %v", err)
			return ""
		}

		if len(localClusters.Items) == 0 {
			klog.Info("Local cluster not found")
			return ""
		}

		a.hubHost = localClusters.Items[0].Spec.ManagedClusterClientConfigs[0].URL
	}

	return a.hubHost
}
