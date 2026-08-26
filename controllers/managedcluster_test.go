// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"
	"testing"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func LocalClusterNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

func newTestMCHForManagedCluster(localClusterName string,
	components ...operatorsv1.ComponentConfig) *operatorsv1.MultiClusterHub {
	mch := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mch",
			Namespace: "open-cluster-management",
		},
		Spec: operatorsv1.MultiClusterHubSpec{
			LocalClusterName: localClusterName,
		},
	}
	if len(components) > 0 {
		mch.Spec.Overrides = &operatorsv1.Overrides{
			Components: components,
		}
	}
	return mch
}

func getUnstructuredField(u *unstructured.Unstructured, fields ...string) (bool, bool) {
	val, found, err := unstructured.NestedBool(u.Object, fields...)
	if err != nil {
		return false, false
	}
	return val, found
}

// Test_getKlusterletAddonConfig verifies the generated KlusterletAddonConfig
// object's static fields (metadata, applicationManager, searchCollector) and
// that certPolicyController/policyController track the GRC component's
// enabled state from Spec.Overrides.
func Test_getKlusterletAddonConfig(t *testing.T) {
	tests := []struct {
		name           string
		mch            *operatorsv1.MultiClusterHub
		expectGRC      bool
		expectedName   string
		expectedNsName string
	}{
		{
			name:           "GRC enabled by default when no overrides set",
			mch:            newTestMCHForManagedCluster("local-cluster"),
			expectGRC:      true,
			expectedName:   "local-cluster",
			expectedNsName: "local-cluster",
		},
		{
			name: "GRC explicitly enabled via override",
			mch: newTestMCHForManagedCluster("local-cluster",
				operatorsv1.ComponentConfig{Name: operatorsv1.GRC, Enabled: true},
			),
			expectGRC:      true,
			expectedName:   "local-cluster",
			expectedNsName: "local-cluster",
		},
		{
			name: "GRC disabled via override",
			mch: newTestMCHForManagedCluster("local-cluster",
				operatorsv1.ComponentConfig{Name: operatorsv1.GRC, Enabled: false},
			),
			expectGRC:      false,
			expectedName:   "local-cluster",
			expectedNsName: "local-cluster",
		},
		{
			name: "GRC unaffected by unrelated overrides",
			mch: newTestMCHForManagedCluster("local-cluster",
				operatorsv1.ComponentConfig{Name: "some-other-component", Enabled: false},
			),
			expectGRC:      true,
			expectedName:   "local-cluster",
			expectedNsName: "local-cluster",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kac := getKlusterletAddonConfig(tt.mch)

			if kac.GetAPIVersion() != "agent.open-cluster-management.io/v1" {
				t.Errorf("unexpected apiVersion: %s", kac.GetAPIVersion())
			}
			if kac.GetKind() != "KlusterletAddonConfig" {
				t.Errorf("unexpected kind: %s", kac.GetKind())
			}
			if kac.GetName() != tt.expectedName {
				t.Errorf("expected name %q, got %q", tt.expectedName, kac.GetName())
			}
			if kac.GetNamespace() != tt.expectedNsName {
				t.Errorf("expected namespace %q, got %q", tt.expectedNsName, kac.GetNamespace())
			}

			appEnabled, _ := getUnstructuredField(kac, "spec", "applicationManager", "enabled")
			if !appEnabled {
				t.Error("expected applicationManager.enabled to always be true")
			}

			searchEnabled, _ := getUnstructuredField(kac, "spec", "searchCollector", "enabled")
			if searchEnabled {
				t.Error("expected searchCollector.enabled to always be false")
			}

			certPolicyEnabled, _ := getUnstructuredField(kac, "spec", "certPolicyController", "enabled")
			if certPolicyEnabled != tt.expectGRC {
				t.Errorf("expected certPolicyController.enabled = %v, got %v", tt.expectGRC, certPolicyEnabled)
			}

			policyEnabled, _ := getUnstructuredField(kac, "spec", "policyController", "enabled")
			if policyEnabled != tt.expectGRC {
				t.Errorf("expected policyController.enabled = %v, got %v", tt.expectGRC, policyEnabled)
			}
		})
	}
}

