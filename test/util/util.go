package util

import (
	"context"
	"fmt"
	"io/ioutil"
	"path"
	"time"

	"github.com/onsi/ginkgo"

	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterclientset "open-cluster-management.io/api/client/cluster/clientset/versioned"
	workclientset "open-cluster-management.io/api/client/work/clientset/versioned"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	clusterv1alpha1 "open-cluster-management.io/api/cluster/v1alpha1"

	"github.com/openshift/library-go/pkg/operator/events"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/util/retry"
)

const (
	expectedBrokerRole  = "submariner-k8s-broker-cluster"
	expectedIPSECSecret = "submariner-ipsec-psk"
)

var HubKubeConfigPath = path.Join("/tmp", "submaddon-integration-test", "kubeconfig")

// on prow env, the /var/run/secrets/kubernetes.io/serviceaccount/namespace can be found
func GetCurrentNamespace(kubeClient kubernetes.Interface, defaultNamespace string) (string, error) {
	nsBytes, err := ioutil.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	if err != nil {
		if _, err := kubeClient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: defaultNamespace,
			},
		}, metav1.CreateOptions{}); err != nil {
			return "", err
		}

		return defaultNamespace, nil
	}

	namespace := string(nsBytes)

	if _, err := kubeClient.CoreV1().Namespaces().Create(context.Background(), &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}, metav1.CreateOptions{}); err != nil && !errors.IsAlreadyExists(err) {
		return "", err
	}

	return namespace, nil
}

func CreateHubKubeConfig(cfg *rest.Config) error {
	config := clientcmdapi.NewConfig()
	config.Clusters["hub"] = &clientcmdapi.Cluster{
		Server: cfg.Host,
	}
	config.Contexts["hub"] = &clientcmdapi.Context{
		Cluster: "hub",
	}
	config.CurrentContext = "hub"

	return clientcmd.WriteToFile(*config, HubKubeConfigPath)
}

func FindExpectedFinalizer(finalizers []string, expected string) bool {
	for _, finalizer := range finalizers {
		if finalizer == expected {
			return true
		}
	}

	return false
}

func UpdateManagedClusterLabels(clusterClient clusterclientset.Interface, managedClusterName string, labels map[string]string) error {
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		managedCluster, err := clusterClient.ClusterV1().ManagedClusters().Get(context.Background(), managedClusterName, metav1.GetOptions{})
		if err != nil {
			return err
		}

		managedCluster.Labels = labels

		_, err = clusterClient.ClusterV1().ManagedClusters().Update(context.Background(), managedCluster, metav1.UpdateOptions{})
		return err
	})
}

func FindSubmarinerBrokerResources(kubeClient kubernetes.Interface, brokerNamespace string) bool {
	_, err := kubeClient.CoreV1().Namespaces().Get(context.Background(), brokerNamespace, metav1.GetOptions{})
	if err != nil {
		return false
	}

	_, err = kubeClient.RbacV1().Roles(brokerNamespace).Get(context.Background(), expectedBrokerRole, metav1.GetOptions{})
	if err != nil {
		return false
	}

	_, err = kubeClient.CoreV1().Secrets(brokerNamespace).Get(context.Background(), expectedIPSECSecret, metav1.GetOptions{})
	if err != nil {
		return false
	}

	return true
}

func FindManifestWorks(workClient workclientset.Interface, managedClusterName string, works ...string) bool {
	for _, work := range works {
		_, err := workClient.WorkV1().ManifestWorks(managedClusterName).Get(context.Background(), work, metav1.GetOptions{})
		if err != nil {
			return false
		}
	}

	return true
}

func SetupServiceAccount(kubeClient kubernetes.Interface, namespace, name string) error {
	return wait.Poll(1*time.Second, 30*time.Second, func() (bool, error) {
		// add a token secret to serviceaccount
		sa, err := kubeClient.CoreV1().ServiceAccounts(namespace).Get(context.Background(), name, metav1.GetOptions{})
		if errors.IsNotFound(err) {
			return false, nil
		}
		if err != nil {
			return false, err
		}

		secretName := fmt.Sprintf("%s-token-%s", name, rand.String(5))
		sa.Secrets = []corev1.ObjectReference{{Name: secretName}}
		if _, err := kubeClient.CoreV1().ServiceAccounts(namespace).Update(context.Background(), sa, metav1.UpdateOptions{}); err != nil {
			return false, err
		}

		// create a serviceaccount token secret
		secret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: namespace,
				Name:      secretName,
				Annotations: map[string]string{
					"kubernetes.io/service-account.name": sa.Name,
				},
			},
			Data: map[string][]byte{
				"ca.crt": []byte("test-ca"),
				"token":  []byte("test-token"),
			},
			Type: corev1.SecretTypeServiceAccountToken,
		}
		if _, err := kubeClient.CoreV1().Secrets(namespace).Create(context.Background(), secret, metav1.CreateOptions{}); err != nil {
			return false, err
		}

		return true, nil
	})
}

