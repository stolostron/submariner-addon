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

package ocp

import (
	"context"
	"fmt"

	"github.com/submariner-io/admiral/pkg/resource"
	"github.com/submariner-io/admiral/pkg/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

// MachineSetDeployer can deploy and delete machinesets from OCP
type MachineSetDeployer interface {
	// Deploy makes sure to deploy the given machine set (creating or updating it)
	Deploy(machineSet *unstructured.Unstructured) error

	// GetWorkerNodeImage returns the image used by OCP worker nodes
	GetWorkerNodeImage(machineSet *unstructured.Unstructured, infraID string) (string, error)

	// Delete will remove the given machineset
	Delete(machineSet *unstructured.Unstructured) error
}

type k8sMachineSetDeployer struct {
	restMapper    meta.RESTMapper
	dynamicClient dynamic.Interface
}

// NewK8sMachinesetDeployer returns a MachineSetDeployer capable deploying directly to Kubernetes
func NewK8sMachinesetDeployer(restMapper meta.RESTMapper, dynamicClient dynamic.Interface) MachineSetDeployer {
	return &k8sMachineSetDeployer{
		dynamicClient: dynamicClient,
		restMapper:    restMapper}
}

func (msd *k8sMachineSetDeployer) clientFor(obj runtime.Object) (resource.Interface, error) {
	machineSet, gvr, err := util.ToUnstructuredResource(obj, msd.restMapper)
	if err != nil {
		return nil, err
	}

	dynamicClient := msd.dynamicClient.Resource(*gvr).Namespace(machineSet.GetNamespace())

	return resource.ForDynamic(dynamicClient), nil
}

func (msd *k8sMachineSetDeployer) GetWorkerNodeImage(machineSet *unstructured.Unstructured, infraID string) (string, error) {
	machineSetClient, err := msd.clientFor(machineSet)
	if err != nil {
		return "", err
	}

	// TODO: After implementing a ListAll method in admiral, modify this code accordingly.
	workerNodeList := []string{infraID + "-worker-b", infraID + "-worker-c", infraID + "-worker-d"}

	for _, nodeName := range workerNodeList {
		existing, err := machineSetClient.Get(context.TODO(), nodeName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) || err != nil {
			continue
		}

		obj, err := resource.ToUnstructured(existing)
		if err != nil {
			continue
		}

		disks, ok, _ := unstructured.NestedSlice(obj.Object, "spec", "template", "spec", "providerSpec", "value", "disks")
		if ok && len(disks) > 0 {
			for _, o := range disks {
				disk := o.(map[string]interface{})
				image, _, _ := unstructured.NestedString(disk, "image")
				if image != "" {
					return image, nil
				}
			}
		}
	}

	return "", fmt.Errorf("could not retrieve the image of one of the worker nodes on gcp infra %q", infraID)
}

func (msd *k8sMachineSetDeployer) Deploy(machineSet *unstructured.Unstructured) error {
	machineSetClient, err := msd.clientFor(machineSet)
	if err != nil {
		return err
	}

	_, err = util.CreateOrUpdate(context.TODO(), machineSetClient, machineSet, util.Replace(machineSet))

	return err
}

func (msd *k8sMachineSetDeployer) Delete(machineSet *unstructured.Unstructured) error {
	machineSetClient, err := msd.clientFor(machineSet)
	if err != nil {
		return err
	}

	err = machineSetClient.Delete(context.TODO(), machineSet.GetName(), metav1.DeleteOptions{})
	if apierrors.IsNotFound(err) {
		return nil
	}

	return err
}
