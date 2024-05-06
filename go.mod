module github.com/stolostron/submariner-addon

go 1.18

require (
	github.com/Azure/azure-sdk-for-go/sdk/azidentity v1.5.2
	github.com/coreos/go-semver v0.3.1
	github.com/ghodss/yaml v1.0.1-0.20190212211648-25d852aebe32
	github.com/golang/mock v1.6.0
	github.com/gophercloud/gophercloud v1.11.0
	github.com/gophercloud/utils v0.0.0-20210909165623-d7085207ff6d
	github.com/onsi/ginkgo v1.16.5
	github.com/onsi/gomega v1.31.1
	github.com/openshift/api v0.0.0-20220525145417-ee5b62754c68
	github.com/openshift/build-machinery-go v0.0.0-20220913142420-e25cf57ea46d
	github.com/openshift/library-go v0.0.0-20220525173854-9b950a41acdc
	github.com/operator-framework/api v0.17.3
	github.com/pkg/errors v0.9.1
	github.com/spf13/cobra v1.8.0
	github.com/spf13/pflag v1.0.5
	github.com/submariner-io/admiral v0.14.7
	github.com/submariner-io/cloud-prepare v0.14.7
	github.com/submariner-io/submariner v0.14.7
	github.com/submariner-io/submariner-operator v0.14.7
	golang.org/x/oauth2 v0.20.0
	google.golang.org/api v0.153.0
	gopkg.in/yaml.v2 v2.4.0
	k8s.io/api v0.25.14
	k8s.io/apiextensions-apiserver v0.25.14
	k8s.io/apimachinery v0.25.14
	k8s.io/client-go v0.25.14
	k8s.io/code-generator v0.25.14
	k8s.io/component-base v0.25.14
	k8s.io/klog/v2 v2.80.1
	open-cluster-management.io/addon-framework v0.5.0
	open-cluster-management.io/api v0.9.0
	sigs.k8s.io/controller-runtime v0.13.1
)

