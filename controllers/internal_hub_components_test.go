// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"testing"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

func Test_buildManagedResources(t *testing.T) {
	templates := []*unstructured.Unstructured{
		newUnstructured("apps/v1", "Deployment", "test-ns", "my-deploy"),
		newUnstructured("v1", "Service", "test-ns", "my-svc"),
		newUnstructured("rbac.authorization.k8s.io/v1", "ClusterRole", "", "my-cr"),
		newUnstructured("networking.k8s.io/v1", "NetworkPolicy", "test-ns", "my-np"),
	}

	resources := buildManagedResources(templates)

	if len(resources) != 4 {
		t.Fatalf("expected 4 managed resources, got %d", len(resources))
	}

	// Deployment — namespaced
	if resources[0].Kind != "Deployment" || resources[0].Scope != operatorv1.NamespaceScoped {
		t.Errorf("expected Deployment/Namespaced, got %s/%s", resources[0].Kind, resources[0].Scope)
	}
	if resources[0].Namespace != "test-ns" {
		t.Errorf("expected namespace test-ns, got %s", resources[0].Namespace)
	}

	// ClusterRole — cluster-scoped
	if resources[2].Kind != "ClusterRole" || resources[2].Scope != operatorv1.ClusterScoped {
		t.Errorf("expected ClusterRole/Cluster, got %s/%s", resources[2].Kind, resources[2].Scope)
	}

	// NetworkPolicy — included (not skipped)
	if resources[3].Kind != "NetworkPolicy" || resources[3].Scope != operatorv1.NamespaceScoped {
		t.Errorf("expected NetworkPolicy/Namespaced, got %s/%s", resources[3].Kind, resources[3].Scope)
	}
}

func Test_findOrphanedResources(t *testing.T) {
	old := []operatorv1.ManagedResource{
		{Group: "", Version: "v1", Kind: "Service", Name: "svc-a", Namespace: "ns", Scope: operatorv1.NamespaceScoped},
		{Group: "", Version: "v1", Kind: "Service", Name: "svc-b", Namespace: "ns", Scope: operatorv1.NamespaceScoped},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole", Name: "cr-a", Scope: operatorv1.ClusterScoped},
	}
	current := []operatorv1.ManagedResource{
		{Group: "", Version: "v1", Kind: "Service", Name: "svc-a", Namespace: "ns", Scope: operatorv1.NamespaceScoped},
	}

	orphans := findOrphanedResources(old, current)

	if len(orphans) != 2 {
		t.Fatalf("expected 2 orphans, got %d", len(orphans))
	}

	orphanNames := map[string]bool{}
	for _, o := range orphans {
		orphanNames[o.Name] = true
	}
	if !orphanNames["svc-b"] || !orphanNames["cr-a"] {
		t.Errorf("expected svc-b and cr-a as orphans, got %v", orphans)
	}
}

func Test_findOrphanedResources_empty_old(t *testing.T) {
	current := []operatorv1.ManagedResource{
		{Group: "", Version: "v1", Kind: "Service", Name: "svc-a", Namespace: "ns", Scope: operatorv1.NamespaceScoped},
	}

	orphans := findOrphanedResources(nil, current)

	if len(orphans) != 0 {
		t.Fatalf("expected 0 orphans from nil old list, got %d", len(orphans))
	}
}

func Test_findOrphanedResources_no_change(t *testing.T) {
	resources := []operatorv1.ManagedResource{
		{Group: "", Version: "v1", Kind: "Service", Name: "svc-a", Namespace: "ns", Scope: operatorv1.NamespaceScoped},
	}

	orphans := findOrphanedResources(resources, resources)

	if len(orphans) != 0 {
		t.Fatalf("expected 0 orphans when lists match, got %d", len(orphans))
	}
}

func Test_updateAndGetManagedResources(t *testing.T) {
	registerScheme()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-managed-resources"}}
	ctx := context.TODO()

	if err := recon.Client.Create(ctx, ns); err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}
	defer recon.Client.Delete(ctx, ns)

	mch := &operatorv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Name: "mch", Namespace: ns.Name},
	}

	component := "test-component"

	// Create IHC first
	if _, err := recon.ensureInternalHubComponent(ctx, mch, component); err != nil {
		t.Fatalf("failed to create IHC: %v", err)
	}

	// Initially should have no managed resources
	got, err := recon.getManagedResources(ctx, mch, component)
	if err != nil {
		t.Fatalf("getManagedResources failed: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected 0 managed resources initially, got %d", len(got))
	}

	// Update with some resources
	resources := []operatorv1.ManagedResource{
		{Group: "apps", Version: "v1", Kind: "Deployment", Name: "deploy-a", Namespace: ns.Name, Scope: operatorv1.NamespaceScoped},
		{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole", Name: "cr-a", Scope: operatorv1.ClusterScoped},
	}
	if err := recon.updateManagedResources(ctx, mch, component, resources); err != nil {
		t.Fatalf("updateManagedResources failed: %v", err)
	}

	// Verify
	got, err = recon.getManagedResources(ctx, mch, component)
	if err != nil {
		t.Fatalf("getManagedResources failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 managed resources, got %d", len(got))
	}
	if got[0].Name != "deploy-a" || got[1].Name != "cr-a" {
		t.Errorf("unexpected resources: %v", got)
	}
}

