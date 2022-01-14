package manifestwork_test

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/openshift/library-go/pkg/operator/events"
	"github.com/stolostron/submariner-addon/pkg/helpers/testing"
	"github.com/stolostron/submariner-addon/pkg/manifestwork"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"open-cluster-management.io/api/client/work/clientset/versioned/fake"
	workv1 "open-cluster-management.io/api/work/v1"
)

var _ = Describe("Apply", func() {
	var (
		work          *workv1.ManifestWork
		existingWorks []runtime.Object
		workClient    *fake.Clientset
	)

	BeforeEach(func() {
		existingWorks = []runtime.Object{}

		work = &workv1.ManifestWork{
			TypeMeta: metav1.TypeMeta{},
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test",
				Namespace: "test-ns",
			},
			Spec: workv1.ManifestWorkSpec{
				Workload: workv1.ManifestsTemplate{
					Manifests: []workv1.Manifest{
						{
							RawExtension: runtime.RawExtension{
								Raw: []byte("test-data"),
							},
						},
					},
				},
			},
		}
	})

	JustBeforeEach(func() {
		workClient = fake.NewSimpleClientset(existingWorks...)
	})

	doApply := func() error {
		return manifestwork.Apply(context.TODO(), workClient, work, events.NewLoggingEventRecorder("test"))
	}

	ensureWork := func() {
		actual, err := workClient.WorkV1().ManifestWorks(work.Namespace).Get(context.TODO(), work.Name, metav1.GetOptions{})
		Expect(err).To(Succeed())
		Expect(actual.Spec).To(Equal(work.Spec))
	}

	When("the Work doesn't exist", func() {
		It("should create it", func() {
			Expect(doApply()).To(Succeed())
			ensureWork()
		})

		Context("and creation fails", func() {
			JustBeforeEach(func() {
				testing.FailOnAction(&workClient.Fake, "manifestworks", "create", nil, false)
			})

			It("should return an error", func() {
				Expect(doApply()).ToNot(Succeed())
			})
		})
	})

	When("the Work exists", func() {
		BeforeEach(func() {
			existingWorks = []runtime.Object{work.DeepCopy()}
		})

		Context("and the workload manifest has changed", func() {
			BeforeEach(func() {
				work.Spec.Workload.Manifests[0].RawExtension.Raw = []byte("test-data-updated")
			})

			It("should update it", func() {
				Expect(doApply()).To(Succeed())
				ensureWork()
			})

			Context("and update fails", func() {
				JustBeforeEach(func() {
					testing.FailOnAction(&workClient.Fake, "manifestworks", "update", nil, false)
				})

				It("should return an error", func() {
					Expect(doApply()).ToNot(Succeed())
				})
			})

			Context("and update initially fails with a conflict error", func() {
				BeforeEach(func() {
					testing.ConflictOnUpdateReactor(&workClient.Fake, "manifestworks")
				})

				It("should eventually update it", func() {
					Expect(doApply()).To(Succeed())
					ensureWork()
				})
			})
		})

		Context("and a workload manifest was added", func() {
			BeforeEach(func() {
				work.Spec.Workload.Manifests = append(work.Spec.Workload.Manifests, workv1.Manifest{
					RawExtension: runtime.RawExtension{
						Raw: []byte("test-data2"),
					},
				})
			})

			It("should update it", func() {
				Expect(doApply()).To(Succeed())
				ensureWork()
			})

			Context("and update fails", func() {
				JustBeforeEach(func() {
					testing.FailOnAction(&workClient.Fake, "manifestworks", "update", nil, false)
				})

				It("should return an error", func() {
					Expect(doApply()).ToNot(Succeed())
				})
			})

			Context("and update initially fails with a conflict error", func() {
				BeforeEach(func() {
					testing.ConflictOnUpdateReactor(&workClient.Fake, "manifestworks")
				})

				It("should eventually update it", func() {
					Expect(doApply()).To(Succeed())
					ensureWork()
				})
			})
		})

		Context("and the Work Spec has not changed", func() {
			It("should not update it", func() {
				Expect(doApply()).To(Succeed())
				testing.EnsureNoActionsForResource(&workClient.Fake, "manifestworks", "update")
			})
		})
	})
})
