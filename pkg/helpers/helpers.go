package helpers

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/openshift/api"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/openshift/library-go/pkg/operator/resource/resourceapply"
	"github.com/openshift/library-go/pkg/operator/resource/resourcehelper"
	errorhelpers "github.com/openshift/library-go/pkg/operator/v1helpers"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	"crypto/rand"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"os"
	"strings"
)

const (
	IPSecPSKSecretLength = 48
	IPSecPSKSecretName   = "submariner-ipsec-psk"
	BrokerAPIServer      = "BROKER_API_SERVER"
)

var (
	genericScheme = runtime.NewScheme()
	genericCodecs = serializer.NewCodecFactory(genericScheme)
	genericCodec  = genericCodecs.UniversalDeserializer()
)

func init() {
	utilruntime.Must(api.InstallKube(genericScheme))
}

// CleanUpSubmarinerManifests clean up submariner resources from its manifest files
func CleanUpSubmarinerManifests(
	ctx context.Context,
	client kubernetes.Interface,
	recorder events.Recorder,
	assetFunc resourceapply.AssetFunc,
	files ...string) error {
	errs := []error{}
	for _, file := range files {
		objectRaw, err := assetFunc(file)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		object, _, err := genericCodec.Decode(objectRaw, nil, nil)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		switch t := object.(type) {
		case *corev1.Namespace:
			err = client.CoreV1().Namespaces().Delete(ctx, t.Name, metav1.DeleteOptions{})
		case *rbacv1.Role:
			err = client.RbacV1().Roles(t.Namespace).Delete(ctx, t.Name, metav1.DeleteOptions{})
		case *rbacv1.RoleBinding:
			err = client.RbacV1().RoleBindings(t.Namespace).Delete(ctx, t.Name, metav1.DeleteOptions{})
		case *corev1.ServiceAccount:
			err = client.CoreV1().ServiceAccounts(t.Namespace).Delete(ctx, t.Name, metav1.DeleteOptions{})
		default:
			err = fmt.Errorf("unhandled type %T", object)
		}
		if errors.IsNotFound(err) {
			continue
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		gvk := resourcehelper.GuessObjectGroupVersionKind(object)
		recorder.Eventf(fmt.Sprintf("Submariner%sDeleted", gvk.Kind), "Deleted %s", resourcehelper.FormatResourceForCLIWithNamespace(object))
	}
	return errorhelpers.NewMultiLineAggregate(errs)
}

func GenerateIPSecPSKSecret(client kubernetes.Interface, brokerNamespace string) error {
	_, err := client.CoreV1().Secrets(brokerNamespace).Get(context.TODO(), IPSecPSKSecretName, metav1.GetOptions{})
	switch {
	case errors.IsNotFound(err):
		psk := make([]byte, IPSecPSKSecretLength)
		if _, err := rand.Read(psk); err != nil {
			return err
		}
		pskSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name: IPSecPSKSecretName,
			},
			Data: map[string][]byte{
				"psk": psk,
			},
		}
		_, err := client.CoreV1().Secrets(brokerNamespace).Create(context.TODO(), pskSecret, metav1.CreateOptions{})
		return err
	case err != nil:
		return err
	}
	return nil
}

func GetIPSecPSK(client kubernetes.Interface, brokerNamespace string) (string, error) {
	secret, err := client.CoreV1().Secrets(brokerNamespace).Get(context.TODO(), IPSecPSKSecretName, metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to broker IPSECPSK secret %v/%v: %v", brokerNamespace, IPSecPSKSecretName, err)
	}

	return base64.StdEncoding.EncodeToString(secret.Data["psk"]), nil
}

var (
	InfrastructureConfigName                             = "cluster"
	infrastructureGVR        schema.GroupVersionResource = schema.GroupVersionResource{
		Group:    "config.openshift.io",
		Version:  "v1",
		Resource: "infrastructures",
	}
)

func GetBrokerAPIServer(dynamicClient dynamic.Interface) (string, error) {
	infrastructureConfig, err := dynamicClient.Resource(infrastructureGVR).Get(context.TODO(), InfrastructureConfigName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			apiServer := os.Getenv(BrokerAPIServer)
			if apiServer == "" {
				return "", fmt.Errorf("failed to get apiserver in env %v", BrokerAPIServer)
			}
			return apiServer, nil
		}
		return "", fmt.Errorf("failed to get infrastructures cluster: %v", err)
	}

	apiServer, found, err := unstructured.NestedString(infrastructureConfig.Object, "status", "apiServerURL")
	if err != nil || !found {
		return "", fmt.Errorf("failed to get apiServerURL in infrastructures cluster:%v,%v", found, err)
	}

	return strings.Trim(apiServer, "https://"), nil
}

func GetBrokerTokenAndCA(client kubernetes.Interface, brokerNS, clusterName string) (token, ca string, err error) {
	sa, err := client.CoreV1().ServiceAccounts(brokerNS).Get(context.TODO(), clusterName, metav1.GetOptions{})
	if err != nil {
		return "", "", fmt.Errorf("failed to get agent ServiceAccount %v/%v: %v", brokerNS, clusterName, err)
	}
	if len(sa.Secrets) < 1 {
		return "", "", fmt.Errorf("ServiceAccount %v does not have any secret", sa.Name)
	}

	brokerTokenPrefix := fmt.Sprintf("%s-token-", clusterName)
	for _, secret := range sa.Secrets {
		if strings.HasPrefix(secret.Name, brokerTokenPrefix) {
			tokenSecret, err := client.CoreV1().Secrets(brokerNS).Get(context.TODO(), secret.Name, metav1.GetOptions{})
			if err != nil {
				return "", "", fmt.Errorf("failed to get secret %v of aget ServiceAccount %v/%v: %v", secret.Name, brokerNS, clusterName, err)
			}

			if tokenSecret.Type == corev1.SecretTypeServiceAccountToken {
				ca := base64.StdEncoding.EncodeToString(tokenSecret.Data["ca.crt"])
				return string(tokenSecret.Data["token"]), ca, nil
			}
		}
	}

	return "", "", fmt.Errorf("ServiceAccount %v/%v does not have a secret of type token", brokerNS, clusterName)

}
