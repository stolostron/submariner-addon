package client

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	compute "google.golang.org/api/compute/v1"
	dns "google.golang.org/api/dns/v1"
	"google.golang.org/api/option"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const (
	gcpCredentialsName   = "osServiceAccount.json"
	credentialsSecretKey = "service_account.json"
)

//go:generate mockgen -source=./client.go -destination=./mock/client_generated.go -package=mock

// Interface wraps an actual GCP library client to allow for easier testing.
type Interface interface {
	GetProjectID() string
	InsertFirewallRule(rule *compute.Firewall) error
	GetFirewallRule(name string) (*compute.Firewall, error)
	DeleteFirewallRule(name string) error
	UpdateFirewallRule(name string, rule *compute.Firewall) error
	GetInstance(zone string, instance string) (*compute.Instance, error)
	EnablePublicIP(instance *compute.Instance) error
	DisablePublicIP(instance *compute.Instance) error
}

type gcpClient struct {
	projectID     string
	computeClient *compute.Service
}

func (g *gcpClient) GetProjectID() string {
	return g.projectID
}

func (g *gcpClient) InsertFirewallRule(rule *compute.Firewall) error {
	if _, err := g.computeClient.Firewalls.Insert(g.projectID, rule).Context(context.TODO()).Do(); err != nil {
		return err
	}
	return nil
}

func (g *gcpClient) GetFirewallRule(name string) (*compute.Firewall, error) {
	resp, err := g.computeClient.Firewalls.Get(g.projectID, name).Context(context.TODO()).Do()
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (g *gcpClient) DeleteFirewallRule(name string) error {
	if _, err := g.computeClient.Firewalls.Delete(g.projectID, name).Context(context.TODO()).Do(); err != nil {
		return err
	}
	return nil
}

func (g *gcpClient) UpdateFirewallRule(name string, rule *compute.Firewall) error {
	if _, err := g.computeClient.Firewalls.Update(g.projectID, name, rule).Context(context.TODO()).Do(); err != nil {
		return err
	}
	return nil
}

func (g *gcpClient) GetInstance(zone string, instance string) (*compute.Instance, error) {
	resp, err := g.computeClient.Instances.Get(g.projectID, zone, instance).Context(context.TODO()).Do()
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (g *gcpClient) EnablePublicIP(instance *compute.Instance) error {
	if len(instance.NetworkInterfaces) == 0 {
		return fmt.Errorf("there are no network interfaces for instance %s", instance.Name)
	}

	// the zone of instance is an URL, so we just need the latest value
	zone := instance.Zone[strings.LastIndex(instance.Zone, "/")+1:]
	networkInterface := instance.NetworkInterfaces[0]
	// the public IP has already been enabled for this instance
	if len(networkInterface.AccessConfigs) > 0 {
		return nil
	}

	if _, err := g.computeClient.Instances.AddAccessConfig(
		g.projectID, zone, instance.Name, networkInterface.Name, &compute.AccessConfig{}).
		Context(context.TODO()).Do(); err != nil {
		return err
	}
	return nil
}

func (g *gcpClient) DisablePublicIP(instance *compute.Instance) error {
	if len(instance.NetworkInterfaces) == 0 {
		return fmt.Errorf("there are no network interfaces for instance %s", instance.Name)
	}

	// the zone of instance is an URL, so we just need the latest value
	zone := instance.Zone[strings.LastIndex(instance.Zone, "/")+1:]
	networkInterface := instance.NetworkInterfaces[0]
	if _, err := g.computeClient.Instances.DeleteAccessConfig(
		g.projectID, zone, instance.Name, "External NAT", networkInterface.Name).
		Context(context.TODO()).Do(); err != nil {
		return err
	}
	return nil
}

func NewClient(kubeClient kubernetes.Interface, secretNamespace, secretName string) (Interface, error) {
	credentialsSecret, err := kubeClient.CoreV1().Secrets(secretNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	authJSON, ok := credentialsSecret.Data[gcpCredentialsName]
	if !ok {
		return nil, fmt.Errorf("the gcp credentials %s is not in secret %s/%s", gcpCredentialsName, secretNamespace, secretName)
	}

	ctx := context.TODO()

	// since we're using a single creds var, we should specify all the required scopes when initializing
	creds, err := google.CredentialsFromJSON(ctx, authJSON, dns.CloudPlatformScope)
	if err != nil {
		return nil, err
	}

	options := []option.ClientOption{
		option.WithCredentials(creds),
		option.WithUserAgent("open-cluster-management.io submarineraddon/v1"),
	}
	computeClient, err := compute.NewService(ctx, options...)
	if err != nil {
		return nil, err
	}

	return &gcpClient{
		projectID:     creds.ProjectID,
		computeClient: computeClient,
	}, nil
}

func NewOauth2Client(kubeClient kubernetes.Interface, secretNamespace, secretName string) (Interface, error) {
	credentialsSecret, err := kubeClient.CoreV1().Secrets(secretNamespace).Get(context.TODO(), secretName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	serviceAccountJSON, ok := credentialsSecret.Data[credentialsSecretKey]
	if !ok {
		return nil, fmt.Errorf("the gcp credentials %s is not in secret %s/%s", gcpCredentialsName, secretNamespace, secretName)
	}

	ctx := context.TODO()
	jwt, err := google.JWTConfigFromJSON(serviceAccountJSON, compute.CloudPlatformScope)
	if err != nil {
		return nil, err
	}

	projectID, err := getProjectIDFromJSONKey(serviceAccountJSON)
	if err != nil {
		return nil, err
	}

	service, err := compute.New(oauth2.NewClient(ctx, jwt.TokenSource(ctx)))
	if err != nil {
		return nil, err
	}

	service.UserAgent = "open-cluster-management.io/submarineraddon/v1"

	return &gcpClient{
		projectID:     projectID,
		computeClient: service,
	}, nil
}

func getProjectIDFromJSONKey(content []byte) (string, error) {
	var JSONKey struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(content, &JSONKey); err != nil {
		return "", fmt.Errorf("error unmarshalling JSON key: %v", err)
	}
	return JSONKey.ProjectID, nil
}
