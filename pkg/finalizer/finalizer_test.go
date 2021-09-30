package finalizer_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/open-cluster-management/submariner-addon/pkg/finalizer"
	helpers "github.com/open-cluster-management/submariner-addon/pkg/helpers/testing"
	"github.com/submariner-io/admiral/pkg/resource"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeFake "k8s.io/client-go/kubernetes/fake"
)

const finalizerName = "test-finalizer"

func TestFinalizer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Finalizer Suite")
}

var _ = Describe("Add", func() {
	var (
		t     *testDriver
		added bool
		err   error
	)

	BeforeEach(func() {
		t = newTestDriver()
	})

	JustBeforeEach(func() {
		t.justBeforeEach()
		added, err = finalizer.Add(context.TODO(), t.client, t.pod, finalizerName)
	})

	When("the resource has no Finalizers", func() {
		It("should add the new one", func() {
			Expect(err).To(Succeed())
			Expect(added).To(BeTrue())
			t.ensureFinalizers(finalizerName)
		})

		Context("and update initially fails with a conflict error", func() {
			BeforeEach(func() {
				helpers.ConflictOnUpdateReactor(&t.kubeClient.Fake, "pods")
			})

			It("should eventually succeed", func() {
				Expect(err).To(Succeed())
				Expect(added).To(BeTrue())
				t.ensureFinalizers(finalizerName)
			})
		})
	})

	When("the resource has other Finalizers", func() {
		BeforeEach(func() {
			t.pod.Finalizers = []string{"other"}
		})

		It("should append the new one", func() {
			Expect(err).To(Succeed())
			Expect(added).To(BeTrue())
			t.ensureFinalizers("other", finalizerName)
		})
	})

	When("the resource already has the Finalizer", func() {
		BeforeEach(func() {
			t.pod.Finalizers = []string{finalizerName}
		})

		It("should not try to re-add it", func() {
			Expect(err).To(Succeed())
			Expect(added).To(BeFalse())
			helpers.EnsureNoActionsForResource(&t.kubeClient.Fake, "pods", "get", "update")
			t.ensureFinalizers(finalizerName)
		})
	})

	When("the resource is being deleted", func() {
		BeforeEach(func() {
			now := metav1.Now()
			t.pod.DeletionTimestamp = &now
		})

		It("should not add the Finalizer", func() {
			Expect(err).To(Succeed())
			Expect(added).To(BeFalse())
			t.ensureFinalizers()
		})
	})

	When("update fails", func() {
		BeforeEach(func() {
			helpers.FailOnAction(&t.kubeClient.Fake, "pods", "update", nil, false)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(added).To(BeFalse())
		})
	})
})

var _ = Describe("Remove", func() {
	var (
		t   *testDriver
		err error
	)

	BeforeEach(func() {
		t = newTestDriver()
		t.pod.Finalizers = []string{finalizerName}
	})

	JustBeforeEach(func() {
		t.justBeforeEach()
		err = finalizer.Remove(context.TODO(), t.client, t.pod, finalizerName)
	})

	When("the Finalizer is present", func() {
		It("should remove it", func() {
			Expect(err).To(Succeed())
			t.ensureFinalizers()
		})

		Context("and update initially fails with a conflict error", func() {
			BeforeEach(func() {
				helpers.ConflictOnUpdateReactor(&t.kubeClient.Fake, "pods")
			})

			It("should eventually succeed", func() {
				Expect(err).To(Succeed())
				t.ensureFinalizers()
			})
		})
	})

	When("other Finalizers are also present", func() {
		BeforeEach(func() {
			t.pod.Finalizers = []string{"other1", finalizerName, "other2"}
		})

		It("should not remove the others", func() {
			Expect(err).To(Succeed())
			t.ensureFinalizers("other1", "other2")
		})
	})

	When("the Finalizer is not present", func() {
		BeforeEach(func() {
			t.pod.Finalizers = []string{"other"}
		})

		It("should not try to remove it", func() {
			Expect(err).To(Succeed())
			helpers.EnsureNoActionsForResource(&t.kubeClient.Fake, "pods", "get", "update")
			t.ensureFinalizers("other")
		})
	})

	When("update fails", func() {
		BeforeEach(func() {
			helpers.FailOnAction(&t.kubeClient.Fake, "pods", "update", nil, false)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
		})
	})
})

type testDriver struct {
	pod        *corev1.Pod
	kubeClient *kubeFake.Clientset
	client     resource.Interface
}

func newTestDriver() *testDriver {
	t := &testDriver{
		pod: &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "test-pod",
				Namespace:  "test-ns",
				Finalizers: []string{},
			},
		},
		kubeClient: kubeFake.NewSimpleClientset(),
	}

	return t
}

func (t *testDriver) justBeforeEach() {
	t.client = resource.ForPod(t.kubeClient, t.pod.Namespace)

	_, err := t.client.Create(context.TODO(), t.pod, metav1.CreateOptions{})
	Expect(err).To(Succeed())

	t.kubeClient.Fake.ClearActions()
}

func (t *testDriver) ensureFinalizers(exp ...string) {
	if exp == nil {
		exp = []string{}
	}

	obj, err := t.client.Get(context.TODO(), t.pod.Name, metav1.GetOptions{})
	Expect(err).To(Succeed())
	Expect(obj.(*corev1.Pod).Finalizers).To(Equal(exp))
}
