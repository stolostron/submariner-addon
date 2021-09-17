package machineset

import "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

//go:generate mockgen -source=./machineset.go -destination=./mock/machineset_generated.go -package=mock

// Interface wraps an actual machineset deployer for mocking
type Interface interface {
	// Deploy makes sure to deploy the given machine set (creating or updating it)
	Deploy(machineSet *unstructured.Unstructured) error

	// GetWorkerNodeImage returns the image used by OCP worker nodes
	GetWorkerNodeImage(machineSet *unstructured.Unstructured, infraID string) (string, error)

	// Delete will remove the given machineset
	Delete(machineSet *unstructured.Unstructured) error
}
