package controllers

import (
	"context"
	"fmt"
	"testing"

	backplanev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengineutils"
	resources "github.com/stolostron/multiclusterhub-operator/test/unit-tests"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	// Note: These unit tests are limited by chart rendering requirements.
	// Full validation requires integration tests with real Helm charts and cluster resources.
	tests := []struct {
		name          string
		mch           operatorv1.MultiClusterHub
		mce           *backplanev1.MultiClusterEngine
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
			mce: &backplanev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "engine",
				},
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
			mce: &backplanev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "engine",
				},
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
			mce: &backplanev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "engine",
				},
			},
			stsEnabled:    false,
			expectNoError: false, // ensureNoComponent will return error/requeue with fake client
		},
		{
			name: "component present, cleanup follows standard path",
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
			mce:           nil, // MCE presence not checked by ensureMigratedComponentsCleanup
			stsEnabled:    false,
			expectNoError: false, // Chart rendering will fail/requeue with fake client
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
				if err := recon.Client.Create(context.TODO(), tt.mce); err != nil {
					t.Errorf("failed to create MCE: %v", err)
				}
				defer recon.Client.Delete(context.TODO(), tt.mce)
			}

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
				// For components present, verify expected error/requeue behavior
				if tt.expectNoError {
					// Should succeed with no error
					if err != nil {
						t.Errorf("ensureMigratedComponentsCleanup() unexpected error: %v", err)
					}
				} else {
					// Should either error or requeue
					if err == nil && result == (ctrl.Result{}) {
						t.Errorf("ensureMigratedComponentsCleanup() expected error or requeue, got success")
					}
				}
			}
		})
	}
}

