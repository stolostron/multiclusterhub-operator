// Copyright Contributors to the Open Cluster Management project

package v1

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	mce "github.com/stolostron/backplane-operator/api/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
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
					Spec: MultiClusterHubSpec{
						LocalClusterName: "test-local-cluster",
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
				mch.Spec.Overrides = &Overrides{}
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
				mch.Spec.AvailabilityConfig = ""
			})

			By("because of existing local-cluster resource", func() {
				managedCluster := NewManagedCluster(mch.Spec.LocalClusterName)
				Expect(k8sClient.Create(ctx, managedCluster)).To(Succeed())

				mch.Spec.LocalClusterName = "updated-local-cluster"
				Expect(k8sClient.Update(ctx, mch)).NotTo(BeNil(), "updating local-cluster name while one exists should not be permitted")
			})

			By("because the local-cluster name must be less than 35 characters long", func() {
				mch.Spec.LocalClusterName = strings.Repeat("t", 35)
				expectedError := &k8serrors.StatusError{
					ErrStatus: metav1.Status{
						TypeMeta: metav1.TypeMeta{Kind: "", APIVersion: ""},
						ListMeta: metav1.ListMeta{
							SelfLink:           "",
							ResourceVersion:    "",
							Continue:           "",
							RemainingItemCount: nil,
						},
						Status:  "Failure",
						Message: "admission webhook \"multiclusterhub.validating-webhook.open-cluster-management.io\" denied the request: local-cluster name must be shorter than 35 characters",
						Reason:  "Forbidden",
						Details: nil,
						Code:    403,
					},
				}
				Expect(k8sClient.Update(ctx, mch)).To(Equal(expectedError), "local-cluster name must be less than 35 characters long")
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
			By("expecting the managedCluster to exist", func() {
				managedCluster := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "cluster.open-cluster-management.io/v1",
						"kind":       "ManagedCluster",
						"metadata": map[string]interface{}{
							"name": mch.Spec.LocalClusterName,
						},
					},
				}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: mch.Spec.LocalClusterName}, managedCluster)).To(Succeed())
			})

			By("deleting", func() {
				Expect(k8sClient.Delete(ctx, mch)).To(BeNil(), "MCH delete was blocked unexpectedly")
			})
		})
	})

	Context("Adopting an MCE during MCH Creation", func() {
		It("Should be the only MCH to exist", func() {
			mch := &MultiClusterHub{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: multiClusterHubName, Namespace: "default"}, mch)
			Expect(k8serrors.IsNotFound(err)).To(BeTrue())
		})
		It("Should create the MCE", func() {
			mce := &mce.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mce",
					Namespace: "test-mce",
				},
				Spec: mce.MultiClusterEngineSpec{
					LocalClusterName: "local-cluster",
				},
			}
			Expect(k8sClient.Create(ctx, mce)).To(Succeed())
		})

		It("Should fail to create the ACM", func() {
			mch := &MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      multiClusterHubName,
					Namespace: "default",
				},
				Spec: MultiClusterHubSpec{
					LocalClusterName: "renamed-local-cluster",
				},
			}
			Expect(k8sClient.Create(ctx, mch)).NotTo(Succeed())
		})

		It("Should succeed in creating the multiclusterhub", func() {
			mch := &MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      multiClusterHubName,
					Namespace: "default",
				},
				Spec: MultiClusterHubSpec{
					LocalClusterName: "local-cluster",
				},
			}
			Expect(k8sClient.Create(ctx, mch)).To(Succeed())
		})
	})

	Context("Deprecated annotation and field warnings", func() {
		It("Should return warnings for deprecated annotations", func() {
			mch := &MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-warnings",
					Namespace: "default",
					Annotations: map[string]string{
						"mch-pause":             "true",
						"mch-imageRepository":   "quay.io/test",
						"mch-imageOverridesCM":  "test-cm",
						"ignoreOCPVersion":      "true",
						"mch-kubeconfig":        "test-kubeconfig",
						"some-other-annotation": "value",
					},
				},
				Spec: MultiClusterHubSpec{
					LocalClusterName: "test-cluster",
				},
			}

			warnings := checkDeprecatedAnnotations(mch)
			Expect(len(warnings)).To(Equal(5), "Should have 5 warnings for deprecated annotations")

			// Check that all deprecated annotations are warned about
			warningText := strings.Join(warnings, " ")
			Expect(warningText).To(ContainSubstring("mch-pause"))
			Expect(warningText).To(ContainSubstring("mch-imageRepository"))
			Expect(warningText).To(ContainSubstring("mch-imageOverridesCM"))
			Expect(warningText).To(ContainSubstring("ignoreOCPVersion"))
			Expect(warningText).To(ContainSubstring("mch-kubeconfig"))

			// Check that warnings mention the replacement annotations
			Expect(warningText).To(ContainSubstring("installer.open-cluster-management.io/pause"))
			Expect(warningText).To(ContainSubstring("installer.open-cluster-management.io/image-repository"))
			Expect(warningText).To(ContainSubstring("installer.open-cluster-management.io/image-overrides-configmap"))
			Expect(warningText).To(ContainSubstring("installer.open-cluster-management.io/ignore-ocp-version"))
			Expect(warningText).To(ContainSubstring("installer.open-cluster-management.io/kubeconfig"))
		})

		It("Should return warnings for deprecated spec fields", func() {
			mch := &MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-spec-warnings",
					Namespace: "default",
				},
				Spec: MultiClusterHubSpec{
					LocalClusterName:              "test-cluster",
					Hive:                          &HiveConfigSpec{},
					CustomCAConfigmap:             "test-ca",
					EnableClusterBackup:           true,
					EnableClusterProxyAddon:       true,
					SeparateCertificateManagement: true,
				},
			}

			warnings := checkDeprecatedAnnotations(mch)
			Expect(len(warnings)).To(Equal(5), "Should have 5 warnings for deprecated spec fields")

			warningText := strings.Join(warnings, " ")
			Expect(warningText).To(ContainSubstring("spec.hive"))
			Expect(warningText).To(ContainSubstring("spec.customCAConfigmap"))
			Expect(warningText).To(ContainSubstring("spec.enableClusterBackup"))
			Expect(warningText).To(ContainSubstring("spec.enableClusterProxyAddon"))
			Expect(warningText).To(ContainSubstring("spec.separateCertificateManagement"))
		})

		It("Should return combined warnings for both annotations and spec fields", func() {
			mch := &MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-combined-warnings",
					Namespace: "default",
					Annotations: map[string]string{
						"mch-pause": "true",
					},
				},
				Spec: MultiClusterHubSpec{
					LocalClusterName:    "test-cluster",
					Hive:                &HiveConfigSpec{},
					EnableClusterBackup: true,
				},
			}

			warnings := checkDeprecatedAnnotations(mch)
			Expect(len(warnings)).To(Equal(3), "Should have 3 warnings (1 annotation + 2 spec fields)")
		})

		It("Should return no warnings for resources without deprecated fields", func() {
			mch := &MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-no-warnings",
					Namespace: "default",
					Annotations: map[string]string{
						"installer.open-cluster-management.io/pause": "true",
						"some-other-annotation":                      "value",
					},
				},
				Spec: MultiClusterHubSpec{
					LocalClusterName: "test-cluster",
				},
			}

			warnings := checkDeprecatedAnnotations(mch)
			Expect(len(warnings)).To(Equal(0), "Should have no warnings")
		})

		It("Should return no warnings when annotations are nil", func() {
			mch := &MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-nil-annotations",
					Namespace: "default",
				},
				Spec: MultiClusterHubSpec{
					LocalClusterName: "test-cluster",
				},
			}

			warnings := checkDeprecatedAnnotations(mch)
			Expect(len(warnings)).To(Equal(0), "Should have no warnings when annotations are nil")
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
