package submarineraddonagent

import (
	"crypto/x509"
	"embed"
	"encoding/pem"
	"fmt"

	"github.com/open-cluster-management/addon-framework/pkg/agent"
	addonapiv1alpha1 "github.com/open-cluster-management/api/addon/v1alpha1"
	clusterv1 "github.com/open-cluster-management/api/cluster/v1"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"

	"github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	operatorhelpers "github.com/openshift/library-go/pkg/operator/v1helpers"

	certificatesv1 "k8s.io/api/certificates/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
)

var (
	genericScheme = runtime.NewScheme()
	genericCodecs = serializer.NewCodecFactory(genericScheme)
	genericCodec  = genericCodecs.UniversalDeserializer()
)

func init() {
	scheme.AddToScheme(genericScheme)
}

const (
	agentName                    = "submariner-addon-agent"
	defaultInstallationNamespace = "open-cluster-management-agent-addon"
)

const (
	addOnGroup         = "system:open-cluster-management:addon:submariner"
	agentUserName      = "system:open-cluster-management:cluster:%s:addon:submariner:agent:submariner-addon-agent"
	clusterAddOnGroup  = "system:open-cluster-management:cluster:%s:addon:submariner"
	authenticatedGroup = "system:authenticated"
)

const agentInstallationNamespaceFile = "manifests/namespace.yaml"

var agentDeploymentFiles = []string{
	"manifests/clusterrole.yaml",
	"manifests/clusterrolebinding.yaml",
	"manifests/deployment.yaml",
	"manifests/role.yaml",
	"manifests/rolebinding.yaml",
	"manifests/serviceaccount.yaml",
}

var agentHubPermissionFiles = []string{
	"manifests/hub_role.yaml",
	"manifests/hub_rolebinding.yaml",
}

//go:embed manifests
var manifestFiles embed.FS

// addOnAgent monitors the Submariner agent status and configure Submariner cluster environment on the managed cluster
type addOnAgent struct {
	kubeClient kubernetes.Interface
	recorder   events.Recorder
	agentImage string
}

// NewAddOnAgent returns an instance of addOnAgent
func NewAddOnAgent(kubeClient kubernetes.Interface, recorder events.Recorder, agentImage string) *addOnAgent {
	return &addOnAgent{
		kubeClient: kubeClient,
		recorder:   recorder,
		agentImage: agentImage,
	}
}

// Manifests generates manifestworks to deploy the submariner-addon agent on the managed cluster
func (a *addOnAgent) Manifests(cluster *clusterv1.ManagedCluster, addon *addonapiv1alpha1.ManagedClusterAddOn) ([]runtime.Object, error) {
	objects := []runtime.Object{}

	// if the installation namespace is not set, to keep consistent with addon-framework,
	// using open-cluster-management-agent-addon namespace as default namespace.
	installNamespace := addon.Spec.InstallNamespace
	if len(installNamespace) == 0 {
		installNamespace = defaultInstallationNamespace
	}

	deploymentFiles := append([]string{}, agentDeploymentFiles...)
	// if the installation namesapce is default namespace (open-cluster-management-agent-addon),
	// we will not maintain (create/delete) it, because other ACM addons will be installed this namespace.
	if installNamespace != defaultInstallationNamespace {
		deploymentFiles = append(deploymentFiles, agentInstallationNamespaceFile)
	}

	manifestConfig := struct {
		KubeConfigSecret      string
		ClusterName           string
		AddonInstallNamespace string
		Image                 string
	}{
		KubeConfigSecret:      fmt.Sprintf("%s-hub-kubeconfig", a.GetAgentAddonOptions().AddonName),
		AddonInstallNamespace: installNamespace,
		ClusterName:           cluster.Name,
		Image:                 a.agentImage,
	}

	for _, file := range deploymentFiles {
		template, err := manifestFiles.ReadFile(file)
		if err != nil {
			return objects, err
		}
		raw := assets.MustCreateAssetFromTemplate(file, template, &manifestConfig).Data
		object, _, err := genericCodec.Decode(raw, nil, nil)
		if err != nil {
			return nil, err
		}
		objects = append(objects, object)
	}
	return objects, nil
}

// GetAgentAddonOptions returns the options of submariner-addon agent
func (a *addOnAgent) GetAgentAddonOptions() agent.AgentAddonOptions {
	return agent.AgentAddonOptions{
		AddonName: helpers.SubmarinerAddOnName,
		Registration: &agent.RegistrationOption{
			CSRConfigurations: agent.KubeClientSignerConfigurations(helpers.SubmarinerAddOnName, agentName),
			CSRApproveCheck:   a.csrApproveCheck,
			PermissionConfig:  a.permissionConfig,
		},
	}
}

// To check the addon agent csr, we check
// 1. if the signer name in csr request is valid.
// 2. if organization field and commonName field in csr request is valid.
// 3. if user name in csr is the same as commonName field in csr request.
func (a *addOnAgent) csrApproveCheck(cluster *clusterv1.ManagedCluster, addon *addonapiv1alpha1.ManagedClusterAddOn, csr *certificatesv1.CertificateSigningRequest) bool {
	if csr.Spec.SignerName != certificatesv1.KubeAPIServerClientSignerName {
		return false
	}

	block, _ := pem.Decode(csr.Spec.Request)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		klog.V(4).Infof("csr %q was not recognized: PEM block type is not CERTIFICATE REQUEST", csr.Name)
		return false
	}

	x509cr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil {
		klog.V(4).Infof("csr %q was not recognized: %v", csr.Name, err)
		return false
	}

	requestingOrgs := sets.NewString(x509cr.Subject.Organization...)
	if requestingOrgs.Len() != 3 {
		return false
	}

	if !requestingOrgs.Has(authenticatedGroup) {
		return false
	}

	if !requestingOrgs.Has(addOnGroup) {
		return false
	}

	if !requestingOrgs.Has(fmt.Sprintf(clusterAddOnGroup, cluster.Name)) {
		return false
	}

	return fmt.Sprintf(agentUserName, cluster.Name) == x509cr.Subject.CommonName
}

// Generates manifestworks to deploy the required roles of submariner-addon agent
func (a *addOnAgent) permissionConfig(cluster *clusterv1.ManagedCluster, addon *addonapiv1alpha1.ManagedClusterAddOn) error {
	config := struct {
		ClusterName string
		Group       string
	}{
		ClusterName: cluster.Name,
		Group:       fmt.Sprintf(clusterAddOnGroup, cluster.Name),
	}

	results := resourceapply.ApplyDirectly(
		resourceapply.NewKubeClientHolder(a.kubeClient),
		a.recorder,
		func(name string) ([]byte, error) {
			template, err := manifestFiles.ReadFile(name)
			if err != nil {
				return nil, err
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

	return operatorhelpers.NewMultiLineAggregate(errs)
}