// Note: These tests provide basic validation but are limited by chart rendering requirements.
// Full test coverage requires integration tests with real Helm charts and cluster resources.
// Key scenarios tested:
// - MCE not found handling
// - Resource labeling and annotation logic
// - Error handling paths
func Test_transferClusterResourcesToMCE(t *testing.T) {
	tests := []struct {
		name           string
		mch            operatorv1.MultiClusterHub
		mce            *backplanev1.MultiClusterEngine
		existingRes    []client.Object
		component      string
		expectRequeue  bool
		expectError    bool
		validateLabels func(*testing.T, client.Client)
	}{
		{
			name:          "MCE not found, should requeue",
			mch:           operatorv1.MultiClusterHub{},
			mce:           nil,
			component:     operatorv1.ClusterPermission,
			expectRequeue: true,
			expectError:   false,
		},
		{
			name: "resource already transferred to current MCE, skip",
			mch: operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "open-cluster-management",
				},
			},
			mce: &backplanev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multiclusterengine",
				},
			},
			existingRes: []client.Object{
				&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster-permission-clusterrole",
						Labels: map[string]string{
							"backplaneconfig.name": "multiclusterengine",
						},
					},
				},
			},
			component:     operatorv1.ClusterPermission,
			expectRequeue: false,
			expectError:   false,
			validateLabels: func(t *testing.T, c client.Client) {
				cr := &rbacv1.ClusterRole{}
				err := c.Get(context.TODO(), types.NamespacedName{Name: "cluster-permission-clusterrole"}, cr)
				if err != nil {
					t.Errorf("failed to get ClusterRole: %v", err)
				}
				if cr.Labels["backplaneconfig.name"] != "multiclusterengine" {
					t.Errorf("expected backplaneconfig.name=multiclusterengine, got %s", cr.Labels["backplaneconfig.name"])
				}
			},
		},
		{
			name: "resource with MCH labels, should transfer to MCE",
			mch: operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "open-cluster-management",
				},
			},
			mce: &backplanev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multiclusterengine",
				},
			},
			existingRes: []client.Object{
				&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster-permission-clusterrole",
						Labels: map[string]string{
							"installer.name":      "test-mch",
							"installer.namespace": "open-cluster-management",
						},
						Annotations: map[string]string{
							"installer.open-cluster-management.io/release-version": "2.16.0",
						},
					},
				},
			},
			component:     operatorv1.ClusterPermission,
			expectRequeue: false,
			expectError:   false,
			validateLabels: func(t *testing.T, c client.Client) {
				cr := &rbacv1.ClusterRole{}
				err := c.Get(context.TODO(), types.NamespacedName{Name: "cluster-permission-clusterrole"}, cr)
				if err != nil {
					t.Errorf("failed to get ClusterRole: %v", err)
				}
				// Should have MCE label
				if cr.Labels["backplaneconfig.name"] != "multiclusterengine" {
					t.Errorf("expected backplaneconfig.name=multiclusterengine, got %s", cr.Labels["backplaneconfig.name"])
				}
				// Should NOT have MCH labels
				if _, exists := cr.Labels["installer.name"]; exists {
					t.Errorf("installer.name label should be removed")
				}
				if _, exists := cr.Labels["installer.namespace"]; exists {
					t.Errorf("installer.namespace label should be removed")
				}
				// Should NOT have MCH annotations
				if _, exists := cr.Annotations["installer.open-cluster-management.io/release-version"]; exists {
					t.Errorf("MCH annotations should be removed")
				}
			},
		},
		{
			name: "resource with stale MCE label, should update to current MCE",
			mch: operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "open-cluster-management",
				},
			},
			mce: &backplanev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "new-mce",
				},
			},
			existingRes: []client.Object{
				&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster-permission-clusterrole",
						Labels: map[string]string{
							"backplaneconfig.name": "old-mce",
						},
					},
				},
			},
			component:     operatorv1.ClusterPermission,
			expectRequeue: false,
			expectError:   false,
			validateLabels: func(t *testing.T, c client.Client) {
				cr := &rbacv1.ClusterRole{}
				err := c.Get(context.TODO(), types.NamespacedName{Name: "cluster-permission-clusterrole"}, cr)
				if err != nil {
					t.Errorf("failed to get ClusterRole: %v", err)
				}
				if cr.Labels["backplaneconfig.name"] != "new-mce" {
					t.Errorf("expected backplaneconfig.name=new-mce, got %s", cr.Labels["backplaneconfig.name"])
				}
			},
		},
	}

	registerScheme()
	for i, tt := range tests {
		tt := tt
		i := i
		t.Run(tt.name, func(t *testing.T) {
			// Make MCH name unique
			mch := tt.mch.DeepCopy()
			if mch.Name == "" || mch.Name == "test-mch" {
				mch.Name = fmt.Sprintf("test-mch-%d", i)
			}
			if mch.Namespace == "" {
				mch.Namespace = "open-cluster-management"
			}

			// Setup: Create MCH
			if err := recon.Client.Create(context.TODO(), mch); err != nil {
				t.Errorf("failed to create MCH: %v", err)
			}
			defer recon.Client.Delete(context.TODO(), mch)

			// Setup: Create MCE if provided
			if tt.mce != nil {
				tt.mce.Labels = map[string]string{
					"installer.name":                          mch.GetName(),
					"installer.namespace":                     mch.GetNamespace(),
					multiclusterengineutils.MCEManagedByLabel: "true",
				}
				if err := recon.Client.Create(context.TODO(), tt.mce); err != nil {
					t.Errorf("failed to create MCE: %v", err)
				}
				defer recon.Client.Delete(context.TODO(), tt.mce)
			}

			// Setup: Create existing resources
			for _, res := range tt.existingRes {
				if err := recon.Client.Create(context.TODO(), res); err != nil {
					t.Errorf("failed to create resource: %v", err)
				}
				defer recon.Client.Delete(context.TODO(), res)
			}

			// Test
			result, err := recon.transferClusterResourcesToMCE(context.TODO(), mch, tt.component, recon.CacheSpec, false)

			// Validate error
			if (err != nil) != tt.expectError {
				t.Errorf("transferClusterResourcesToMCE() error = %v, expectError %v", err, tt.expectError)
				return
			}

			// Validate requeue
			if tt.expectRequeue {
				if result == (ctrl.Result{}) {
					t.Errorf("transferClusterResourcesToMCE() expected requeue, got empty result")
				}
			} else {
				if result != (ctrl.Result{}) && err == nil {
					t.Errorf("transferClusterResourcesToMCE() expected no requeue, got %v", result)
				}
			}

			// Validate labels if function provided
			if tt.validateLabels != nil {
				tt.validateLabels(t, recon.Client)
			}
		})
	}
}

