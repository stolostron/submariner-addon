package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	fakeclusterclient "github.com/open-cluster-management/api/client/cluster/clientset/versioned/fake"
	clusterv1 "github.com/open-cluster-management/api/cluster/v1"

	"github.com/openshift/library-go/pkg/operator/events/eventstesting"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestGenerateAndGetIPSecPSKSecret(t *testing.T) {
	cases := []struct {
		name            string
		brokerNamespace string
		existings       []runtime.Object
		expectErr       bool
	}{
		{
			name:            "exist secret",
			brokerNamespace: "cluster1-broker",
			existings: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      IPSecPSKSecretName,
						Namespace: "cluster1-broker",
					},
					Data: map[string][]byte{
						"psk": []byte("abcd1234"),
					},
				},
			},
			expectErr: false,
		},
		{
			name:            "no secret",
			brokerNamespace: "cluster2-broker",
			existings:       []runtime.Object{},
			expectErr:       false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeClient := kubefake.NewSimpleClientset(c.existings...)
			err := GenerateIPSecPSKSecret(fakeClient, c.brokerNamespace)
			if err != nil && !c.expectErr {
				t.Errorf("expect no err: %+v", err)
			}
			if err == nil && c.expectErr {
				t.Errorf("expect err")
			}

			ipSecPSK, err := GetIPSecPSK(fakeClient, c.brokerNamespace)
			if err != nil && !c.expectErr {
				t.Errorf("expect no err: %+v", err)
			}
			if err == nil && c.expectErr {
				t.Errorf("expect err")
			}
			if ipSecPSK == "" && !c.expectErr {
				t.Errorf("expect an valid ipSecPSK")
			}
		})
	}
}

func TestGetBrokerAPIServer(t *testing.T) {
	cases := []struct {
		name      string
		existings []runtime.Object
		expectErr bool
	}{
		{
			name:      "no infrastructure",
			existings: []runtime.Object{},
			expectErr: true,
		},
		{
			name: "exist infrastructure",
			existings: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "config.openshift.io/v1",
						"kind":       "Infrastructure",
						"metadata": map[string]interface{}{
							"name": InfrastructureConfigName,
						},
						"status": map[string]interface{}{
							"apiServerURL": "https://api.test.dev04.red-chesterfield.com:6443",
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "no apiServerURL in infrastructure status",
			existings: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "config.openshift.io/v1",
						"kind":       "Infrastructure",
						"metadata": map[string]interface{}{
							"name": InfrastructureConfigName,
						},
					},
				},
			},
			expectErr: true,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), c.existings...)
			apiServer, err := GetBrokerAPIServer(fakeClient)
			if c.expectErr && err == nil {
				t.Errorf("expect err")
			}
			if !c.expectErr && err != nil {
				t.Errorf("expect no err: %v", err)
			}
			if !c.expectErr && err == nil && len(apiServer) == 0 {
				t.Errorf("expect valid apiServer")
			}
		})
	}
}

func TestGetBrokerTokenAndCA(t *testing.T) {
	cases := []struct {
		name            string
		brokerNamespace string
		clusterName     string
		existings       []runtime.Object
		expectErr       bool
	}{
		{
			name:            "no sa no secret",
			brokerNamespace: "cluster1-broker",
			clusterName:     "cluster1",
			existings:       []runtime.Object{},
			expectErr:       true,
		},
		{
			name:            "exist sa no secret",
			brokerNamespace: "cluster1-broker",
			clusterName:     "cluster1",
			existings: []runtime.Object{
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "cluster1-broker",
					},
				},
			},
			expectErr: true,
		},
		{
			name:            "exist sa and secret, but the secret cannot be found",
			brokerNamespace: "cluster1-broker",
			clusterName:     "cluster1",
			existings: []runtime.Object{
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "cluster1-broker",
					},
					Secrets: []corev1.ObjectReference{{Name: "cluster-cluster1-token-5pw5c"}},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-cluster1-token-5pw5c",
						Namespace: "cluster1-broker",
					},
					Data: map[string][]byte{
						"ca.crt": []byte("ca"),
						"token":  []byte("token"),
					},
					Type: corev1.SecretTypeServiceAccountToken,
				},
			},
			expectErr: true,
		},
		{
			name:            "exist sa and secret",
			brokerNamespace: "cluster1-broker",
			clusterName:     "cluster1",
			existings: []runtime.Object{
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "cluster1-broker",
					},
					Secrets: []corev1.ObjectReference{{Name: "cluster1-token-5pw5c"}},
				},
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1-token-5pw5c",
						Namespace: "cluster1-broker",
					},
					Data: map[string][]byte{
						"ca.crt": []byte("ca"),
						"token":  []byte("token"),
					},
					Type: corev1.SecretTypeServiceAccountToken,
				},
			},
			expectErr: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeClient := kubefake.NewSimpleClientset(c.existings...)
			token, ca, err := GetBrokerTokenAndCA(fakeClient, c.brokerNamespace, c.clusterName)
			if err != nil && !c.expectErr {
				t.Errorf("expect no err: %+v", err)
			}
			if err == nil && c.expectErr {
				t.Errorf("expect err")
			}
			if (token == "" || ca == "") && !c.expectErr {
				t.Errorf("expect valid token and ca")
			}
		})
	}
}

