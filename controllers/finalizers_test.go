package controllers

import (
	"context"
	"fmt"
	"strings"
	"testing"

	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	backplanev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine"
	mceolmv0 "github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine/olm/v0"
	mceolmv1 "github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine/olm/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengineutils"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	resources "github.com/stolostron/multiclusterhub-operator/test/unit-tests"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/interceptor"
)

func TestBackupNamespace(t *testing.T) {
	tests := []struct {
		name  string
		want  string
		want2 string
		want3 string
	}{
		{
			name:  "basic return values test",
			want:  "v1",
			want2: "Namespace",
			want3: "open-cluster-management-backup",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BackupNamespace()
			if got.APIVersion != tt.want {
				t.Errorf("BackupNamespace() = %v, want %v", got, tt.want)
			}
			if got.Kind != tt.want2 {
				t.Errorf("BackupNamespace() = %v, want %v", got, tt.want)
			}
			if got.Name != tt.want3 {
				t.Errorf("BackupNamespace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBackupNamespaceUnstructured(t *testing.T) {
	tests := []struct {
		name  string
		want  string
		want2 string
		want3 string
	}{
		{
			name:  "basic return values test",
			want:  "v1",
			want2: "Namespace",
			want3: "open-cluster-management-backup",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BackupNamespaceUnstructured()
			if got.GetAPIVersion() != tt.want {
				t.Errorf("BackupNamespace() = %v, want %v", got, tt.want)
			}
			if got.GetKind() != tt.want2 {
				t.Errorf("BackupNamespace() = %v, want %v", got, tt.want)
			}
			if got.GetName() != tt.want3 {
				t.Errorf("BackupNamespace() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_cleanupMultiClusterEngine(t *testing.T) {
	tests := []struct {
		name string
		mch  operatorv1.MultiClusterHub
		mce  backplanev1.MultiClusterEngine
		want bool
	}{
		{
			name: "should cleanup MultiClusterEngine",
			mce:  resources.EmptyMCE(),
			mch:  resources.EmptyMCH(),
			want: true,
		},
	}

	registerScheme()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := recon.Client.Create(context.TODO(), &tt.mch); err != nil {
				t.Errorf("failed to create MultiClusterHub: %v", err)
			}

			tt.mce.Labels = map[string]string{
				"installer.name":                          tt.mch.GetName(),
				"installer.namespace":                     tt.mch.GetNamespace(),
				multiclusterengineutils.MCEManagedByLabel: "true",
			}
			if err := recon.Client.Create(context.TODO(), &tt.mce); err != nil {
				t.Errorf("failed to create MultiClusterEngine: %v", err)
			}

			// If MCE exists the first time it will return an error.
			if err := recon.cleanupMultiClusterEngine(context.TODO(), log, &tt.mch); err == nil {
				t.Errorf("failed to cleanup MultiClusterEngine: %v", err)
			}

			if err := recon.cleanupMultiClusterEngine(context.TODO(), log, &tt.mch); err != nil {
				t.Errorf("failed to cleanup MultiClusterEngine: %v", err)
			}
		})
	}
}

func Test_cleanupMultiClusterEngine_OLMv1(t *testing.T) {
	registerScheme()

	mch := resources.EmptyMCH()
	mch.Name = "test-mch-olmv1"
	mce := resources.EmptyMCE()
	mce.Name = "test-mce-olmv1"
	mce.Labels = map[string]string{
		"installer.name":                          mch.GetName(),
		"installer.namespace":                     mch.GetNamespace(),
		multiclusterengineutils.MCEManagedByLabel: "true",
	}

	// Setup client with MCE
	if err := recon.Client.Create(context.TODO(), &mch); err != nil {
		t.Fatalf("failed to create MultiClusterHub: %v", err)
	}
	if err := recon.Client.Create(context.TODO(), &mce); err != nil {
		t.Fatalf("failed to create MultiClusterEngine: %v", err)
	}

	// Set OLM v1 mode
	recon.OLMVersion = "v1"

	// First call should return error (MCE still exists)
	err := recon.cleanupMultiClusterEngine(context.TODO(), log, &mch)
	if err == nil {
		t.Error("expected error on first cleanup call, got nil")
	}

	// Second call should succeed (MCE deleted)
	err = recon.cleanupMultiClusterEngine(context.TODO(), log, &mch)
	if err != nil {
		t.Errorf("expected no error on second cleanup call, got: %v", err)
	}

	// Reset OLM version
	recon.OLMVersion = ""
}

func Test_cleanupMultiClusterEngine_OLMv0(t *testing.T) {
	registerScheme()

	mch := resources.EmptyMCH()
	mch.Name = "test-mch-olmv0"
	mce := resources.EmptyMCE()
	mce.Name = "test-mce-olmv0"
	mce.Labels = map[string]string{
		"installer.name":                          mch.GetName(),
		"installer.namespace":                     mch.GetNamespace(),
		multiclusterengineutils.MCEManagedByLabel: "true",
	}

	// Setup client with MCE
	if err := recon.Client.Create(context.TODO(), &mch); err != nil {
		t.Fatalf("failed to create MultiClusterHub: %v", err)
	}
	if err := recon.Client.Create(context.TODO(), &mce); err != nil {
		t.Fatalf("failed to create MultiClusterEngine: %v", err)
	}

	// Set OLM v0 mode
	recon.OLMVersion = "v0"

	// First call should return error (MCE still exists)
	err := recon.cleanupMultiClusterEngine(context.TODO(), log, &mch)
	if err == nil {
		t.Error("expected error on first cleanup call, got nil")
	}

	// Second call should succeed (MCE deleted)
	err = recon.cleanupMultiClusterEngine(context.TODO(), log, &mch)
	if err != nil {
		t.Errorf("expected no error on second cleanup call, got: %v", err)
	}

	// Reset OLM version
	recon.OLMVersion = ""
}

// --- HubCondition tests ---
//
// The following tests cover the status condition updates added so that
// `kubectl get mch` surfaces which resource is blocking finalization
// (Terminating conditions) or component removal (Progressing conditions).

// Test_cleanupMultiClusterEngine_SetsTerminatingCondition_MCE verifies that
// while a preexisting MultiClusterEngine is still being deleted, a
// Terminating/DeleteTimestampReason condition naming the MCE is recorded on
// the MultiClusterHub status.
func Test_cleanupMultiClusterEngine_SetsTerminatingCondition_MCE(t *testing.T) {
	registerScheme()

	mch := resources.EmptyMCH()
	mch.Name = "test-mch-mce-condition"

	mce := resources.EmptyMCE()
	mce.Name = "test-mce-condition"
	mce.Labels = map[string]string{
		"installer.name":                          mch.GetName(),
		"installer.namespace":                     mch.GetNamespace(),
		multiclusterengineutils.MCEManagedByLabel: "true",
	}

	r := newTestReconciler(&mce)

	// First call finds the MCE still present, deletes it, and should report
	// that it is waiting for termination.
	err := r.cleanupMultiClusterEngine(context.TODO(), log, &mch)
	if err == nil {
		t.Fatal("expected error while MCE is still terminating, got nil")
	}

	condition := GetHubCondition(mch.Status, operatorv1.Terminating)
	if condition == nil {
		t.Fatal("expected Terminating condition to be set")
	}
	if condition.Reason != DeleteTimestampReason {
		t.Errorf("expected reason %q, got %q", DeleteTimestampReason, condition.Reason)
	}
	if !strings.Contains(condition.Message, mce.GetName()) {
		t.Errorf("expected message to mention MCE name %q, got %q", mce.GetName(), condition.Message)
	}
}

// Test_cleanupMultiClusterEngine_SetsTerminatingCondition_Namespace verifies
// that when the MCE operand namespace is still present (and owned
// separately from the MCH namespace), a Terminating condition naming the
// namespace is recorded and the namespace delete is requested.
func Test_cleanupMultiClusterEngine_SetsTerminatingCondition_Namespace(t *testing.T) {
	registerScheme()

	mch := resources.EmptyMCH()
	mch.Name = "test-mch-ns-condition"
	// EmptyMCH() defaults to namespace "open-cluster-management", which is
	// distinct from the MCE operand namespace ("multicluster-engine").

	mceNamespace := multiclusterengine.Namespace()

	r := newTestReconciler(mceNamespace)
	// UNIT_TEST is normally forced to "true" by the Ginkgo suite's
	// BeforeSuite; pin it explicitly here so this test's expected code path
	// (past the IsUnitTest short-circuit) is exercised deterministically
	// regardless of test execution order.
	t.Setenv(utils.UnitTestEnvVar, "false")
	r.OLMVersion = "" // skip OLM-specific cleanup, go straight to namespace check

	err := r.cleanupMultiClusterEngine(context.TODO(), log, &mch)
	if err == nil {
		t.Fatal("expected error while MCE namespace is still terminating, got nil")
	}

	condition := GetHubCondition(mch.Status, operatorv1.Terminating)
	if condition == nil {
		t.Fatal("expected Terminating condition to be set")
	}
	if condition.Reason != DeleteTimestampReason {
		t.Errorf("expected reason %q, got %q", DeleteTimestampReason, condition.Reason)
	}
	if !strings.Contains(condition.Message, mceNamespace.GetName()) {
		t.Errorf("expected message to mention namespace %q, got %q", mceNamespace.GetName(), condition.Message)
	}

	// The namespace should have a delete request issued against it.
	ns := &corev1.Namespace{}
	getErr := r.Client.Get(context.TODO(), client.ObjectKeyFromObject(mceNamespace), ns)
	if getErr == nil && ns.DeletionTimestamp == nil {
		t.Error("expected namespace deletion to have been requested")
	}
}

// Test_cleanupMultiClusterEngine_NoNamespaceCondition_WhenSharedNamespace
// verifies that no Terminating condition is set (and no error returned) when
// the MCH and MCE share a namespace, since the namespace shouldn't be
// deleted out from under the MCH itself.
func Test_cleanupMultiClusterEngine_NoNamespaceCondition_WhenSharedNamespace(t *testing.T) {
	registerScheme()

	mch := resources.EmptyMCH()
	mch.Name = "test-mch-shared-ns"
	mch.Namespace = multiclusterengine.OperandNamespace()

	r := newTestReconciler()
	t.Setenv(utils.UnitTestEnvVar, "false")
	r.OLMVersion = ""

	err := r.cleanupMultiClusterEngine(context.TODO(), log, &mch)
	if err != nil {
		t.Fatalf("expected no error when MCH shares namespace with MCE, got: %v", err)
	}

	if condition := GetHubCondition(mch.Status, operatorv1.Terminating); condition != nil {
		t.Errorf("expected no Terminating condition, got: %+v", condition)
	}
}

// Test_cleanupMultiClusterEngine_OLMv1_SetsTerminatingCondition_ClusterExtension
// verifies that when the OLM v1 MCE ClusterExtension is deleted but still
// present (e.g. blocked by a finalizer), a Terminating condition naming the
// ClusterExtension is recorded.
func Test_cleanupMultiClusterEngine_OLMv1_SetsTerminatingCondition_ClusterExtension(t *testing.T) {
	registerScheme()

	mch := resources.EmptyMCH()
	mch.Name = "test-mch-ce-condition"

	ce := mceolmv1.NewClusterExtension(&mch)
	// Simulate a stuck deletion: the fake client keeps a finalized object
	// around (with a DeletionTimestamp) until its finalizers are cleared.
	ce.Finalizers = []string{"test.io/block-deletion"}

	r := newTestReconciler(ce)
	t.Setenv(utils.UnitTestEnvVar, "false")
	r.OLMVersion = "v1"

	err := r.cleanupMultiClusterEngine(context.TODO(), log, &mch)
	if err == nil {
		t.Fatal("expected error while ClusterExtension is still terminating, got nil")
	}

	condition := GetHubCondition(mch.Status, operatorv1.Terminating)
	if condition == nil {
		t.Fatal("expected Terminating condition to be set")
	}
	if condition.Reason != DeleteTimestampReason {
		t.Errorf("expected reason %q, got %q", DeleteTimestampReason, condition.Reason)
	}
	if !strings.Contains(condition.Message, ce.GetName()) {
		t.Errorf("expected message to mention ClusterExtension name %q, got %q", ce.GetName(), condition.Message)
	}
}

// Test_cleanupMultiClusterEngine_OLMv0_SetsTerminatingCondition_CSV verifies
// that when the OLM v0 MCE ClusterServiceVersion is deleted but still
// present, a Terminating condition naming the CSV is recorded.
func Test_cleanupMultiClusterEngine_OLMv0_SetsTerminatingCondition_CSV(t *testing.T) {
	registerScheme()

	mch := resources.EmptyMCH()
	mch.Name = "test-mch-csv-condition"

	sub := mceolmv0.NewSubscription(&mch, nil, nil)
	sub.Status.CurrentCSV = "multiclusterengine.v1.0.0"

	csv := &subv1alpha1.ClusterServiceVersion{
		ObjectMeta: metav1.ObjectMeta{
			Name:       sub.Status.CurrentCSV,
			Namespace:  sub.GetNamespace(),
			Finalizers: []string{"test.io/block-deletion"},
		},
	}

	r := newTestReconciler(sub, csv)
	t.Setenv(utils.UnitTestEnvVar, "false")
	r.OLMVersion = "v0"

	err := r.cleanupMultiClusterEngine(context.TODO(), log, &mch)
	if err == nil {
		t.Fatal("expected error while CSV is still terminating, got nil")
	}

	condition := GetHubCondition(mch.Status, operatorv1.Terminating)
	if condition == nil {
		t.Fatal("expected Terminating condition to be set")
	}
	if condition.Reason != DeleteTimestampReason {
		t.Errorf("expected reason %q, got %q", DeleteTimestampReason, condition.Reason)
	}
	if !strings.Contains(condition.Message, csv.GetName()) {
		t.Errorf("expected message to mention CSV name %q, got %q", csv.GetName(), condition.Message)
	}
}

// Test_cleanupMultiClusterEngine_OLMv0_SetsTerminatingCondition_Subscription
// verifies that when the OLM v0 MCE Subscription is deleted but still
// present (no CSV to resolve), a Terminating condition naming the
// Subscription is recorded.
func Test_cleanupMultiClusterEngine_OLMv0_SetsTerminatingCondition_Subscription(t *testing.T) {
	registerScheme()

	mch := resources.EmptyMCH()
	mch.Name = "test-mch-sub-condition"

	sub := mceolmv0.NewSubscription(&mch, nil, nil)
	// Leave Status.CurrentCSV empty so the CSV lookup is skipped and the
	// Subscription-terminating branch is what gets exercised.
	sub.Finalizers = []string{"test.io/block-deletion"}

	r := newTestReconciler(sub)
	t.Setenv(utils.UnitTestEnvVar, "false")
	r.OLMVersion = "v0"

	err := r.cleanupMultiClusterEngine(context.TODO(), log, &mch)
	if err == nil {
		t.Fatal("expected error while Subscription is still terminating, got nil")
	}

	condition := GetHubCondition(mch.Status, operatorv1.Terminating)
	if condition == nil {
		t.Fatal("expected Terminating condition to be set")
	}
	if condition.Reason != DeleteTimestampReason {
		t.Errorf("expected reason %q, got %q", DeleteTimestampReason, condition.Reason)
	}
	if !strings.Contains(condition.Message, sub.GetName()) {
		t.Errorf("expected message to mention Subscription name %q, got %q", sub.GetName(), condition.Message)
	}
}

// Test_cleanupNamespaces_SetsTerminatingCondition verifies that while the
// cluster-backup namespace is still present, a Terminating condition naming
// the namespace is recorded.
func Test_cleanupNamespaces_SetsTerminatingCondition(t *testing.T) {
	backupNs := BackupNamespace()
	r := newTestReconciler(backupNs)
	mch := resources.EmptyMCH()
	mch.Name = "test-mch-backup-ns"

	err := r.cleanupNamespaces(context.TODO(), log, &mch)
	if err == nil {
		t.Fatal("expected error while backup namespace is still terminating, got nil")
	}

	condition := GetHubCondition(mch.Status, operatorv1.Terminating)
	if condition == nil {
		t.Fatal("expected Terminating condition to be set")
	}
	if condition.Reason != DeleteTimestampReason {
		t.Errorf("expected reason %q, got %q", DeleteTimestampReason, condition.Reason)
	}
	if !strings.Contains(condition.Message, utils.ClusterSubscriptionNamespace) {
		t.Errorf("expected message to mention namespace %q, got %q", utils.ClusterSubscriptionNamespace, condition.Message)
	}
}

// Test_cleanupNamespaces_SetsTerminatingCondition_StuckMessage verifies that
// when the backup namespace reports a NamespaceDeletionContentFailure
// condition, the surfaced Terminating condition message includes the
// underlying blocking-resource detail instead of the generic waiting text.
func Test_cleanupNamespaces_SetsTerminatingCondition_StuckMessage(t *testing.T) {
	backupNs := BackupNamespace()
	backupNs.Status = corev1.NamespaceStatus{
		Phase: corev1.NamespaceTerminating,
		Conditions: []corev1.NamespaceCondition{
			{
				Type:    corev1.NamespaceDeletionContentFailure,
				Status:  corev1.ConditionTrue,
				Message: "some-resource.example.com \"blocker\" still present",
			},
		},
	}

	r := newTestReconciler(backupNs)
	// Update status subresource explicitly since fake client Create ignores
	// status by default for typed objects with a status subresource... use
	// Status().Update to ensure the condition is persisted.
	if err := r.Client.Status().Update(context.TODO(), backupNs); err != nil {
		t.Fatalf("failed to set namespace status: %v", err)
	}

	mch := resources.EmptyMCH()
	mch.Name = "test-mch-backup-ns-stuck"

	err := r.cleanupNamespaces(context.TODO(), log, &mch)
	if err == nil {
		t.Fatal("expected error while backup namespace is stuck terminating, got nil")
	}

	condition := GetHubCondition(mch.Status, operatorv1.Terminating)
	if condition == nil {
		t.Fatal("expected Terminating condition to be set")
	}
	if !strings.Contains(condition.Message, "stuck terminating") {
		t.Errorf("expected message to mention 'stuck terminating', got %q", condition.Message)
	}
	if !strings.Contains(condition.Message, "blocker") {
		t.Errorf("expected message to mention blocking resource, got %q", condition.Message)
	}
}

// Test_cleanupNamespaces_NoCondition_WhenNamespaceAbsent verifies that no
// error or condition is set once the backup namespace has been fully
// removed.
func Test_cleanupNamespaces_NoCondition_WhenNamespaceAbsent(t *testing.T) {
	r := newTestReconciler()
	mch := resources.EmptyMCH()
	mch.Name = "test-mch-backup-ns-absent"

	if err := r.cleanupNamespaces(context.TODO(), log, &mch); err != nil {
		t.Fatalf("expected no error when backup namespace is absent, got: %v", err)
	}

	if condition := GetHubCondition(mch.Status, operatorv1.Terminating); condition != nil {
		t.Errorf("expected no Terminating condition, got: %+v", condition)
	}
}

// Test_cleanupNamespaces_NoError_WhenDeleteRacesToNotFound verifies that if
// the backup namespace is deleted concurrently between the initial Get and
// the Delete call, cleanupNamespaces recognizes it's already gone and
// returns nil immediately, instead of reporting "waiting to terminate" and
// returning a retry error for a namespace that's no longer there.
func Test_cleanupNamespaces_NoError_WhenDeleteRacesToNotFound(t *testing.T) {
	backupNs := BackupNamespace()
	r := newTestReconcilerWithInterceptor(interceptor.Funcs{
		Delete: func(ctx context.Context, c client.WithWatch, obj client.Object, opts ...client.DeleteOption) error {
			if _, ok := obj.(*corev1.Namespace); ok {
				return apierrors.NewNotFound(corev1.Resource("namespaces"), obj.GetName())
			}
			return c.Delete(ctx, obj, opts...)
		},
	}, backupNs)

	mch := resources.EmptyMCH()
	mch.Name = "test-mch-backup-ns-delete-race"

	if err := r.cleanupNamespaces(context.TODO(), log, &mch); err != nil {
		t.Fatalf("expected no error when Delete races to NotFound, got: %v", err)
	}

	if condition := GetHubCondition(mch.Status, operatorv1.Terminating); condition != nil {
		t.Errorf("expected no Terminating condition, got: %+v", condition)
	}
}

// Test_cleanupNamespaces_ReturnsError_OnGetFailure verifies that a real
// lookup failure (not NotFound) on the initial namespace Get is returned as
// an error, instead of being silently treated the same as "namespace
// doesn't exist" — which would let finalization proceed as if the backup
// namespace were already gone.
func Test_cleanupNamespaces_ReturnsError_OnGetFailure(t *testing.T) {
	r := newTestReconcilerWithInterceptor(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object,
			opts ...client.GetOption) error {
			if _, ok := obj.(*corev1.Namespace); ok && key.Name == utils.ClusterSubscriptionNamespace {
				return apierrors.NewInternalError(fmt.Errorf("simulated get failure"))
			}
			return c.Get(ctx, key, obj, opts...)
		},
	})
	mch := resources.EmptyMCH()
	mch.Name = "test-mch-backup-ns-get-error"

	err := r.cleanupNamespaces(context.TODO(), log, &mch)
	if err == nil {
		t.Fatal("expected the Get error to be returned, got nil")
	}
	if !strings.Contains(err.Error(), "simulated get failure") {
		t.Errorf("expected the underlying Get error to propagate, got: %v", err)
	}
}

func mchLabeledAppSubscription(name, namespace, mchName, mchNamespace string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps.open-cluster-management.io",
		Kind:    "Subscription",
		Version: "v1",
	})
	u.SetName(name)
	u.SetNamespace(namespace)
	u.SetUID(types.UID("test-uid-12345"))
	u.SetLabels(map[string]string{
		"installer.name":      mchName,
		"installer.namespace": mchNamespace,
	})
	return u
}

// Test_cleanupAppSubscriptions_SetsProgressingCondition verifies that when
// an MCH-owned app subscription still needs to be terminated, a
// Progressing/HelmReleaseTerminatingReason condition is recorded and the
// subscription is deleted.
func Test_cleanupAppSubscriptions_SetsProgressingCondition(t *testing.T) {
	mch := resources.EmptyMCH()
	mch.Name = "test-mch-appsub-condition"

	appSub := mchLabeledAppSubscription("test-appsub", mch.GetNamespace(), mch.GetName(), mch.GetNamespace())
	r := newTestReconciler(appSub)

	err := r.cleanupAppSubscriptions(context.TODO(), log, &mch)
	if err == nil {
		t.Fatal("expected error while waiting for helmreleases to terminate, got nil")
	}

	condition := GetHubCondition(mch.Status, operatorv1.Progressing)
	if condition == nil {
		t.Fatal("expected Progressing condition to be set")
	}
	if condition.Reason != HelmReleaseTerminatingReason {
		t.Errorf("expected reason %q, got %q", HelmReleaseTerminatingReason, condition.Reason)
	}

	// The app subscription should have been deleted as part of cleanup.
	appSubList := &unstructured.UnstructuredList{}
	appSubList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps.open-cluster-management.io",
		Kind:    "SubscriptionList",
		Version: "v1",
	})
	if err := r.Client.List(context.TODO(), appSubList, client.MatchingLabels{
		"installer.name":      mch.GetName(),
		"installer.namespace": mch.GetNamespace(),
	}); err != nil {
		t.Fatalf("failed to list app subscriptions: %v", err)
	}
	if len(appSubList.Items) != 0 {
		t.Errorf("expected app subscription to be deleted, found %d remaining", len(appSubList.Items))
	}
}

