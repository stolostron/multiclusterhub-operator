// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"strings"
	"testing"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	clog "sigs.k8s.io/controller-runtime/pkg/log"
)

// TestEnsureComponent_RenderError verifies that a chart render failure sets a
// ComponentFailure status condition on the MultiClusterHub and returns an error
// (triggering controller-runtime backoff) instead of silently requeuing.
func TestEnsureComponent_RenderError(t *testing.T) {
	_ = operatorv1.AddToScheme(scheme.Scheme)

	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme.Scheme).
		Build()

	testImages := map[string]string{}
	for _, v := range utils.GetTestImages() {
		testImages[v] = "quay.io/test/test:Test"
	}

	reconciler := &MultiClusterHubReconciler{
		Client: fakeClient,
		Scheme: scheme.Scheme,
		Log:    clog.Log.WithName("test-render-error"),
		CacheSpec: CacheSpec{
			ImageOverrides:    testImages,
			TemplateOverrides: map[string]string{},
		},
	}

	mch := &operatorv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mch",
			Namespace: "test-ns",
		},
		Spec: operatorv1.MultiClusterHubSpec{},
	}

	// "nonexistent-component" maps to /chart/toggle/nonexistent-component via
	// fetchChartLocation's default case — a path that does not exist, causing
	// RenderChart to return errors.
	component := "nonexistent-component"

	cachespec := CacheSpec{
		ImageOverrides:    testImages,
		TemplateOverrides: map[string]string{},
	}

	t.Run("ensureComponent returns error on render failure", func(t *testing.T) {
		result, err := reconciler.ensureComponent(context.TODO(), mch, component, cachespec, false)

		if err == nil {
			t.Fatal("expected non-nil error from ensureComponent when chart render fails, got nil")
		}

		if result != (ctrl.Result{}) {
			t.Errorf("expected empty Result (for backoff), got %+v", result)
		}

		if !strings.Contains(err.Error(), "failed to render chart") {
			t.Errorf("expected error to contain 'failed to render chart', got: %s", err.Error())
		}

		// Verify a ComponentFailure condition was set on the MCH status
		condType := operatorv1.HubConditionType(
			string(operatorv1.ComponentFailure) + ": " + component,
		)
		cond := GetHubCondition(mch.Status, condType)
		if cond == nil {
			t.Fatalf("expected ComponentFailure condition for %q, got nil", component)
		}

		if cond.Reason != ResourceRenderReason {
			t.Errorf("expected condition reason %q, got %q", ResourceRenderReason, cond.Reason)
		}

		if cond.Status != metav1.ConditionTrue {
			t.Errorf("expected condition status True, got %s", cond.Status)
		}
	})

	// Reset status for removal test
	mch.Status = operatorv1.MultiClusterHubStatus{}

	t.Run("ensureNoComponent returns error on render failure", func(t *testing.T) {
		result, err := reconciler.ensureNoComponent(context.TODO(), mch, component, cachespec, false)

		if err == nil {
			t.Fatal("expected non-nil error from ensureNoComponent when chart render fails, got nil")
		}

		if result != (ctrl.Result{}) {
			t.Errorf("expected empty Result (for backoff), got %+v", result)
		}

		if !strings.Contains(err.Error(), "failed to render chart for removal") {
			t.Errorf("expected error to contain 'failed to render chart for removal', got: %s", err.Error())
		}

		// Verify a ComponentFailure condition was set on the MCH status
		condType := operatorv1.HubConditionType(
			string(operatorv1.ComponentFailure) + ": " + component,
		)
		cond := GetHubCondition(mch.Status, condType)
		if cond == nil {
			t.Fatalf("expected ComponentFailure condition for %q, got nil", component)
		}

		if cond.Reason != ResourceRenderReason {
			t.Errorf("expected condition reason %q, got %q", ResourceRenderReason, cond.Reason)
		}
	})
}