require (
	cloud.google.com/go/compute/metadata v0.3.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/azcore v1.11.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/internal v1.5.2 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute v1.0.0 // indirect
	github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork v1.1.0 // indirect
	github.com/AzureAD/microsoft-authentication-library-for-go v1.2.2 // indirect
	github.com/NYTimes/gziphandler v1.1.1 // indirect
	github.com/aws/aws-sdk-go-v2 v1.17.0 // indirect
	github.com/aws/aws-sdk-go-v2/config v1.17.9 // indirect
	github.com/aws/aws-sdk-go-v2/credentials v1.12.22 // indirect
	github.com/aws/aws-sdk-go-v2/feature/ec2/imds v1.12.18 // indirect
	github.com/aws/aws-sdk-go-v2/internal/configsources v1.1.24 // indirect
	github.com/aws/aws-sdk-go-v2/internal/endpoints/v2 v2.4.18 // indirect
	github.com/aws/aws-sdk-go-v2/internal/ini v1.3.25 // indirect
	github.com/aws/aws-sdk-go-v2/service/ec2 v1.63.2 // indirect
	github.com/aws/aws-sdk-go-v2/service/internal/presigned-url v1.9.18 // indirect
	github.com/aws/aws-sdk-go-v2/service/sso v1.11.24 // indirect
	github.com/aws/aws-sdk-go-v2/service/ssooidc v1.13.7 // indirect
	github.com/aws/aws-sdk-go-v2/service/sts v1.17.0 // indirect
	github.com/aws/smithy-go v1.13.3 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/blang/semver/v4 v4.0.0 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/coreos/go-systemd/v22 v22.3.3-0.20220203105225-a9a7ef127534 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/emicklei/go-restful/v3 v3.9.0 // indirect
	github.com/evanphx/json-patch v5.6.0+incompatible // indirect
	github.com/evanphx/json-patch/v5 v5.6.0 // indirect
	github.com/felixge/httpsnoop v1.0.1 // indirect
	github.com/fsnotify/fsnotify v1.5.4 // indirect
	github.com/go-logr/logr v1.3.0 // indirect
	github.com/go-logr/zapr v1.2.3 // indirect
	github.com/go-openapi/jsonpointer v0.19.5 // indirect
	github.com/go-openapi/jsonreference v0.20.0 // indirect
	github.com/go-openapi/swag v0.22.3 // indirect
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/golang-jwt/jwt/v5 v5.2.1 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/gnostic v0.6.9 // indirect
	github.com/google/go-cmp v0.6.0 // indirect
	github.com/google/gofuzz v1.2.0 // indirect
	github.com/google/s2a-go v0.1.7 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/googleapis/enterprise-certificate-proxy v0.3.2 // indirect
	github.com/googleapis/gax-go/v2 v2.12.0 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway v1.16.0 // indirect
	github.com/imdario/mergo v0.3.12 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/json-iterator/go v1.1.12 // indirect
	github.com/kylelemons/godebug v1.1.0 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.2-0.20181231171920-c182affec369 // indirect
	github.com/mitchellh/go-homedir v1.1.0 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v1.0.2 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/nxadm/tail v1.4.8 // indirect
	github.com/openshift/client-go v0.0.0-20220525160904-9e1acff93e4a // indirect
	github.com/pkg/browser v0.0.0-20240102092130-5ac0b6a4141c // indirect
	github.com/pkg/profile v1.3.0 // indirect
	github.com/prometheus/client_golang v1.13.0 // indirect
	github.com/prometheus/client_model v0.2.0 // indirect
	github.com/prometheus/common v0.37.0 // indirect
	github.com/prometheus/procfs v0.8.0 // indirect
	github.com/robfig/cron v1.2.0 // indirect
	github.com/sirupsen/logrus v1.8.1 // indirect
	go.etcd.io/etcd/api/v3 v3.5.4 // indirect
	go.etcd.io/etcd/client/pkg/v3 v3.5.4 // indirect
	go.etcd.io/etcd/client/v3 v3.5.4 // indirect
	go.opencensus.io v0.24.0 // indirect
	go.opentelemetry.io/contrib v0.20.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.20.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.20.0 // indirect
	go.opentelemetry.io/otel v0.20.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp v0.20.0 // indirect
	go.opentelemetry.io/otel/metric v0.20.0 // indirect
	go.opentelemetry.io/otel/sdk v0.20.0 // indirect
	go.opentelemetry.io/otel/sdk/export/metric v0.20.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v0.20.0 // indirect
	go.opentelemetry.io/otel/trace v0.20.0 // indirect
	go.opentelemetry.io/proto/otlp v0.7.0 // indirect
	go.uber.org/atomic v1.9.0 // indirect
	go.uber.org/multierr v1.6.0 // indirect
	go.uber.org/zap v1.21.0 // indirect
	golang.org/x/crypto v0.21.0 // indirect
	golang.org/x/mod v0.14.0 // indirect
	golang.org/x/net v0.22.0 // indirect
	golang.org/x/sync v0.5.0 // indirect
	golang.org/x/sys v0.18.0 // indirect
	golang.org/x/term v0.18.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/time v0.5.0 // indirect
	golang.org/x/tools v0.16.1 // indirect
	google.golang.org/genproto v0.0.0-20231106174013-bbf56f31fb17 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20231106174013-bbf56f31fb17 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231120223509-83a465c0220f // indirect
	google.golang.org/grpc v1.59.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/inf.v0 v0.9.1 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
	gopkg.in/tomb.v1 v1.0.0-20141024135613-dd632973f1e7 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	k8s.io/apiserver v0.25.14 // indirect
	k8s.io/gengo v0.0.0-20211129171323-c02415ce4185 // indirect
	k8s.io/klog v1.0.0 // indirect
	k8s.io/kube-aggregator v0.24.1 // indirect
	k8s.io/kube-openapi v0.0.0-20220803164354-a70c9af30aea // indirect
	k8s.io/utils v0.0.0-20220823124924-e9cbc92d1a73 // indirect
	sigs.k8s.io/apiserver-network-proxy/konnectivity-client v0.0.37 // indirect
	sigs.k8s.io/json v0.0.0-20220713155537-f223a00ba0e2 // indirect
	sigs.k8s.io/kube-storage-version-migrator v0.0.4 // indirect
	sigs.k8s.io/structured-merge-diff/v4 v4.2.3 // indirect
	sigs.k8s.io/yaml v1.3.0 // indirect
)
