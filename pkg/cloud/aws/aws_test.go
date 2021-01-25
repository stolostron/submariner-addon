package aws

import (
	"fmt"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/ec2"

	"github.com/golang/mock/gomock"

	"github.com/openshift/library-go/pkg/operator/events/eventstesting"

	fakeworkclient "github.com/open-cluster-management/api/client/work/clientset/versioned/fake"
	workv1 "github.com/open-cluster-management/api/work/v1"
	mockaws "github.com/open-cluster-management/submariner-addon/pkg/cloud/aws/client/mock"
	testinghelpers "github.com/open-cluster-management/submariner-addon/pkg/helpers/testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clienttesting "k8s.io/client-go/testing"
)

func TestPrepareSubmarinerClusterEnv(t *testing.T) {
	cases := []struct {
		name            string
		works           []runtime.Object
		expectInvoking  func(*mockaws.MockInterface)
		validateActions func(t *testing.T, workActions []clienttesting.Action)
	}{
		{
			name:  "prepare a new submariner cluster env",
			works: []runtime.Object{},
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
				routeWorkerPermission, routeMasterPermission := getRoutePortPermission(masterSG, workerSG, int64(4800), "udp")
				mock.EXPECT().AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
					GroupId:       workerSG.GroupId,
					IpPermissions: []*ec2.IpPermission{routeWorkerPermission},
				}).Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, nil)
				mock.EXPECT().AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
					GroupId:       masterSG.GroupId,
					IpPermissions: []*ec2.IpPermission{routeMasterPermission},
				}).Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, nil)
				metricsWorkerPermission, metricsMasterPermission := getRoutePortPermission(masterSG, workerSG, int64(8080), "tcp")
				mock.EXPECT().AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
					GroupId:       workerSG.GroupId,
					IpPermissions: []*ec2.IpPermission{metricsWorkerPermission},
				}).Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, nil)
				mock.EXPECT().AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
					GroupId:       masterSG.GroupId,
					IpPermissions: []*ec2.IpPermission{metricsMasterPermission},
				}).Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, nil)
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
				}).Return(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: []*ec2.SecurityGroup{}}, nil)
				mock.EXPECT().CreateSecurityGroup(&ec2.CreateSecurityGroupInput{
					GroupName:   aws.String("testcluster-a1b1-submariner-gw-sg"),
					VpcId:       aws.String("vpc-1234"),
					Description: aws.String("For submariner gateway"),
					TagSpecifications: []*ec2.TagSpecification{
						{
							ResourceType: aws.String("security-group"),
							Tags: []*ec2.Tag{
								{
									Key:   aws.String("Name"),
									Value: aws.String("testcluster-a1b1-submariner-gw-sg"),
								},
							},
						},
					},
				}).Return(&ec2.CreateSecurityGroupOutput{GroupId: aws.String("sg-3")}, nil)
				mock.EXPECT().AuthorizeSecurityGroupIngress(&ec2.AuthorizeSecurityGroupIngressInput{
					GroupId:       aws.String("sg-3"),
					IpPermissions: getIPsecPortsPermission(int64(500), int64(4500)),
				}).Return(&ec2.AuthorizeSecurityGroupIngressOutput{}, nil)
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
					},
				}).Return(&ec2.DescribeSubnetsOutput{Subnets: []*ec2.Subnet{{SubnetId: aws.String("sub-1"), AvailabilityZone: aws.String("z1")}}}, nil)
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
				}}, nil)
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
				testinghelpers.AssertActions(t, workActions, "get", "create")
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
				gwSG := &ec2.SecurityGroup{
					GroupId:       aws.String("sg-3"),
					IpPermissions: getIPsecPortsPermission(int64(500), int64(4500)),
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
					},
				}).Return(&ec2.DescribeSecurityGroupsOutput{SecurityGroups: []*ec2.SecurityGroup{gwSG}}, nil)
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
				}}, nil)
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
				}}, nil)
			},
			validateActions: func(t *testing.T, workActions []clienttesting.Action) {
				testinghelpers.AssertActions(t, workActions, "get", "update")
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
				workClinet:    workClient,
				awsClinet:     awsClient,
				region:        "us-east-1",
				infraId:       "testcluster-a1b1",
				ikePort:       int64(500),
				nattPort:      int64(4500),
				clusterName:   "testcluster",
				eventRecorder: eventstesting.NewTestingEventRecorder(t),
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
							Name:   aws.String(fmt.Sprintf("tag:kubernetes.io/role/internal-elb")),
							Values: []*string{aws.String("")},
						},
						{
							Name:   aws.String("tag:open-cluster-management.io/submariner-addon/gateway"),
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
				sg := &ec2.SecurityGroup{GroupId: aws.String("sg-3"), IpPermissions: getIPsecPortsPermission(int64(500), int64(4500))}
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
				workClinet:    workClient,
				awsClinet:     awsClient,
				region:        "us-east-1",
				infraId:       "testcluster-a1b1",
				ikePort:       int64(500),
				nattPort:      int64(4500),
				clusterName:   "testcluster",
				eventRecorder: eventstesting.NewTestingEventRecorder(t),
			}

			err := awsProvider.CleanUpSubmarinerClusterEnv()
			if err != nil {
				t.Errorf("expect no err, bug got %v", err)
			}

			c.validateActions(t, workClient.Actions())
		})
	}
}