// Test_ensureKlusterletAddonConfig_NamespaceNotFound verifies that when the
// local-cluster namespace doesn't exist yet, reconciliation requeues and
// records a Progressing/WaitingForNamespaceReason condition instead of
// erroring.
func Test_ensureKlusterletAddonConfig_NamespaceNotFound(t *testing.T) {
	r := newTestReconciler()
	mch := newTestMCHForManagedCluster("local-cluster")

	result, err := r.ensureKlusterletAddonConfig(mch)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result.RequeueAfter != resyncPeriod {
		t.Errorf("expected RequeueAfter %v, got %v", resyncPeriod, result.RequeueAfter)
	}

	condition := GetHubCondition(mch.Status, operatorsv1.Progressing)
	if condition == nil {
		t.Fatal("expected Progressing condition to be set")
	}
	if condition.Reason != WaitingForNamespaceReason {
		t.Errorf("expected reason %q, got %q", WaitingForNamespaceReason, condition.Reason)
	}
}

// Test_ensureKlusterletAddonConfig_ClearsStaleNamespaceCondition verifies
// that once the local-cluster namespace exists, a previously-set
// WaitingForNamespaceReason condition is cleared rather than left stale.
func Test_ensureKlusterletAddonConfig_ClearsStaleNamespaceCondition(t *testing.T) {
	ns := LocalClusterNamespace("local-cluster")
	r := newTestReconciler(ns)
	mch := newTestMCHForManagedCluster("local-cluster")
	stale := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, WaitingForNamespaceReason,
		"Waiting for ManagedCluster namespace local-cluster to be created")
	SetHubCondition(&mch.Status, *stale)

	if _, err := r.ensureKlusterletAddonConfig(mch); err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	if condition := GetHubCondition(mch.Status, operatorsv1.Progressing); condition != nil {
		t.Errorf("expected stale WaitingForNamespaceReason condition to be cleared, got: %+v", condition)
	}
}

// Test_ensureKlusterletAddonConfig_NamespaceGetError verifies that a
// non-NotFound error while checking for the namespace is propagated as-is
// (using an interceptor to force a transient error, e.g. API server hiccup).
func Test_ensureKlusterletAddonConfig_NamespaceGetError(t *testing.T) {
	r := newTestReconcilerWithInterceptor(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object,
			opts ...client.GetOption) error {
			if _, ok := obj.(*corev1.Namespace); ok {
				return fmt.Errorf("simulated get error")
			}
			return c.Get(ctx, key, obj, opts...)
		},
	})
	mch := newTestMCHForManagedCluster("local-cluster")

	_, err := r.ensureKlusterletAddonConfig(mch)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// Test_ensureKlusterletAddonConfig_CreatesNew is the happy path: given an
// existing namespace and no prior KlusterletAddonConfig, one is created and
// stamped with the installer.name/installer.namespace labels.
func Test_ensureKlusterletAddonConfig_CreatesNew(t *testing.T) {
	ns := LocalClusterNamespace("local-cluster")
	r := newTestReconciler(ns)
	mch := newTestMCHForManagedCluster("local-cluster")

	result, err := r.ensureKlusterletAddonConfig(mch)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("expected empty result, got: %v", result)
	}

	kac := &unstructured.Unstructured{}
	kac.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "agent.open-cluster-management.io",
		Version: "v1",
		Kind:    "KlusterletAddonConfig",
	})
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "local-cluster", Namespace: "local-cluster"}, kac)
	if err != nil {
		t.Fatalf("expected KlusterletAddonConfig to be created: %v", err)
	}

	labels := kac.GetLabels()
	if labels["installer.name"] != mch.GetName() || labels["installer.namespace"] != mch.GetNamespace() {
		t.Errorf("expected installer labels to be set, got: %v", labels)
	}
}

// Test_ensureKlusterletAddonConfig_GetErrorOtherThanNotFound verifies that a
// KlusterletAddonConfig Get failure unrelated to "not found" is returned as
// an error rather than treated as "needs creation".
func Test_ensureKlusterletAddonConfig_GetErrorOtherThanNotFound(t *testing.T) {
	ns := LocalClusterNamespace("local-cluster")
	r := newTestReconcilerWithInterceptor(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object,
			opts ...client.GetOption) error {
			if u, ok := obj.(*unstructured.Unstructured); ok && u.GetKind() == "KlusterletAddonConfig" {
				return fmt.Errorf("simulated get error")
			}
			return c.Get(ctx, key, obj, opts...)
		},
	}, ns)
	mch := newTestMCHForManagedCluster("local-cluster")

	_, err := r.ensureKlusterletAddonConfig(mch)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// Test_ensureKlusterletAddonConfig_CreateError verifies Create failures on
