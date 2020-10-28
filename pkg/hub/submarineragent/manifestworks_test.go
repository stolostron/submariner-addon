package submarineragent

import (
	"context"
	fakeclusterclient "github.com/open-cluster-management/api/client/cluster/clientset/versioned/fake"
	fakeworkclient "github.com/open-cluster-management/api/client/work/clientset/versioned/fake"
	clusterv1 "github.com/open-cluster-management/api/cluster/v1"
	"github.com/open-cluster-management/submariner-addon/pkg/helpers"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kubefake "k8s.io/client-go/kubernetes/fake"
	"testing"
)

func TestWrapManifestWorks(t *testing.T) {
	cases := []struct {
		name              string
		clusterName       string
		brokerName        string
		existings         []runtime.Object
		clusters          []runtime.Object
		dynamicExsistings []runtime.Object
		expectErr         bool
	}{
		{
			name:        "apply ok",
			clusterName: "cluster1",
			brokerName:  "broker1",
			expectErr:   false,
			existings: []runtime.Object{
				&corev1.Secret{
					ObjectMeta: metav1.ObjectMeta{
						Name:      helpers.IPSecPSKSecretName,
						Namespace: "broker1",
					},
					Data: map[string][]byte{
						"psk": []byte("PkEgFvaFTwzJtuqcZcYiTkrEizUKiF8ln7gPrJt3EApnpW32vSpulkP8i6h1fxRh"),
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
						"ca.crt": []byte("LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCk1JSUN5RENDQWJDZ0F3SUJBZ0lCQURBTkJna3Foa2lHOXcwQkFRc0ZBREFWTVJNd0VRWURWUVFERXdwcmRXSmwKY201bGRHVnpNQjRYRFRJd01Ea3hPVEUyTlRJeU5Gb1hEVE13TURreE56RTJOVEl5TkZvd0ZURVRNQkVHQTFVRQpBeE1LYTNWaVpYSnVaWFJsY3pDQ0FTSXdEUVlKS29aSWh2Y05BUUVCQlFBRGdnRVBBRENDQVFvQ2dnRUJBTWxYCkJnWjZuMlpTZ2I4SjVRTlREUmgxUmhKdHdjbytmRVpGWnRDQk1jY1RHVExqTXVGZUpyOUw0N2xsbCtaSnNudE0KR0FPNmZ5RzRaT3FFY0swRUJkZHMybGpmazc3TXIrcWpORFpBcXJHREcrVE5OVDVCUUk1ekJNdS9DdkFRU0NnagpaWW5mckRjUHdHK1FjajA1aFNncUNSK1AzVFhXZ3ZPdnp1V05aZTBHTDlBR0dnZnVpUG14Nlo5OFNVdDN6dVVoCllLM2lWTEZiVGhCQ01zL0ZJdDE2TlFIaERqeWdURWZpdVovTTNRVWZ0MnlKMC9CenNvUGpDME5KM3ZIcEV3VGsKd2pwSktCTStkdHNtZ3JlcGt6S05MdmozcjVTMERwcnUxbmRydU5VU3pCcnlYdWFxUTNlNWY3OGxISnZHQnRLVApRS3VFaUo0TFRtZVMzL0V5ejhVQ0F3RUFBYU1qTUNFd0RnWURWUjBQQVFIL0JBUURBZ0trTUE4R0ExVWRFd0VCCi93UUZNQU1CQWY4d0RRWUpLb1pJaHZjTkFRRUxCUUFEZ2dFQkFHR0I5K21wSnJaby9oR0E1NkwycVBHSFhkb0gKY1hiVXR0b3NHczdGZVJTd3BWMkI4VWdNT2dRTjhDYWdWOWg0YndoSWQ2ZU4vbmZyaTdMMTZzelNSbWsxUDI4NQo2emhhZUJjSERpRDhIcnFnK290RVBwMDA4emdVaVJHbWNuSjF0dlJWRGJOc04wVFBNa2MwTWgwU3hNcFhkallICmsyaGdIRVNNVGVPQU4wN2ZqWDFPcVFLSm5EdTAyY083cXo3WjhYQmc3czNhRHZNRDM0bUZ2R3Qrc2hBSGF1cGsKUW03UkhxWXYyNVVBM25mbWU1cGxTTDdwRWZ4SGJxRkdmM3ZBV1ZhME9hS0VuMXByR2Y4MStZK2IvYkdYQW4yYgo0d2FoTlRxN0dWeXprN2o4clR0a1Q0MW4rU3RFbjdPMVVUTjBKYjQvV2svcFZrclZEbDlwU3pGcDBMVT0KLS0tLS1FTkQgQ0VSVElGSUNBVEUtLS0tLQo="),
						"token":  []byte("ZXlKaGJHY2lPaUpTVXpJMU5pSXNJbXRwWkNJNklqRm5iek16Y0U1aFlXNUJUSEl4UWxkeWFuaE5Na0pXTkdKM00wbG9XWFpSWVV4blV6QkpRVUk1T0ZVaWZRLmV5SnBjM01pT2lKcmRXSmxjbTVsZEdWekwzTmxjblpwWTJWaFkyTnZkVzUwSWl3aWEzVmlaWEp1WlhSbGN5NXBieTl6WlhKMmFXTmxZV05qYjNWdWRDOXVZVzFsYzNCaFkyVWlPaUp6ZFdKdFlYSnBibVZ5TFdzNGN5MWljbTlyWlhJaUxDSnJkV0psY201bGRHVnpMbWx2TDNObGNuWnBZMlZoWTJOdmRXNTBMM05sWTNKbGRDNXVZVzFsSWpvaWMzVmliV0Z5YVc1bGNpMXJPSE10WW5KdmEyVnlMV0ZrYldsdUxYUnZhMlZ1TFhkdGJqUnRJaXdpYTNWaVpYSnVaWFJsY3k1cGJ5OXpaWEoyYVdObFlXTmpiM1Z1ZEM5elpYSjJhV05sTFdGalkyOTFiblF1Ym1GdFpTSTZJbk4xWW0xaGNtbHVaWEl0YXpoekxXSnliMnRsY2kxaFpHMXBiaUlzSW10MVltVnlibVYwWlhNdWFXOHZjMlZ5ZG1salpXRmpZMjkxYm5RdmMyVnlkbWxqWlMxaFkyTnZkVzUwTG5WcFpDSTZJakU0WXpCa1lXUTNMVGd5T1dFdE5EZ3paaTFpT0dJMkxUQXpZbVJsTkRBM1ptUTBPQ0lzSW5OMVlpSTZJbk41YzNSbGJUcHpaWEoyYVdObFlXTmpiM1Z1ZERwemRXSnRZWEpwYm1WeUxXczRjeTFpY205clpYSTZjM1ZpYldGeWFXNWxjaTFyT0hNdFluSnZhMlZ5TFdGa2JXbHVJbjAuV01wUmg0eFJiaWxCWHdwYjRtU1lTVGlVRnJrUW5NODVBWXhJbElpeGdaWlhEeDVhLU9wQlR2dEZPVmNYRXJFYm5qTnpXNlZNbk9GMmJfdHo4MXdtTTR5OVFLcWVHcTR5Rk9VWGlMZW9FZ3RhYXZ2ZDZBdTZFTU1Cb3NRQU9QZjFibU9wU0kyVldBR2NrcVhWcVY4bkpscklGWnJ3czM5WmFtLW5FZVpiLUFrR3ZnT01DYXBKLUNZRlMyQlZhWUIyX2ExeGhqQ2lTZ3luT2UtOFc2aE80bkhZS2ttdFhtMjZlYVZJZDFoUTFpVHJrajZ0Q0J3MEZjdEstREZxV2dHTzFnMkZJUlFDQlQza3VBZW9YU043TU1RbUpPaFRQNkozTFo3UTliY21LcGFqYV9Fc1RhRDFsTTR6SXFKVnFMYU9Rd0VLVUZIM1A0SHp1Zzl2NlpwRkhn"),
					},
					Type: corev1.SecretTypeServiceAccountToken,
				},
			},
			clusters: []runtime.Object{
				&clusterv1.ManagedCluster{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster1",
						Labels: map[string]string{
							"vendor": "OpenShift",
						},
					},
				},
			},
			dynamicExsistings: []runtime.Object{
				&unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "config.openshift.io/v1",
						"kind":       "Infrastructure",
						"metadata": map[string]interface{}{
							"name": helpers.InfrastructureConfigName,
						},
						"status": map[string]interface{}{
							"apiServerURL": "https://api.test.dev04.red-chesterfield.com:6443",
						},
					},
				},
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			fakeClient := kubefake.NewSimpleClientset(c.existings...)
			fakeDynamicClient := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), c.dynamicExsistings...)
			fakeWorkClient := fakeworkclient.NewSimpleClientset()
			fakeClusterClient := fakeclusterclient.NewSimpleClientset(c.clusters...)
			err := ApplySubmarinerManifestWorks(fakeClient, fakeDynamicClient, fakeWorkClient, fakeClusterClient, c.clusterName, c.brokerName, context.TODO())
			if err != nil && !c.expectErr {
				t.Errorf("expect no err: %v", err)
			}
			if err == nil && c.expectErr {
				t.Errorf("expect err")
			}
		})
	}
}
