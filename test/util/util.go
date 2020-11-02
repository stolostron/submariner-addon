package util

import (
	"context"
	"fmt"
	"time"

	"github.com/onsi/ginkgo"

	clusterclientset "github.com/open-cluster-management/api/client/cluster/clientset/versioned"
	workclientset "github.com/open-cluster-management/api/client/work/clientset/versioned"
	clusterv1 "github.com/open-cluster-management/api/cluster/v1"
	clusterv1alpha1 "github.com/open-cluster-management/api/cluster/v1alpha1"

	"github.com/openshift/library-go/pkg/operator/events"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

const (
	expectedBrokerRole  = "submariner-k8s-broker-cluster"
	expectedIPSECSecret = "submariner-ipsec-psk"
)

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
