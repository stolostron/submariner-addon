package aws

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/openshift/library-go/pkg/operator/events"
	operatorhelpers "github.com/openshift/library-go/pkg/operator/v1helpers"

	"github.com/open-cluster-management/submariner-addon/pkg/helpers"
	workclient "open-cluster-management.io/api/client/work/clientset/versioned"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/submariner-io/admiral/pkg/util"
	cpapi "github.com/submariner-io/cloud-prepare/pkg/api"
	cpaws "github.com/submariner-io/cloud-prepare/pkg/aws"
	cpclient "github.com/submariner-io/cloud-prepare/pkg/aws/client"
	"github.com/submariner-io/cloud-prepare/pkg/ocp"
)

const (
	defaultInstanceType       = "m5n.large"
	accessKeyIDSecretKey      = "aws_access_key_id"
	accessKeySecretKey        = "aws_secret_access_key"
	internalELBLabel          = "kubernetes.io/role/internal-elb"
	internalGatewayLabel      = "submariner.io/gateway"
	workName                  = "aws-submariner-gateway-machineset"
	manifestFile              = "manifests/machineset.yaml"
	aggeragateClusterroleFile = "manifests/machineset-aggeragate-clusterrole.yaml"
)

type awsProvider struct {
	workClient         workclient.Interface
	awsClient          cpclient.Interface
	eventRecorder      events.Recorder
	region             string
	infraId            string
	nattPort           int64
	nattDiscoveryPort  int64
	clusterName        string
	instanceType       string
	gateways           int
	cloudPrepare       cpapi.Cloud
	machinesetDeployer ocp.MachineSetDeployer
}