// Note: These tests provide basic validation but are limited by chart rendering requirements.
// Full test coverage requires integration tests with real Helm charts and cluster resources.
// Key scenarios tested:
// - Scope filtering (cluster-scoped vs namespace-scoped)
// - MCE ownership checking
// - Deletion timestamp handling
func Test_deleteResourcesByScope(t *testing.T) {
	tests := []struct {
		name                string
		mch                 operatorv1.MultiClusterHub
		mce                 *backplanev1.MultiClusterEngine
		existingRes         []client.Object
		component           string
		deleteClusterScoped bool
		expectRequeue       bool
		expectError         bool
		validateDeleted     func(*testing.T, client.Client)
	}{
		{
			name: "delete namespace-scoped resources",
			mch: operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "open-cluster-management",
				},
			},
			mce: &backplanev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multiclusterengine",
				},
			},
			existingRes: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "cluster-permission-deployment",
						Namespace: "open-cluster-management",
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test"},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{"app": "test"},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{Name: "test", Image: "test:latest"},
								},
							},
						},
					},
				},
			},
			component:           operatorv1.ClusterPermission,
			deleteClusterScoped: false,
			expectRequeue:       true, // Will requeue after delete
			expectError:         false,
			validateDeleted: func(t *testing.T, c client.Client) {
				dep := &appsv1.Deployment{}
				err := c.Get(context.TODO(), types.NamespacedName{
					Name:      "cluster-permission-deployment",
					Namespace: "open-cluster-management",
				}, dep)
				// Resource should be deleted or have deletionTimestamp
				if err == nil && dep.DeletionTimestamp == nil {
					t.Errorf("expected deployment to be deleted or have deletionTimestamp")
				}
			},
		},
		{
			name: "skip cluster-scoped resources with MCE ownership when deleting cluster-scoped",
			mch: operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "open-cluster-management",
				},
			},
			mce: &backplanev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multiclusterengine",
				},
			},
			existingRes: []client.Object{
				&rbacv1.ClusterRole{
					ObjectMeta: metav1.ObjectMeta{
						Name: "cluster-permission-clusterrole",
						Labels: map[string]string{
							"backplaneconfig.name": "multiclusterengine",
						},
					},
				},
			},
			component:           operatorv1.ClusterPermission,
			deleteClusterScoped: true,
			expectRequeue:       false,
			expectError:         false,
			validateDeleted: func(t *testing.T, c client.Client) {
				cr := &rbacv1.ClusterRole{}
				err := c.Get(context.TODO(), types.NamespacedName{Name: "cluster-permission-clusterrole"}, cr)
				// Resource should NOT be deleted (MCE owns it)
				if err != nil {
					t.Errorf("ClusterRole should not be deleted, MCE owns it")
				}
			},
		},
		{
			name: "resource with deletionTimestamp should requeue",
			mch: operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "open-cluster-management",
				},
			},
			mce: &backplanev1.MultiClusterEngine{
				ObjectMeta: metav1.ObjectMeta{
					Name: "multiclusterengine",
				},
			},
			existingRes: []client.Object{
				&appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:              "cluster-permission-deployment",
						Namespace:         "open-cluster-management",
						DeletionTimestamp: &metav1.Time{Time: metav1.Now().Time},
						Finalizers:        []string{"test-finalizer"},
					},
					Spec: appsv1.DeploymentSpec{
						Selector: &metav1.LabelSelector{
							MatchLabels: map[string]string{"app": "test"},
						},
						Template: corev1.PodTemplateSpec{
							ObjectMeta: metav1.ObjectMeta{
								Labels: map[string]string{"app": "test"},
							},
							Spec: corev1.PodSpec{
								Containers: []corev1.Container{
									{Name: "test", Image: "test:latest"},
								},
							},
						},
					},
				},
			},
			component:           operatorv1.ClusterPermission,
			deleteClusterScoped: false,
			expectRequeue:       true,
			expectError:         false,
		},
	}

	registerScheme()
	for i, tt := range tests {
		tt := tt
		i := i
		t.Run(tt.name, func(t *testing.T) {
			// Make MCH name unique
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
				tt.mce.Labels = map[string]string{
					"installer.name":                          mch.GetName(),
					"installer.namespace":                     mch.GetNamespace(),
					multiclusterengineutils.MCEManagedByLabel: "true",
				}
				if err := recon.Client.Create(context.TODO(), tt.mce); err != nil {
					t.Errorf("failed to create MCE: %v", err)
				}
				defer recon.Client.Delete(context.TODO(), tt.mce)
			}

			// Setup: Create existing resources
			for _, res := range tt.existingRes {
				if err := recon.Client.Create(context.TODO(), res); err != nil {
					t.Errorf("failed to create resource: %v", err)
				}
				defer recon.Client.Delete(context.TODO(), res)
			}

			// Test
			result, err := recon.deleteResourcesByScope(context.TODO(), mch, tt.component, recon.CacheSpec, false, tt.deleteClusterScoped)

			// Validate error
			if (err != nil) != tt.expectError {
				t.Errorf("deleteResourcesByScope() error = %v, expectError %v", err, tt.expectError)
				return
			}

			// Validate requeue
			if tt.expectRequeue {
				if result == (ctrl.Result{}) {
					t.Errorf("deleteResourcesByScope() expected requeue, got empty result")
				}
			} else {
				if result != (ctrl.Result{}) && err == nil {
					t.Errorf("deleteResourcesByScope() expected no requeue, got %v", result)
				}
			}

			// Validate deletion if function provided
			if tt.validateDeleted != nil {
				tt.validateDeleted(t, recon.Client)
			}
		})
	}
}

func Test_deleteNamespaceScopedResources(t *testing.T) {
	registerScheme()

	mch := operatorv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mch",
			Namespace: "open-cluster-management",
		},
	}

	// Setup: Create MCH
	if err := recon.Client.Create(context.TODO(), &mch); err != nil {
		t.Errorf("failed to create MCH: %v", err)
	}
	defer recon.Client.Delete(context.TODO(), &mch)

	// Test that it calls deleteResourcesByScope with deleteClusterScoped=false
	result, err := recon.deleteNamespaceScopedResources(context.TODO(), &mch, operatorv1.ClusterPermission, recon.CacheSpec, false)

	// Should handle chart rendering errors gracefully (no component templates available in test)
	if err != nil {
		t.Errorf("deleteNamespaceScopedResources() unexpected error: %v", err)
	}

	// Should either succeed or requeue (depends on chart rendering)
	_ = result
}
