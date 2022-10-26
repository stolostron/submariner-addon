/*
SPDX-License-Identifier: Apache-2.0

Copyright Contributors to the Submariner project.

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

//nolint:wrapcheck // The functions are wrappers so let the caller wrap errors.
package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	compute "google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"
)

//go:generate mockgen -source=./client.go -destination=./fake/client.go -package=fake

// Interface wraps an actual GCP library client to allow for easier testing.
type Interface interface {
	InsertFirewallRule(projectID string, rule *compute.Firewall) error
	GetFirewallRule(projectID, name string) (*compute.Firewall, error)
	DeleteFirewallRule(projectID, name string) error
	UpdateFirewallRule(projectID, name string, rule *compute.Firewall) error
	GetInstance(zone string, instance string) (*compute.Instance, error)
	ListInstances(zone string) (*compute.InstanceList, error)
	ListZones() (*compute.ZoneList, error)
	InstanceHasPublicIP(instance *compute.Instance) (bool, error)
	UpdateInstanceNetworkTags(project, zone, instance string, tags *compute.Tags) error
	ConfigurePublicIPOnInstance(instance *compute.Instance) error
	DeletePublicIPOnInstance(instance *compute.Instance) error
}

type gcpClient struct {
	projectID     string
	computeClient *compute.Service
}

func (g *gcpClient) InsertFirewallRule(projectID string, rule *compute.Firewall) error {
	_, err := g.computeClient.Firewalls.Insert(projectID, rule).Context(context.TODO()).Do()
	return err
}

func (g *gcpClient) GetFirewallRule(projectID, name string) (*compute.Firewall, error) {
	return g.computeClient.Firewalls.Get(projectID, name).Context(context.TODO()).Do()
}

func (g *gcpClient) DeleteFirewallRule(projectID, name string) error {
	_, err := g.computeClient.Firewalls.Delete(projectID, name).Context(context.TODO()).Do()
	return err
}

func (g *gcpClient) UpdateFirewallRule(projectID, name string, rule *compute.Firewall) error {
	_, err := g.computeClient.Firewalls.Update(projectID, name, rule).Context(context.TODO()).Do()
	return err
}

func NewClient(projectID string, options []option.ClientOption) (Interface, error) {
	ctx := context.TODO()

	computeClient, err := compute.NewService(ctx, options...)
	if err != nil {
		return nil, err
	}

	return &gcpClient{
		projectID:     projectID,
		computeClient: computeClient,
	}, nil
}

func IsGCPNotFoundError(err error) bool {
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		return gerr.Code == http.StatusNotFound
	}

	return false
}

func (g *gcpClient) GetInstance(zone, instance string) (*compute.Instance, error) {
	return g.computeClient.Instances.Get(g.projectID, zone, instance).Context(context.TODO()).Do()
}

func (g *gcpClient) ListInstances(zone string) (*compute.InstanceList, error) {
	return g.computeClient.Instances.List(g.projectID, zone).Context(context.TODO()).Do()
}

func (g *gcpClient) ListZones() (*compute.ZoneList, error) {
	return g.computeClient.Zones.List(g.projectID).Context(context.TODO()).Do()
}

func (g *gcpClient) InstanceHasPublicIP(instance *compute.Instance) (bool, error) {
	if len(instance.NetworkInterfaces) == 0 {
		return false, fmt.Errorf("there are no network interfaces for instance %s", instance.Name)
	}

	networkInterface := instance.NetworkInterfaces[0]

	return len(networkInterface.AccessConfigs) > 0, nil
}

func (g *gcpClient) UpdateInstanceNetworkTags(project, zone, instance string, tags *compute.Tags) error {
	_, err := g.computeClient.Instances.SetTags(project, zone, instance, tags).Context(context.TODO()).Do()

	return err
}

func (g *gcpClient) ConfigurePublicIPOnInstance(instance *compute.Instance) error {
	if len(instance.NetworkInterfaces) == 0 {
		return fmt.Errorf("there are no network interfaces for instance %s", instance.Name)
	}

	// The zone of an instance is on URL, so we just need the latest value
	zone := instance.Zone[strings.LastIndex(instance.Zone, "/")+1:]
	networkInterface := instance.NetworkInterfaces[0]
	// Public IP has already been enabled for this instance
	if len(networkInterface.AccessConfigs) > 0 {
		return nil
	}

	_, err := g.computeClient.Instances.AddAccessConfig(g.projectID, zone, instance.Name,
		networkInterface.Name, &compute.AccessConfig{}).
		Context(context.TODO()).Do()

	return err
}

func (g *gcpClient) DeletePublicIPOnInstance(instance *compute.Instance) error {
	if len(instance.NetworkInterfaces) == 0 {
		return fmt.Errorf("there are no network interfaces for instance %s", instance.Name)
	}

	// The zone of an instance is on URL, so we just need the latest value
	zone := instance.Zone[strings.LastIndex(instance.Zone, "/")+1:]
	networkInterface := instance.NetworkInterfaces[0]
	_, err := g.computeClient.Instances.DeleteAccessConfig(
		g.projectID, zone, instance.Name, "External NAT", networkInterface.Name).
		Context(context.TODO()).Do()

	return err
}
