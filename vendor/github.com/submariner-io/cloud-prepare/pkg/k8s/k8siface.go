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

package k8s

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
)

const (
	submarinerGatewayLabel = "submariner.io/gateway"
)

type K8sInterface interface {
	ListWorkerNodes(labelSelector string) (*v1.NodeList, error)
	AddGWLabelOnNode(nodeName string) error
	RemoveGWLabelFromWorkerNodes() error
}

type k8sIface struct {
	clientSet kubernetes.Interface
}

func NewK8sInterface(clientSet kubernetes.Interface) (K8sInterface, error) {
	return &k8sIface{clientSet: clientSet}, nil
}

func (k *k8sIface) ListWorkerNodes(labelSelector string) (*v1.NodeList, error) {
	nodes, err := k.clientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, fmt.Errorf("unable to list the nodes in the cluster, err: %s", err)
	}

	return nodes, nil
}

func (k *k8sIface) AddGWLabelOnNode(nodeName string) error {
	retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		node, err := k.clientSet.CoreV1().Nodes().Get(context.TODO(), nodeName, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("unable to get node info for node %v, err: %s", nodeName, err)
		}

		labels := node.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}
		labels[submarinerGatewayLabel] = "true"
		node.SetLabels(labels)
		_, updateErr := k.clientSet.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
		return updateErr
	})

	if retryErr != nil {
		return fmt.Errorf("error updatating node %q, err: %s", nodeName, retryErr)
	}

	return nil
}

func (k *k8sIface) RemoveGWLabelFromWorkerNodes() error {
	nodeList, err := k.clientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: "node-role.kubernetes.io/worker"})
	if err != nil {
		return err
	}

	for _, node := range nodeList.Items {
		retryErr := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			node, err := k.clientSet.CoreV1().Nodes().Get(context.TODO(), node.Name, metav1.GetOptions{})
			if err != nil {
				return fmt.Errorf("unable to get node info for node %v, err: %s", node.Name, err)
			}

			labels := node.GetLabels()
			if labels == nil {
				return nil
			}
			delete(labels, submarinerGatewayLabel)
			node.SetLabels(labels)
			_, updateErr := k.clientSet.CoreV1().Nodes().Update(context.TODO(), node, metav1.UpdateOptions{})
			return updateErr
		})

		if retryErr != nil {
			return fmt.Errorf("error updatating node %q, err: %s", node.Name, retryErr)
		}
	}

	return nil
}