// Test_cleanupAppSubscriptions_NoCondition_WhenNoneOwned verifies that no
// error or condition is set when there are no MCH-owned app subscriptions or
// helm releases left to terminate.
func Test_cleanupAppSubscriptions_NoCondition_WhenNoneOwned(t *testing.T) {
	r := newTestReconciler()
	mch := resources.EmptyMCH()
	mch.Name = "test-mch-appsub-none"

	if err := r.cleanupAppSubscriptions(context.TODO(), log, &mch); err != nil {
		t.Fatalf("expected no error when no app subscriptions exist, got: %v", err)
	}

	if condition := GetHubCondition(mch.Status, operatorv1.Progressing); condition != nil {
		t.Errorf("expected no Progressing condition, got: %+v", condition)
	}
}

// Test_finalizeHub_SetsTerminatingCondition_ComponentName verifies that when
// a component's teardown (via ensureNoComponent/InternalHubComponent) needs a
// requeue, finalizeHub records a Terminating condition naming the specific
// component still being torn down, instead of only logging it. This is the
// first phase of finalization (looping over operatorv1.MCHComponents) which
// runs before the more granular per-resource cleanup steps
// (cleanupNamespaces, cleanupMultiClusterEngine, etc.) even start.
func Test_finalizeHub_SetsTerminatingCondition_ComponentName(t *testing.T) {
	registerScheme()

	mch := resources.EmptyMCH()
	mch.Name = "test-mch-finalize-component"

	// operatorv1.Appsub ("app-lifecycle") is the first entry in MCHComponents.
	// Give its InternalHubComponent a finalizer so Delete() only marks it for
	// deletion (fake client behavior), forcing ensureNoInternalHubComponent to
	// report "still terminating" and requeue.
	ihc := &operatorv1.InternalHubComponent{
		ObjectMeta: metav1.ObjectMeta{
			Name:       operatorv1.Appsub,
			Namespace:  mch.GetNamespace(),
			Finalizers: []string{"test.io/block-deletion"},
		},
	}

	r := newTestReconciler(ihc)

	err := r.finalizeHub(context.TODO(), log, &mch, false, false)
	if err == nil {
		t.Fatal("expected error while component is still terminating, got nil")
	}

	condition := GetHubCondition(mch.Status, operatorv1.Terminating)
	if condition == nil {
		t.Fatal("expected Terminating condition to be set")
	}
	if condition.Reason != DeleteTimestampReason {
		t.Errorf("expected reason %q, got %q", DeleteTimestampReason, condition.Reason)
	}
	if !strings.Contains(condition.Message, operatorv1.Appsub) {
		t.Errorf("expected message to mention component %q, got %q", operatorv1.Appsub, condition.Message)
	}
}

