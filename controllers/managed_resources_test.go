// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"testing"

	promv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func init() {
	// ServiceMonitor is not registered by registerScheme(); needed for legacy managed resource
	// cleanup tests (ACM-40355).
	_ = promv1.AddToScheme(scheme.Scheme)
}

func newManagedResource(apiVersion, kind, name, namespace string) operatorv1.ManagedResource {
	return operatorv1.ManagedResource{
		APIVersion: apiVersion,
		Kind:       kind,
		Name:       name,
		Namespace:  namespace,
	}
}

func TestManagedResourcesEqual(t *testing.T) {
	tests := []struct {
		name string
		a    []operatorv1.ManagedResource
		b    []operatorv1.ManagedResource
		want bool
	}{
		{
			name: "both empty",
			a:    nil,
			b:    []operatorv1.ManagedResource{},
			want: true,
		},
		{
			name: "identical order",
			a: []operatorv1.ManagedResource{
				newManagedResource("apps/v1", "Deployment", "a", "ns"),
				newManagedResource("v1", "Service", "b", "ns"),
			},
			b: []operatorv1.ManagedResource{
				newManagedResource("apps/v1", "Deployment", "a", "ns"),
				newManagedResource("v1", "Service", "b", "ns"),
			},
			want: true,
		},
		{
			name: "same set, different order",
			a: []operatorv1.ManagedResource{
				newManagedResource("apps/v1", "Deployment", "a", "ns"),
				newManagedResource("v1", "Service", "b", "ns"),
			},
			b: []operatorv1.ManagedResource{
				newManagedResource("v1", "Service", "b", "ns"),
				newManagedResource("apps/v1", "Deployment", "a", "ns"),
			},
			want: true,
		},
		{
			name: "different lengths",
			a: []operatorv1.ManagedResource{
				newManagedResource("apps/v1", "Deployment", "a", "ns"),
			},
			b: []operatorv1.ManagedResource{
				newManagedResource("apps/v1", "Deployment", "a", "ns"),
				newManagedResource("v1", "Service", "b", "ns"),
			},
			want: false,
		},
		{
			name: "same length, different contents",
			a: []operatorv1.ManagedResource{
				newManagedResource("apps/v1", "Deployment", "a", "ns"),
			},
			b: []operatorv1.ManagedResource{
				newManagedResource("apps/v1", "Deployment", "c", "ns"),
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := managedResourcesEqual(tt.a, tt.b); got != tt.want {
				t.Errorf("managedResourcesEqual() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractManagedResources(t *testing.T) {
	deployment := &unstructured.Unstructured{}
	deployment.SetAPIVersion("apps/v1")
	deployment.SetKind("Deployment")
	deployment.SetName("my-deploy")
	deployment.SetNamespace("test-ns")

	networkPolicy := &unstructured.Unstructured{}
	networkPolicy.SetAPIVersion("networking.k8s.io/v1")
	networkPolicy.SetKind("NetworkPolicy")
	networkPolicy.SetName("my-np")
	networkPolicy.SetNamespace("test-ns")

	clusterRole := &unstructured.Unstructured{}
	clusterRole.SetAPIVersion("rbac.authorization.k8s.io/v1")
	clusterRole.SetKind("ClusterRole")
	clusterRole.SetName("my-cr")

	resources := extractManagedResources([]*unstructured.Unstructured{deployment, networkPolicy, clusterRole})

	want := []operatorv1.ManagedResource{
		newManagedResource("apps/v1", "Deployment", "my-deploy", "test-ns"),
		newManagedResource("rbac.authorization.k8s.io/v1", "ClusterRole", "my-cr", ""),
	}

	if !managedResourcesEqual(resources, want) {
		t.Errorf("extractManagedResources() = %v, want %v (NetworkPolicy should be excluded)", resources, want)
	}
	if len(resources) != 2 {
		t.Errorf("extractManagedResources() returned %d resources, want 2 (NetworkPolicy should be skipped)",
			len(resources))
	}
}

func TestGetAndUpdateManagedResources(t *testing.T) {
	registerScheme()

	mch := &operatorv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Name: "mch", Namespace: "test-ns"},
	}

	fakeClient := fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
	r := &MultiClusterHubReconciler{Client: fakeClient, Log: ctrl.Log.WithName("test")}

	// No InternalHubComponent exists yet - should return nil without error.
	if got := r.getManagedResources(context.TODO(), mch, "console"); got != nil {
		t.Errorf("getManagedResources() with no CR = %v, want nil", got)
	}

	// updateManagedResources should be a no-op (not an error) when the CR doesn't exist yet.
	if err := r.updateManagedResources(context.TODO(), mch, "console",
		[]operatorv1.ManagedResource{newManagedResource("v1", "ConfigMap", "cm", "test-ns")}); err != nil {
		t.Errorf("updateManagedResources() with no CR returned error: %v", err)
	}

	// Create the InternalHubComponent CR (mirrors ensureInternalHubComponent).
	ihc := &operatorv1.InternalHubComponent{
		ObjectMeta: metav1.ObjectMeta{Name: "console", Namespace: "test-ns"},
	}
	if err := fakeClient.Create(context.TODO(), ihc); err != nil {
		t.Fatalf("failed to create InternalHubComponent: %v", err)
	}

	initial := []operatorv1.ManagedResource{
		newManagedResource("apps/v1", "Deployment", "console", "test-ns"),
		newManagedResource("monitoring.coreos.com/v1", "ServiceMonitor", "console-monitor", "test-ns"),
	}
	if err := r.updateManagedResources(context.TODO(), mch, "console", initial); err != nil {
		t.Fatalf("updateManagedResources() returned error: %v", err)
	}

	got := r.getManagedResources(context.TODO(), mch, "console")
	if !managedResourcesEqual(got, initial) {
		t.Errorf("getManagedResources() = %v, want %v", got, initial)
	}

	// Updating with the same set (different order) should be a no-op but not error, and should
	// not change the stored resource version unexpectedly.
	reordered := []operatorv1.ManagedResource{initial[1], initial[0]}
	if err := r.updateManagedResources(context.TODO(), mch, "console", reordered); err != nil {
		t.Fatalf("updateManagedResources() with reordered list returned error: %v", err)
	}

	// Updating with a smaller set (ServiceMonitor removed from chart) should persist the new list.
	updated := []operatorv1.ManagedResource{initial[0]}
	if err := r.updateManagedResources(context.TODO(), mch, "console", updated); err != nil {
		t.Fatalf("updateManagedResources() returned error: %v", err)
	}

	got = r.getManagedResources(context.TODO(), mch, "console")
	if !managedResourcesEqual(got, updated) {
		t.Errorf("getManagedResources() after update = %v, want %v", got, updated)
	}
}

func TestCleanupOrphanedManagedResources(t *testing.T) {
	registerScheme()

	mch := &operatorv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Name: "mch", Namespace: "open-cluster-management"},
	}

	tests := []struct {
		name           string
		component      string
		oldResources   []operatorv1.ManagedResource
		newResources   []operatorv1.ManagedResource
		setupClient    func(t *testing.T) client.Client
		verify         func(t *testing.T, c client.Client)
		expectRequeue  bool
		expectErrorMsg string
	}{
		{
			name:      "resource removed from chart and owned by MCH is deleted",
			component: "example",
			oldResources: []operatorv1.ManagedResource{
				newManagedResource("apps/v1", "Deployment", "old-deploy", "open-cluster-management"),
			},
			newResources: nil,
			setupClient: func(t *testing.T) client.Client {
				deploy := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "old-deploy",
						Namespace: "open-cluster-management",
						Labels: map[string]string{
							"installer.name":      "mch",
							"installer.namespace": "open-cluster-management",
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(deploy).Build()
			},
			verify: func(t *testing.T, c client.Client) {
				err := c.Get(context.TODO(), types.NamespacedName{Name: "old-deploy",
					Namespace: "open-cluster-management"}, &appsv1.Deployment{})
				if err == nil {
					t.Errorf("expected old-deploy to be deleted, but it still exists")
				}
			},
		},
		{
			name:      "resource removed from chart but manually recreated without MCH labels is left alone",
			component: "example",
			oldResources: []operatorv1.ManagedResource{
				newManagedResource("apps/v1", "Deployment", "adopted-deploy", "open-cluster-management"),
			},
			newResources: nil,
			setupClient: func(t *testing.T) client.Client {
				deploy := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "adopted-deploy",
						Namespace: "open-cluster-management",
						// No installer labels - simulates a resource manually recreated by a user
						// after MCH deleted it, or a resource never owned by MCH.
					},
				}
				return fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(deploy).Build()
			},
			verify: func(t *testing.T, c client.Client) {
				err := c.Get(context.TODO(), types.NamespacedName{Name: "adopted-deploy",
					Namespace: "open-cluster-management"}, &appsv1.Deployment{})
				if err != nil {
					t.Errorf("expected adopted-deploy to be left alone, but got error: %v", err)
				}
			},
		},
		{
			name:      "resource still present in current templates is not touched",
			component: "example",
			oldResources: []operatorv1.ManagedResource{
				newManagedResource("apps/v1", "Deployment", "kept-deploy", "open-cluster-management"),
			},
			newResources: []operatorv1.ManagedResource{
				newManagedResource("apps/v1", "Deployment", "kept-deploy", "open-cluster-management"),
			},
			setupClient: func(t *testing.T) client.Client {
				deploy := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "kept-deploy",
						Namespace: "open-cluster-management",
						Labels: map[string]string{
							"installer.name":      "mch",
							"installer.namespace": "open-cluster-management",
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(deploy).Build()
			},
			verify: func(t *testing.T, c client.Client) {
				err := c.Get(context.TODO(), types.NamespacedName{Name: "kept-deploy",
					Namespace: "open-cluster-management"}, &appsv1.Deployment{})
				if err != nil {
					t.Errorf("expected kept-deploy to still exist, got error: %v", err)
				}
			},
		},
		{
			name:         "legacy console ServiceMonitor is cleaned up even with no tracked history (ACM-40355)",
			component:    operatorv1.Console,
			oldResources: nil, // Simulates an InternalHubComponent CR from before resource tracking existed.
			newResources: nil,
			setupClient: func(t *testing.T) client.Client {
				sm := &promv1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "console-monitor",
						Namespace: "open-cluster-management",
						Labels: map[string]string{
							"installer.name":      "mch",
							"installer.namespace": "open-cluster-management",
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(sm).Build()
			},
			verify: func(t *testing.T, c client.Client) {
				err := c.Get(context.TODO(), types.NamespacedName{Name: "console-monitor",
					Namespace: "open-cluster-management"}, &promv1.ServiceMonitor{})
				if err == nil {
					t.Errorf("expected legacy console-monitor ServiceMonitor to be deleted, but it still exists")
				}
			},
		},
		{
			name:         "legacy console ServiceMonitor absent from cluster is a no-op",
			component:    operatorv1.Console,
			oldResources: nil,
			newResources: nil,
			setupClient: func(t *testing.T) client.Client {
				return fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
			},
			verify: func(t *testing.T, c client.Client) {
				// Nothing to verify beyond "no error/requeue", asserted below.
			},
		},
		{
			name:      "legacy cleanup does not run for unrelated components",
			component: "example",
			setupClient: func(t *testing.T) client.Client {
				sm := &promv1.ServiceMonitor{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "console-monitor",
						Namespace: "open-cluster-management",
						Labels: map[string]string{
							"installer.name":      "mch",
							"installer.namespace": "open-cluster-management",
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(sm).Build()
			},
			verify: func(t *testing.T, c client.Client) {
				err := c.Get(context.TODO(), types.NamespacedName{Name: "console-monitor",
					Namespace: "open-cluster-management"}, &promv1.ServiceMonitor{})
				if err != nil {
					t.Errorf("console-monitor should only be cleaned up for the console component, got error: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := tt.setupClient(t)
			r := &MultiClusterHubReconciler{Client: c, Log: ctrl.Log.WithName("test")}

			result, err := r.cleanupOrphanedManagedResources(context.TODO(), mch, tt.component,
				tt.oldResources, tt.newResources)

			if err != nil {
				t.Errorf("cleanupOrphanedManagedResources() unexpected error: %v", err)
			}
			if tt.expectRequeue && result == (ctrl.Result{}) {
				t.Errorf("cleanupOrphanedManagedResources() expected requeue, got empty result")
			}
			if !tt.expectRequeue && result != (ctrl.Result{}) {
				t.Errorf("cleanupOrphanedManagedResources() expected no requeue, got %v", result)
			}

			if tt.verify != nil {
				tt.verify(t, c)
			}
		})
	}
}
