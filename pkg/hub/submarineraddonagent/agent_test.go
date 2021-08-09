package submarineraddonagent

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/rand"
	"net"
	"testing"

	testinghelpers "github.com/open-cluster-management/submariner-addon/pkg/helpers/testing"
	addonapiv1alpha1 "open-cluster-management.io/api/addon/v1alpha1"
	clusterv1 "open-cluster-management.io/api/cluster/v1"

	"github.com/openshift/library-go/pkg/operator/events/eventstesting"

	certificatesv1 "k8s.io/api/certificates/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubefake "k8s.io/client-go/kubernetes/fake"
)

func TestManifests(t *testing.T) {
	cases := []struct {
		name              string
		addOn             *addonapiv1alpha1.ManagedClusterAddOn
		expectedFileCount int
	}{
		{
			name:              "using default installation namespace",
			addOn:             &addonapiv1alpha1.ManagedClusterAddOn{},
			expectedFileCount: 6,
		},
		{
			name: "using customized installation namespace",
			addOn: &addonapiv1alpha1.ManagedClusterAddOn{
				Spec: addonapiv1alpha1.ManagedClusterAddOnSpec{
					InstallNamespace: "test",
				},
			},
			expectedFileCount: 7,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			agent := &addOnAgent{agentImage: "test"}
			actualFiles, err := agent.Manifests(&clusterv1.ManagedCluster{}, c.addOn)
			if err != nil {
				t.Errorf("unexpected error %v", err)
			}
			if len(actualFiles) != c.expectedFileCount {
				t.Errorf("expected %d files, but get %d, %v", c.expectedFileCount, len(actualFiles), actualFiles)
			}
		})
	}
}

func TestPermissionConfig(t *testing.T) {
	kubeClient := kubefake.NewSimpleClientset()
	agent := NewAddOnAgent(kubeClient, eventstesting.NewTestingEventRecorder(t), "test")
	err := agent.permissionConfig(&clusterv1.ManagedCluster{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	}, &addonapiv1alpha1.ManagedClusterAddOn{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
		},
	})
	if err != nil {
		t.Errorf("unexpected error %v", err)
	}
	testinghelpers.AssertActions(t, kubeClient.Actions(), "get", "create", "get", "create")
}

func TestCSRApproveCheck(t *testing.T) {
	cases := []struct {
		name    string
		csr     csrHolder
		checked bool
	}{
		{
			name: "an invalid signer name",
			csr: csrHolder{
				SignerName: "invalidsigner",
			},
			checked: false,
		},
		{
			name: "a wrong block type",
			csr: csrHolder{
				SignerName:   certificatesv1.KubeAPIServerClientSignerName,
				ReqBlockType: "RSA PRIVATE KEY",
			},
			checked: false,
		},
		{
			name: "an empty organization",
			csr: csrHolder{
				SignerName:   certificatesv1.KubeAPIServerClientSignerName,
				ReqBlockType: "CERTIFICATE REQUEST",
			},
			checked: false,
		},
		{
			name: "an invalid common name",
			csr: csrHolder{
				SignerName:   certificatesv1.KubeAPIServerClientSignerName,
				ReqBlockType: "CERTIFICATE REQUEST",
				Orgs: []string{
					"system:authenticated",
					"system:open-cluster-management:addon:submariner",
					"system:open-cluster-management:cluster:test:addon:submariner",
				},
				CN: "system:open-cluster-management:cluster:test:addon:submariner:agent:invalid",
			},
			checked: false,
		},
		{
			name: "a valid csr",
			csr: csrHolder{
				SignerName:   certificatesv1.KubeAPIServerClientSignerName,
				ReqBlockType: "CERTIFICATE REQUEST",
				Orgs: []string{
					"system:authenticated",
					"system:open-cluster-management:addon:submariner",
					"system:open-cluster-management:cluster:test:addon:submariner",
				},
				CN: "system:open-cluster-management:cluster:test:addon:submariner:agent:submariner-addon-agent",
			},
			checked: true,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			agent := &addOnAgent{}
			if checked := agent.csrApproveCheck(&clusterv1.ManagedCluster{
				ObjectMeta: metav1.ObjectMeta{
					Name: "test",
				},
			}, &addonapiv1alpha1.ManagedClusterAddOn{
				ObjectMeta: metav1.ObjectMeta{
					Name: "submariner-addon",
				},
			}, newCSR(c.csr)); checked != c.checked {
				t.Errorf("expected %t, but failed", c.checked)
			}
		})
	}
}

type csrHolder struct {
	SignerName   string
	CN           string
	Orgs         []string
	ReqBlockType string
}

func newCSR(holder csrHolder) *certificatesv1.CertificateSigningRequest {
	insecureRand := rand.New(rand.NewSource(0))
	pk, err := ecdsa.GenerateKey(elliptic.P256(), insecureRand)
	if err != nil {
		panic(err)
	}
	csrb, err := x509.CreateCertificateRequest(insecureRand, &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:   holder.CN,
			Organization: holder.Orgs,
		},
		DNSNames:       []string{},
		EmailAddresses: []string{},
		IPAddresses:    []net.IP{},
	}, pk)
	if err != nil {
		panic(err)
	}
	return &certificatesv1.CertificateSigningRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name:         "test-csr",
			GenerateName: "csr-",
		},
		Spec: certificatesv1.CertificateSigningRequestSpec{
			Username:   "test",
			Usages:     []certificatesv1.KeyUsage{},
			SignerName: holder.SignerName,
			Request:    pem.EncodeToMemory(&pem.Block{Type: holder.ReqBlockType, Bytes: csrb}),
		},
	}
}