// the KlusterletAddonConfig are surfaced to the caller.
func Test_ensureKlusterletAddonConfig_CreateError(t *testing.T) {
	ns := LocalClusterNamespace("local-cluster")
	r := newTestReconcilerWithInterceptor(interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if u, ok := obj.(*unstructured.Unstructured); ok && u.GetKind() == "KlusterletAddonConfig" {
				return fmt.Errorf("simulated create error")
			}
			return c.Create(ctx, obj, opts...)
		},
	}, ns)
	mch := newTestMCHForManagedCluster("local-cluster")

	_, err := r.ensureKlusterletAddonConfig(mch)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// Test_ensureKlusterletAddonConfig_UpdatesExisting verifies that when a
// KlusterletAddonConfig already exists, it's updated (installer labels
// applied) rather than recreated, and unrelated existing labels are kept.
func Test_ensureKlusterletAddonConfig_UpdatesExisting(t *testing.T) {
	ns := LocalClusterNamespace("local-cluster")

	existingKac := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "agent.open-cluster-management.io/v1",
			"kind":       "KlusterletAddonConfig",
			"metadata": map[string]interface{}{
				"name":      "local-cluster",
				"namespace": "local-cluster",
				"labels": map[string]interface{}{
					"custom-label": "should-persist",
				},
			},
			"spec": map[string]interface{}{
				"applicationManager": map[string]interface{}{
					"enabled": true,
				},
			},
		},
	}

	r := newTestReconciler(ns, existingKac)
	mch := newTestMCHForManagedCluster("local-cluster")

	result, err := r.ensureKlusterletAddonConfig(mch)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("expected empty result, got: %v", result)
	}

	kac := &unstructured.Unstructured{}
	kac.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "agent.open-cluster-management.io",
		Version: "v1",
		Kind:    "KlusterletAddonConfig",
	})
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: "local-cluster", Namespace: "local-cluster"}, kac)
	if err != nil {
		t.Fatalf("expected KlusterletAddonConfig to still exist: %v", err)
	}

	labels := kac.GetLabels()
	if labels["custom-label"] != "should-persist" {
		t.Errorf("expected existing labels to persist, got: %v", labels)
	}
	if labels["installer.name"] != mch.GetName() || labels["installer.namespace"] != mch.GetNamespace() {
		t.Errorf("expected installer labels to be set, got: %v", labels)
	}
}

// Test_ensureKlusterletAddonConfig_UpdateError verifies Update failures on
// an existing KlusterletAddonConfig are surfaced to the caller.
func Test_ensureKlusterletAddonConfig_UpdateError(t *testing.T) {
	ns := LocalClusterNamespace("local-cluster")

	existingKac := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "agent.open-cluster-management.io/v1",
			"kind":       "KlusterletAddonConfig",
			"metadata": map[string]interface{}{
				"name":      "local-cluster",
				"namespace": "local-cluster",
			},
		},
	}

	r := newTestReconcilerWithInterceptor(interceptor.Funcs{
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if u, ok := obj.(*unstructured.Unstructured); ok && u.GetKind() == "KlusterletAddonConfig" {
				return fmt.Errorf("simulated update error")
			}
			return c.Update(ctx, obj, opts...)
		},
	}, ns, existingKac)
	mch := newTestMCHForManagedCluster("local-cluster")

	_, err := r.ensureKlusterletAddonConfig(mch)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

// sanity check that errors.IsNotFound behaves as expected for unstructured Get
// results, matching the assumption made in ensureKlusterletAddonConfig.
func Test_ensureKlusterletAddonConfig_NotFoundIsDetected(t *testing.T) {
	ns := LocalClusterNamespace("local-cluster")
	r := newTestReconciler(ns)

	kac := &unstructured.Unstructured{}
	kac.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "agent.open-cluster-management.io",
		Version: "v1",
		Kind:    "KlusterletAddonConfig",
	})
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: "local-cluster", Namespace: "local-cluster"}, kac)
	if !errors.IsNotFound(err) {
		t.Fatalf("expected IsNotFound error, got: %v", err)
	}
}
