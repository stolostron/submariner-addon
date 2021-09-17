package aws

import (
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
	cpaws "github.com/submariner-io/cloud-prepare/pkg/aws"

	"github.com/golang/mock/gomock"

	"github.com/openshift/library-go/pkg/operator/events/eventstesting"

	mockaws "github.com/open-cluster-management/submariner-addon/pkg/cloud/aws/client/mock"
	mockmsd "github.com/open-cluster-management/submariner-addon/pkg/cloud/aws/machineset/mock"
	testinghelpers "github.com/open-cluster-management/submariner-addon/pkg/helpers/testing"
	fakeworkclient "open-cluster-management.io/api/client/work/clientset/versioned/fake"
	workv1 "open-cluster-management.io/api/work/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
)

func TestPrepareSubmarinerClusterEnv(t *testing.T) {
	cases := []struct {
		name              string
		works             []runtime.Object
		expectAWSInvoking func(*mockaws.MockInterface)
		expectMSDInvoking func(*mockmsd.MockInterface)
		validateActions   func(t *testing.T, workActions []clienttesting.Action)
	}{
		{
			name:  "prepare a new submariner cluster env",
			works: []runtime.Object{},
			expectMSDInvoking: func(mock *mockmsd.MockInterface) {
				mock.EXPECT().Deploy(gomock.Any()).Return(nil)
			},
			expectAWSInvoking: func(mock *mockaws.MockInterface) {
				mock.EXPECT().DescribeVpcs(&ec2.DescribeVpcsInput{
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("tag:Name"),
							Values: []*string{aws.String("testcluster-a1b1-vpc")},
						},
						{
							Name:   aws.String("tag:kubernetes.io/cluster/testcluster-a1b1"),
							Values: []*string{aws.String("owned")},
						},
					},
				}).Return(&ec2.DescribeVpcsOutput{Vpcs: []*ec2.Vpc{{VpcId: aws.String("vpc-1234")}}}, nil).AnyTimes()
				mock.EXPECT().DescribeInstances(&ec2.DescribeInstancesInput{
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("vpc-id"),
							Values: []*string{aws.String("vpc-1234")},
						},
						{
							Name:   aws.String("tag:Name"),
							Values: []*string{aws.String("testcluster-a1b1-worker*")},
						},
						{
							Name:   aws.String("tag:kubernetes.io/cluster/testcluster-a1b1"),
							Values: []*string{aws.String("owned")},
						},
					},
				}).Return(&ec2.DescribeInstancesOutput{Reservations: []*ec2.Reservation{{Instances: []*ec2.Instance{{ImageId: aws.String("ami-1234")}}}}}, nil)
				masterSG := &ec2.SecurityGroup{GroupId: aws.String("sg-1"), OwnerId: aws.String("1234")}
				workerSG := &ec2.SecurityGroup{GroupId: aws.String("sg-2"), OwnerId: aws.String("1234")}
				mock.EXPECT().DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("vpc-id"),
							Values: []*string{aws.String("vpc-1234")},
						},
						{
							Name:   aws.String("tag:Name"),
							Values: []*string{aws.String("testcluster-a1b1-worker-sg")},
						},
						{
							Name:   aws.String("tag:kubernetes.io/cluster/testcluster-a1b1"),
							Values: []*string{aws.String("owned")},
						},
					},
				}).Return(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: []*ec2.SecurityGroup{workerSG}}, nil).AnyTimes()
				mock.EXPECT().DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("vpc-id"),
							Values: []*string{aws.String("vpc-1234")},
						},
						{
							Name:   aws.String("tag:Name"),
							Values: []*string{aws.String("testcluster-a1b1-master-sg")},
						},
						{
							Name:   aws.String("tag:kubernetes.io/cluster/testcluster-a1b1"),
							Values: []*string{aws.String("owned")},
						},
					},
				}).Return(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: []*ec2.SecurityGroup{masterSG}}, nil).AnyTimes()
				mock.EXPECT().AuthorizeSecurityGroupIngress(gomock.Any()).Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, nil).AnyTimes()
				/*
					routeWorkerPermission, routeMasterPermission := getRoutePortPermission(masterSG, workerSG, int64(4800), "udp")
					mock.EXPECT().AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
						DryRun:  aws.Bool(true),
						GroupId: workerSG.GroupId,
					}).Return(nil, nil).AnyTimes()
					mock.EXPECT().AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
						GroupId:       workerSG.GroupId,
						IpPermissions: []*ec2.IpPermission{routeWorkerPermission},
					}).Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, nil).AnyTimes()
					mock.EXPECT().AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
						GroupId:       masterSG.GroupId,
						IpPermissions: []*ec2.IpPermission{routeMasterPermission},
					}).Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, nil).AnyTimes()
					metricsWorkerPermission, metricsMasterPermission := getRoutePortPermission(masterSG, workerSG, int64(8080), "tcp")
					mock.EXPECT().AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
						GroupId:       workerSG.GroupId,
						IpPermissions: []*ec2.IpPermission{metricsWorkerPermission},
					}).Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, nil).AnyTimes()
					mock.EXPECT().AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
						GroupId:       masterSG.GroupId,
						IpPermissions: []*ec2.IpPermission{metricsMasterPermission},
					}).Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, nil).AnyTimes()
				*/
				mock.EXPECT().DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("vpc-id"),
							Values: []*string{aws.String("vpc-1234")},
						},
						{
							Name:   aws.String("tag:Name"),
							Values: []*string{aws.String("testcluster-a1b1-submariner-gw-sg")},
						},
						{
							Name:   aws.String("tag:kubernetes.io/cluster/testcluster-a1b1"),
							Values: []*string{aws.String("owned")},
						},
					},
				}).Return(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: []*ec2.SecurityGroup{}}, nil)
				mock.EXPECT().CreateSecurityGroup(&ec2.CreateSecurityGroupInput{
					Description: aws.String("permissions-test"),
					DryRun:      aws.Bool(true),
					GroupName:   aws.String("permissions-test"),
					VpcId:       aws.String("vpc-1234"),
				}).Return(nil, nil)
				mock.EXPECT().CreateSecurityGroup(&ec2.CreateSecurityGroupInput{
					Description: aws.String("Submariner Gateway"),
					GroupName:   aws.String("testcluster-a1b1-submariner-gw-sg"),
					TagSpecifications: []*ec2.TagSpecification{
						{
							ResourceType: aws.String("security-group"),
							Tags: []*ec2.Tag{
								{
									Key:   aws.String("Name"),
									Value: aws.String("testcluster-a1b1-submariner-gw-sg"),
								},
								{
									Key:   aws.String("kubernetes.io/cluster/testcluster-a1b1"),
									Value: aws.String("owned"),
								},
							},
						},
					},
					VpcId: aws.String("vpc-1234"),
				}).Return(&ec2.CreateSecurityGroupOutput{GroupId: aws.String("sg-3")}, nil)
				/*
					mock.EXPECT().AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
						GroupId:       aws.String("sg-3"),
						IpPermissions: getIPsecPortsPermission(int64(500), int64(500), int64(4900)),
					}).Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, nil)
					mock.EXPECT().AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
						GroupId:       aws.String("sg-3"),
						IpPermissions: getIPsecPortsPermission(int64(500), int64(4500)),
					}).Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, nil)
					mock.EXPECT().AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
						GroupId: aws.String("sg-3"),
						IpPermissions: []*ec2.IpPermission{
							{
								FromPort:   aws.Int64(0),
								IpProtocol: aws.String("50"),
								IpRanges:   []*ec2.IpRange{{CidrIp: aws.String("0.0.0.0/0"), Description: aws.String("Public Submariner traffic")}},
								ToPort:     aws.Int64(0),
							},
						},
					}).Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, nil)
					mock.EXPECT().AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
						GroupId: aws.String("sg-3"),
						IpPermissions: []*ec2.IpPermission{
							{
								FromPort:   aws.Int64(0),
								IpProtocol: aws.String("51"),
								IpRanges:   []*ec2.IpRange{{CidrIp: aws.String("0.0.0.0/0"), Description: aws.String("Public Submariner traffic")}},
								ToPort:     aws.Int64(0),
							},
						},
					}).Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, nil)
				*/
				mock.EXPECT().DescribeSubnets(&ec2.DescribeSubnetsInput{
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("vpc-id"),
							Values: []*string{aws.String("vpc-1234")},
						},
						{
							Name:   aws.String("tag:kubernetes.io/cluster/testcluster-a1b1"),
							Values: []*string{aws.String("owned")},
						},
						{
							Name:   aws.String("tag:Name"),
							Values: []*string{aws.String("testcluster-a1b1-public-us-east-1*")},
						},
					},
				}).Return(&ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{{SubnetId: aws.String("sub-1"), AvailabilityZone: aws.String("z1")}}}, nil).AnyTimes()
				mock.EXPECT().DescribeInstanceTypeOfferings(&ec2.DescribeInstanceTypeOfferingsInput{
					DryRun: aws.Bool(true),
				}).Return(nil, nil).AnyTimes()
				mock.EXPECT().DescribeInstanceTypeOfferings(&ec2.DescribeInstanceTypeOfferingsInput{
					LocationType: aws.String("availability-zone"),
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("location"),
							Values: []*string{aws.String("z1")},
						},
						{
							Name:   aws.String("instance-type"),
							Values: []*string{aws.String("m5n.large")},
						},
					},
				}).Return(&ec2.DescribeInstanceTypeOfferingsOutput{InstanceTypeOfferings: []*ec2.InstanceTypeOffering{
					{
						InstanceType: aws.String("instance-type"),
					},
				}}, nil).AnyTimes()
				mock.EXPECT().CreateTags(&ec2.CreateTagsInput{
					DryRun:    aws.Bool(true),
					Resources: []*string{aws.String("sub-1")},
					Tags:      []*ec2.Tag{{Key: aws.String(internalGatewayLabel), Value: aws.String("")}},
				}).Return(nil, nil)
				mock.EXPECT().CreateTags(&ec2.CreateTagsInput{
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
					Resources: []*string{aws.String("sub-1")},
				}).Return(&ec2.CreateTagsOutput{}, nil)
			},
			validateActions: func(t *testing.T, workActions []clienttesting.Action) {
				//testinghelpers.AssertActions(t, workActions, "get", "create")
			},
		},
		{
			name: "the submariner cluster env has been prepared",
			works: []runtime.Object{
				&workv1.ManifestWork{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "aws-submariner-gateway-machineset",
						Namespace: "testcluster",
					},
				},
			},
			expectMSDInvoking: func(mock *mockmsd.MockInterface) {
				mock.EXPECT().Deploy(gomock.Any()).Return(nil)
			},
			expectAWSInvoking: func(mock *mockaws.MockInterface) {
				mock.EXPECT().CreateSecurityGroup(&ec2.CreateSecurityGroupInput{
					Description: aws.String("permissions-test"),
					DryRun:      aws.Bool(true),
					GroupName:   aws.String("permissions-test"),
					VpcId:       aws.String("vpc-1234"),
				}).Return(nil, nil)
				mock.EXPECT().CreateTags(&ec2.CreateTagsInput{
					DryRun:    aws.Bool(true),
					Resources: []*string{aws.String("sub-1")},
					Tags:      []*ec2.Tag{{Key: aws.String(internalGatewayLabel), Value: aws.String("")}},
				}).Return(nil, nil)
				mock.EXPECT().CreateTags(&ec2.CreateTagsInput{
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
					Resources: []*string{aws.String("sub-1")},
				}).Return(&ec2.CreateTagsOutput{}, nil)
				mock.EXPECT().DescribeVpcs(&ec2.DescribeVpcsInput{
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("tag:Name"),
							Values: []*string{aws.String("testcluster-a1b1-vpc")},
						},
						{
							Name:   aws.String("tag:kubernetes.io/cluster/testcluster-a1b1"),
							Values: []*string{aws.String("owned")},
						},
					},
				}).Return(&ec2.DescribeVpcsOutput{Vpcs: []*ec2.Vpc{{VpcId: aws.String("vpc-1234")}}}, nil).AnyTimes()
				mock.EXPECT().DescribeInstances(&ec2.DescribeInstancesInput{
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("vpc-id"),
							Values: []*string{aws.String("vpc-1234")},
						},
						{
							Name:   aws.String("tag:Name"),
							Values: []*string{aws.String("testcluster-a1b1-worker*")},
						},
						{
							Name:   aws.String("tag:kubernetes.io/cluster/testcluster-a1b1"),
							Values: []*string{aws.String("owned")},
						},
					},
				}).Return(&ec2.DescribeInstancesOutput{Reservations: []*ec2.Reservation{{Instances: []*ec2.Instance{{ImageId: aws.String("ami-1234")}}}}}, nil)
				masterSG := &ec2.SecurityGroup{GroupId: aws.String("sg-1"), OwnerId: aws.String("1234")}
				workerSG := &ec2.SecurityGroup{GroupId: aws.String("sg-2"), OwnerId: aws.String("1234")}
				mock.EXPECT().AuthorizeSecurityGroupIngress(gomock.Any()).Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, nil).AnyTimes()
				/*
					mock.EXPECT().AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
						DryRun:  aws.Bool(true),
						GroupId: workerSG.GroupId,
					}).Return(nil, nil)
				*/
				mock.EXPECT().DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("vpc-id"),
							Values: []*string{aws.String("vpc-1234")},
						},
						{
							Name:   aws.String("tag:Name"),
							Values: []*string{aws.String("testcluster-a1b1-worker-sg")},
						},
						{
							Name:   aws.String("tag:kubernetes.io/cluster/testcluster-a1b1"),
							Values: []*string{aws.String("owned")},
						},
					},
				}).Return(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: []*ec2.SecurityGroup{workerSG}}, nil).AnyTimes()
				mock.EXPECT().DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("vpc-id"),
							Values: []*string{aws.String("vpc-1234")},
						},
						{
							Name:   aws.String("tag:Name"),
							Values: []*string{aws.String("testcluster-a1b1-master-sg")},
						},
						{
							Name:   aws.String("tag:kubernetes.io/cluster/testcluster-a1b1"),
							Values: []*string{aws.String("owned")},
						},
					},
				}).Return(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: []*ec2.SecurityGroup{masterSG}}, nil).AnyTimes()
				/*
					routeWorkerPermission, routeMasterPermission := getRoutePortPermission(masterSG, workerSG, int64(4800), "udp")
					mock.EXPECT().AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
						GroupId:       workerSG.GroupId,
						IpPermissions: []*ec2.IpPermission{routeWorkerPermission},
					}).Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, awserr.New("InvalidPermission.Duplicate", "test", nil))
					mock.EXPECT().AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
						GroupId:       masterSG.GroupId,
						IpPermissions: []*ec2.IpPermission{routeMasterPermission},
					}).Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, awserr.New("InvalidPermission.Duplicate", "test", nil))
					metricsWorkerPermission, metricsMasterPermission := getRoutePortPermission(masterSG, workerSG, int64(8080), "tcp")
					mock.EXPECT().AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
						GroupId:       workerSG.GroupId,
						IpPermissions: []*ec2.IpPermission{metricsWorkerPermission},
					}).Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, awserr.New("InvalidPermission.Duplicate", "test", nil))
					mock.EXPECT().AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
						GroupId:       masterSG.GroupId,
						IpPermissions: []*ec2.IpPermission{metricsMasterPermission},
					}).Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, awserr.New("InvalidPermission.Duplicate", "test", nil))
				*/
				gwSG := &ec2.SecurityGroup{
					GroupId:       aws.String("sg-3"),
					IpPermissions: getIPsecPortsPermission(int64(4500), int64(4900)),
				}
				mock.EXPECT().DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("vpc-id"),
							Values: []*string{aws.String("vpc-1234")},
						},
						{
							Name:   aws.String("tag:Name"),
							Values: []*string{aws.String("testcluster-a1b1-submariner-gw-sg")},
						},
						{
							Name:   aws.String("tag:kubernetes.io/cluster/testcluster-a1b1"),
							Values: []*string{aws.String("owned")},
						},
					},
				}).Return(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: []*ec2.SecurityGroup{gwSG}}, nil)
				mock.EXPECT().DescribeSubnets(&ec2.DescribeSubnetsInput{
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("vpc-id"),
							Values: []*string{aws.String("vpc-1234")},
						},
						{
							Name:   aws.String("tag:kubernetes.io/cluster/testcluster-a1b1"),
							Values: []*string{aws.String("owned")},
						},
						{
							Name:   aws.String("tag:Name"),
							Values: []*string{aws.String("testcluster-a1b1-public-us-east-1*")},
						},
					},
				}).Return(&ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{
					{
						SubnetId:         aws.String("sub-1"),
						AvailabilityZone: aws.String("z1"),
						Tags: []*ec2.Tag{
							{
								Key:   aws.String(internalELBLabel),
								Value: aws.String(""),
							},
						},
					},
				}}, nil).AnyTimes()
				mock.EXPECT().DescribeInstanceTypeOfferings(&ec2.DescribeInstanceTypeOfferingsInput{
					DryRun: aws.Bool(true),
				}).Return(nil, nil)
				mock.EXPECT().DescribeInstanceTypeOfferings(&ec2.DescribeInstanceTypeOfferingsInput{
					LocationType: aws.String("availability-zone"),
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("location"),
							Values: []*string{aws.String("z1")},
						},
						{
							Name:   aws.String("instance-type"),
							Values: []*string{aws.String("m5n.large")},
						},
					},
				}).Return(&ec2.DescribeInstanceTypeOfferingsOutput{InstanceTypeOfferings: []*ec2.InstanceTypeOffering{
					{
						InstanceType: aws.String("instance-type"),
					},
				}}, nil).AnyTimes()
			},
			validateActions: func(t *testing.T, workActions []clienttesting.Action) {
				//testinghelpers.AssertActions(t, workActions, "get", "update")
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			awsClient := mockaws.NewMockInterface(mockCtrl)
			c.expectAWSInvoking(awsClient)

			machinesetDeployer := mockmsd.NewMockInterface(mockCtrl)
			if c.expectMSDInvoking != nil {
				c.expectMSDInvoking(machinesetDeployer)
			}

			workClient := fakeworkclient.NewSimpleClientset(c.works...)
			infraId := "testcluster-a1b1"
			region := "us-east-1"
			awsProvider := &awsProvider{
				workClient:         workClient,
				awsClient:          awsClient,
				region:             region,
				infraId:            infraId,
				nattPort:           int64(4500),
				nattDiscoveryPort:  int64(4900),
				clusterName:        "testcluster",
				instanceType:       "m5n.large",
				gateways:           1,
				eventRecorder:      eventstesting.NewTestingEventRecorder(t),
				cloudPrepare:       cpaws.NewCloud(awsClient, infraId, region),
				machinesetDeployer: machinesetDeployer,
			}

			err := awsProvider.PrepareSubmarinerClusterEnv()
			if err != nil {
				t.Errorf("expect no err, bug got %v", err)
			}

			c.validateActions(t, workClient.Actions())
		})
	}
}