func NewManagedClusterNamespace(namespace string) *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
}

func NewManagedCluster(name string, labels map[string]string) *clusterv1.ManagedCluster {
	return &clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name:   name,
			Labels: labels,
		},
	}
}

func NewManagedClusterSet(name string) *clusterv1alpha1.ManagedClusterSet {
	return &clusterv1alpha1.ManagedClusterSet{
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func NewSubmarinerConifg(namespace string) *configv1alpha1.SubmarinerConfig {
	return &configv1alpha1.SubmarinerConfig{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "submariner",
			Namespace: namespace,
		},
		Spec: configv1alpha1.SubmarinerConfigSpec{},
	}
}

func NewManagedClusterAddOn(namespace string) *addonv1alpha1.ManagedClusterAddOn {
	return &addonv1alpha1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "submariner",
			Namespace: namespace,
		},
		Spec: addonv1alpha1.ManagedClusterAddOnSpec{
			InstallNamespace: "submariner-operator",
		},
	}
}

func NewSubmariner() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "submariner.io/v1alpha1",
			"kind":       "Submariner",
			"metadata": map[string]interface{}{
				"name": "submariner",
			},
			"spec": map[string]interface{}{
				"broker":                   "k8s",
				"brokerK8sApiServer":       "api:6443",
				"brokerK8sApiServerToken":  "token",
				"brokerK8sCA":              "ca",
				"brokerK8sRemoteNamespace": "subm-broker",
				"cableDriver":              "libreswan",
				"ceIPSecDebug":             false,
				"ceIPSecPSK":               "psk",
				"clusterCIDR":              "",
				"clusterID":                "test",
				"debug":                    false,
				"namespace":                "submariner-operator",
				"natEnabled":               true,
				"serviceCIDR":              "",
			},
		},
	}
}

func SetSubmarinerDeployedStatus(submariner *unstructured.Unstructured) {
	submariner.Object["status"] = map[string]interface{}{
		"clusterID":  "test",
		"natEnabled": true,
		"gatewayDaemonSetStatus": map[string]interface{}{
			"mismatchedContainerImages": false,
			"status": map[string]interface{}{
				"currentNumberScheduled": int64(1),
				"desiredNumberScheduled": int64(1),
				"numberMisscheduled":     int64(0),
				"numberReady":            int64(1),
			},
		},
		"routeAgentDaemonSetStatus": map[string]interface{}{
			"mismatchedContainerImages": false,
			"status": map[string]interface{}{
				"currentNumberScheduled": int64(6),
				"desiredNumberScheduled": int64(6),
				"numberMisscheduled":     int64(0),
				"numberReady":            int64(6),
			},
		},
	}
}

func NewIntegrationTestEventRecorder(componet string) events.Recorder {
	return &IntegrationTestEventRecorder{component: componet}
}

type IntegrationTestEventRecorder struct {
	component string
}

func (r *IntegrationTestEventRecorder) ComponentName() string {
	return r.component
}

func (r *IntegrationTestEventRecorder) ForComponent(c string) events.Recorder {
	return &IntegrationTestEventRecorder{component: c}
}

func (r *IntegrationTestEventRecorder) WithComponentSuffix(suffix string) events.Recorder {
	return r.ForComponent(fmt.Sprintf("%s-%s", r.ComponentName(), suffix))
}

func (r *IntegrationTestEventRecorder) Event(reason, message string) {
	fmt.Fprintf(ginkgo.GinkgoWriter, "Event: [%s] %v: %v \n", r.component, reason, message)
}

func (r *IntegrationTestEventRecorder) Eventf(reason, messageFmt string, args ...interface{}) {
	r.Event(reason, fmt.Sprintf(messageFmt, args...))
}

func (r *IntegrationTestEventRecorder) Warning(reason, message string) {
	fmt.Fprintf(ginkgo.GinkgoWriter, "Warning: [%s] %v: %v \n", r.component, reason, message)
}

func (r *IntegrationTestEventRecorder) Warningf(reason, messageFmt string, args ...interface{}) {
	r.Warning(reason, fmt.Sprintf(messageFmt, args...))
}

func (r *IntegrationTestEventRecorder) Shutdown() {
	return
}