func NewAWSProvider(
	restConfig *rest.Config,
	kubeClient kubernetes.Interface, workClient workclient.Interface,
	dynamicClient dynamic.Interface,
	eventRecorder events.Recorder,
	region, infraId, clusterName, credentialsSecretName string,
	instanceType string,
	nattPort, nattDiscoveryPort int,
	gateways int) (*awsProvider, error) {
	if region == "" {
		return nil, fmt.Errorf("cluster region is empty")
	}

	if infraId == "" {
		return nil, fmt.Errorf("cluster infraId is empty")
	}

	if gateways < 1 {
		return nil, fmt.Errorf("the count of gateways is less than 1")
	}

	if nattPort == 0 {
		nattPort = helpers.SubmarinerNatTPort
	}

	if nattDiscoveryPort == 0 {
		nattDiscoveryPort = helpers.SubmarinerNatTDiscoveryPort
	}

	if instanceType == "" {
		instanceType = defaultInstanceType
	}

	credentialsSecret, err := kubeClient.CoreV1().Secrets(clusterName).Get(context.TODO(), credentialsSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	accessKeyID, ok := credentialsSecret.Data[accessKeyIDSecretKey]
	if !ok {
		return nil, fmt.Errorf("the aws credentials key %s is not in secret %s/%s", accessKeyIDSecretKey, clusterName, credentialsSecretName)
	}
	secretAccessKey, ok := credentialsSecret.Data[accessKeySecretKey]
	if !ok {
		return nil, fmt.Errorf("the aws credentials key %s is not in secret %s/%s", accessKeySecretKey, clusterName, credentialsSecretName)
	}

	awsClient, err := cpclient.New(string(accessKeyID), string(secretAccessKey), region)
	if err != nil {
		return nil, err
	}

	restMapper, err := util.BuildRestMapper(restConfig)
	if err != nil {
		return nil, err
	}

	return &awsProvider{
		workClient:         workClient,
		awsClient:          awsClient,
		eventRecorder:      eventRecorder,
		region:             region,
		infraId:            infraId,
		nattPort:           int64(nattPort),
		nattDiscoveryPort:  int64(nattDiscoveryPort),
		clusterName:        clusterName,
		instanceType:       instanceType,
		gateways:           gateways,
		cloudPrepare:       cpaws.NewCloud(awsClient, infraId, region),
		machinesetDeployer: ocp.NewK8sMachinesetDeployer(restMapper, dynamicClient),
	}, nil
}

// PrepareSubmarinerClusterEnv prepares submariner cluster environment on AWS
func (a *awsProvider) PrepareSubmarinerClusterEnv() error {
	// See prepareAws() in https://github.com/submariner-io/submariner-operator/blob/devel/pkg/subctl/cmd/cloud/prepare/aws.go
	// For now we only support at least one gateway (no load-balancer)
	gwDeployer, err := cpaws.NewOcpGatewayDeployer(a.cloudPrepare, a.machinesetDeployer, a.instanceType)
	if err != nil {
		return err
	}
	if err := gwDeployer.Deploy(cpapi.GatewayDeployInput{
		PublicPorts: []cpapi.PortSpec{
			{Port: uint16(a.nattPort), Protocol: "udp"},
			{Port: uint16(a.nattDiscoveryPort), Protocol: "udp"},
			// ESP & AH protocols are used for private-ip to private-ip gateway communications
			{Port: 0, Protocol: "50"},
			{Port: 0, Protocol: "51"},
		},
		Gateways: a.gateways,
	}, a); err != nil {
		return err
	}
	if err := a.cloudPrepare.PrepareForSubmariner(cpapi.PrepareForSubmarinerInput{
		InternalPorts: []cpapi.PortSpec{
			{Port: helpers.SubmarinerRoutePort, Protocol: "udp"},
			{Port: helpers.SubmarinerMetricsPort, Protocol: "tcp"},
		},
	}, a); err != nil {
		return err
	}

	a.eventRecorder.Eventf("SubmarinerClusterEnvBuild", "the submariner cluster env is build on aws")

	return nil
}

// CleanUpSubmarinerClusterEnv clean up submariner cluster environment on AWS after the SubmarinerConfig was deleted
// The below tasks will be executed
// 1. delete the applied machineset manifest work to delete the gateway instance from managed cluster
// 2. untag the subnet that was tagged on preparation phase
// 3. revoke Submariner IPsec ports
// 4. revoke Submariner metrics ports
// 5. revoke Submariner route ports
func (a *awsProvider) CleanUpSubmarinerClusterEnv() error {
	var errs []error
	// delete the applied machineset manifest work to delete the gateway instance from managed cluster
	if err := a.deleteGatewayNode(); err != nil {
		errs = append(errs, fmt.Errorf("failed to delete gateway node for %s: %v \n", a.infraId, err))
	}

	vpc, err := a.findVPC()
	// cannot find the vpc, the below tasks will not continue, return directly
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to find aws vpc with %s: %v \n", a.infraId, err))
		return operatorhelpers.NewMultiLineAggregate(errs)
	}

	// untag the subnet that was tagged on preparation phase
	if err := a.untagSubnet(vpc); err != nil {
		errs = append(errs, fmt.Errorf("failed to untag subnet for %s: %v \n", a.infraId, err))
	}

	// the ipsec sg may has references, result in the sg cannot be deleted, so we revoke ports here
	if err := a.revokeIPsecPorts(vpc); err != nil {
		errs = append(errs, fmt.Errorf("failed to revoke ipsec ports for %s: %v \n", a.infraId, err))
	}

	// cannot find the worker security group, the below tasks will not continue, return directly
	workerSecurityGroup, err := a.findSecurityGroup(*vpc.VpcId, fmt.Sprintf("%s-worker-sg", a.infraId))
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to find security group %s-worker-sg: %v \n", a.infraId, err))
		return operatorhelpers.NewMultiLineAggregate(errs)
	}

	// cannot find the master security group, the below tasks will not continue, return directly
	masterSecurityGroup, err := a.findSecurityGroup(*vpc.VpcId, fmt.Sprintf("%s-master-sg", a.infraId))
	if err != nil {
		errs = append(errs, fmt.Errorf("failed to find security group %s-worker-sg: %v \n", a.infraId, err))
		return operatorhelpers.NewMultiLineAggregate(errs)
	}

	// revoke Submariner metrics ports
	if err := a.revokePort(masterSecurityGroup, workerSecurityGroup, helpers.SubmarinerMetricsPort, "tcp"); err != nil {
		errs = append(errs, fmt.Errorf("failed to revoke metrics port for %s: %v \n", a.infraId, err))
	}

	// revoke Submariner route ports
	if err := a.revokePort(masterSecurityGroup, workerSecurityGroup, helpers.SubmarinerRoutePort, "udp"); err != nil {
		errs = append(errs, fmt.Errorf("failed to revoke route port for %s: %v \n", a.infraId, err))
	}

	if len(errs) == 0 {
		a.eventRecorder.Eventf("SubmarinerClusterEnvCleanedUp", "the submariner cluster env is cleaned up on aws")
	}

	return operatorhelpers.NewMultiLineAggregate(errs)
}

