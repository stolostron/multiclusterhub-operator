// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"
	"testing"

	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	monitorv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	clog "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestFindMultiClusterHubOperatorSubscription(t *testing.T) {
	testNamespace := "test-namespace"

	tests := []struct {
		name           string
		objects        []client.Object
		wantError      bool
		wantApproval   subv1alpha1.Approval
		setupEnvVars   func()
		cleanupEnvVars func()
	}{
		{
			name: "Successfully finds subscription with manual approval",
			objects: []client.Object{
				&subv1alpha1.Subscription{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-mch-subscription",
						Namespace: testNamespace,
					},
					Spec: &subv1alpha1.SubscriptionSpec{
						InstallPlanApproval: subv1alpha1.ApprovalManual,
						Package:             "test-package",
					},
					Status: subv1alpha1.SubscriptionStatus{
						CurrentCSV: "test-csv-v1.0.0",
					},
				},
				&subv1alpha1.ClusterServiceVersion{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-csv-v1.0.0",
						Namespace: testNamespace,
					},
					Spec: subv1alpha1.ClusterServiceVersionSpec{
						InstallStrategy: subv1alpha1.NamedInstallStrategy{
							StrategyName: "deployment",
							StrategySpec: subv1alpha1.StrategyDetailsDeployment{
								DeploymentSpecs: []subv1alpha1.StrategyDeploymentSpec{
									{
										Name: utils.MCHOperatorName,
										Spec: appsv1.DeploymentSpec{
											Replicas: &[]int32{1}[0],
											Selector: &metav1.LabelSelector{
												MatchLabels: map[string]string{"app": "test"},
											},
											Template: corev1.PodTemplateSpec{
												ObjectMeta: metav1.ObjectMeta{
													Labels: map[string]string{"app": "test"},
												},
												Spec: corev1.PodSpec{
													Containers: []corev1.Container{{Name: "test", Image: "test:latest"}},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			setupEnvVars: func() {
				t.Setenv("POD_NAMESPACE", testNamespace)
			},
			wantError:    false,
			wantApproval: subv1alpha1.ApprovalManual,
		},
		{
			name: "Successfully finds subscription with automatic approval",
			objects: []client.Object{
				&subv1alpha1.Subscription{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-mch-subscription-auto",
						Namespace: testNamespace,
					},
					Spec: &subv1alpha1.SubscriptionSpec{
						InstallPlanApproval: subv1alpha1.ApprovalAutomatic,
						Package:             "test-package",
					},
					Status: subv1alpha1.SubscriptionStatus{
						CurrentCSV: "test-csv-auto-v1.0.0",
					},
				},
				&subv1alpha1.ClusterServiceVersion{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-csv-auto-v1.0.0",
						Namespace: testNamespace,
					},
					Spec: subv1alpha1.ClusterServiceVersionSpec{
						InstallStrategy: subv1alpha1.NamedInstallStrategy{
							StrategyName: "deployment",
							StrategySpec: subv1alpha1.StrategyDetailsDeployment{
								DeploymentSpecs: []subv1alpha1.StrategyDeploymentSpec{
									{
										Name: utils.MCHOperatorName,
										Spec: appsv1.DeploymentSpec{
											Replicas: &[]int32{1}[0],
											Selector: &metav1.LabelSelector{
												MatchLabels: map[string]string{"app": "test"},
											},
											Template: corev1.PodTemplateSpec{
												ObjectMeta: metav1.ObjectMeta{
													Labels: map[string]string{"app": "test"},
												},
												Spec: corev1.PodSpec{
													Containers: []corev1.Container{{Name: "test", Image: "test:latest"}},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			setupEnvVars: func() {
				t.Setenv("POD_NAMESPACE", testNamespace)
			},
			wantError:    false,
			wantApproval: subv1alpha1.ApprovalAutomatic,
		},
		{
			name: "No subscription found",
			objects: []client.Object{
				&subv1alpha1.Subscription{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-subscription",
						Namespace: testNamespace,
					},
					Spec: &subv1alpha1.SubscriptionSpec{
						InstallPlanApproval: subv1alpha1.ApprovalManual,
						Package:             "other-package",
					},
					Status: subv1alpha1.SubscriptionStatus{
						CurrentCSV: "other-csv-v1.0.0",
					},
				},
				&subv1alpha1.ClusterServiceVersion{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "other-csv-v1.0.0",
						Namespace: testNamespace,
					},
					Spec: subv1alpha1.ClusterServiceVersionSpec{
						InstallStrategy: subv1alpha1.NamedInstallStrategy{
							StrategyName: "deployment",
							StrategySpec: subv1alpha1.StrategyDetailsDeployment{
								DeploymentSpecs: []subv1alpha1.StrategyDeploymentSpec{
									{
										Name: "other-operator",
										Spec: appsv1.DeploymentSpec{
											Replicas: &[]int32{1}[0],
											Selector: &metav1.LabelSelector{
												MatchLabels: map[string]string{"app": "other"},
											},
											Template: corev1.PodTemplateSpec{
												ObjectMeta: metav1.ObjectMeta{
													Labels: map[string]string{"app": "other"},
												},
												Spec: corev1.PodSpec{
													Containers: []corev1.Container{{Name: "other", Image: "other:latest"}},
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
			setupEnvVars: func() {
				t.Setenv("POD_NAMESPACE", testNamespace)
			},
			wantError: true,
		},
		{
			name:    "Missing POD_NAMESPACE environment variable",
			objects: []client.Object{},
			setupEnvVars: func() {
				// Don't set POD_NAMESPACE
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment variables
			if tt.setupEnvVars != nil {
				tt.setupEnvVars()
			}

			// Create scheme and add types
			s := scheme.Scheme
			_ = subv1alpha1.AddToScheme(s)
			_ = operatorsv1.AddToScheme(s)

			// Create fake client with test objects
			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				WithObjects(tt.objects...).
				Build()

			// Create reconciler
			reconciler := &MultiClusterHubReconciler{
				Client: fakeClient,
				Scheme: s,
				Log:    clog.Log.WithName("test"),
			}

			// Test FindMultiClusterHubOperatorSubscription
			ctx := context.TODO()
			sub, err := reconciler.FindMultiClusterHubOperatorSubscription(ctx)

			if tt.wantError {
				if err == nil {
					t.Errorf("FindMultiClusterHubOperatorSubscription() expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("FindMultiClusterHubOperatorSubscription() unexpected error: %v", err)
				return
			}

			if sub == nil {
				t.Errorf("FindMultiClusterHubOperatorSubscription() returned nil subscription")
				return
			}

			// Test GetInstallPlanApprovalFromSubscription
			approval := reconciler.GetInstallPlanApprovalFromSubscription(sub)
			if approval != tt.wantApproval {
				t.Errorf("GetInstallPlanApprovalFromSubscription() = %v, want %v", approval, tt.wantApproval)
			}
		})
	}
}

func TestGetInstallPlanApprovalFromSubscription(t *testing.T) {
	reconciler := &MultiClusterHubReconciler{
		Log: clog.Log.WithName("test"),
	}

	tests := []struct {
		name         string
		subscription *subv1alpha1.Subscription
		want         subv1alpha1.Approval
	}{
		{
			name: "Manual approval",
			subscription: &subv1alpha1.Subscription{
				Spec: &subv1alpha1.SubscriptionSpec{
					InstallPlanApproval: subv1alpha1.ApprovalManual,
				},
			},
			want: subv1alpha1.ApprovalManual,
		},
		{
			name: "Automatic approval",
			subscription: &subv1alpha1.Subscription{
				Spec: &subv1alpha1.SubscriptionSpec{
					InstallPlanApproval: subv1alpha1.ApprovalAutomatic,
				},
			},
			want: subv1alpha1.ApprovalAutomatic,
		},
		{
			name:         "Nil subscription",
			subscription: nil,
			want:         subv1alpha1.ApprovalAutomatic,
		},
		{
			name: "Nil spec",
			subscription: &subv1alpha1.Subscription{
				Spec: nil,
			},
			want: subv1alpha1.ApprovalAutomatic,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := reconciler.GetInstallPlanApprovalFromSubscription(tt.subscription)
			if got != tt.want {
				t.Errorf("GetInstallPlanApprovalFromSubscription() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestEnsureDeprecatedResourceCleanup(t *testing.T) {
	tests := []struct {
		name string
		mch  *operatorsv1.MultiClusterHub
		want error
	}{
		{
			name: "should ensure deprecated resources are cleaned up",
			mch: &operatorsv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "multiclusterhub",
					Namespace: "test-ns",
				},
			},
			want: nil,
		},
	}

	s := scheme.Scheme
	_ = operatorsv1.AddToScheme(s)
	_ = rbacv1.AddToScheme(s)
	_ = monitorv1.AddToScheme(s)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fakeClient := fake.NewClientBuilder().
				WithScheme(s).
				Build()

			reconciler := &MultiClusterHubReconciler{
				Client: fakeClient,
				Scheme: s,
				Log:    clog.Log.WithName("test"),
			}

			for _, obj := range reconciler.GetDeprecatedResources(tt.mch) {
				if err := fakeClient.Create(context.TODO(), obj); err != nil {
					t.Errorf("Failed to create %v: %v", obj, err)
				}

				if _, err := reconciler.EnsureDeprecatedResourceCleanup(context.TODO(), obj); err != nil {
					t.Errorf("EnsureDeprecatedResourceCleanup() = %v, want: %v", err, tt.want)
				}

				// Verify the resource was actually deleted from the cluster
				check := obj.DeepCopyObject().(client.Object)
				err := fakeClient.Get(context.TODO(), types.NamespacedName{
					Name:      obj.GetName(),
					Namespace: obj.GetNamespace(),
				}, check)
				if !apierrors.IsNotFound(err) {
					t.Errorf("expected NotFound after cleanup, got: %v", err)
				}
			}
		})
	}

	// Test: resource does not exist (NotFound) — no cleanup needed
	t.Run("should skip cleanup when resource does not exist", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().
			WithScheme(s).
			Build()

		reconciler := &MultiClusterHubReconciler{
			Client: fakeClient,
			Scheme: s,
			Log:    clog.Log.WithName("test"),
		}

		obj := &rbacv1.Role{
			TypeMeta:   metav1.TypeMeta{Kind: "Role"},
			ObjectMeta: metav1.ObjectMeta{Name: "nonexistent-role", Namespace: "test-ns"},
		}

		result, err := reconciler.EnsureDeprecatedResourceCleanup(context.TODO(), obj)
		if err != nil {
			t.Errorf("EnsureDeprecatedResourceCleanup() unexpected error: %v", err)
		}
		if result.RequeueAfter != 0 || result.Requeue {
			t.Errorf("EnsureDeprecatedResourceCleanup() expected empty result, got: %v", result)
		}
	})

	// Test: CRD not installed (NoMatchError) — nothing to clean up
	t.Run("should skip cleanup when CRD is not installed", func(t *testing.T) {
		noMatchErr := &apimeta.NoResourceMatchError{
			PartialResource: schema.GroupVersionResource{
				Group: "monitoring.coreos.com", Version: "v1", Resource: "prometheusrules",
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(s).
			WithInterceptorFuncs(interceptor.Funcs{
				Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return noMatchErr
				},
			}).
			Build()

		reconciler := &MultiClusterHubReconciler{
			Client: fakeClient,
			Scheme: s,
			Log:    clog.Log.WithName("test"),
		}

		obj := &monitorv1.PrometheusRule{
			ObjectMeta: metav1.ObjectMeta{Name: "test-rule", Namespace: "test-ns"},
		}

		result, err := reconciler.EnsureDeprecatedResourceCleanup(context.TODO(), obj)
		if err != nil {
			t.Errorf("EnsureDeprecatedResourceCleanup() unexpected error: %v", err)
		}
		if result.RequeueAfter != 0 || result.Requeue {
			t.Errorf("EnsureDeprecatedResourceCleanup() expected empty result, got: %v", result)
		}
	})

	// Test: unexpected Get error — should propagate error
	t.Run("should return error on unexpected Get failure", func(t *testing.T) {
		fakeClient := fake.NewClientBuilder().
			WithScheme(s).
			WithInterceptorFuncs(interceptor.Funcs{
				Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					return fmt.Errorf("unexpected API server error")
				},
			}).
			Build()

		reconciler := &MultiClusterHubReconciler{
			Client: fakeClient,
			Scheme: s,
			Log:    clog.Log.WithName("test"),
		}

		obj := &rbacv1.Role{
			TypeMeta:   metav1.TypeMeta{Kind: "Role"},
			ObjectMeta: metav1.ObjectMeta{Name: "test-role", Namespace: "test-ns"},
		}

		_, err := reconciler.EnsureDeprecatedResourceCleanup(context.TODO(), obj)
		if err == nil {
			t.Errorf("EnsureDeprecatedResourceCleanup() expected error but got nil")
		}
	})

	// Test: resource has DeletionTimestamp (being terminated) — should requeue
	t.Run("should requeue when resource has DeletionTimestamp", func(t *testing.T) {
		existingRole := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "console-metrics-monitor",
				Namespace: "test-ns",
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(s).
			WithObjects(existingRole).
			WithInterceptorFuncs(interceptor.Funcs{
				Get: func(ctx context.Context, cl client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
					if err := cl.Get(ctx, key, obj, opts...); err != nil {
						return err
					}
					now := metav1.Now()
					obj.SetDeletionTimestamp(&now)
					return nil
				},
			}).
			Build()

		reconciler := &MultiClusterHubReconciler{
			Client: fakeClient,
			Scheme: s,
			Log:    clog.Log.WithName("test"),
		}

		obj := &rbacv1.Role{
			TypeMeta:   metav1.TypeMeta{Kind: "Role"},
			ObjectMeta: metav1.ObjectMeta{Name: "console-metrics-monitor", Namespace: "test-ns"},
		}

		result, err := reconciler.EnsureDeprecatedResourceCleanup(context.TODO(), obj)
		if err != nil {
			t.Errorf("EnsureDeprecatedResourceCleanup() unexpected error: %v", err)
		}
		if result.RequeueAfter == 0 {
			t.Errorf("EnsureDeprecatedResourceCleanup() expected RequeueAfter to be set")
		}
	})

	// Test: Delete returns error — should propagate error
	t.Run("should return error when Delete fails", func(t *testing.T) {
		existingRole := &rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "console-metrics-monitor",
				Namespace: "test-ns",
			},
		}

		fakeClient := fake.NewClientBuilder().
			WithScheme(s).
			WithObjects(existingRole).
			WithInterceptorFuncs(interceptor.Funcs{
				Delete: func(ctx context.Context, cl client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
					return fmt.Errorf("failed to delete resource")
				},
			}).
			Build()

		reconciler := &MultiClusterHubReconciler{
			Client: fakeClient,
			Scheme: s,
			Log:    clog.Log.WithName("test"),
		}

		obj := &rbacv1.Role{
			TypeMeta:   metav1.TypeMeta{Kind: "Role"},
			ObjectMeta: metav1.ObjectMeta{Name: "console-metrics-monitor", Namespace: "test-ns"},
		}

		_, err := reconciler.EnsureDeprecatedResourceCleanup(context.TODO(), obj)
		if err == nil {
			t.Errorf("EnsureDeprecatedResourceCleanup() expected error but got nil")
		}
	})
}

func TestGetDeprecatedResources(t *testing.T) {
	mch := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multiclusterhub",
			Namespace: "test-ns",
		},
	}

	s := runtime.NewScheme()
	reconciler := &MultiClusterHubReconciler{
		Scheme: s,
		Log:    clog.Log.WithName("test"),
	}

	resources := reconciler.GetDeprecatedResources(mch)

	expectedNames := []string{
		"console-metrics-monitor",
		"console-metrics-monitor",
		"acm-console-prometheus-rules",
		"console-monitor",
	}

	if len(resources) != len(expectedNames) {
		t.Fatalf("GetDeprecatedResources() returned %d resources, want %d", len(resources), len(expectedNames))
	}

	for i, res := range resources {
		if res.GetName() != expectedNames[i] {
			t.Errorf("resource[%d] name = %q, want %q", i, res.GetName(), expectedNames[i])
		}
		if res.GetNamespace() != mch.Namespace {
			t.Errorf("resource[%d] namespace = %q, want %q", i, res.GetNamespace(), mch.Namespace)
		}
	}
}
