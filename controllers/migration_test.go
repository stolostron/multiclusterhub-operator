package controllers

import (
	"context"
	"fmt"
	"os"
	"testing"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

func init() {
	// Set TEMPLATES_PATH so chart rendering works in these tests
	os.Setenv("TEMPLATES_PATH", "../pkg/templates")
}

func Test_pruneMigratedComponents(t *testing.T) {
	tests := []struct {
		name          string
		mch           operatorv1.MultiClusterHub
		stsEnabled    bool
		expectNoError bool
	}{
		{
			name: "component not present, should skip and succeed",
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
			name: "component present, STS disabled - will attempt cleanup",
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
			expectNoError: true, // With no actual resources in fake client, cleanup succeeds and prunes component
		},
		{
			name: "component present, STS enabled - cleanup with STS mode",
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
			stsEnabled:    true,
			expectNoError: true, // Chart renders successfully with STS, cleanup succeeds with no resources, prunes component
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
			result, err := recon.pruneMigratedComponents(context.TODO(), mch, tt.stsEnabled)

			// For components not present, should succeed with empty result
			if !hadComponent {
				if err != nil {
					t.Errorf("pruneMigratedComponents() error = %v when component not present", err)
				}
				if result != (ctrl.Result{}) {
					t.Errorf("pruneMigratedComponents() returned non-empty result when component not present: %v", result)
				}
			} else {
				// For components present, verify expected error/requeue behavior
				if tt.expectNoError {
					// Should succeed with no error
					if err != nil {
						t.Errorf("pruneMigratedComponents() unexpected error: %v", err)
					}
					// Verify component was pruned from MCH CR after successful cleanup
					updatedMCH := &operatorv1.MultiClusterHub{}
					if err := recon.Client.Get(context.TODO(), types.NamespacedName{
						Name:      mch.Name,
						Namespace: mch.Namespace,
					}, updatedMCH); err != nil {
						t.Errorf("failed to get updated MCH: %v", err)
					} else if updatedMCH.ComponentPresent(operatorv1.ClusterPermission) {
						t.Errorf("pruneMigratedComponents() component should be pruned after successful cleanup")
					}
				} else {
					// Should either error or requeue
					if err == nil && result == (ctrl.Result{}) {
						t.Errorf("pruneMigratedComponents() expected error or requeue, got success")
					}
				}
			}
		})
	}
}
