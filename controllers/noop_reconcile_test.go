// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"

	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	ocv1 "github.com/operator-framework/operator-controller/api/v1"
	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine"
	v0 "github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine/olm/v0"
	v1 "github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine/olm/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengineutils"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"
	resources "github.com/stolostron/multiclusterhub-operator/test/unit-tests"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

// ---------------------------------------------------------------------------
// 1. ensureMultiClusterEngineCR — skip update when live matches desired
// ---------------------------------------------------------------------------

func Test_ensureMultiClusterEngineCR_SkipsUpdateWhenNoChange(t *testing.T) {
	registerScheme()

	mch := resources.EmptyMCH()
	mch.Name = "test-mch-noop-mce"

	// Build a live MCE that matches what RenderMultiClusterEngine would produce.
	liveMCE := multiclusterengine.NewMultiClusterEngine(&mch, "multicluster-engine")
	liveMCE.Labels = map[string]string{
		multiclusterengineutils.MCEManagedByLabel: "true",
	}

	var updateCount int32
	r := newTestReconcilerWithInterceptor(interceptor.Funcs{
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if _, ok := obj.(*mcev1.MultiClusterEngine); ok {
				atomic.AddInt32(&updateCount, 1)
			}
			return c.Update(ctx, obj, opts...)
		},
	}, liveMCE)

	// Call twice — neither call should trigger an MCE update.
	for i := 0; i < 2; i++ {
		result, err := r.ensureMultiClusterEngineCR(context.Background(), &mch)
		if err != nil {
			t.Fatalf("call %d: unexpected error: %v", i+1, err)
		}
		if result != (ctrl.Result{}) {
			t.Fatalf("call %d: unexpected result: %+v", i+1, result)
		}
	}

	if n := atomic.LoadInt32(&updateCount); n != 0 {
		t.Errorf("expected 0 MCE Update calls, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// 2. ensureMCEInstallation — skip subscription update when live matches
// ---------------------------------------------------------------------------

func Test_ensureMCEInstallation_SkipsSubscriptionUpdateWhenNoChange(t *testing.T) {
	registerScheme()
	t.Setenv("POD_NAMESPACE", "test-ns")

	mch := resources.EmptyMCH()
	mch.Name = "test-mch-noop-sub"
	mch.Namespace = "test-ns"
	// Supply catalog source via annotation to bypass package manifest lookup.
	mch.Annotations = map[string]string{
		utils.AnnotationMCESubscriptionSpec: `{"source":"redhat-operators"}`,
	}

	mchOperatorDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      utils.MCHOperatorName,
			Namespace: "test-ns",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "operator", Image: "test"}}},
			},
		},
	}

	// Seed the MCH operator subscription + CSV so FindMultiClusterHubOperatorSubscription
	// succeeds (otherwise its stale error leaks through the noop path).
	mchOperatorCSV := &subv1alpha1.ClusterServiceVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "advanced-cluster-management.v2.0.0",
			Namespace: "test-ns",
		},
		Spec: subv1alpha1.ClusterServiceVersionSpec{
			InstallStrategy: subv1alpha1.NamedInstallStrategy{
				StrategyName: "deployment",
				StrategySpec: subv1alpha1.StrategyDetailsDeployment{
					DeploymentSpecs: []subv1alpha1.StrategyDeploymentSpec{
						{Name: utils.MCHOperatorName},
					},
				},
			},
		},
	}
	mchOperatorSub := &subv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "acm-operator-subscription",
			Namespace: "test-ns",
		},
		Spec: &subv1alpha1.SubscriptionSpec{
			Package:             "advanced-cluster-management",
			InstallPlanApproval: subv1alpha1.ApprovalAutomatic,
		},
		Status: subv1alpha1.SubscriptionStatus{
			CurrentCSV: "advanced-cluster-management.v2.0.0",
		},
	}

	// Build the subscription config that GetSubConfig() would return from the
	// test deployment (no node selector, no tolerations, empty proxy env slice).
	subConfig := &subv1alpha1.SubscriptionConfig{
		Env: []corev1.EnvVar{},
	}

	// Build the override that the annotation produces.
	subOverrides := &subv1alpha1.SubscriptionSpec{
		CatalogSource:       "redhat-operators",
		InstallPlanApproval: subv1alpha1.ApprovalAutomatic,
	}

	// Create a "live" subscription that was already created by a different MCH so
	// CreatedByMCH() is false, pushing the reconciler straight to render+compare.
	otherMCH := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Name: "other-mch", Namespace: "other-ns"},
	}
	existingSub := v0.NewSubscription(otherMCH, subConfig, subOverrides)

	var updateCount int32
	r := newTestReconcilerWithInterceptor(interceptor.Funcs{
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if _, ok := obj.(*subv1alpha1.Subscription); ok {
				atomic.AddInt32(&updateCount, 1)
			}
			return c.Update(ctx, obj, opts...)
		},
	}, mchOperatorDeployment, mchOperatorCSV, mchOperatorSub, existingSub)
	r.OLMVersion = "v0"

	result, err := r.ensureMCEInstallation(context.Background(), &mch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected result: %+v", result)
	}

	if n := atomic.LoadInt32(&updateCount); n != 0 {
		t.Errorf("expected 0 Subscription Update calls, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// 3. ensureMCEClusterExtension — skip update when live matches desired
// ---------------------------------------------------------------------------

func Test_ensureMCEClusterExtension_SkipsUpdateWhenNoChange(t *testing.T) {
	registerScheme()

	mch := resources.EmptyMCH()
	mch.Name = "test-mch-noop-ce"

	// Build a live ClusterExtension that matches desired.
	otherMCH := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Name: "other-mch", Namespace: "other-ns"},
	}
	existingCE := v1.NewClusterExtension(otherMCH)

	var updateCount int32
	r := newTestReconcilerWithInterceptor(interceptor.Funcs{
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if _, ok := obj.(*ocv1.ClusterExtension); ok {
				atomic.AddInt32(&updateCount, 1)
			}
			return c.Update(ctx, obj, opts...)
		},
	}, existingCE)

	result, err := r.ensureMCEClusterExtension(context.Background(), &mch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected result: %+v", result)
	}

	if n := atomic.LoadInt32(&updateCount); n != 0 {
		t.Errorf("expected 0 ClusterExtension Update calls, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// 4. ensureMultiClusterEngineCR — updates when spec changes
// ---------------------------------------------------------------------------

func Test_ensureMultiClusterEngineCR_UpdatesOnSpecChange(t *testing.T) {
	registerScheme()

	mch := resources.EmptyMCH()
	mch.Name = "test-mch-mce-update"
	mch.Spec.AvailabilityConfig = operatorsv1.HAHigh

	// Live MCE has AvailabilityConfig = HABasic, which should differ.
	liveMCE := multiclusterengine.NewMultiClusterEngine(&mch, "multicluster-engine")
	liveMCE.Labels = map[string]string{
		multiclusterengineutils.MCEManagedByLabel: "true",
	}
	liveMCE.Spec.AvailabilityConfig = mcev1.HABasic

	var updateCount int32
	r := newTestReconcilerWithInterceptor(interceptor.Funcs{
		Update: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.UpdateOption) error {
			if _, ok := obj.(*mcev1.MultiClusterEngine); ok {
				atomic.AddInt32(&updateCount, 1)
			}
			return c.Update(ctx, obj, opts...)
		},
	}, liveMCE)

	result, err := r.ensureMultiClusterEngineCR(context.Background(), &mch)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("unexpected result: %+v", result)
	}

	if n := atomic.LoadInt32(&updateCount); n != 1 {
		t.Errorf("expected exactly 1 MCE Update call, got %d", n)
	}
}

// ---------------------------------------------------------------------------
// 5. errors.Is works through joined errors (errors.Join)
// ---------------------------------------------------------------------------

func Test_errorsIs_WorksThroughJoinedErrors(t *testing.T) {
	conflictErr := apierrors.NewConflict(
		schema.GroupResource{Group: "test", Resource: "widgets"},
		"widget-1", fmt.Errorf("the object has been modified"))

	otherErr := fmt.Errorf("something else went wrong")

	joined := errors.Join(otherErr, conflictErr)
	if !apierrors.IsConflict(joined) {
		t.Error("expected errors.Is(joined, Conflict) to be true — errors.Join should preserve Is() semantics")
	}
	if !errors.Is(joined, otherErr) {
		t.Error("expected errors.Is to find otherErr in joined error")
	}
}
