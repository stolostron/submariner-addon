package helpers

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	configv1alpha1 "github.com/open-cluster-management/submariner-addon/pkg/apis/submarinerconfig/v1alpha1"
	fakeconfigclient "github.com/open-cluster-management/submariner-addon/pkg/client/submarinerconfig/clientset/versioned/fake"
	testinghelpers "github.com/open-cluster-management/submariner-addon/pkg/helpers/testing"
	addonv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	addonfake "open-cluster-management.io/api/client/addon/clientset/versioned/fake"
	fakeworkclient "open-cluster-management.io/api/client/work/clientset/versioned/fake"
	clusterv1 "open-cluster-management.io/api/cluster/v1"
	workv1 "open-cluster-management.io/api/work/v1"

	"github.com/openshift/library-go/pkg/operator/events/eventstesting"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	clienttesting "k8s.io/client-go/testing"
)

func TestUpdateStatusCondition(t *testing.T) {
	nowish := metav1.Now()
	beforeish := metav1.Time{Time: nowish.Add(-10 * time.Second)}
	afterish := metav1.Time{Time: nowish.Add(10 * time.Second)}

	cases := []struct {
		name               string
		startingConditions []metav1.Condition
		newCondition       metav1.Condition
		expextedUpdated    bool
		expectedConditions []metav1.Condition
	}{
		{
			name:               "add to empty",
			startingConditions: []metav1.Condition{},
			newCondition:       testinghelpers.NewSubmarinerConfigCondition("test", "True", "my-reason", "my-message", nil),
			expextedUpdated:    true,
			expectedConditions: []metav1.Condition{testinghelpers.NewSubmarinerConfigCondition("test", "True", "my-reason", "my-message", nil)},
		},
		{
			name: "add to non-conflicting",
			startingConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
			},
			newCondition:    testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", nil),
			expextedUpdated: true,
			expectedConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
				testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", nil),
			},
		},
		{
			name: "change existing status",
			startingConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
				testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", nil),
			},
			newCondition:    testinghelpers.NewSubmarinerConfigCondition("one", "False", "my-different-reason", "my-othermessage", nil),
			expextedUpdated: true,
			expectedConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
				testinghelpers.NewSubmarinerConfigCondition("one", "False", "my-different-reason", "my-othermessage", nil),
			},
		},
		{
			name: "leave existing transition time",
			startingConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
				testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", &beforeish),
			},
			newCondition:    testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", &afterish),
			expextedUpdated: false,
			expectedConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
				testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", &beforeish),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeClusterClient := fakeconfigclient.NewSimpleClientset(&configv1alpha1.SubmarinerConfig{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "testconfig",
					Namespace: "cluster1",
				},
				Status: configv1alpha1.SubmarinerConfigStatus{
					Conditions: c.startingConditions,
				},
			})

			status, updated, err := UpdateSubmarinerConfigStatus(
				fakeClusterClient,
				"cluster1", "testconfig",
				UpdateSubmarinerConfigConditionFn(c.newCondition),
			)
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
			if updated != c.expextedUpdated {
				t.Errorf("expected %t, but %t", c.expextedUpdated, updated)
			}
			for i := range c.expectedConditions {
				expected := c.expectedConditions[i]
				actual := status.Conditions[i]
				if expected.LastTransitionTime == (metav1.Time{}) {
					actual.LastTransitionTime = metav1.Time{}
				}
				if !equality.Semantic.DeepEqual(expected, actual) {
					t.Errorf(diff.ObjectDiff(expected, actual))
				}
			}
		})
	}
}