// Reporter functions

// Started will report that an operation started on the cloud
func (a *awsProvider) Started(message string, args ...interface{}) {
	a.eventRecorder.Eventf("SubmarinerClusterEnvBuild", message, args...)
}

// Succeeded will report that the last operation on the cloud has succeeded
func (a *awsProvider) Succeeded(message string, args ...interface{}) {
	a.eventRecorder.Eventf("SubmarinerClusterEnvBuild", message, args...)
}

// Failed will report that the last operation on the cloud has failed
func (a *awsProvider) Failed(errs ...error) {
	message := "Failed"
	errMessages := []string{}
	for i := range errs {
		message += "\n%s"
		errMessages = append(errMessages, errs[i].Error())
	}
	a.eventRecorder.Warningf("SubmarinerClusterEnvBuild", message, errMessages)
}

func (a *awsProvider) findVPC() (*ec2.Vpc, error) {
	vpcsOutput, err := a.awsClient.DescribeVpcs(&ec2.DescribeVpcsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("tag:Name"),
				Values: []*string{aws.String(fmt.Sprintf("%s-vpc", a.infraId))},
			},
			{
				Name:   aws.String(fmt.Sprintf("tag:kubernetes.io/cluster/%s", a.infraId)),
				Values: []*string{aws.String("owned")},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(vpcsOutput.Vpcs) == 0 {
		return nil, &errors.StatusError{
			ErrStatus: metav1.Status{
				Reason:  metav1.StatusReasonNotFound,
				Message: "there are no vpcs",
			},
		}
	}
	return vpcsOutput.Vpcs[0], nil
}

func (a *awsProvider) findSecurityGroup(vpcId, nameTag string) (*ec2.SecurityGroup, error) {
	securityGroups, err := a.awsClient.DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{aws.String(vpcId)},
			},
			{
				Name:   aws.String("tag:Name"),
				Values: []*string{aws.String(nameTag)},
			},
		},
	})
	if err != nil {
		return nil, err
	}
	if len(securityGroups.SecurityGroups) == 0 {
		return nil, &errors.StatusError{
			ErrStatus: metav1.Status{
				Reason:  metav1.StatusReasonNotFound,
				Message: "there are no security groups",
			},
		}
	}
	return securityGroups.SecurityGroups[0], nil
}

func (a *awsProvider) revokePort(masterSecurityGroup, workerSecurityGroup *ec2.SecurityGroup, port int64, protocol string) error {
	workerPermission, masterPermission := getRoutePortPermission(masterSecurityGroup, workerSecurityGroup, port, protocol)

	_, err := a.awsClient.RevokeSecurityGroupIngress(&ec2.RevokeSecurityGroupIngressInput{
		GroupId:       workerSecurityGroup.GroupId,
		IpPermissions: []*ec2.IpPermission{workerPermission},
	})
	switch {
	case isAWSNotFoundError(err):
		klog.V(4).Infof("there is no port %d/%s in security group %s on aws", port, protocol, *workerSecurityGroup.GroupId)
		return nil
	case err != nil:
		return err
	}

	_, err = a.awsClient.RevokeSecurityGroupIngress(&ec2.RevokeSecurityGroupIngressInput{
		GroupId:       masterSecurityGroup.GroupId,
		IpPermissions: []*ec2.IpPermission{masterPermission},
	})
	switch {
	case isAWSNotFoundError(err):
		klog.V(4).Infof("there is no port %d/%s in security group %s on aws", port, protocol, *workerSecurityGroup.GroupId)
		return nil
	case err != nil:
		return err
	}

	return nil
}

func (a *awsProvider) revokeIPsecPorts(vpc *ec2.Vpc) error {
	groupName := fmt.Sprintf("%s-submariner-gw-sg", a.infraId)
	sg, err := a.findSecurityGroup(*vpc.VpcId, groupName)
	if errors.IsNotFound(err) {
		klog.V(4).Infof("there is no security group %s on aws", groupName)
		return nil
	}
	if err != nil {
		return err
	}

	_, err = a.awsClient.RevokeSecurityGroupIngress(&ec2.RevokeSecurityGroupIngressInput{
		GroupId:       sg.GroupId,
		IpPermissions: sg.IpPermissions,
	})
	if isAWSNotFoundError(err) {
		klog.V(4).Infof("there is no ipsec ports in security group %s on aws", *sg.GroupId)
		return nil
	}
	return err
}

