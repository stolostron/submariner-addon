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
	"strings"

	"github.com/pkg/errors"
	"github.com/submariner-io/admiral/pkg/resource"
	"github.com/submariner-io/admiral/pkg/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
)

//go:generate mockgen -source=./machinesets.go -destination=./fake/machineset.go -package=fake

// MachineSetDeployer can deploy and delete machinesets from OCP.
type MachineSetDeployer interface {
	// Deploy makes sure to deploy the given machine set (creating or updating it).
	Deploy(machineSet *unstructured.Unstructured) error

	// GetWorkerNodeImage returns the image used by OCP worker nodes.
	// If an empty workerNodeList is passed, the API will internally query the worker nodes.
	GetWorkerNodeImage(workerNodeList []string, machineSet *unstructured.Unstructured, infraID string) (string, error)

	// List will list all the machinesets that contains name.
	List(machineSet *unstructured.Unstructured, name string) ([]unstructured.Unstructured, error)

	// Delete will remove the given machineset.
	Delete(machineSet *unstructured.Unstructured) error
}

type k8sMachineSetDeployer struct {
	restMapper    meta.RESTMapper
	dynamicClient dynamic.Interface
}

// NewK8sMachinesetDeployer returns a MachineSetDeployer capable deploying directly to Kubernetes.
func NewK8sMachinesetDeployer(restMapper meta.RESTMapper, dynamicClient dynamic.Interface) MachineSetDeployer {
	return &k8sMachineSetDeployer{
		dynamicClient: dynamicClient,
		restMapper:    restMapper,
	}
}

func (msd *k8sMachineSetDeployer) clientFor(obj runtime.Object) (dynamic.ResourceInterface, error) {
	machineSet, gvr, err := util.ToUnstructuredResource(obj, msd.restMapper)
	if err != nil {
		return nil, errors.Wrap(err, "error converting to unstructured")
	}

	return msd.dynamicClient.Resource(*gvr).Namespace(machineSet.GetNamespace()), nil
}

func (msd *k8sMachineSetDeployer) GetWorkerNodeImage(workerNodeList []string, machineSet *unstructured.Unstructured,
	infraID string,
) (string, error) {
	machineSetClient, err := msd.clientFor(machineSet)
	if err != nil {
		return "", err
	}

	if len(workerNodeList) == 0 {
		nodeList, err := machineSetClient.List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return "", errors.Wrapf(err, "error listing the machineSets")
		}

		for _, machineName := range nodeList.Items {
			workerNodeList = append(workerNodeList, machineName.GetName())
		}
	}

	for _, nodeName := range workerNodeList {
		existing, err := machineSetClient.Get(context.TODO(), nodeName, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			continue
		}

		if err != nil {
			return "", errors.Wrapf(err, "error retrieving machine set %q", nodeName)
		}

		labels, found, _ := unstructured.NestedStringMap(existing.Object, "spec", "template", "metadata", "labels")
		if found {
			role := labels["machine.openshift.io/cluster-api-machine-role"]
			if strings.Compare(strings.ToLower(role), "worker") != 0 {
				continue
			}
		}

		disks, _, _ := unstructured.NestedSlice(existing.Object, "spec", "template", "spec", "providerSpec", "value", "disks")
		if len(disks) == 0 {
			image, _, _ := unstructured.NestedString(existing.Object, "spec", "template", "spec", "providerSpec", "value", "image")
			if image != "" {
				return image, nil
			}
		} else {
			for _, o := range disks {
				disk := o.(map[string]interface{})

				image, _, _ := unstructured.NestedString(disk, "image")
				if image != "" {
					return image, nil
				}
			}
		}
	}

	return "", fmt.Errorf("could not retrieve the image of one of the worker nodes from the infra %q", infraID)
}

func (msd *k8sMachineSetDeployer) Deploy(machineSet *unstructured.Unstructured) error {
	machineSetClient, err := msd.clientFor(machineSet)
	if err != nil {
		return err
	}

	_, err = util.CreateOrUpdate(context.TODO(), resource.ForDynamic(machineSetClient), machineSet, util.Replace(machineSet))

	return errors.Wrapf(err, "error creating machine set %#v", machineSet)
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

	return errors.Wrapf(err, "error deleting machine set %q", machineSet.GetName())
}

func (msd *k8sMachineSetDeployer) List(machineSet *unstructured.Unstructured, name string) ([]unstructured.Unstructured, error) {
	machineSetClient, err := msd.clientFor(machineSet)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to create machinesetclient")
	}

	machineSetList, err := machineSetClient.List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to list machinesets")
	}

	var resultList []unstructured.Unstructured
	machinesetItems := machineSetList.Items

	for i := range machinesetItems {
		machineName, _, _ := unstructured.NestedString(machinesetItems[i].Object, "metadata", "name")
		if strings.Contains(machineName, name) {
			resultList = append(resultList, machinesetItems[i])
		}
	}

	return resultList, nil
}