func TestUpdateManagedClusterAddOnStatus(t *testing.T) {
	nowish := metav1.Now()
	beforeish := metav1.Time{Time: nowish.Add(-10 * time.Second)}
	afterish := metav1.Time{Time: nowish.Add(10 * time.Second)}

	cases := []struct {
		name               string
		startingConditions []metav1.Condition
		newCondition       metav1.Condition
		expextedUpdated    bool
		expectedConditions []metav1.Condition
	}{
		{
			name:               "add to empty",
			startingConditions: []metav1.Condition{},
			newCondition:       testinghelpers.NewSubmarinerConfigCondition("test", "True", "my-reason", "my-message", nil),
			expextedUpdated:    true,
			expectedConditions: []metav1.Condition{testinghelpers.NewSubmarinerConfigCondition("test", "True", "my-reason", "my-message", nil)},
		},
		{
			name: "add to non-conflicting",
			startingConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
			},
			newCondition:    testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", nil),
			expextedUpdated: true,
			expectedConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
				testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", nil),
			},
		},
		{
			name: "change existing status",
			startingConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
				testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", nil),
			},
			newCondition:    testinghelpers.NewSubmarinerConfigCondition("one", "False", "my-different-reason", "my-othermessage", nil),
			expextedUpdated: true,
			expectedConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
				testinghelpers.NewSubmarinerConfigCondition("one", "False", "my-different-reason", "my-othermessage", nil),
			},
		},
		{
			name: "leave existing transition time",
			startingConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
				testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", &beforeish),
			},
			newCondition:    testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", &afterish),
			expextedUpdated: false,
			expectedConditions: []metav1.Condition{
				testinghelpers.NewSubmarinerConfigCondition("two", "True", "my-reason", "my-message", nil),
				testinghelpers.NewSubmarinerConfigCondition("one", "True", "my-reason", "my-message", &beforeish),
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeAddOnClient := addonfake.NewSimpleClientset(&addonv1alpha1.ManagedClusterAddOn{
				ObjectMeta: metav1.ObjectMeta{Namespace: "test", Name: "test"},
				Status: addonv1alpha1.ManagedClusterAddOnStatus{
					Conditions: c.startingConditions,
				},
			})

			status, updated, err := UpdateManagedClusterAddOnStatus(
				context.TODO(),
				fakeAddOnClient,
				"test", "test",
				UpdateManagedClusterAddOnStatusFn(c.newCondition),
			)
			if err != nil {
				t.Errorf("unexpected err: %v", err)
			}
			if updated != c.expextedUpdated {
				t.Errorf("expected %t, but %t", c.expextedUpdated, updated)
			}
			for i := range c.expectedConditions {
				expected := c.expectedConditions[i]
				actual := status.Conditions[i]
				if expected.LastTransitionTime == (metav1.Time{}) {
					actual.LastTransitionTime = metav1.Time{}
				}
				if !equality.Semantic.DeepEqual(expected, actual) {
					t.Errorf(diff.ObjectDiff(expected, actual))
				}
			}
		})
	}
}

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
							"name": "cluster",
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
							"name": "cluster",
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
	os.Setenv("BROKER_API_SERVER", "127.0.0.1:6443")
	defer os.Unsetenv("BROKER_API_SERVER")

	cases := []struct {
		name            string
		brokerNamespace string
		clusterName     string
		kubeObjs        []runtime.Object
		ocpObjs         []runtime.Object
		expectErr       bool
	}{
		{
			name:            "no sa no secret",
			brokerNamespace: "cluster1-broker",
			clusterName:     "cluster1",
			kubeObjs:        []runtime.Object{},
			ocpObjs:         []runtime.Object{},
			expectErr:       true,
		},
		{
			name:            "exist sa no secret",
			brokerNamespace: "cluster1-broker",
			clusterName:     "cluster1",
			kubeObjs: []runtime.Object{
				&corev1.ServiceAccount{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster1",
						Namespace: "cluster1-broker",
					},
				},
			},
			ocpObjs:   []runtime.Object{},
			expectErr: true,
		},
		{
			name:            "exist sa and secret, but the secret cannot be found",
			brokerNamespace: "cluster1-broker",
			clusterName:     "cluster1",
			kubeObjs: []runtime.Object{
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
			ocpObjs:   []runtime.Object{},
			expectErr: true,
		},
		{
			name:            "exist sa and secret",
			brokerNamespace: "cluster1-broker",
			clusterName:     "cluster1",
			kubeObjs: []runtime.Object{
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
			ocpObjs:   []runtime.Object{},
			expectErr: false,
		},
		{
			name:            "get sa from kubeapi server secret",
			brokerNamespace: "cluster1-broker",
			clusterName:     "cluster1",
			kubeObjs: []runtime.Object{
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
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "openshift-config",
					},
					Data: map[string][]byte{
						"tls.crt": []byte("tls"),
					},
					Type: corev1.SecretTypeTLS,
				},
			},
			ocpObjs: []runtime.Object{
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
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "config.openshift.io/v1",
						"kind":       "APIServer",
						"metadata": map[string]interface{}{
							"name": "cluster",
						},
						"spec": map[string]interface{}{
							"servingCerts": map[string]interface{}{
								"namedCertificates": []interface{}{
									map[string]interface{}{
										"names": []interface{}{"api.test.dev04.red-chesterfield.com"},
										"servingCertificate": map[string]interface{}{
											"name": "test",
										},
									},
								},
							},
						},
					},
				},
			},
			expectErr: false,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeKubeClient := kubefake.NewSimpleClientset(c.kubeObjs...)
			fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), c.ocpObjs...)
			token, ca, err := GetBrokerTokenAndCA(fakeKubeClient, fakeDynamicClient, c.brokerNamespace, c.clusterName)
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
		name           string
		clusterName    string
		managedCluster *clusterv1.ManagedCluster
		expectType     string
	}{
		{
			name:        "cluster is OCP",
			clusterName: "cluster1",
			managedCluster: &clusterv1.ManagedCluster{
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
			expectType: "OpenShift",
		},
		{
			name:        "cluster is not OCP",
			clusterName: "cluster1",
			managedCluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
				},
				Status: clusterv1.ManagedClusterStatus{
					ClusterClaims: []clusterv1.ManagedClusterClaim{
						{
							Name:  "product.open-cluster-management.io",
							Value: "others",
						},
					},
				},
			},
			expectType: "others",
		},
		{
			name:        "cluster has no vendor",
			clusterName: "cluster1",
			managedCluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "cluster1",
				},
			},
			expectType: "",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			clusterType := GetClusterProduct(c.managedCluster)
			if clusterType != c.expectType {
				t.Errorf("expect %s, but %s", c.expectType, clusterType)
			}
		})
	}
}

