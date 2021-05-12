/*
© 2021 Red Hat, Inc. and others.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package aws

import (
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

func ec2Filter(name, value string) *ec2.Filter {
	return &ec2.Filter{
		Name:   aws.String(name),
		Values: []*string{aws.String(value)},
	}
}

func ec2Tag(key, value string) *ec2.Tag {
	return &ec2.Tag{
		Key:   aws.String(key),
		Value: aws.String(value),
	}
}

func ec2FilterByTag(tag *ec2.Tag) *ec2.Filter {
	return ec2Filter(fmt.Sprintf("tag:%s", *tag.Key), *tag.Value)
}

func hasTag(tags []*ec2.Tag, desired *ec2.Tag) bool {
	for _, tag := range tags {
		if *tag.Key == *desired.Key {
			return true
		}
	}

	return false
}

func extractName(tags []*ec2.Tag) string {
	for _, tag := range tags {
		if *tag.Key == "Name" {
			return *tag.Value
		}
	}

	return ""
}

func (ac *awsCloud) withAWSInfo(str string) string {
	r := strings.NewReplacer("{infraID}", ac.infraID, "{region}", ac.region)
	return r.Replace(str)
}

func (ac *awsCloud) filterByName(name string) *ec2.Filter {
	return ec2Filter("tag:Name", ac.withAWSInfo(name))
}

func (ac *awsCloud) filterByCurrentCluster() *ec2.Filter {
	return ec2Filter(ac.withAWSInfo("tag:kubernetes.io/cluster/{infraID}"), "owned")
}
