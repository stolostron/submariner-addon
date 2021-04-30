module github.com/open-cluster-management/submariner-addon

go 1.16

require (
	github.com/aws/aws-sdk-go v1.38.1
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/go-bindata/go-bindata v3.1.2+incompatible
	github.com/golang/mock v1.4.1
	github.com/onsi/ginkgo v1.14.2
	github.com/onsi/gomega v1.10.4
	github.com/open-cluster-management/addon-framework v0.0.0-20210427093923-e978b3b08bf7
	github.com/open-cluster-management/api v0.0.0-20210409125704-06f2aec1a73f
	github.com/openshift/api v0.0.0-20210325044225-ef3741adfc31
	github.com/openshift/build-machinery-go v0.0.0-20210209125900-0da259a2c359
	github.com/openshift/library-go v0.0.0-20210330121802-ebbc677c82a5
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/submariner-io/submariner v0.8.1
	github.com/submariner-io/submariner-operator v0.8.1
	golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	google.golang.org/api v0.20.0
	k8s.io/api v0.20.5
	k8s.io/apiextensions-apiserver v0.20.1
	k8s.io/apimachinery v0.20.5
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/code-generator v0.20.1
	k8s.io/component-base v0.20.5
	k8s.io/klog/v2 v2.8.0
	sigs.k8s.io/controller-runtime v0.8.3
)

// ensure compatible between controller-runtime and kube-openapi
replace github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.1

// ensure compatible with submariner-operator
// TODO if submariner has an independent api repo in future, we can remove this
replace k8s.io/client-go v12.0.0+incompatible => k8s.io/client-go v0.20.5