func (a *awsProvider) untagSubnet(vpc *ec2.Vpc) error {
	// find subnets and filter them with internal ELB label and internal gateway label, then we will untag
	// the subnet that has these labels
	subnets, err := a.awsClient.DescribeSubnets(&ec2.DescribeSubnetsInput{
		Filters: []*ec2.Filter{
			{
				Name:   aws.String("vpc-id"),
				Values: []*string{vpc.VpcId},
			},
			{
				Name:   aws.String("tag:Name"),
				Values: []*string{aws.String(fmt.Sprintf("%s-public-%s*", a.infraId, a.region))},
			},
			{
				Name:   aws.String(fmt.Sprintf("tag:kubernetes.io/cluster/%s", a.infraId)),
				Values: []*string{aws.String("owned")},
			},
			{
				Name:   aws.String(fmt.Sprintf("tag:%s", internalELBLabel)),
				Values: []*string{aws.String("")},
			},
			{
				Name:   aws.String(fmt.Sprintf("tag:%s", internalGatewayLabel)),
				Values: []*string{aws.String("")},
			},
		},
	})
	if err != nil {
		return err
	}

	var errs []error
	for _, subnet := range subnets.Subnets {
		if _, err = a.awsClient.DeleteTags(&ec2.DeleteTagsInput{
			Tags: []*ec2.Tag{
				{
					Key:   aws.String(internalELBLabel),
					Value: aws.String(""),
				},
				{
					Key:   aws.String(internalGatewayLabel),
					Value: aws.String(""),
				},
			},
			Resources: []*string{
				subnet.SubnetId,
			},
		}); err != nil {
			errs = append(errs, err)
		}
	}

	return operatorhelpers.NewMultiLineAggregate(errs)
}

func (a *awsProvider) deleteGatewayNode() error {
	err := a.workClient.WorkV1().ManifestWorks(a.clusterName).Delete(context.TODO(), workName, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

func getRoutePortPermission(
	masterSecurityGroup, workerSecurityGroup *ec2.SecurityGroup,
	port int64,
	protocol string) (workerPermission, masterPermission *ec2.IpPermission) {
	return (&ec2.IpPermission{}).
			SetFromPort(port).
			SetToPort(port).
			SetIpProtocol(protocol).
			SetUserIdGroupPairs([]*ec2.UserIdGroupPair{
				{
					// route traffic for all workers
					GroupId: workerSecurityGroup.GroupId,
					UserId:  workerSecurityGroup.OwnerId,
				},
				{
					// route traffic from master nodes to worker nodes
					GroupId: masterSecurityGroup.GroupId,
					UserId:  masterSecurityGroup.OwnerId,
				},
			}), (&ec2.IpPermission{}).
			SetFromPort(port).
			SetToPort(port).
			SetIpProtocol(protocol).
			SetUserIdGroupPairs([]*ec2.UserIdGroupPair{
				{
					// route traffic from worker nodes to master nodes
					GroupId: workerSecurityGroup.GroupId,
					UserId:  workerSecurityGroup.OwnerId,
				},
			})
}

func getIPsecPortsPermission(nattPort, nattDiscoveryPort int64) []*ec2.IpPermission {
	return []*ec2.IpPermission{
		(&ec2.IpPermission{}).
			SetIpProtocol("udp").
			SetFromPort(nattPort).
			SetToPort(nattPort).
			SetIpRanges([]*ec2.IpRange{
				(&ec2.IpRange{}).SetCidrIp("0.0.0.0/0").SetDescription("Public Submariner traffic"),
			}),
		(&ec2.IpPermission{}).
			SetIpProtocol("udp").
			SetFromPort(nattDiscoveryPort).
			SetToPort(nattDiscoveryPort).
			SetIpRanges([]*ec2.IpRange{
				(&ec2.IpRange{}).SetCidrIp("0.0.0.0/0"),
			}),
	}
}

func isAWSNotFoundError(err error) bool {
	if awsErr, ok := err.(awserr.Error); ok {
		// we had to hardcoded, see https://github.com/aws/aws-sdk-go/issues/3235
		if awsErr.Code() == "InvalidPermission.NotFound" {
			return true
		}
	}
	return false
}
