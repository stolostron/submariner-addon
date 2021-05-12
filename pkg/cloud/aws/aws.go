package aws

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/route53"
	"github.com/open-cluster-management/submariner-addon/pkg/cloud/reporter"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/klog"

	"path/filepath"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/ghodss/yaml"

	"github.com/openshift/library-go/pkg/assets"
	"github.com/openshift/library-go/pkg/operator/events"

	workclient "github.com/open-cluster-management/api/client/work/clientset/versioned"
	workv1 "github.com/open-cluster-management/api/work/v1"
	"github.com/open-cluster-management/submariner-addon/pkg/cloud/aws/bindata"

	"github.com/open-cluster-management/submariner-addon/pkg/helpers"
	prepareapi "github.com/submariner-io/cloud-prepare/pkg/api"
	prepareaws "github.com/submariner-io/cloud-prepare/pkg/aws"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

const (
	instanceType              = "m5n.large"
	accessKeyIDSecretKey      = "aws_access_key_id"
	accessKeySecretKey        = "aws_secret_access_key"
	internalELBLabel          = "kubernetes.io/role/internal-elb"
	internalGatewayLabel      = "open-cluster-management.io/submariner-addon/gateway"
	workName                  = "aws-submariner-gateway-machineset"
	aggeragateClusterroleFile = "pkg/cloud/aws/manifests/machineset-aggeragate-clusterrole.yaml"
)

type awsProvider struct {
	awsCloud      prepareapi.Cloud
	input         prepareapi.PrepareForSubmarinerInput
	reporter      prepareapi.Reporter
	eventRecorder events.Recorder
}

func NewAWSProvider(
	kubeClient kubernetes.Interface, workClient workclient.Interface,
	eventRecorder events.Recorder,
	region, infraId, clusterName, credentialsSecretName string,
	ikePort, nattPort int) (*awsProvider, error) {
	if region == "" {
		return nil, fmt.Errorf("cluster region is empty")
	}

	if infraId == "" {
		return nil, fmt.Errorf("cluster infraId is empty")
	}

	if ikePort == 0 {
		ikePort = helpers.SubmarinerIKEPort
	}

	if nattPort == 0 {
		nattPort = helpers.SubmarinerNatTPort
	}

	deployer := &machineSetDeployer{workClient: workClient, clusterName: clusterName}
	ec2Client, err := newEC2Client(kubeClient, clusterName, credentialsSecretName, region)
	if err != nil {
		return nil, err
	}
	awsCloud := prepareaws.NewCloud(deployer, ec2Client, infraId, region, instanceType)

	input := prepareapi.PrepareForSubmarinerInput{
		InternalPorts: []prepareapi.PortSpec{
			{Port: helpers.SubmarinerMetricsPort, Protocol: "tcp"},
		},
		PublicPorts: []prepareapi.PortSpec{
			{Port: uint16(nattPort), Protocol: "udp"},
			{Port: uint16(ikePort), Protocol: "udp"},
		},
		Gateways: 1,
	}

	return &awsProvider{
		awsCloud:      awsCloud,
		input:         input,
		reporter:      reporter.NewCloudPrepareReporter(),
		eventRecorder: eventRecorder,
	}, nil
}

// PrepareSubmarinerClusterEnv prepares submariner cluster environment on AWS
func (a *awsProvider) PrepareSubmarinerClusterEnv() error {
	err := a.awsCloud.PrepareForSubmariner(a.input, a.reporter)
	if err != nil {
		klog.Errorf("failed to prepare cluster env on aws. error: %v", err)
		return err
	}
	a.eventRecorder.Eventf("SubmarinerClusterEnvBuild", "the submariner cluster env is build on aws")
	return nil

}

// CleanUpSubmarinerClusterEnv clean up submariner cluster environment on AWS after the SubmarinerConfig was deleted
func (a *awsProvider) CleanUpSubmarinerClusterEnv() error {
	err := a.awsCloud.CleanupAfterSubmariner(a.reporter)
	if err != nil {
		klog.Errorf("failed to cleanup cluster env on aws. error: %v", err)
		return err
	}
	a.eventRecorder.Eventf("SubmarinerClusterEnvCleanedUp", "the submariner cluster env is cleaned up on aws")
	return nil
}

type machineSetDeployer struct {
	workClient  workclient.Interface
	clusterName string
}

func (d *machineSetDeployer) Deploy(machineSet *unstructured.Unstructured) error {
	clusterRoleYamlData := assets.MustCreateAssetFromTemplate(
		aggeragateClusterroleFile,
		bindata.MustAsset(filepath.Join("", aggeragateClusterroleFile)),
		nil).Data
	clusterRoleJsonData, err := yaml.YAMLToJSON(clusterRoleYamlData)
	if err != nil {
		return err
	}

	msJsonData, err := machineSet.MarshalJSON()
	if err != nil {
		return err
	}
	return helpers.ApplyManifestWork(context.TODO(), d.workClient, &workv1.ManifestWork{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      workName,
			Namespace: d.clusterName,
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: []workv1.Manifest{
					{
						RawExtension: runtime.RawExtension{Raw: clusterRoleJsonData},
					},
					{
						RawExtension: runtime.RawExtension{Raw: msJsonData},
					},
				},
			},
		},
	})
}

func (d *machineSetDeployer) Delete(machineSet *unstructured.Unstructured) error {
	err := d.workClient.WorkV1().ManifestWorks(d.clusterName).Delete(context.TODO(), workName, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

func newEC2Client(kubeClient kubernetes.Interface, secretNamespace, secretName, region string) (ec2iface.EC2API, error) {
	credentialsSecret, err := kubeClient.CoreV1().Secrets(secretNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	accessKeyID, ok := credentialsSecret.Data[accessKeyIDSecretKey]
	if !ok {
		return nil, fmt.Errorf("the aws credentials key %s is not in secret %s/%s", accessKeyIDSecretKey, secretNamespace, secretName)
	}
	secretAccessKey, ok := credentialsSecret.Data[accessKeySecretKey]
	if !ok {
		return nil, fmt.Errorf("the aws credentials key %s is not in secret %s/%s", accessKeySecretKey, secretNamespace, secretName)
	}

	sess, err := session.NewSession(&aws.Config{
		Credentials:      credentials.NewStaticCredentials(string(accessKeyID), string(secretAccessKey), ""),
		Region:           aws.String(region),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create aws session, %v \n", err)
	}

	return ec2.New(sess), nil
}

func awsChinaEndpointResolver(service, region string, optFns ...func(*endpoints.Options)) (endpoints.ResolvedEndpoint, error) {
	if service != route53.EndpointsID || region != "cn-northwest-1" {
		return endpoints.DefaultResolver().EndpointFor(service, region, optFns...)
	}

	return endpoints.ResolvedEndpoint{
		URL:         "https://route53.amazonaws.com.cn",
		PartitionID: "aws-cn",
	}, nil
}
