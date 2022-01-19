module github.com/stolostron/submariner-addon

go 1.16

require (
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/golang/mock v1.6.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.17.0
	github.com/openshift/api v0.0.0-20210915110300-3cd8091317c4
	github.com/openshift/build-machinery-go v0.0.0-20210922160744-a9caf93aef90
	github.com/openshift/library-go v0.0.0-20210916194400-ae21aab32431
	github.com/operator-framework/api v0.5.2
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.30.0 // indirect
	github.com/prometheus/procfs v0.7.2 // indirect
	github.com/spf13/cobra v1.3.0
	github.com/spf13/pflag v1.0.5
	github.com/submariner-io/admiral v0.12.0-m3
	github.com/submariner-io/cloud-prepare v0.12.0-m3.0.20220117145104-8c71f70f26cb
	github.com/submariner-io/submariner v0.12.0-m3.0.20220118140616-86d7b7924805
	github.com/submariner-io/submariner-operator v0.12.0-m3.0.20220118152331-5eb19252606e
	golang.org/x/oauth2 v0.0.0-20211104180415-d3ed0bb246c8
	google.golang.org/api v0.65.0
	k8s.io/api v0.22.1
	k8s.io/apiextensions-apiserver v0.22.1
	k8s.io/apimachinery v0.22.1
	k8s.io/client-go v12.0.0+incompatible
	k8s.io/code-generator v0.22.1
	k8s.io/component-base v0.22.1
	k8s.io/klog/v2 v2.9.0
	k8s.io/utils v0.0.0-20210802155522-efc7438f0176 // indirect
	open-cluster-management.io/addon-framework v0.1.0
	open-cluster-management.io/api v0.5.0
	sigs.k8s.io/controller-runtime v0.8.3
)

replace (
	golang.org/x/oauth2 => golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	google.golang.org/api => google.golang.org/api v0.29.0
	k8s.io/client-go => k8s.io/client-go v0.22.1
)