func TestGetManagedClusterInfo(t *testing.T) {
	cases := []struct {
		name           string
		managedCluster *clusterv1.ManagedCluster
		expected       configv1alpha1.ManagedClusterInfo
	}{
		{
			name: "no claims",
			managedCluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			},
			expected: configv1alpha1.ManagedClusterInfo{
				ClusterName: "test",
			},
		},
		{
			name: "has claims",
			managedCluster: &clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
				Status: clusterv1.ManagedClusterStatus{
					ClusterClaims: []clusterv1.ManagedClusterClaim{
						{
							Name:  "product.open-cluster-management.io",
							Value: "OpenShift",
						},
						{
							Name:  "platform.open-cluster-management.io",
							Value: "AWS",
						},
						{
							Name:  "region.open-cluster-management.io",
							Value: "us-east-1",
						},
						{
							Name:  "infrastructure.openshift.io",
							Value: "{\"infraName\":\"cluster-1234\"}",
						},
					},
				},
			},
			expected: configv1alpha1.ManagedClusterInfo{
				ClusterName: "test",
				Vendor:      "OpenShift",
				Platform:    "AWS",
				Region:      "us-east-1",
				InfraId:     "cluster-1234",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			info := GetManagedClusterInfo(c.managedCluster)
			if !reflect.DeepEqual(info, c.expected) {
				t.Errorf("expect %v, but %s", c.expected, info)
			}
		})
	}
}