func TestCleanUpSubmarinerClusterEnv(t *testing.T) {
	cases := []struct {
		name            string
		works           []runtime.Object
		expectInvoking  func(*mockaws.MockInterface)
		validateActions func(t *testing.T, workActions []clienttesting.Action)
	}{
		{
			name: "clean up submariner cluster env",
			works: []runtime.Object{
				&workv1.ManifestWork{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "aws-submariner-gateway-machineset",
						Namespace: "testcluster",
					},
				},
			},
			expectInvoking: func(mock *mockaws.MockInterface) {
				mock.EXPECT().DescribeVpcs(&ec2.DescribeVpcsInput{
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("tag:Name"),
							Values: []*string{aws.String("testcluster-a1b1-vpc")},
						},
						{
							Name:   aws.String("tag:kubernetes.io/cluster/testcluster-a1b1"),
							Values: []*string{aws.String("owned")},
						},
					},
				}).Return(&ec2.DescribeVpcsOutput{Vpcs: []*ec2.Vpc{{VpcId: aws.String("vpc-1234")}}}, nil)
				mock.EXPECT().DescribeSubnets(&ec2.DescribeSubnetsInput{
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("vpc-id"),
							Values: []*string{aws.String("vpc-1234")},
						},
						{
							Name:   aws.String("tag:Name"),
							Values: []*string{aws.String("testcluster-a1b1-public-us-east-1*")},
						},
						{
							Name:   aws.String("tag:kubernetes.io/cluster/testcluster-a1b1"),
							Values: []*string{aws.String("owned")},
						},
						{
							Name:   aws.String("tag:" + internalELBLabel),
							Values: []*string{aws.String("")},
						},
						{
							Name:   aws.String("tag:" + internalGatewayLabel),
							Values: []*string{aws.String("")},
						},
					},
				}).Return(&ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{{SubnetId: aws.String("sub-1"), AvailabilityZone: aws.String("z1")}}}, nil)
				mock.EXPECT().DeleteTags(&ec2.DeleteTagsInput{
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
					Resources: []*string{aws.String("sub-1")},
				}).Return(&ec2.DeleteTagsOutput{}, nil)
				sg := &ec2.SecurityGroup{GroupId: aws.String("sg-3"), IpPermissions: getIPsecPortsPermission(
					int64(4500), int64(4900))}
				mock.EXPECT().DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("vpc-id"),
							Values: []*string{aws.String("vpc-1234")},
						},
						{
							Name:   aws.String("tag:Name"),
							Values: []*string{aws.String("testcluster-a1b1-submariner-gw-sg")},
						},
					},
				}).Return(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: []*ec2.SecurityGroup{sg}}, nil)
				mock.EXPECT().RevokeSecurityGroupIngress(&ec2.RevokeSecurityGroupIngressInput{
					GroupId:       sg.GroupId,
					IpPermissions: sg.IpPermissions,
				}).Return(&ec2.RevokeSecurityGroupIngressOutput{}, nil)

				masterSG := &ec2.SecurityGroup{GroupId: aws.String("sg-1"), OwnerId: aws.String("1234")}
				workerSG := &ec2.SecurityGroup{GroupId: aws.String("sg-2"), OwnerId: aws.String("1234")}
				routeWorkerPermission, routeMasterPermission := getRoutePortPermission(masterSG, workerSG, int64(4800), "udp")
				metricsWorkerPermission, metricsMasterPermission := getRoutePortPermission(masterSG, workerSG, int64(8080), "tcp")
				workerSG.IpPermissions = []*ec2.IpPermission{routeWorkerPermission, metricsWorkerPermission}
				masterSG.IpPermissions = []*ec2.IpPermission{routeMasterPermission, metricsMasterPermission}
				mock.EXPECT().DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("vpc-id"),
							Values: []*string{aws.String("vpc-1234")},
						},
						{
							Name:   aws.String("tag:Name"),
							Values: []*string{aws.String("testcluster-a1b1-worker-sg")},
						},
					},
				}).Return(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: []*ec2.SecurityGroup{workerSG}}, nil)
				mock.EXPECT().DescribeSecurityGroups(&ec2.DescribeSecurityGroupsInput{
					Filters: []*ec2.Filter{
						{
							Name:   aws.String("vpc-id"),
							Values: []*string{aws.String("vpc-1234")},
						},
						{
							Name:   aws.String("tag:Name"),
							Values: []*string{aws.String("testcluster-a1b1-master-sg")},
						},
					},
				}).Return(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: []*ec2.SecurityGroup{masterSG}}, nil)
				mock.EXPECT().RevokeSecurityGroupIngress(&ec2.RevokeSecurityGroupIngressInput{
					GroupId:       workerSG.GroupId,
					IpPermissions: []*ec2.IpPermission{metricsWorkerPermission},
				}).Return(&ec2.RevokeSecurityGroupIngressOutput{}, nil)
				mock.EXPECT().RevokeSecurityGroupIngress(&ec2.RevokeSecurityGroupIngressInput{
					GroupId:       masterSG.GroupId,
					IpPermissions: []*ec2.IpPermission{metricsMasterPermission},
				}).Return(&ec2.RevokeSecurityGroupIngressOutput{}, nil)
				mock.EXPECT().RevokeSecurityGroupIngress(&ec2.RevokeSecurityGroupIngressInput{
					GroupId:       workerSG.GroupId,
					IpPermissions: []*ec2.IpPermission{routeWorkerPermission},
				}).Return(&ec2.RevokeSecurityGroupIngressOutput{}, nil)
				mock.EXPECT().RevokeSecurityGroupIngress(&ec2.RevokeSecurityGroupIngressInput{
					GroupId:       masterSG.GroupId,
					IpPermissions: []*ec2.IpPermission{routeMasterPermission},
				}).Return(&ec2.RevokeSecurityGroupIngressOutput{}, nil)
			},
			validateActions: func(t *testing.T, workActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, workActions, "delete")
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mockCtrl := gomock.NewController(t)
			defer mockCtrl.Finish()

			awsClient := mockaws.NewMockInterface(mockCtrl)
			c.expectInvoking(awsClient)

			workClient := fakeworkclient.NewSimpleClientset(c.works...)
			awsProvider := &awsProvider{
				workClient:        workClient,
				awsClient:         awsClient,
				region:            "us-east-1",
				infraId:           "testcluster-a1b1",
				nattPort:          int64(4500),
				nattDiscoveryPort: int64(4900),
				clusterName:       "testcluster",
				eventRecorder:     eventstesting.NewTestingEventRecorder(t),
			}

			err := awsProvider.CleanUpSubmarinerClusterEnv()
			if err != nil {
				t.Errorf("expect no err, bug got %v", err)
			}

			c.validateActions(t, workClient.Actions())
		})
	}
}
