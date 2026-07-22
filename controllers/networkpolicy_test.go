// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"
	"os"
	"testing"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
	clog "sigs.k8s.io/controller-runtime/pkg/log"
)

func newTestMCH(name, namespace string, networkPoliciesEnabled *bool, components ...operatorv1.ComponentConfig) *operatorv1.MultiClusterHub {
	mch := &operatorv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	if networkPoliciesEnabled != nil {
		mch.Spec.NetworkPolicies = &operatorv1.NetworkPoliciesConfig{
			Enabled: *networkPoliciesEnabled,
		}
	}
	if len(components) > 0 {
		mch.Spec.Overrides = &operatorv1.Overrides{
			Components: components,
		}
	}
	return mch
}

func boolPtr(b bool) *bool {
	return &b
}

func setChartEnv(t *testing.T) {
	t.Helper()
	os.Setenv("DIRECTORY_OVERRIDE", "../pkg/templates")
	os.Setenv("ACM_HUB_OCP_VERSION", "4.16.0")
	t.Cleanup(func() {
		os.Unsetenv("DIRECTORY_OVERRIDE")
		os.Unsetenv("ACM_HUB_OCP_VERSION")
	})
}

func newTestReconciler(objs ...client.Object) *MultiClusterHubReconciler {
	return &MultiClusterHubReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(objs...).Build(),
		Scheme: scheme.Scheme,
		Log:    clog.Log.WithName("test"),
		CacheSpec: CacheSpec{
			ImageOverrides:    getTestImageOverrides(),
			TemplateOverrides: map[string]string{},
		},
	}
}

func newTestReconcilerWithInterceptor(funcs interceptor.Funcs, objs ...client.Object) *MultiClusterHubReconciler {
	return &MultiClusterHubReconciler{
		Client: fake.NewClientBuilder().WithScheme(scheme.Scheme).WithObjects(objs...).WithInterceptorFuncs(funcs).Build(),
		Scheme: scheme.Scheme,
		Log:    clog.Log.WithName("test"),
		CacheSpec: CacheSpec{
			ImageOverrides:    getTestImageOverrides(),
			TemplateOverrides: map[string]string{},
		},
	}
}

func mchNetworkPolicy(name, namespace, mchName, mchNamespace string) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"installer.name":      mchName,
				"installer.namespace": mchNamespace,
			},
		},
	}
}

// --- Disabled path tests ---

func Test_ensureNetworkPolicies_Disabled_DeletesAll(t *testing.T) {
	np1 := mchNetworkPolicy("np-one", "ocm", "mch", "ocm")
	np2 := mchNetworkPolicy("np-two", "ocm", "mch", "ocm")

	r := newTestReconciler(np1, np2)
	mch := newTestMCH("mch", "ocm", boolPtr(false))

	result, err := r.ensureNetworkPolicies(context.TODO(), mch, r.CacheSpec, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("expected empty result, got: %v", result)
	}

	npList := &networkingv1.NetworkPolicyList{}
	if err := r.Client.List(context.TODO(), npList, client.InNamespace("ocm")); err != nil {
		t.Fatalf("failed to list NPs: %v", err)
	}
	if len(npList.Items) != 0 {
		t.Errorf("expected 0 NPs after disable, got %d", len(npList.Items))
	}
}

func Test_ensureNetworkPolicies_Disabled_NoNPs(t *testing.T) {
	r := newTestReconciler()
	mch := newTestMCH("mch", "ocm", boolPtr(false))

	result, err := r.ensureNetworkPolicies(context.TODO(), mch, r.CacheSpec, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("expected empty result, got: %v", result)
	}
}

