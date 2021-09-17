package client

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/endpoints"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/ec2/ec2iface"
	"github.com/aws/aws-sdk-go/service/route53"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	accessKeyIDSecretKey = "aws_access_key_id"
	accessKeySecretKey   = "aws_secret_access_key"
)

//go:generate mockgen -source=./client.go -destination=./mock/client_generated.go -package=mock

// Interface wraps an actual AWS SDK ec2 client to allow for easier testing.
type Interface interface {
	AuthorizeSecurityGroupIngress(input *ec2.AuthorizeSecurityGroupIngressInput) (*ec2.AuthorizeSecurityGroupIngressOutput, error)

	CreateSecurityGroup(input *ec2.CreateSecurityGroupInput) (*ec2.CreateSecurityGroupOutput, error)
	CreateTags(input *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error)

	DescribeInstances(input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error)
	DescribeVpcs(input *ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error)
	DescribeSecurityGroups(input *ec2.DescribeSecurityGroupsInput) (*ec2.DescribeSecurityGroupsOutput, error)
	DescribeSubnets(input *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error)
	DescribeInstanceTypeOfferings(input *ec2.DescribeInstanceTypeOfferingsInput) (*ec2.DescribeInstanceTypeOfferingsOutput, error)

	DeleteSecurityGroup(input *ec2.DeleteSecurityGroupInput) (*ec2.DeleteSecurityGroupOutput, error)
	DeleteTags(input *ec2.DeleteTagsInput) (*ec2.DeleteTagsOutput, error)

	RevokeSecurityGroupIngress(input *ec2.RevokeSecurityGroupIngressInput) (*ec2.RevokeSecurityGroupIngressOutput, error)
}

type awsClient struct {
	ec2Client ec2iface.EC2API
}

func (ac *awsClient) AuthorizeSecurityGroupIngress(input *ec2.AuthorizeSecurityGroupIngressInput) (*ec2.AuthorizeSecurityGroupIngressOutput, error) {
	return ac.ec2Client.AuthorizeSecurityGroupIngress(input)
}

func (ac *awsClient) CreateSecurityGroup(input *ec2.CreateSecurityGroupInput) (*ec2.CreateSecurityGroupOutput, error) {
	return ac.ec2Client.CreateSecurityGroup(input)
}

func (ac *awsClient) CreateTags(input *ec2.CreateTagsInput) (*ec2.CreateTagsOutput, error) {
	return ac.ec2Client.CreateTags(input)
}

func (ac *awsClient) DescribeInstances(input *ec2.DescribeInstancesInput) (*ec2.DescribeInstancesOutput, error) {
	return ac.ec2Client.DescribeInstances(input)
}
func (ac *awsClient) DescribeVpcs(input *ec2.DescribeVpcsInput) (*ec2.DescribeVpcsOutput, error) {
	return ac.ec2Client.DescribeVpcs(input)
}

func (ac *awsClient) DescribeSecurityGroups(input *ec2.DescribeSecurityGroupsInput) (*ec2.DescribeSecurityGroupsOutput, error) {
	return ac.ec2Client.DescribeSecurityGroups(input)
}

func (ac *awsClient) DescribeSubnets(input *ec2.DescribeSubnetsInput) (*ec2.DescribeSubnetsOutput, error) {
	return ac.ec2Client.DescribeSubnets(input)
}

func (ac *awsClient) DeleteSecurityGroup(input *ec2.DeleteSecurityGroupInput) (*ec2.DeleteSecurityGroupOutput, error) {
	return ac.ec2Client.DeleteSecurityGroup(input)
}

func (ac *awsClient) DeleteTags(input *ec2.DeleteTagsInput) (*ec2.DeleteTagsOutput, error) {
	return ac.ec2Client.DeleteTags(input)
}

func (ac *awsClient) RevokeSecurityGroupIngress(input *ec2.RevokeSecurityGroupIngressInput) (*ec2.RevokeSecurityGroupIngressOutput, error) {
	return ac.ec2Client.RevokeSecurityGroupIngress(input)
}

func (ac *awsClient) DescribeInstanceTypeOfferings(input *ec2.DescribeInstanceTypeOfferingsInput) (*ec2.DescribeInstanceTypeOfferingsOutput, error) {
	return ac.ec2Client.DescribeInstanceTypeOfferings(input)
}

func NewClient(kubeClient kubernetes.Interface, secretNamespace, secretName, region string) (Interface, error) {
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
		EndpointResolver: endpoints.ResolverFunc(awsChinaEndpointResolver),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create aws session, %v \n", err)
	}

	return &awsClient{
		ec2Client: ec2.New(sess),
	}, nil
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
