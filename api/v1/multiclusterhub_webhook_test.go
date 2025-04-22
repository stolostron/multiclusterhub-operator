// Copyright Contributors to the Open Cluster Management project

package v1

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

var (
	multiClusterHubName = "multiclusterhub"
)

var _ = Describe("Multiclusterhub webhook", func() {

	Context("Creating a Multiclusterhub", func() {
		It("Should successfully create multiclusterhub", func() {
			By("by creating a new standalone Multiclusterhub resource", func() {
				mch := &MultiClusterHub{
					ObjectMeta: metav1.ObjectMeta{
						Name:      multiClusterHubName,
						Namespace: "default",
					},
				}
				Expect(k8sClient.Create(ctx, mch)).Should(Succeed())
			})
		})

		It("Should fail to create multiclusterhub", func() {
			By("because of invalid AvailabilityConfig", func() {
				mch := &MultiClusterHub{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-2", multiClusterHubName),
						Namespace: "default",
					},
					Spec: MultiClusterHubSpec{
						AvailabilityConfig: "low",
					},
				}
				Expect(k8sClient.Create(ctx, mch)).NotTo(BeNil(), "Invalid availability config is not allowed")
			})
			By("because of component configuration", func() {
				mch := &MultiClusterHub{
					ObjectMeta: metav1.ObjectMeta{
						Name:      fmt.Sprintf("%s-2", multiClusterHubName),
						Namespace: "default",
					},
					Spec: MultiClusterHubSpec{
						Overrides: &Overrides{
							Components: []ComponentConfig{
								{
									Name:    "fake-component",
									Enabled: true,
								},
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, mch)).NotTo(BeNil(), "Invalid components not allowed in config")
			})
		})

		It("Should fail to update multiclusterhub", func() {
			mch := &MultiClusterHub{}
			By("because of invalid component", func() {
				Expect(k8sClient.Get(ctx,
					types.NamespacedName{Name: multiClusterHubName, Namespace: "default"}, mch)).To(Succeed())

				mch.Spec.Overrides = &Overrides{
					Components: []ComponentConfig{
						{
							Name:    "fake-component",
							Enabled: true,
						},
					},
				}
				Expect(k8sClient.Update(ctx, mch)).NotTo(BeNil(), "invalid components should not be permitted")
			})
			By("because of updating SeparateCertificateManagement", func() {
				Expect(k8sClient.Get(ctx,
					types.NamespacedName{Name: multiClusterHubName, Namespace: "default"}, mch)).To(Succeed())

				// flipping it directly
				mch.Spec.SeparateCertificateManagement = !mch.Spec.SeparateCertificateManagement
				Expect(k8sClient.Update(ctx, mch)).NotTo(BeNil(),
					"updating SeparateCertificateManagement should be forbidden")
			})
			By("because of updating hive", func() {
				Expect(k8sClient.Get(ctx,
					types.NamespacedName{Name: multiClusterHubName, Namespace: "default"}, mch)).To(Succeed())

				mch.Spec.Hive = &HiveConfigSpec{}
				Expect(k8sClient.Update(ctx, mch)).NotTo(BeNil(), "hive updates are forbidden")
			})
			By("because of invalid AvailablityConfig", func() {
				Expect(k8sClient.Get(ctx,
					types.NamespacedName{Name: multiClusterHubName, Namespace: "default"}, mch)).To(Succeed())

				mch.Spec.AvailabilityConfig = "INVALID"
				Expect(k8sClient.Update(ctx, mch)).NotTo(BeNil(),
					"AvailabilityConfig must be %v or %v, but %v was allowed", HABasic, HAHigh,
					mch.Spec.AvailabilityConfig)
			})
		})

		It("Should succeed in updating multiclusterhub", func() {
			mch := &MultiClusterHub{}
			By("Updating absolutely nothing", func() {
				Expect(k8sClient.Get(ctx,
					types.NamespacedName{Name: multiClusterHubName, Namespace: "default"}, mch)).To(Succeed())
				Expect(k8sClient.Update(ctx, mch)).To(BeNil(), "Changing nothing should not throw an error")
			})
		})

		It("Should delete multiclusterhub", func() {
			mch := &MultiClusterHub{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: multiClusterHubName, Namespace: "default"}, mch)).To(Succeed())
			By("Creating the managedCluster", func() {
				managedCluster := NewManagedCluster(mch.Spec.LocalClusterName)
				Expect(k8sClient.Create(ctx, managedCluster)).To(Succeed())
			})
			By("deleting", func() {
				Expect(k8sClient.Delete(ctx, mch)).To(BeNil(), "MCH delete was blocked unexpectedly")
			})
		})
	})
})

// re-defining the function here to avoid a import cycle
func NewManagedCluster(name string) *unstructured.Unstructured {
	managedCluster := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cluster.open-cluster-management.io/v1",
			"kind":       "ManagedCluster",
			"metadata": map[string]interface{}{
				"name": name,
				"labels": map[string]interface{}{
					"local-cluster":                 "true",
					"cloud":                         "auto-detect",
					"vendor":                        "auto-detect",
					"velero.io/exclude-from-backup": "true",
				},
			},
			"spec": map[string]interface{}{
				"hubAcceptsClient": true,
			},
		},
	}
	return managedCluster
}