func TestApplyManifestWork(t *testing.T) {
	cases := []struct {
		name            string
		existingWorks   []runtime.Object
		work            *workv1.ManifestWork
		validateActions func(t *testing.T, workActions []clienttesting.Action)
	}{
		{
			name:          "create a work",
			existingWorks: []runtime.Object{},
			work: &workv1.ManifestWork{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: workv1.ManifestWorkSpec{
					Workload: workv1.ManifestsTemplate{
						Manifests: []workv1.Manifest{},
					},
				},
			},
			validateActions: func(t *testing.T, workActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, workActions, "get", "create")
			},
		},
		{
			name: "update a work",
			existingWorks: []runtime.Object{
				&workv1.ManifestWork{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Spec: workv1.ManifestWorkSpec{
						Workload: workv1.ManifestsTemplate{
							Manifests: []workv1.Manifest{
								{
									RawExtension: runtime.RawExtension{
										Raw: []byte("test1"),
									},
								},
							},
						},
					},
				},
			},
			work: &workv1.ManifestWork{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: workv1.ManifestWorkSpec{
					Workload: workv1.ManifestsTemplate{
						Manifests: []workv1.Manifest{
							{
								RawExtension: runtime.RawExtension{
									Raw: []byte("test2"),
								},
							},
						},
					},
				},
			},
			validateActions: func(t *testing.T, workActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, workActions, "get", "update")
			},
		},
		{
			name: "update a same work",
			existingWorks: []runtime.Object{
				&workv1.ManifestWork{
					TypeMeta: metav1.TypeMeta{},
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test",
						Namespace: "test",
					},
					Spec: workv1.ManifestWorkSpec{
						Workload: workv1.ManifestsTemplate{
							Manifests: []workv1.Manifest{
								{
									RawExtension: runtime.RawExtension{
										Raw: []byte("test"),
									},
								},
							},
						},
					},
				},
			},
			work: &workv1.ManifestWork{
				TypeMeta: metav1.TypeMeta{},
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "test",
				},
				Spec: workv1.ManifestWorkSpec{
					Workload: workv1.ManifestsTemplate{
						Manifests: []workv1.Manifest{
							{
								RawExtension: runtime.RawExtension{
									Raw: []byte("test"),
								},
							},
						},
					},
				},
			},
			validateActions: func(t *testing.T, workActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, workActions, "get")
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			workClient := fakeworkclient.NewSimpleClientset(c.existingWorks...)
			err := ApplyManifestWork(context.TODO(), workClient, c.work, eventstesting.NewTestingEventRecorder(t))
			if err != nil {
				t.Errorf("expect no err, but got: %v", err)
			}
			c.validateActions(t, workClient.Actions())
		})
	}
}

func TestGetEnv(t *testing.T) {
	os.Setenv("test_env", "test_val")
	defer os.Unsetenv("test_env")

	cases := []struct {
		name          string
		envKey        string
		defaultValue  string
		expectedValue string
	}{
		{
			name:          "env exists",
			envKey:        "test_env",
			expectedValue: "test_val",
		},
		{
			name:          "env does not exist",
			envKey:        "nonexistent",
			defaultValue:  "default_val",
			expectedValue: "default_val",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			value := GetEnv(c.envKey, c.defaultValue)
			if value != c.expectedValue {
				t.Errorf("expect %v, but got: %v", c.expectedValue, value)
			}
		})
	}
}

func TestGernerateBrokerName(t *testing.T) {
	cases := []struct {
		name           string
		clusterSetName string
		expectedLength int
	}{
		{
			name:           "short name",
			clusterSetName: "test-clustr-set",
			expectedLength: len("test-clustr-set-broker"),
		},
		{
			name:           "long name",
			clusterSetName: "clusterset-geographically-distributed-workloads-long-set-name",
			expectedLength: 63,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			actual := GernerateBrokerName(c.clusterSetName)
			if len(actual) != c.expectedLength {
				t.Errorf("expected %d, but: %d, %q", c.expectedLength, len(actual), actual)
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
