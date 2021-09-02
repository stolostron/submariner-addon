package testing

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/dynamic/dynamicinformer"
	dynamicFake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/informers"
)

func NewDynamicClientWithInformer(namespace string) (dynamic.ResourceInterface, dynamicinformer.DynamicSharedInformerFactory,
	informers.GenericInformer) {
	fakeGVR := schema.GroupVersionResource{
		Group:    "fake-dynamic-client-group",
		Version:  "v1",
		Resource: "unstructureds",
	}

	scheme := runtime.NewScheme()
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: fakeGVR.Group, Version: fakeGVR.Version, Kind: "UnstructuredList"},
		&unstructured.UnstructuredList{})

	dynamicClient := dynamicFake.NewSimpleDynamicClient(scheme)
	dynamicInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, 0)

	return dynamicClient.Resource(fakeGVR).Namespace(namespace), dynamicInformerFactory, dynamicInformerFactory.ForResource(fakeGVR)
}
