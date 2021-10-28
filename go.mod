module github.com/open-cluster-management/submariner-addon

go 1.16

require (
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/golang/mock v1.6.0
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.16.0
	github.com/openshift/api v0.0.0-20210521075222-e273a339932a
	github.com/openshift/build-machinery-go v0.0.0-20210922160744-a9caf93aef90
	github.com/openshift/library-go v0.0.0-20210609150209-1c980926414c
	github.com/operator-framework/api v0.5.2
	github.com/pkg/errors v0.9.1
	github.com/prometheus/common v0.30.0 // indirect
	github.com/prometheus/procfs v0.7.2 // indirect
	github.com/spf13/cobra v1.1.3
	github.com/spf13/pflag v1.0.5
	github.com/submariner-io/admiral v0.11.0
	github.com/submariner-io/cloud-prepare v0.11.0-m2.0.20210920144917-0b33718646a7
	github.com/submariner-io/submariner-operator/apis v0.0.0-20210817145008-861856b068a1
	github.com/submariner-io/submariner/pkg/apis v0.0.0-20210817085048-59f656555db0
	golang.org/x/crypto v0.0.0-20210711020723-a769d52b0f97 // indirect
	golang.org/x/net v0.0.0-20210805182204-aaa1db679c0d // indirect
	golang.org/x/oauth2 v0.0.0-20210810183815-faf39c7919d5
	golang.org/x/sys v0.0.0-20210809222454-d867a43fc93e // indirect
	golang.org/x/term v0.0.0-20210615171337-6886f2dfbf5b // indirect
	golang.org/x/text v0.3.7 // indirect
	google.golang.org/api v0.57.0
	k8s.io/api v0.21.3
	k8s.io/apiextensions-apiserver v0.21.3
	k8s.io/apimachinery v0.21.3
	k8s.io/client-go v0.21.3
	k8s.io/code-generator v0.21.3
	k8s.io/component-base v0.21.3
	k8s.io/klog/v2 v2.8.0
	k8s.io/utils v0.0.0-20210802155522-efc7438f0176 // indirect
	open-cluster-management.io/addon-framework v0.0.0-20210803032803-58eac513499e
	open-cluster-management.io/api v0.0.0-20210927063308-2c6896161c48
	sigs.k8s.io/controller-runtime v0.8.3
)

// ensure compatible between controller-runtime and kube-openapi
replace k8s.io/kube-openapi => k8s.io/kube-openapi v0.0.0-20210305001622-591a79e4bda7

replace github.com/googleapis/gnostic => github.com/googleapis/gnostic v0.4.1

replace (
	golang.org/x/oauth2 => golang.org/x/oauth2 v0.0.0-20200107190931-bf48bf16ab8d
	google.golang.org/api => google.golang.org/api v0.29.0
)
