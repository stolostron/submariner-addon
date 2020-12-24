module github.com/open-cluster-management/submariner-addon

go 1.14

require (
	github.com/aws/aws-sdk-go v1.35.32
	github.com/ghodss/yaml v1.0.0
	github.com/go-bindata/go-bindata v3.1.2+incompatible
	github.com/golang/mock v1.3.1
	github.com/onsi/ginkgo v1.14.0
	github.com/onsi/gomega v1.10.1
	github.com/open-cluster-management/api v0.0.0-20201126023000-353dd8370f4d
	github.com/openshift/api v0.0.0-20200827090112-c05698d102cf
	github.com/openshift/build-machinery-go v0.0.0-20200819073603-48aa266c95f7
	github.com/openshift/library-go v0.0.0-20200902171820-35f48b6ef30c
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5
	k8s.io/api v0.19.0
	k8s.io/apimachinery v0.19.0
	k8s.io/client-go v0.19.0
	k8s.io/code-generator v0.19.0
	k8s.io/component-base v0.19.0
	k8s.io/klog/v2 v2.3.0
	sigs.k8s.io/controller-runtime v0.6.1-0.20200829232221-efc74d056b24
)

// ensure compatible between controller-runtime and kube-openapi
replace github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.1
