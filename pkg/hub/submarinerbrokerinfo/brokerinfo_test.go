package submarinerbrokerinfo

import (
	"testing"

	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	fakeconfigclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned/fake"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"

	"github.com/openshift/library-go/pkg/operator/events/eventstesting"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestNewSubmarinerBrokerInfo(t *testing.T) {
	cases := []struct {
		name             string
		clusterName      string
		brokerName       string
		secrets          []runtime.Object
		cluster          *clusterv1.ManagedCluster
		infrastructures  []runtime.Object
		submarinerConfig *configv1alpha1.SubmarinerConfig
		expectedSource   string
	}{
		{
			name:        "upstream build",
			clusterName: "cluster1",
			brokerName:  "broker1",
			secrets: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner-ipsec-psk",
						Namespace: "broker1",
					},
					Data: map[string][]byte{
						"psk": []byte("test-psk"),
					},
				},
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "broker1",
					},
					Secrets: []corev1.ObjectReference{{Name: "cluster1-token-5pw5c"}},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1-token-5pw5c",
						Namespace: "broker1",
					},
					Data: map[string][]byte{
						"ca.crt": []byte("test-ca"),
						"token":  []byte("test-token"),
					},
					Type: corev1.SecretTypeServiceAccountToken,
				},
			},
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
				},
				Status: clusterv1.ManagedClusterStatus{
					ClusterClaims: []clusterv1.ManagedClusterClaim{
						{
							Name:  "product.open-cluster-management.io",
							Value: "OpenShift",
						},
					},
				},
			},
			infrastructures: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "config.openshift.io/v1",
						"kind":       "Infrastructure",
						"metadata": map[string]interface{}{
							"name": "cluster",
						},
						"status": map[string]interface{}{
							"apiServerURL": "https://api.test.dev04.red-chesterfield.com:6443",
						},
					},
				},
			},
			submarinerConfig: &configv1alpha1.SubmarinerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "cluster1",
				},
				Spec: configv1alpha1.SubmarinerConfigSpec{
					SubscriptionConfig: configv1alpha1.SubscriptionConfig{
						Source:          "operatorhubio-catalog",
						SourceNamespace: "olm",
					},
				},
			},
			expectedSource: "operatorhubio-catalog",
		},
		{
			name:        "default build",
			clusterName: "cluster1",
			brokerName:  "broker1",
			secrets: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner-ipsec-psk",
						Namespace: "broker1",
					},
					Data: map[string][]byte{
						"psk": []byte("test-psk"),
					},
				},
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "broker1",
					},
					Secrets: []corev1.ObjectReference{{Name: "cluster1-token-5pw5c"}},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1-token-5pw5c",
						Namespace: "broker1",
					},
					Data: map[string][]byte{
						"ca.crt": []byte("test-ca"),
						"token":  []byte("test-token"),
					},
					Type: corev1.SecretTypeServiceAccountToken,
				},
			},
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
				},
				Status: clusterv1.ManagedClusterStatus{
					ClusterClaims: []clusterv1.ManagedClusterClaim{
						{
							Name:  "product.open-cluster-management.io",
							Value: "OpenShift",
						},
					},
				},
			},
			infrastructures: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "config.openshift.io/v1",
						"kind":       "Infrastructure",
						"metadata": map[string]interface{}{
							"name": "cluster",
						},
						"status": map[string]interface{}{
							"apiServerURL": "https://api.test.dev04.red-chesterfield.com:6443",
						},
					},
				},
			},
			expectedSource: "redhat-operators",
		},
		{
			name:        "with submariner conifg",
			clusterName: "cluster1",
			brokerName:  "broker1",
			secrets: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner-ipsec-psk",
						Namespace: "broker1",
					},
					Data: map[string][]byte{
						"psk": []byte("test-psk"),
					},
				},
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "broker1",
					},
					Secrets: []corev1.ObjectReference{{Name: "cluster1-token-5pw5c"}},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1-token-5pw5c",
						Namespace: "broker1",
					},
					Data: map[string][]byte{
						"ca.crt": []byte("test-ca"),
						"token":  []byte("test-token"),
					},
					Type: corev1.SecretTypeServiceAccountToken,
				},
			},
			cluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
				},
				Status: clusterv1.ManagedClusterStatus{
					ClusterClaims: []clusterv1.ManagedClusterClaim{
						{
							Name:  "product.open-cluster-management.io",
							Value: "OpenShift",
						},
					},
				},
			},
			infrastructures: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "config.openshift.io/v1",
						"kind":       "Infrastructure",
						"metadata": map[string]interface{}{
							"name": "cluster",
						},
						"status": map[string]interface{}{
							"apiServerURL": "https://api.test.dev04.red-chesterfield.com:6443",
						},
					},
				},
			},
			submarinerConfig: &configv1alpha1.SubmarinerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "cluster1",
				},
				Spec: configv1alpha1.SubmarinerConfigSpec{
					CableDriver:   "test",
					IPSecIKEPort:  501,
					IPSecNATTPort: 4501,
					ImagePullSpecs: configv1alpha1.SubmarinerImagePullSpecs{
						SubmarinerImagePullSpec:           "test-submariner",
						LighthouseCoreDNSImagePullSpec:    "test-lighthouse-coredns",
						LighthouseAgentImagePullSpec:      "test-lighthosse-agent",
						SubmarinerRouteAgentImagePullSpec: "test-route-agent",
					},
				},
			},
			expectedSource: "redhat-operators",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeConfigClient := fakeconfigclient.NewSimpleClientset()
			if c.submarinerConfig != nil {
				fakeConfigClient = fakeconfigclient.NewSimpleClientset(c.submarinerConfig)
			}
			brokerInfo, err := NewSubmarinerBrokerInfo(
				kubefake.NewSimpleClientset(c.secrets...),
				dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), c.infrastructures...),
				fakeConfigClient,
				eventstesting.NewTestingEventRecorder(t),
				c.cluster,
				c.brokerName,
				c.submarinerConfig,
				&addonapiv1alpha1.ManagedClusterAddOn{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "submariner",
						Namespace: "cluster1",
					},
				},
			)
			if err != nil {
				t.Errorf("expect no err, but got: %v", err)
			}
			if !brokerInfo.NATEnabled {
				t.Errorf("expect NATEnabled true, but false")
			}
			if brokerInfo.CatalogSource != c.expectedSource {
				t.Errorf("expect %s, but got %v", c.expectedSource, brokerInfo)
			}
		})
	}
}
