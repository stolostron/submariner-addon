package azure

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
)

const servicePrincipalJSONData = `
{
  "clientId": "my-client-id",
  "clientSecret": "my-client-secret",
  "tenantId": "my-tenant-id",
  "subscriptionId": "my-subscription-id"
}`

var _ = Describe("initializeFromAuthFile", func() {
	It("should correctly parse the credentials Secret data", func() {
		subscriptionID, err := initializeFromAuthFile(&corev1.Secret{
			Data: map[string][]byte{
				servicePrincipalJSON: []byte(servicePrincipalJSONData),
			},
		})
		Expect(err).NotTo(HaveOccurred())

		Expect(subscriptionID).To(Equal("my-subscription-id"))
		Expect(os.Getenv("AZURE_CLIENT_ID")).To(Equal("my-client-id"))
		Expect(os.Getenv("AZURE_CLIENT_SECRET")).To(Equal("my-client-secret"))
		Expect(os.Getenv("AZURE_TENANT_ID")).To(Equal("my-tenant-id"))
	})

	It("should return an error if the service principal JSON is missing", func() {
		_, err := initializeFromAuthFile(&corev1.Secret{})
		Expect(err).To(HaveOccurred())
	})

	It("should return an error if the service principal JSON is invalid", func() {
		_, err := initializeFromAuthFile(&corev1.Secret{
			Data: map[string][]byte{
				servicePrincipalJSON: []byte("invalid"),
			},
		})
		Expect(err).To(HaveOccurred())
	})
})
