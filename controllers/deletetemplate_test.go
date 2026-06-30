package controllers

import (
	"context"
	"fmt"
	"testing"
	"time"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestDeleteTemplate(t *testing.T) {
	tests := []struct {
		name              string
		template          *unstructured.Unstructured
		setupClient       func(*testing.T) client.Client
		expectRequeue     bool
		expectError       bool
		expectRequeueTime bool
	}{
		{
			name:     "resource already deleted (NotFound)",
			template: newTestDeployment("test-deploy", "test-ns"),
			setupClient: func(t *testing.T) client.Client {
				// Empty client - resource doesn't exist
				return fake.NewClientBuilder().WithScheme(scheme.Scheme).Build()
			},
			expectRequeue:     false,
			expectError:       false,
			expectRequeueTime: false,
		},
		{
			name:     "resource still terminating (has deletionTimestamp)",
			template: newTestDeploymentWithLabels("test-deploy", "test-ns", "test-mch", "test-ns"),
			setupClient: func(t *testing.T) client.Client {
				// Create resource with MCH labels - deleteTemplate will set deletionTimestamp after Delete
				deploy := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deploy",
						Namespace: "test-ns",
						Labels: map[string]string{
							"installer.name":      "test-mch",
							"installer.namespace": "test-ns",
						},
					},
				}
				// Use terminatingClient to simulate resource still terminating after Delete
				return &terminatingResourceClient{
					Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(deploy).Build(),
				}
			},
			expectRequeue:     true,
			expectError:       false,
			expectRequeueTime: true,
		},
		{
			name:     "transient API error during verification (timeout)",
			template: newTestDeploymentWithLabels("test-deploy", "test-ns", "test-mch", "test-ns"),
			setupClient: func(t *testing.T) client.Client {
				// Create resource with MCH labels
				deploy := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deploy",
						Namespace: "test-ns",
						Labels: map[string]string{
							"installer.name":      "test-mch",
							"installer.namespace": "test-ns",
						},
					},
				}
				// Use interceptor to simulate transient error on second Get (verification)
				return &transientErrorClient{
					Client:     fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(deploy).Build(),
					errorOnGet: true,
					errorToRet: fmt.Errorf("connection timeout"),
				}
			},
			expectRequeue:     true,
			expectError:       false,
			expectRequeueTime: true,
		},
		{
			name:     "resource deleted successfully",
			template: newTestDeploymentWithLabels("test-deploy", "test-ns", "test-mch", "test-ns"),
			setupClient: func(t *testing.T) client.Client {
				// Create resource with MCH labels, delete will succeed, Get returns NotFound
				deploy := &appsv1.Deployment{
					ObjectMeta: metav1.ObjectMeta{
						Name:      "test-deploy",
						Namespace: "test-ns",
						Labels: map[string]string{
							"installer.name":      "test-mch",
							"installer.namespace": "test-ns",
						},
					},
				}
				return fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(deploy).Build()
			},
			expectRequeue:     false,
			expectError:       false,
			expectRequeueTime: false,
		},
		{
			name:     "CRD removed (NoMatchError)",
			template: newTestDeployment("test-deploy", "test-ns"),
			setupClient: func(t *testing.T) client.Client {
				return &noMatchErrorClient{
					Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).Build(),
				}
			},
			expectRequeue:     false,
			expectError:       false,
			expectRequeueTime: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup reconciler with test client
			r := &MultiClusterHubReconciler{
				Client: tt.setupClient(t),
				Log:    ctrl.Log.WithName("test"),
			}

			mch := &operatorv1.MultiClusterHub{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-mch",
					Namespace: "test-ns",
				},
			}

			// Execute
			result, err := r.deleteTemplate(context.TODO(), mch, tt.template)

			// Verify error expectation
			if tt.expectError && err == nil {
				t.Errorf("deleteTemplate() expected error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("deleteTemplate() unexpected error: %v", err)
			}

			// Verify requeue expectation
			if tt.expectRequeue && result == (ctrl.Result{}) {
				t.Errorf("deleteTemplate() expected requeue, got empty result")
			}
			if !tt.expectRequeue && result != (ctrl.Result{}) {
				t.Errorf("deleteTemplate() expected no requeue, got result: %v", result)
			}

			// Verify RequeueAfter is set when expected
			if tt.expectRequeueTime && result.RequeueAfter == 0 {
				t.Errorf("deleteTemplate() expected RequeueAfter to be set, got 0")
			}
		})
	}
}

// Helper to create test deployment unstructured
func newTestDeployment(name, namespace string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	})
	u.SetName(name)
	u.SetNamespace(namespace)
	return u
}

// Helper to create test deployment with MCH owner labels
func newTestDeploymentWithLabels(name, namespace, mchName, mchNamespace string) *unstructured.Unstructured {
	u := newTestDeployment(name, namespace)
	u.SetLabels(map[string]string{
		"installer.name":      mchName,
		"installer.namespace": mchNamespace,
	})
	return u
}

// Mock client that returns transient errors on second Get (verification)
type transientErrorClient struct {
	client.Client
	errorOnGet bool
	errorToRet error
	getCount   int
}

func (c *transientErrorClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	c.getCount++
	// First Get succeeds (initial check), second Get returns error (verification after Delete)
	if c.errorOnGet && c.getCount > 1 {
		return c.errorToRet
	}
	return c.Client.Get(ctx, key, obj, opts...)
}

func (c *transientErrorClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	// Delete succeeds
	return c.Client.Delete(ctx, obj, opts...)
}

// Mock client that simulates resource still terminating after Delete
type terminatingResourceClient struct {
	client.Client
	deleteCount int
}

func (c *terminatingResourceClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	err := c.Client.Get(ctx, key, obj, opts...)
	// After Delete is called, return resource with deletionTimestamp
	if c.deleteCount > 0 && err == nil {
		now := metav1.Now()
		obj.SetDeletionTimestamp(&now)
		obj.SetFinalizers([]string{"test-finalizer"})
	}
	return err
}

func (c *terminatingResourceClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	c.deleteCount++
	// Don't actually delete - simulate stuck finalizer
	return nil
}

// Mock client that returns NoMatchError
type noMatchErrorClient struct {
	client.Client
}

func (c *noMatchErrorClient) Get(ctx context.Context, key types.NamespacedName, obj client.Object, opts ...client.GetOption) error {
	gvk := obj.GetObjectKind().GroupVersionKind()
	return &apimeta.NoKindMatchError{
		GroupKind:        schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind},
		SearchedVersions: []string{gvk.Version},
	}
}

func (c *noMatchErrorClient) Delete(ctx context.Context, obj client.Object, opts ...client.DeleteOption) error {
	// Simulate successful delete even though CRD doesn't exist
	return nil
}

