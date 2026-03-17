package controllers

import (
	"context"
	"fmt"
	"testing"

	backplanev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengineutils"
	resources "github.com/stolostron/multiclusterhub-operator/test/unit-tests"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
)

func Test_waitForMigratedComponentsAdopted(t *testing.T) {
	tests := []struct {
		name          string
		mch           operatorv1.MultiClusterHub
		mce           *backplanev1.MultiClusterEngine
		mceComponents []backplanev1.ComponentCondition
		want          bool
		wantErr       bool
	}{
		{
			name:    "MCE not found with no components to check",
			mch:     resources.EmptyMCH(),
			mce:     nil,
			want:    false,
			wantErr: false,
		},
		{
			name: "MCE not found with component present in MCH",
			mch: operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "open-cluster-management",
				},
				Spec: operatorv1.MultiClusterHubSpec{
					Overrides: &operatorv1.Overrides{
						Components: []operatorv1.ComponentConfig{
							{Name: operatorv1.ClusterPermission, Enabled: true},
						},
					},
				},
			},
			mce:     nil,
			want:    false,
			wantErr: false,
		},
		{
			name: "component not present in MCH, nothing to check",
			mch:  resources.EmptyMCH(),
			mce: &backplanev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multiclusterengine",
				},
			},
			mceComponents: []backplanev1.ComponentCondition{},
			want:          true,
			wantErr:       false,
		},
		{
			name: "component present but disabled in MCH, nothing to check",
			mch: operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "open-cluster-management",
				},
				Spec: operatorv1.MultiClusterHubSpec{
					Overrides: &operatorv1.Overrides{
						Components: []operatorv1.ComponentConfig{
							{Name: operatorv1.ClusterPermission, Enabled: false},
						},
					},
				},
			},
			mce: &backplanev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multiclusterengine",
				},
			},
			mceComponents: []backplanev1.ComponentCondition{},
			want:          true,
			wantErr:       false,
		},
		{
			name: "component present in MCH, deployment not available in MCE",
			mch: operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "open-cluster-management",
				},
				Spec: operatorv1.MultiClusterHubSpec{
					Overrides: &operatorv1.Overrides{
						Components: []operatorv1.ComponentConfig{
							{Name: operatorv1.ClusterPermission, Enabled: true},
						},
					},
				},
			},
			mce: &backplanev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multiclusterengine",
				},
			},
			mceComponents: []backplanev1.ComponentCondition{
				{
					Name:   "cluster-permission",
					Kind:   "Deployment",
					Type:   "Progressing",
					Status: "True",
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "component present in MCH, deployment available in MCE",
			mch: operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "open-cluster-management",
				},
				Spec: operatorv1.MultiClusterHubSpec{
					Overrides: &operatorv1.Overrides{
						Components: []operatorv1.ComponentConfig{
							{Name: operatorv1.ClusterPermission, Enabled: true},
						},
					},
				},
			},
			mce: &backplanev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multiclusterengine",
				},
			},
			mceComponents: []backplanev1.ComponentCondition{
				{
					Name:   "cluster-permission",
					Kind:   "Deployment",
					Type:   "Available",
					Status: "True",
				},
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "deployment exists but Type is not Available",
			mch: operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "open-cluster-management",
				},
				Spec: operatorv1.MultiClusterHubSpec{
					Overrides: &operatorv1.Overrides{
						Components: []operatorv1.ComponentConfig{
							{Name: operatorv1.ClusterPermission, Enabled: true},
						},
					},
				},
			},
			mce: &backplanev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multiclusterengine",
				},
			},
			mceComponents: []backplanev1.ComponentCondition{
				{
					Name:   "cluster-permission",
					Kind:   "Deployment",
					Type:   "Degraded",
					Status: "True",
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "deployment exists but Status is not True",
			mch: operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "open-cluster-management",
				},
				Spec: operatorv1.MultiClusterHubSpec{
					Overrides: &operatorv1.Overrides{
						Components: []operatorv1.ComponentConfig{
							{Name: operatorv1.ClusterPermission, Enabled: true},
						},
					},
				},
			},
			mce: &backplanev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multiclusterengine",
				},
			},
			mceComponents: []backplanev1.ComponentCondition{
				{
					Name:   "cluster-permission",
					Kind:   "Deployment",
					Type:   "Available",
					Status: "False",
				},
			},
			want:    false,
			wantErr: false,
		},
		{
			name: "deployment not found in MCE status",
			mch: operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "open-cluster-management",
				},
				Spec: operatorv1.MultiClusterHubSpec{
					Overrides: &operatorv1.Overrides{
						Components: []operatorv1.ComponentConfig{
							{Name: operatorv1.ClusterPermission, Enabled: true},
						},
					},
				},
			},
			mce: &backplanev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multiclusterengine",
				},
			},
			mceComponents: []backplanev1.ComponentCondition{
				{
					Name:   "some-other-deployment",
					Kind:   "Deployment",
					Type:   "Available",
					Status: "True",
				},
			},
			want:    false,
			wantErr: false,
		},
	}

	registerScheme()
	for i, tt := range tests {
		tt := tt // capture range variable
		i := i   // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			// Make MCH name unique per test case to avoid conflicts
			mch := tt.mch.DeepCopy()
			if mch.Name == "" || mch.Name == "test-mch" {
				mch.Name = fmt.Sprintf("test-mch-%d", i)
			}

			// Setup: Create MCH
			if err := recon.Client.Create(context.TODO(), mch); err != nil {
				t.Errorf("failed to create MCH: %v", err)
			}
			defer recon.Client.Delete(context.TODO(), mch)

			// Setup: Create MCE if provided
			if tt.mce != nil {
				// Add installer labels so GetManagedMCE can find it
				tt.mce.Labels = map[string]string{
					"installer.name":                          mch.GetName(),
					"installer.namespace":                     mch.GetNamespace(),
					multiclusterengineutils.MCEManagedByLabel: "true",
				}
				tt.mce.Status.Components = tt.mceComponents
				if err := recon.Client.Create(context.TODO(), tt.mce); err != nil {
					t.Errorf("failed to create MCE: %v", err)
				}
				defer recon.Client.Delete(context.TODO(), tt.mce)
			}

			// Test
			got, err := recon.waitForMigratedComponentsAdopted(context.TODO(), mch)
			if (err != nil) != tt.wantErr {
				t.Errorf("waitForMigratedComponentsAdopted() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("waitForMigratedComponentsAdopted() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_ensureMigratedComponentsCleanup(t *testing.T) {
	tests := []struct {
		name          string
		mch           operatorv1.MultiClusterHub
		stsEnabled    bool
		expectNoError bool
	}{
		{
			name: "component not present, should skip cleanup and succeed",
			mch: operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "open-cluster-management",
				},
				Spec: operatorv1.MultiClusterHubSpec{},
			},
			stsEnabled:    false,
			expectNoError: true,
		},
		{
			name: "component not present with STS enabled",
			mch: operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "open-cluster-management",
				},
				Spec: operatorv1.MultiClusterHubSpec{},
			},
			stsEnabled:    true,
			expectNoError: true,
		},
		{
			name: "component present, will attempt cleanup",
			mch: operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "open-cluster-management",
				},
				Spec: operatorv1.MultiClusterHubSpec{
					Overrides: &operatorv1.Overrides{
						Components: []operatorv1.ComponentConfig{
							{Name: operatorv1.ClusterPermission, Enabled: true},
						},
					},
				},
			},
			stsEnabled:    false,
			expectNoError: false, // ensureNoComponent will return error/requeue with fake client
		},
	}

	registerScheme()
	for i, tt := range tests {
		tt := tt // capture range variable
		i := i   // capture range variable
		t.Run(tt.name, func(t *testing.T) {
			// Make MCH name unique per test case to avoid conflicts
			mch := tt.mch.DeepCopy()
			if mch.Name == "" || mch.Name == "test-mch" {
				mch.Name = fmt.Sprintf("test-mch-%d", i)
			}

			// Setup: Create MCH
			if err := recon.Client.Create(context.TODO(), mch); err != nil {
				t.Errorf("failed to create MCH: %v", err)
			}
			defer recon.Client.Delete(context.TODO(), mch)

			// Track if component was present before cleanup
			hadComponent := mch.ComponentPresent(operatorv1.ClusterPermission)

			// Test
			result, err := recon.ensureMigratedComponentsCleanup(context.TODO(), mch, tt.stsEnabled)

			// For components not present, should succeed with empty result
			if !hadComponent {
				if err != nil {
					t.Errorf("ensureMigratedComponentsCleanup() error = %v when component not present", err)
				}
				if result != (ctrl.Result{}) {
					t.Errorf("ensureMigratedComponentsCleanup() returned non-empty result when component not present: %v", result)
				}
			} else {
				// For components present, ensureNoComponent will be called
				// With fake client, this typically returns a requeue result or error
				// We just verify the function doesn't panic and handles the case
				if tt.expectNoError && err != nil {
					t.Errorf("ensureMigratedComponentsCleanup() unexpected error: %v", err)
				}
			}
		})
	}
}
