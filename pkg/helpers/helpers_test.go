package helpers

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"testing"
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
					Secrets: []corev1.ObjectReference{{Name: "cluster-cluster1-token-5pw5c"}},
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