func Test_ensureNetworkPolicies_Disabled_ListError(t *testing.T) {
	r := newTestReconcilerWithInterceptor(interceptor.Funcs{
		List: func(ctx context.Context, c client.WithWatch, list client.ObjectList, opts ...client.ListOption) error {
			if _, ok := list.(*networkingv1.NetworkPolicyList); ok {
				return fmt.Errorf("simulated list error")
			}
			return c.List(ctx, list, opts...)
		},
	})
	mch := newTestMCH("mch", "ocm", boolPtr(false))

	_, err := r.ensureNetworkPolicies(context.TODO(), mch, r.CacheSpec, false)
	if err == nil {
		t.Fatal("expected error from List, got nil")
	}
	if expected := "failed to list NetworkPolicies: simulated list error"; err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func Test_ensureNetworkPolicies_Disabled_DeleteError(t *testing.T) {
	np := mchNetworkPolicy("np-one", "ocm", "mch", "ocm")

	r := newTestReconcilerWithInterceptor(interceptor.Funcs{
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			if _, ok := obj.(*networkingv1.NetworkPolicy); ok {
				return fmt.Errorf("simulated delete error")
			}
			return c.Delete(ctx, obj, opts...)
		},
	}, np)
	mch := newTestMCH("mch", "ocm", boolPtr(false))

	_, err := r.ensureNetworkPolicies(context.TODO(), mch, r.CacheSpec, false)
	if err == nil {
		t.Fatal("expected error from Delete, got nil")
	}
	if expected := "failed to delete NetworkPolicy ocm/np-one: simulated delete error"; err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func Test_ensureNetworkPolicies_DefaultEnabled(t *testing.T) {
	// NetworkPolicies field nil → defaults to enabled, should not delete existing NPs
	np := mchNetworkPolicy("np-one", "ocm", "mch", "ocm")
	r := newTestReconciler(np)
	mch := newTestMCH("mch", "ocm", nil) // nil = default enabled

	setChartEnv(t)

	result, err := r.ensureNetworkPolicies(context.TODO(), mch, r.CacheSpec, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("expected empty result, got: %v", result)
	}

	// NP should still exist (not deleted)
	npList := &networkingv1.NetworkPolicyList{}
	if err := r.Client.List(context.TODO(), npList, client.InNamespace("ocm")); err != nil {
		t.Fatalf("failed to list NPs: %v", err)
	}
	if len(npList.Items) != 1 {
		t.Errorf("expected NP to still exist, got %d NPs", len(npList.Items))
	}
}

// --- Enabled path tests ---

func Test_ensureNetworkPolicies_Enabled_CreatesNP(t *testing.T) {
	setChartEnv(t)

	registerScheme()
	r := newTestReconciler()
	mch := newTestMCH("mch", "open-cluster-management", boolPtr(true),
		operatorv1.ComponentConfig{Name: operatorv1.MTVIntegrations, Enabled: true},
	)

	result, err := r.ensureNetworkPolicies(context.TODO(), mch, r.CacheSpec, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("expected empty result, got: %v", result)
	}

	// Verify NP was created
	npList := &networkingv1.NetworkPolicyList{}
	if err := r.Client.List(context.TODO(), npList, client.InNamespace("open-cluster-management")); err != nil {
		t.Fatalf("failed to list NPs: %v", err)
	}
	if len(npList.Items) == 0 {
		t.Fatal("expected NetworkPolicy to be created, got 0")
	}

	found := false
	for _, np := range npList.Items {
		if np.Name == "mtv-integrations-controller" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected mtv-integrations-controller NetworkPolicy to be created")
	}
}

func Test_ensureNetworkPolicies_Enabled_SkipsExisting(t *testing.T) {
	setChartEnv(t)

	registerScheme()

	existingNP := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mtv-integrations-controller",
			Namespace: "open-cluster-management",
			Labels: map[string]string{
				"installer.name":      "mch",
				"installer.namespace": "open-cluster-management",
				"custom-label":        "should-persist",
			},
		},
	}
	r := newTestReconciler(existingNP)
	mch := newTestMCH("mch", "open-cluster-management", boolPtr(true),
		operatorv1.ComponentConfig{Name: operatorv1.MTVIntegrations, Enabled: true},
	)

	result, err := r.ensureNetworkPolicies(context.TODO(), mch, r.CacheSpec, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("expected empty result, got: %v", result)
	}

	// Verify existing NP was not modified (custom label still present)
	np := &networkingv1.NetworkPolicy{}
	if err := r.Client.Get(context.TODO(), client.ObjectKey{
		Name:      "mtv-integrations-controller",
		Namespace: "open-cluster-management",
	}, np); err != nil {
		t.Fatalf("expected NP to still exist: %v", err)
	}
	if np.Labels["custom-label"] != "should-persist" {
		t.Error("expected existing NP to be untouched (custom-label missing)")
	}
}

func Test_ensureNetworkPolicies_Enabled_GetError(t *testing.T) {
	setChartEnv(t)

	registerScheme()

	r := newTestReconcilerWithInterceptor(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*networkingv1.NetworkPolicy); ok {
				return fmt.Errorf("simulated get error")
			}
			return c.Get(ctx, key, obj, opts...)
		},
	})
	mch := newTestMCH("mch", "open-cluster-management", boolPtr(true),
		operatorv1.ComponentConfig{Name: operatorv1.MTVIntegrations, Enabled: true},
	)

	_, err := r.ensureNetworkPolicies(context.TODO(), mch, r.CacheSpec, false)
	if err == nil {
		t.Fatal("expected error from Get, got nil")
	}
	if expected := "failed to get NetworkPolicy deny-all: simulated get error"; err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}

