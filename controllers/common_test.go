// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"testing"

	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
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