func TestCleanUpSubmarinerManifests(t *testing.T) {
	applyFiles := map[string]runtime.Object{
		"role":           newUnstructured("rbac.authorization.k8s.io/v1", "Role", "ns", "rb", map[string]interface{}{}),
		"rolebinding":    newUnstructured("rbac.authorization.k8s.io/v1", "RoleBinding", "ns", "r", map[string]interface{}{}),
		"serviceaccount": newUnstructured("v1", "ServiceAccount", "ns", "sa", map[string]interface{}{}),
		"namespace":      newUnstructured("v1", "Namespace", "", "ns", map[string]interface{}{}),
		"kind1":          newUnstructured("v1", "Kind1", "ns", "k1", map[string]interface{}{}),
	}

	testcase := []struct {
		name          string
		applyFileName string
		expectErr     bool
	}{
		{
			name:          "Delete role",
			applyFileName: "role",
			expectErr:     false,
		},
		{
			name:          "Delete rolebinding",
			applyFileName: "rolebinding",
			expectErr:     false,
		},
		{
			name:          "Delete serviceaccount",
			applyFileName: "serviceaccount",
			expectErr:     false,
		},
		{
			name:          "Delete namespace",
			applyFileName: "namespace",
			expectErr:     false,
		},
		{
			name:          "Delete unhandled object",
			applyFileName: "kind1",
			expectErr:     true,
		},
	}

	for _, c := range testcase {
		t.Run(c.name, func(t *testing.T) {
			fakeKubeClient := kubefake.NewSimpleClientset()
			err := CleanUpSubmarinerManifests(
				context.TODO(),
				fakeKubeClient,
				eventstesting.NewTestingEventRecorder(t),
				func(name string) ([]byte, error) {
					if applyFiles[name] == nil {
						return nil, fmt.Errorf("Failed to find file")
					}

					return json.Marshal(applyFiles[name])
				},
				c.applyFileName,
			)

			if err == nil && c.expectErr {
				t.Errorf("Expect an apply error")
			}
			if err != nil && !c.expectErr {
				t.Errorf("Expect no apply error, %v", err)
			}
		})
	}
}

func TestGetClusterType(t *testing.T) {
	cases := []struct {
		name        string
		clusterName string
		existings   []runtime.Object
		expectErr   bool
		expectType  string
	}{
		{
			name:        "cluster is not existed",
			clusterName: "cluster1",
			existings:   []runtime.Object{},
			expectErr:   true,
			expectType:  "",
		},
		{
			name:        "cluster is OCP",
			clusterName: "cluster1",
			existings: []runtime.Object{
				&clusterv1.ManagedCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster1",
						Labels: map[string]string{
							"vendor": "OpenShift",
						},
					},
				},
			},
			expectErr:  false,
			expectType: ClusterTypeOCP,
		},
		{
			name:        "cluster is not OCP",
			clusterName: "cluster1",
			existings: []runtime.Object{
				&clusterv1.ManagedCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster1",
						Labels: map[string]string{
							"vendor": "others",
						},
					},
				},
			},
			expectErr:  false,
			expectType: "others",
		},
		{
			name:        "cluster has no vendor",
			clusterName: "cluster1",
			existings: []runtime.Object{
				&clusterv1.ManagedCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster1",
					},
				},
			},
			expectErr:  false,
			expectType: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeClusterClient := fakeclusterclient.NewSimpleClientset(c.existings...)
			clusterType, err := GetClusterType(fakeClusterClient, c.clusterName)
			t.Logf("type:%v, err:%v", clusterType, err)
			if err != nil && !c.expectErr {
				t.Errorf("expect no err: %+v", err)
			}
			if err == nil && c.expectErr {
				t.Errorf("expect err")
			}
			if err == nil && !c.expectErr {
				if clusterType != c.expectType {
					t.Errorf("cluster type is not the expectType")
				}
			}
		})
	}
}

func newUnstructured(apiVersion, kind, namespace, name string, content map[string]interface{}) *unstructured.Unstructured {
	object := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": apiVersion,
			"kind":       kind,
			"metadata": map[string]interface{}{
				"namespace": namespace,
				"name":      name,
			},
		},
	}
	for key, val := range content {
		object.Object[key] = val
	}

	return object
}
