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

	"github.com/pkg/errors"
	"github.com/submariner-io/admiral/pkg/resource"
	"github.com/submariner-io/admiral/pkg/util"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
)

const (
	SubmarinerGatewayLabel = "submariner.io/gateway"
)

type Interface interface {
	ListNodesWithLabel(labelSelector string) (*v1.NodeList, error)
	ListGatewayNodes() (*v1.NodeList, error)
	AddGWLabelOnNode(nodeName string) error
	RemoveGWLabelFromWorkerNodes() error
	RemoveGWLabelFromWorkerNode(node *v1.Node) error
}

type k8sIface struct {
	clientSet kubernetes.Interface
}

func NewInterface(clientSet kubernetes.Interface) Interface {
	return &k8sIface{clientSet: clientSet}
}

func (k *k8sIface) ListNodesWithLabel(labelSelector string) (*v1.NodeList, error) {
	nodes, err := k.clientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, errors.Wrap(err, "unable to list the nodes in the cluster")
	}

	return nodes, nil
}

func (k *k8sIface) ListGatewayNodes() (*v1.NodeList, error) {
	labelSelector := SubmarinerGatewayLabel + "=true"

	nodes, err := k.clientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		return nil, errors.Wrap(err, "unable to list the Gateway nodes in the cluster")
	}

	return nodes, nil
}

func (k *k8sIface) updateLabel(nodeName string, mutate func(existing *v1.Node)) error {
	//nolint:wrapcheck // Let the caller wrap these errors.
	client := &resource.InterfaceFuncs{
		GetFunc: func(ctx context.Context, name string, options metav1.GetOptions) (runtime.Object, error) {
			return k.clientSet.CoreV1().Nodes().Get(ctx, name, options)
		},
		UpdateFunc: func(ctx context.Context, obj runtime.Object, options metav1.UpdateOptions) (runtime.Object, error) {
			return k.clientSet.CoreV1().Nodes().Update(ctx, obj.(*v1.Node), options)
		},
	}

	return errors.Wrap(util.Update(context.TODO(), client, &v1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name: nodeName,
		},
	}, func(existing runtime.Object) (runtime.Object, error) {
		mutate(existing.(*v1.Node))
		return existing, nil
	}), "error updating node")
}

func (k *k8sIface) AddGWLabelOnNode(nodeName string) error {
	return k.updateLabel(nodeName, func(existing *v1.Node) {
		labels := existing.GetLabels()
		if labels == nil {
			labels = map[string]string{}
		}

		labels[SubmarinerGatewayLabel] = "true"
		existing.SetLabels(labels)
	})
}

func (k *k8sIface) RemoveGWLabelFromWorkerNodes() error {
	gwNodeList, err := k.clientSet.CoreV1().Nodes().List(context.TODO(), metav1.ListOptions{LabelSelector: SubmarinerGatewayLabel})
	if err != nil {
		return errors.Wrap(err, "error listing submariner gateway nodes")
	}

	gwNodes := gwNodeList.Items
	for i := range gwNodes {
		err = k.RemoveGWLabelFromWorkerNode(&gwNodes[i])
		if err != nil {
			return errors.Wrap(err, fmt.Sprintf("error removing the label from the gateway node %q", gwNodes[i].Name))
		}
	}

	return nil
}

func (k *k8sIface) RemoveGWLabelFromWorkerNode(node *v1.Node) error {
	return k.updateLabel(node.Name, func(existing *v1.Node) {
		delete(existing.Labels, SubmarinerGatewayLabel)
	})
}