func Test_ensureNetworkPolicies_Enabled_CreateError(t *testing.T) {
	setChartEnv(t)

	registerScheme()

	r := newTestReconcilerWithInterceptor(interceptor.Funcs{
		Create: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.CreateOption) error {
			if obj.GetObjectKind().GroupVersionKind().Kind == "NetworkPolicy" {
				return fmt.Errorf("simulated create error")
			}
			return c.Create(ctx, obj, opts...)
		},
	})
	mch := newTestMCH("mch", "open-cluster-management", boolPtr(true),
		operatorv1.ComponentConfig{Name: operatorv1.MTVIntegrations, Enabled: true},
	)

	_, err := r.ensureNetworkPolicies(context.TODO(), mch, r.CacheSpec, false)
	if err == nil {
		t.Fatal("expected error from Create, got nil")
	}
}

// --- Disabled component path tests ---

func Test_ensureNetworkPolicies_DisabledComponent_DeletesNP(t *testing.T) {
	setChartEnv(t)

	registerScheme()

	existingNP := mchNetworkPolicy("mtv-integrations-controller", "open-cluster-management", "mch", "open-cluster-management")
	r := newTestReconciler(existingNP)
	mch := newTestMCH("mch", "open-cluster-management", boolPtr(true),
		operatorv1.ComponentConfig{Name: operatorv1.MTVIntegrations, Enabled: false},
	)

	result, err := r.ensureNetworkPolicies(context.TODO(), mch, r.CacheSpec, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("expected empty result, got: %v", result)
	}

	// NP should be deleted
	npList := &networkingv1.NetworkPolicyList{}
	if err := r.Client.List(context.TODO(), npList, client.InNamespace("open-cluster-management")); err != nil {
		t.Fatalf("failed to list NPs: %v", err)
	}
	if len(npList.Items) != 0 {
		t.Errorf("expected NP to be deleted, got %d", len(npList.Items))
	}
}

func Test_ensureNetworkPolicies_DisabledComponent_SkipsNonMCH(t *testing.T) {
	setChartEnv(t)

	registerScheme()

	// NP without MCH installer labels should NOT be deleted
	existingNP := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mtv-integrations-controller",
			Namespace: "open-cluster-management",
			Labels: map[string]string{
				"installer.name":      "other-installer",
				"installer.namespace": "other-ns",
			},
		},
	}
	r := newTestReconciler(existingNP)
	mch := newTestMCH("mch", "open-cluster-management", boolPtr(true),
		operatorv1.ComponentConfig{Name: operatorv1.MTVIntegrations, Enabled: false},
	)

	result, err := r.ensureNetworkPolicies(context.TODO(), mch, r.CacheSpec, false)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if result != (ctrl.Result{}) {
		t.Fatalf("expected empty result, got: %v", result)
	}

	// NP should still exist (not owned by this MCH)
	np := &networkingv1.NetworkPolicy{}
	if err := r.Client.Get(context.TODO(), client.ObjectKey{
		Name:      "mtv-integrations-controller",
		Namespace: "open-cluster-management",
	}, np); err != nil {
		t.Error("expected non-MCH NP to still exist")
	}
}

func Test_ensureNetworkPolicies_DisabledComponent_GetError(t *testing.T) {
	setChartEnv(t)

	registerScheme()

	r := newTestReconcilerWithInterceptor(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object, opts ...client.GetOption) error {
			if _, ok := obj.(*networkingv1.NetworkPolicy); ok {
				return fmt.Errorf("simulated get error")
			}
			return c.Get(ctx, key, obj, opts...)
		},
	})
	mch := newTestMCH("mch", "open-cluster-management", boolPtr(true),
		operatorv1.ComponentConfig{Name: operatorv1.MTVIntegrations, Enabled: false},
	)

	_, err := r.ensureNetworkPolicies(context.TODO(), mch, r.CacheSpec, false)
	if err == nil {
		t.Fatal("expected error from Get, got nil")
	}
	if expected := "failed to get NetworkPolicy deny-all: simulated get error"; err.Error() != expected {
		t.Errorf("error = %q, want %q", err.Error(), expected)
	}
}
