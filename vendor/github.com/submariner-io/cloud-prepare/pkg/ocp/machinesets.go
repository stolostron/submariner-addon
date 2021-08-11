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

	"github.com/submariner-io/admiral/pkg/resource"
	"github.com/submariner-io/admiral/pkg/util"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

// MachineSetDeployer can deploy and delete machinesets from OCP
type MachineSetDeployer interface {
	// Deploy makes sure to deploy the given machine set (creating or updating it)
	Deploy(machineSet *unstructured.Unstructured) error

	// Delete will remove the given machineset
	Delete(machineSet *unstructured.Unstructured) error
}

type k8sMachineSetDeployer struct {
	k8sConfig *rest.Config
}

// NewK8sMachinesetDeployer returns a MachineSetDeployer capable deploying directly to Kubernetes
func NewK8sMachinesetDeployer(k8sConfig *rest.Config) MachineSetDeployer {
	return &k8sMachineSetDeployer{k8sConfig: k8sConfig}
}

func (msd *k8sMachineSetDeployer) clientFor(obj runtime.Object) (resource.Interface, error) {
	k8sClient, err := dynamic.NewForConfig(msd.k8sConfig)
	if err != nil {
		return nil, err
	}

	restMapper, err := util.BuildRestMapper(msd.k8sConfig)
	if err != nil {
		return nil, err
	}

	machineSet, gvr, err := util.ToUnstructuredResource(obj, restMapper)
	if err != nil {
		return nil, err
	}

	dynamicClient := k8sClient.Resource(*gvr).Namespace(machineSet.GetNamespace())

	return resource.ForDynamic(dynamicClient), nil
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
