// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"testing"

	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengineutils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestHealStaleOLMState_NoInstalledCSV(t *testing.T) {
	registerScheme()
	sub := &subv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multicluster-engine",
			Namespace: "multicluster-engine",
		},
		Spec: &subv1alpha1.SubscriptionSpec{
			Package: "multicluster-engine",
		},
		// Status.InstalledCSV is empty — nothing to heal.
	}
	r := newTestReconciler(sub)

	result, err := r.healStaleOLMState(context.TODO(), sub)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Errorf("expected empty result, got: %+v", result)
	}

	// Subscription should still exist.
	got := &subv1alpha1.Subscription{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: sub.Name, Namespace: sub.Namespace}, got); err != nil {
		t.Errorf("subscription should still exist: %v", err)
	}
}

func TestHealStaleOLMState_CSVExists(t *testing.T) {
	registerScheme()
	csv := &subv1alpha1.ClusterServiceVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multicluster-engine.v5.1.0-186",
			Namespace: "multicluster-engine",
		},
	}
	sub := &subv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multicluster-engine",
			Namespace: "multicluster-engine",
		},
		Spec: &subv1alpha1.SubscriptionSpec{
			Package: "multicluster-engine",
		},
		Status: subv1alpha1.SubscriptionStatus{
			InstalledCSV: "multicluster-engine.v5.1.0-186",
		},
	}
	r := newTestReconciler(sub, csv)

	result, err := r.healStaleOLMState(context.TODO(), sub)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Errorf("expected empty result when CSV exists, got: %+v", result)
	}

	// Both should still exist.
	got := &subv1alpha1.Subscription{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: sub.Name, Namespace: sub.Namespace}, got); err != nil {
		t.Errorf("subscription should still exist: %v", err)
	}
	gotCSV := &subv1alpha1.ClusterServiceVersion{}
	if err := r.Client.Get(context.TODO(), types.NamespacedName{Name: csv.Name, Namespace: csv.Namespace}, gotCSV); err != nil {
		t.Errorf("CSV should still exist: %v", err)
	}
}

func TestHealStaleOLMState_OrphanedState(t *testing.T) {
	registerScheme()
	// Subscription references a CSV that does NOT exist — orphaned state.
	sub := &subv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multicluster-engine",
			Namespace: "multicluster-engine",
			Labels: map[string]string{
				"installer.name":                          "multiclusterhub",
				"installer.namespace":                     "open-cluster-management",
				multiclusterengineutils.MCEManagedByLabel: "true",
			},
		},
		Spec: &subv1alpha1.SubscriptionSpec{
			Package: "multicluster-engine",
		},
		Status: subv1alpha1.SubscriptionStatus{
			InstalledCSV: "multicluster-engine.v5.1.0-186",
		},
	}
	installPlan := &subv1alpha1.InstallPlan{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "install-abc12",
			Namespace: "multicluster-engine",
		},
		Spec: subv1alpha1.InstallPlanSpec{
			ClusterServiceVersionNames: []string{"multicluster-engine.v5.1.0-186"},
		},
	}
	r := newTestReconciler(sub, installPlan)

	result, err := r.healStaleOLMState(context.TODO(), sub)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !result.Requeue {
		t.Errorf("expected Requeue=true after cleanup, got: %+v", result)
	}

	// Subscription should be deleted.
	got := &subv1alpha1.Subscription{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: sub.Name, Namespace: sub.Namespace}, got)
	if err == nil {
		t.Errorf("expected subscription to be deleted, but it still exists")
	}

	// InstallPlan should be deleted.
	gotIP := &subv1alpha1.InstallPlanList{}
	if err := r.Client.List(context.TODO(), gotIP, client.InNamespace("multicluster-engine")); err != nil {
		t.Fatalf("listing InstallPlans: %v", err)
	}
	if len(gotIP.Items) != 0 {
		t.Errorf("expected 0 InstallPlans after cleanup, got %d", len(gotIP.Items))
	}
}

func TestHealStaleOLMState_FullEnsurePath(t *testing.T) {
	// Verify that ensureMCEInstallation calls healStaleOLMState and self-heals.
	registerScheme()

	mch := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multiclusterhub",
			Namespace: "open-cluster-management",
		},
		Spec: operatorsv1.MultiClusterHubSpec{},
	}
	// Orphaned subscription (CSV missing).
	sub := &subv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multicluster-engine",
			Namespace: "multicluster-engine",
			Labels: map[string]string{
				"installer.name":                          "multiclusterhub",
				"installer.namespace":                     "open-cluster-management",
				multiclusterengineutils.MCEManagedByLabel: "true",
			},
		},
		Spec: &subv1alpha1.SubscriptionSpec{
			Package: "multicluster-engine",
		},
		Status: subv1alpha1.SubscriptionStatus{
			InstalledCSV: "multicluster-engine.v5.1.0-186",
		},
	}

	r := newTestReconciler(mch, sub)
	r.OLMVersion = "v0"

	result, err := r.ensureMCEInstallation(context.TODO(), mch)
	// Should requeue after healing (sub deleted).
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if !result.Requeue {
		t.Errorf("expected Requeue=true after stale OLM healing, got: %+v", result)
	}

	// Subscription should be gone.
	got := &subv1alpha1.Subscription{}
	err = r.Client.Get(context.TODO(), types.NamespacedName{Name: sub.Name, Namespace: sub.Namespace}, got)
	if err == nil {
		t.Error("expected subscription to be deleted after self-healing")
	}
}
