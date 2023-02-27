package manifestwork

import (
	"bytes"
	"context"
	"reflect"

	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/stolostron/submariner-addon/pkg/manifestwork"
	"github.com/submariner-io/admiral/pkg/resource"
	"github.com/submariner-io/cloud-prepare/pkg/ocp"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	workclient "open-cluster-management.io/api/client/work/clientset/versioned"
	workv1 "open-cluster-management.io/api/work/v1"
)

type manifestWorkMachineSetDeployer struct {
	client        workclient.Interface
	workName      string
	clusterName   string
	eventRecorder events.Recorder
}

// NewMachineSetDeployer creates a new MachineSet deployer which uses ManifestWork
func NewMachineSetDeployer(client workclient.Interface, workName, clusterName string, eventRecorder events.Recorder) ocp.MachineSetDeployer {
	return &manifestWorkMachineSetDeployer{
		client:        client,
		workName:      workName,
		clusterName:   clusterName,
		eventRecorder: eventRecorder,
	}
}

func (msd *manifestWorkMachineSetDeployer) List() ([]unstructured.Unstructured, error) {
	// This MachineSetDeployer implementation is (currently) only used for AWS and this method is only used for RHOS
	// which uses a different implementation.
	panic("Not implemented")
}

func (msd *manifestWorkMachineSetDeployer) Deploy(machineSet *unstructured.Unstructured) error {
	existingManifestWork, err := msd.client.WorkV1().ManifestWorks(msd.clusterName).Get(context.TODO(), msd.workName, metav1.GetOptions{})

	if err != nil {
		if !errors.IsNotFound(err) {
			return err
		}

		existingManifestWork = nil
	}

	if existingManifestWork != nil {
		return msd.deployAdditionalMachineset(machineSet, existingManifestWork)
	}

	manifests := []workv1.Manifest{}

	// Ensure that we're allowed to manipulate machinesets
	aggregateClusterRole := rbacv1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "open-cluster-management:submariner-addon-machinesets-aggregate-clusterrole",
			Labels: map[string]string{
				"rbac.authorization.k8s.io/aggregate-to-admin": "true",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"machine.openshift.io"},
				Resources: []string{"machinesets"},
				Verbs:     []string{"get", "list", "watch", "create", "update", "patch", "delete"},
			},
		},
	}
	unstructuredClusterRole, err := resource.ToUnstructured(&aggregateClusterRole)
	if err != nil {
		return err
	}
	aggregateClusterRoleJson, err := toJSON(unstructuredClusterRole)
	if err != nil {
		return err
	}
	manifests = append(manifests, workv1.Manifest{RawExtension: runtime.RawExtension{Raw: aggregateClusterRoleJson}})

	machineSetJson, err := machineSet.MarshalJSON()
	if err != nil {
		return err
	}
	manifests = append(manifests, workv1.Manifest{RawExtension: runtime.RawExtension{Raw: machineSetJson}})

	return manifestwork.Apply(context.TODO(), msd.client, &workv1.ManifestWork{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:      msd.workName,
			Namespace: msd.clusterName,
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{Manifests: manifests},
		},
	}, msd.eventRecorder)
}

// deployAdditionalMachineset only if it doesn't already exist in the manifest work
func (msd *manifestWorkMachineSetDeployer) deployAdditionalMachineset(machineSet *unstructured.Unstructured, existingManifestWork *workv1.ManifestWork) error {
	manifests := existingManifestWork.Spec.Workload.Manifests

	// In case the machineset is already in the manifestwork, skip it not to cause disruptions
	for _, manifest := range manifests {
		existingMachineSet := &unstructured.Unstructured{}
		if err := existingMachineSet.UnmarshalJSON(manifest.RawExtension.Raw); err != nil {
			return err
		}

		if reflect.DeepEqual(machineSet, existingMachineSet) {
			return nil
		}
	}

	machineSetJson, err := machineSet.MarshalJSON()
	if err != nil {
		return err
	}

	manifests = append(manifests, workv1.Manifest{RawExtension: runtime.RawExtension{Raw: machineSetJson}})
	existingManifestWork.Spec.Workload.Manifests = manifests

	msd.eventRecorder.Eventf("ManifestWorkMachineSetDeployer",
		"Adding MachineSet %s to ManifestWork %s", machineSet.GetName(), existingManifestWork.Name)
	return manifestwork.Apply(context.TODO(), msd.client, existingManifestWork, msd.eventRecorder)
}

func (msd *manifestWorkMachineSetDeployer) GetWorkerNodeImage(workerNodeList []string, machineSet *unstructured.Unstructured, infraID string) (string, error) {
	// This isn't used for AWS
	return "", nil
}

func (msd *manifestWorkMachineSetDeployer) Delete(machineSet *unstructured.Unstructured) error {
	err := msd.client.WorkV1().ManifestWorks(msd.clusterName).Delete(context.TODO(), msd.workName, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

func (msd *manifestWorkMachineSetDeployer) DeleteByName(name, namespace string) error {
	err := msd.client.WorkV1().ManifestWorks(namespace).Delete(context.TODO(), name, metav1.DeleteOptions{})
	if errors.IsNotFound(err) {
		return nil
	}
	return err
}

func toJSON(obj runtime.Object) ([]byte, error) {
	jsonSerializer := json.NewSerializerWithOptions(json.DefaultMetaFactory, nil, nil, json.SerializerOptions{})

	var b bytes.Buffer
	writer := json.Framer.NewFrameWriter(&b)
	if err := jsonSerializer.Encode(obj, writer); err != nil {
		return []byte{}, err
	} else {
		return b.Bytes(), nil
	}
}