// Test_finalizeHub_SetsTerminatingCondition_ComponentError verifies that when
// a component's teardown fails with a genuine error (not just "still
// terminating, needs requeue"), finalizeHub also records a Terminating
// condition naming the component and the underlying error, matching its
// sibling "needs requeue" branch instead of leaving this path with no
// condition at all.
func Test_finalizeHub_SetsTerminatingCondition_ComponentError(t *testing.T) {
	registerScheme()

	mch := resources.EmptyMCH()
	mch.Name = "test-mch-finalize-component-error"

	r := newTestReconcilerWithInterceptor(interceptor.Funcs{
		Get: func(ctx context.Context, c client.WithWatch, key client.ObjectKey, obj client.Object,
			opts ...client.GetOption) error {
			if _, ok := obj.(*operatorv1.InternalHubComponent); ok && key.Name == operatorv1.Appsub {
				return apierrors.NewInternalError(fmt.Errorf("simulated get failure"))
			}
			return c.Get(ctx, key, obj, opts...)
		},
	})

	err := r.finalizeHub(context.TODO(), log, &mch, false, false)
	if err == nil {
		t.Fatal("expected error while component removal fails, got nil")
	}

	condition := GetHubCondition(mch.Status, operatorv1.Terminating)
	if condition == nil {
		t.Fatal("expected Terminating condition to be set")
	}
	if condition.Reason != DeleteTimestampReason {
		t.Errorf("expected reason %q, got %q", DeleteTimestampReason, condition.Reason)
	}
	if !strings.Contains(condition.Message, operatorv1.Appsub) {
		t.Errorf("expected message to mention component %q, got %q", operatorv1.Appsub, condition.Message)
	}
	if !strings.Contains(condition.Message, "simulated get failure") {
		t.Errorf("expected message to include the underlying error detail, got %q", condition.Message)
	}
}
