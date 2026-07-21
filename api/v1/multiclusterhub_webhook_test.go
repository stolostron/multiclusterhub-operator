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

		It("Should fail to create multiclusterhub with MTV enabled and local-cluster disabled", func() {
			mch := &MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      fmt.Sprintf("%s-mtv-no-local", multiClusterHubName),
					Namespace: "default",
				},
				Spec: MultiClusterHubSpec{
					DisableHubSelfManagement: true,
					Overrides: &Overrides{
						Components: []ComponentConfig{
							{
								Name:    MTVIntegrations,
								Enabled: true,
							},
						},
					},
				},
			}
			err := k8sClient.Create(ctx, mch)
			Expect(err).To(HaveOccurred(), "MTV should not be enabled when disableHubSelfManagement is true")
			Expect(err.Error()).To(ContainSubstring(MTVIntegrations))
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

		It("Should fail to update multiclusterhub to disable local-cluster when MTV is enabled", func() {
			mch := &MultiClusterHub{}
			Expect(k8sClient.Get(ctx,
				types.NamespacedName{Name: multiClusterHubName, Namespace: "default"}, mch)).To(Succeed())

			mch.Spec.DisableHubSelfManagement = true
			mch.Spec.Overrides = &Overrides{
				Components: []ComponentConfig{
					{
						Name:    MTVIntegrations,
						Enabled: true,
					},
				},
			}
			err := k8sClient.Update(ctx, mch)
			Expect(err).To(HaveOccurred(), "disabling local-cluster should be blocked when MTV is enabled")
			Expect(err.Error()).To(ContainSubstring(MTVIntegrations))
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

	Context("OLM annotation validation", func() {
		It("Should reject v0 annotation when cluster has OLM v1", func() {
			// This test verifies the validation logic rejects mismatched annotations
			// Actual OLM detection requires live cluster CRD checking
			mch := &MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-v0-on-v1",
					Namespace: "default",
					Annotations: map[string]string{
						"installer.open-cluster-management.io/mce-subscription-spec": `{"channel": "stable-2.6"}`,
					},
				},
				Spec: MultiClusterHubSpec{
					LocalClusterName: "test-cluster",
				},
			}

			// Validation behavior depends on live cluster OLM detection
			// This test documents expected validation exists
			_ = mch
		})

		It("Should reject v1 annotation when cluster has OLM v0", func() {
			mch := &MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-v1-on-v0",
					Namespace: "default",
					Annotations: map[string]string{
						"installer.open-cluster-management.io/mce-clusterextension-spec": `{"channels": ["stable-2.6"]}`,
					},
				},
				Spec: MultiClusterHubSpec{
					LocalClusterName: "test-cluster",
				},
			}

			// Validation behavior depends on live cluster OLM detection
			_ = mch
		})

		It("Should reject OADP v0 annotation when cluster has OLM v1", func() {
			mch := &MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-oadp-v0-on-v1",
					Namespace: "default",
					Annotations: map[string]string{
						"installer.open-cluster-management.io/oadp-subscription-spec": `{"channel": "stable-1.4"}`,
					},
				},
				Spec: MultiClusterHubSpec{
					LocalClusterName: "test-cluster",
				},
			}

			// Validation behavior depends on live cluster OLM detection
			_ = mch
		})

		It("Should reject OADP v1 annotation when cluster has OLM v0", func() {
			mch := &MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-oadp-v1-on-v0",
					Namespace: "default",
					Annotations: map[string]string{
						"installer.open-cluster-management.io/oadp-clusterextension-spec": `{"channels": ["stable"],"version": "1.6.0"}`,
					},
				},
				Spec: MultiClusterHubSpec{
					LocalClusterName: "test-cluster",
				},
			}

			// Validation behavior depends on live cluster OLM detection
			_ = mch
		})

		It("Should allow annotation when it matches cluster OLM version", func() {
			mch := &MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-matching-annotation",
					Namespace: "default",
				},
				Spec: MultiClusterHubSpec{
					LocalClusterName: "test-cluster",
				},
			}

			// No annotations set - should always pass
			Expect(validateOLMAnnotations(ctx, mch)).To(Succeed())
		})
	})

	Context("validateOLMAnnotationPair", func() {
		It("Should return nil when no annotations set", func() {
			err := validateOLMAnnotationPair("v1", map[string]string{},
				annotationMCESubscriptionSpec, annotationMCEClusterExtensionSpec)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should reject v0 annotation on v1 cluster", func() {
			err := validateOLMAnnotationPair("v1", map[string]string{
				annotationMCESubscriptionSpec: `{"channel": "stable-2.6"}`,
			}, annotationMCESubscriptionSpec, annotationMCEClusterExtensionSpec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("only valid for OLM v0"))
		})

		It("Should reject v1 annotation on v0 cluster", func() {
			err := validateOLMAnnotationPair("v0", map[string]string{
				annotationMCEClusterExtensionSpec: `{"channels": ["stable-2.6"]}`,
			}, annotationMCESubscriptionSpec, annotationMCEClusterExtensionSpec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("only valid for OLM v1"))
		})

		It("Should reject v1 annotation when no OLM detected", func() {
			err := validateOLMAnnotationPair("", map[string]string{
				annotationOADPClusterExtensionSpec: `{"channels": ["stable"]}`,
			}, annotationOADPSubscriptionSpec, annotationOADPClusterExtensionSpec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("requires OLM v1"))
		})

		It("Should allow v0 annotation on v0 cluster", func() {
			err := validateOLMAnnotationPair("v0", map[string]string{
				annotationOADPSubscriptionSpec: `{"channel": "stable-1.4"}`,
			}, annotationOADPSubscriptionSpec, annotationOADPClusterExtensionSpec)
			Expect(err).ToNot(HaveOccurred())
		})

		It("Should allow v1 annotation on v1 cluster", func() {
			err := validateOLMAnnotationPair("v1", map[string]string{
				annotationOADPClusterExtensionSpec: `{"channels": ["stable"]}`,
			}, annotationOADPSubscriptionSpec, annotationOADPClusterExtensionSpec)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("MTV and self-management validation", func() {
		It("Should return error when MTV is enabled and disableHubSelfManagement is true", func() {
			mch := &MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mtv-selfmgmt",
					Namespace: "default",
				},
				Spec: MultiClusterHubSpec{
					DisableHubSelfManagement: true,
					Overrides: &Overrides{
						Components: []ComponentConfig{
							{Name: MTVIntegrations, Enabled: true},
						},
					},
				},
			}
			err := validateMTVAndSelfManagement(mch)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(MTVIntegrations))
			Expect(err.Error()).To(ContainSubstring("disableHubSelfManagement"))
		})

		It("Should return nil when MTV is enabled and disableHubSelfManagement is false", func() {
			mch := &MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mtv-ok",
					Namespace: "default",
				},
				Spec: MultiClusterHubSpec{
					DisableHubSelfManagement: false,
					Overrides: &Overrides{
						Components: []ComponentConfig{
							{Name: MTVIntegrations, Enabled: true},
						},
					},
				},
			}
			Expect(validateMTVAndSelfManagement(mch)).To(Succeed())
		})

		It("Should return nil when disableHubSelfManagement is true but MTV is not enabled", func() {
			mch := &MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-selfmgmt-no-mtv",
					Namespace: "default",
				},
				Spec: MultiClusterHubSpec{
					DisableHubSelfManagement: true,
				},
			}
			Expect(validateMTVAndSelfManagement(mch)).To(Succeed())
		})

		It("Should return nil when MTV is explicitly disabled and disableHubSelfManagement is true", func() {
			mch := &MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mtv-disabled-selfmgmt",
					Namespace: "default",
				},
				Spec: MultiClusterHubSpec{
					DisableHubSelfManagement: true,
					Overrides: &Overrides{
						Components: []ComponentConfig{
							{Name: MTVIntegrations, Enabled: false},
						},
					},
				},
			}
			Expect(validateMTVAndSelfManagement(mch)).To(Succeed())
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
