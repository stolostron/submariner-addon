package client

import (
	"context"
	"fmt"

	"golang.org/x/oauth2/google"

	compute "google.golang.org/api/compute/v1"
	dns "google.golang.org/api/dns/v1"
	"google.golang.org/api/option"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

const gcpCredentialsName = "osServiceAccount.json"

//go:generate mockgen -source=./client.go -destination=./mock/client_generated.go -package=mock

// Interface wraps an actual GCP library client to allow for easier testing.
type Interface interface {
	GetProjectID() string
	InsertFirewallRule(rule *compute.Firewall) error
	GetFirewallRule(name string) (*compute.Firewall, error)
	DeleteFirewallRule(name string) error
	UpdateFirewallRule(name string, rule *compute.Firewall) error
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