func Test_pruneOrphanedResources(t *testing.T) {
	registerScheme()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-prune"}}
	ctx := context.TODO()

	if err := recon.Client.Create(ctx, ns); err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}
	defer recon.Client.Delete(ctx, ns)

	mch := &operatorv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Name: "mch", Namespace: ns.Name},
	}

	// Create a ConfigMap on cluster that should be pruned
	cm := &unstructured.Unstructured{}
	cm.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})
	cm.SetName("orphan-cm")
	cm.SetNamespace(ns.Name)
	cm.SetLabels(map[string]string{
		"installer.name":      mch.Name,
		"installer.namespace": mch.Namespace,
	})
	if err := recon.Client.Create(ctx, cm); err != nil {
		t.Fatalf("failed to create configmap: %v", err)
	}

	orphans := []operatorv1.ManagedResource{
		{Group: "", Version: "v1", Kind: "ConfigMap", Name: "orphan-cm", Namespace: ns.Name, Scope: operatorv1.NamespaceScoped},
	}

	if err := recon.pruneOrphanedResources(ctx, mch, orphans); err != nil {
		t.Fatalf("pruneOrphanedResources failed: %v", err)
	}

	// Verify ConfigMap is deleted
	check := &unstructured.Unstructured{}
	check.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})
	err := recon.Client.Get(ctx, types.NamespacedName{Name: "orphan-cm", Namespace: ns.Name}, check)
	if err == nil {
		t.Error("expected orphaned ConfigMap to be deleted, but it still exists")
	}
}

func Test_pruneOrphanedResources_already_gone(t *testing.T) {
	registerScheme()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-prune-gone"}}
	ctx := context.TODO()

	if err := recon.Client.Create(ctx, ns); err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}
	defer recon.Client.Delete(ctx, ns)

	mch := &operatorv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Name: "mch", Namespace: ns.Name},
	}

	// Resource doesn't exist on cluster — should not error
	orphans := []operatorv1.ManagedResource{
		{Group: "", Version: "v1", Kind: "ConfigMap", Name: "nonexistent", Namespace: ns.Name, Scope: operatorv1.NamespaceScoped},
	}

	if err := recon.pruneOrphanedResources(ctx, mch, orphans); err != nil {
		t.Fatalf("pruneOrphanedResources should not error for already-deleted resources: %v", err)
	}
}

func Test_pruneOrphanedResources_skips_unowned(t *testing.T) {
	registerScheme()

	ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "test-prune-unowned"}}
	ctx := context.TODO()

	if err := recon.Client.Create(ctx, ns); err != nil {
		t.Fatalf("failed to create namespace: %v", err)
	}
	defer recon.Client.Delete(ctx, ns)

	mch := &operatorv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Name: "mch", Namespace: ns.Name},
	}

	// Create a ConfigMap without installer labels — should not be deleted
	cm := &unstructured.Unstructured{}
	cm.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})
	cm.SetName("unowned-cm")
	cm.SetNamespace(ns.Name)
	if err := recon.Client.Create(ctx, cm); err != nil {
		t.Fatalf("failed to create configmap: %v", err)
	}

	orphans := []operatorv1.ManagedResource{
		{Group: "", Version: "v1", Kind: "ConfigMap", Name: "unowned-cm", Namespace: ns.Name, Scope: operatorv1.NamespaceScoped},
	}

	if err := recon.pruneOrphanedResources(ctx, mch, orphans); err != nil {
		t.Fatalf("pruneOrphanedResources failed: %v", err)
	}

	// ConfigMap should still exist since it lacks installer labels
	check := &unstructured.Unstructured{}
	check.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Version: "v1", Kind: "ConfigMap"})
	if err := recon.Client.Get(ctx, types.NamespacedName{Name: "unowned-cm", Namespace: ns.Name}, check); err != nil {
		t.Error("expected unowned ConfigMap to survive pruning, but it was deleted")
	}
}

func newUnstructured(apiVersion, kind, namespace, name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion(apiVersion)
	u.SetKind(kind)
	u.SetName(name)
	if namespace != "" {
		u.SetNamespace(namespace)
	}
	return u
}
