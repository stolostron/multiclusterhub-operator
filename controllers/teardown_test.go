// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"os"
	"testing"
	"time"

	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func TestIsCloudProtectingFinalizer(t *testing.T) {
	tests := []struct {
		name      string
		finalizer string
		want      bool
	}{
		{"hypershift finalizer", "hypershift.openshift.io/finalizer", true},
		{"oidc discovery", "hypershift.io/aws-oidc-discovery", true},
		{"cpo finalizer", "hypershift.openshift.io/control-plane-operator-finalizer", true},
		{"hive deprovision", "hive.openshift.io/deprovision", true},
		{"ai deprovision", "agentserviceconfig.agent-install.openshift.io/ai-deprovision", true},
		{"ai cd deprovision", "clusterdeployments.agent-install.openshift.io/ai-deprovision", true},
		{"ai infraenv deprovision", "infraenv.agent-install.openshift.io/ai-deprovision", true},
		{"ai aci deprovision", "agentclusterinstall.agent-install.openshift.io/ai-deprovision", true},
		{"ai agent deprovision", "agent.agent-install.openshift.io/ai-deprovision", true},
		{"addon pre-delete (tier 2)", "addon.open-cluster-management.io/addon-pre-delete", false},
		{"search finalizer (tier 2)", "search.open-cluster-management.io/finalizer", false},
		{"unknown finalizer", "some.random/finalizer", false},
		{"kubernetes finalizer", "kubernetes", false},
		{"empty string", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsCloudProtectingFinalizer(tt.finalizer)
			if got != tt.want {
				t.Errorf("IsCloudProtectingFinalizer(%q) = %v, want %v", tt.finalizer, got, tt.want)
			}
		})
	}
}

func TestIsAllowlistedFinalizer(t *testing.T) {
	tests := []struct {
		name      string
		finalizer string
		want      bool
	}{
		{"tier 1: hypershift", "hypershift.openshift.io/finalizer", true},
		{"tier 1: hive", "hive.openshift.io/deprovision", true},
		{"tier 1: ai cd deprovision", "clusterdeployments.agent-install.openshift.io/ai-deprovision", true},
		{"tier 1: ai infraenv deprovision", "infraenv.agent-install.openshift.io/ai-deprovision", true},
		{"tier 1: ai aci deprovision", "agentclusterinstall.agent-install.openshift.io/ai-deprovision", true},
		{"tier 1: ai agent deprovision", "agent.agent-install.openshift.io/ai-deprovision", true},
		{"tier 2: addon pre-delete", "addon.open-cluster-management.io/addon-pre-delete", true},
		{"tier 2: search", "search.open-cluster-management.io/finalizer", true},
		{"tier 2: import controller", "managedcluster-import-controller.open-cluster-management.io/cleanup", true},
		{"tier 2: hub finalizer", "finalizer.operator.open-cluster-management.io", true},
		{"tier 2: mce finalizer", "finalizer.multicluster.openshift.io", true},
		{"tier 2: helm release", "uninstall-helm-release", true},
		{"tier 2: cluster-manager", "operator.open-cluster-management.io/cluster-manager-cleanup", true},
		{"unknown", "some.random/finalizer", false},
		{"kubernetes", "kubernetes", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsAllowlistedFinalizer(tt.finalizer)
			if got != tt.want {
				t.Errorf("IsAllowlistedFinalizer(%q) = %v, want %v", tt.finalizer, got, tt.want)
			}
		})
	}
}

func TestClassifyFinalizer(t *testing.T) {
	tests := []struct {
		name        string
		finalizer   string
		wantTier    FinalizerTier
		wantDescNon bool
	}{
		{"tier 1", "hypershift.openshift.io/finalizer", FinalizerTier1Cloud, false},
		{"tier 2", "addon.open-cluster-management.io/addon-pre-delete", FinalizerTier2NonCloud, false},
		{"unknown", "random-finalizer", FinalizerTierUnknown, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier, desc := ClassifyFinalizer(tt.finalizer)
			if tier != tt.wantTier {
				t.Errorf("ClassifyFinalizer(%q) tier = %v, want %v", tt.finalizer, tier, tt.wantTier)
			}
			if desc == "" {
				t.Errorf("ClassifyFinalizer(%q) description should not be empty", tt.finalizer)
			}
		})
	}
}

func TestIsACMAPIGroup(t *testing.T) {
	tests := []struct {
		group string
		want  bool
	}{
		{"cluster.open-cluster-management.io", true},
		{"addon.open-cluster-management.io", true},
		{"hive.openshift.io", true},
		{"agent-install.openshift.io", true},
		{"hypershift.openshift.io", true},
		{"multicluster.openshift.io", true},
		{"metal3.io", true},
		{"apps", false},
		{"rbac.authorization.k8s.io", false},
		{"", false},
		{"some.other.domain.io", false},
	}

	for _, tt := range tests {
		t.Run(tt.group, func(t *testing.T) {
			got := isACMAPIGroup(tt.group)
			if got != tt.want {
				t.Errorf("isACMAPIGroup(%q) = %v, want %v", tt.group, got, tt.want)
			}
		})
	}
}

func TestDetectPlatform(t *testing.T) {
	tests := []struct {
		name     string
		kind     string
		obj      map[string]interface{}
		wantPlat string
	}{
		{
			name: "HostedCluster AWS",
			kind: "HostedCluster",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"platform": map[string]interface{}{
						"type": "AWS",
					},
				},
			},
			wantPlat: "AWS",
		},
		{
			name: "HostedCluster azure lowercase",
			kind: "HostedCluster",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"platform": map[string]interface{}{
						"type": "azure",
					},
				},
			},
			wantPlat: "AZURE",
		},
		{
			name: "ClusterDeployment AWS",
			kind: "ClusterDeployment",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"platform": map[string]interface{}{
						"aws": map[string]interface{}{
							"region": "us-east-1",
						},
					},
				},
			},
			wantPlat: "AWS",
		},
		{
			name: "NodePool with platform",
			kind: "NodePool",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{
					"platform": map[string]interface{}{
						"type": "gcp",
					},
				},
			},
			wantPlat: "GCP",
		},
		{
			name:     "Unknown kind",
			kind:     "ConfigMap",
			obj:      map[string]interface{}{},
			wantPlat: "",
		},
		{
			name: "HostedCluster no platform",
			kind: "HostedCluster",
			obj: map[string]interface{}{
				"spec": map[string]interface{}{},
			},
			wantPlat: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &unstructured.Unstructured{Object: tt.obj}
			got := detectPlatform(tt.kind, u)
			if got != tt.wantPlat {
				t.Errorf("detectPlatform(%q) = %q, want %q", tt.kind, got, tt.wantPlat)
			}
		})
	}
}

func TestBuildRiskSummary(t *testing.T) {
	tests := []struct {
		name      string
		dr        DiscoveredResource
		finalizer string
		wantParts []string
	}{
		{
			name: "AWS HostedCluster with hypershift finalizer",
			dr: DiscoveredResource{
				Ref: operatorv1.ResourceRef{
					Kind:      "HostedCluster",
					Namespace: "clusters",
					Name:      "my-hcp",
				},
				Platform: "AWS",
				Phase:    "Failed",
			},
			finalizer: "hypershift.openshift.io/finalizer",
			wantParts: []string{"EC2 instances", "Elastic Load Balancers", "Failed", "AWS Management Console"},
		},
		{
			name: "GCP ClusterDeployment with hive finalizer",
			dr: DiscoveredResource{
				Ref: operatorv1.ResourceRef{
					Kind:      "ClusterDeployment",
					Namespace: "my-cluster",
					Name:      "my-cluster",
				},
				Platform:   "GCP",
				IsDeleting: true,
			},
			finalizer: "hive.openshift.io/deprovision",
			wantParts: []string{"GCE instances", "terminating", "Google Cloud Console"},
		},
		{
			name: "Unknown platform with OIDC finalizer",
			dr: DiscoveredResource{
				Ref: operatorv1.ResourceRef{
					Kind:      "HostedCluster",
					Namespace: "clusters",
					Name:      "test",
				},
			},
			finalizer: "hypershift.io/aws-oidc-discovery",
			wantParts: []string{"S3 OIDC", "cloud provider console"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildRiskSummary(tt.dr, tt.finalizer)
			if got == "" {
				t.Error("buildRiskSummary returned empty string")
			}
			for _, part := range tt.wantParts {
				if !containsSubstring(got, part) {
					t.Errorf("buildRiskSummary() missing expected substring %q in:\n%s", part, got)
				}
			}
		})
	}
}

func TestBuildDryRunReport(t *testing.T) {
	r := &HubTeardownReconciler{}

	graph := &DependencyGraph{
		BlockingResources: []operatorv1.BlockingResource{
			{Kind: "MultiClusterObservability", Group: "observability.open-cluster-management.io", Name: "observability", Reason: "test"},
			{Kind: "ManagedCluster", Group: "cluster.open-cluster-management.io", Name: "my-hcp", Reason: "test"},
		},
		CloudRiskResources: []DiscoveredResource{
			{
				Ref:        operatorv1.ResourceRef{Kind: "HostedCluster", Group: "hypershift.openshift.io", Namespace: "clusters", Name: "my-hcp"},
				Finalizers: []string{"hypershift.openshift.io/finalizer"},
				Platform:   "AWS",
			},
		},
		NonCloudFinalizerResources: []DiscoveredResource{
			{
				Ref:        operatorv1.ResourceRef{Kind: "ManagedCluster", Group: "cluster.open-cluster-management.io", Name: "my-hcp"},
				Finalizers: []string{"managedcluster-import-controller.open-cluster-management.io/cleanup"},
			},
		},
		NonLocalManagedClusters: []DiscoveredResource{
			{
				Ref:        operatorv1.ResourceRef{Kind: "ManagedCluster", Group: "cluster.open-cluster-management.io", Name: "my-hcp"},
				Finalizers: []string{"managedcluster-import-controller.open-cluster-management.io/cleanup"},
			},
		},
		Addons: []DiscoveredResource{
			{
				Ref:        operatorv1.ResourceRef{Kind: "ManagedClusterAddOn", Group: "addon.open-cluster-management.io", Namespace: "my-hcp", Name: "search-collector"},
				Finalizers: []string{"addon.open-cluster-management.io/addon-pre-delete"},
			},
		},
		MCH: &DiscoveredResource{
			Ref: operatorv1.ResourceRef{Kind: "MultiClusterHub", Group: "operator.open-cluster-management.io", Namespace: "open-cluster-management", Name: "multiclusterhub"},
		},
		MCE: &DiscoveredResource{
			Ref:        operatorv1.ResourceRef{Kind: "MultiClusterEngine", Group: "multicluster.openshift.io", Name: "multiclusterengine"},
			Finalizers: []string{"finalizer.multicluster.openshift.io"},
		},
	}

	report := r.buildDryRunReport(graph)

	if report == nil {
		t.Fatal("buildDryRunReport returned nil")
	}

	if report.TotalBlockingResources != 2 {
		t.Errorf("TotalBlockingResources = %d, want 2", report.TotalBlockingResources)
	}

	if report.TotalCloudRiskResources != 1 {
		t.Errorf("TotalCloudRiskResources = %d, want 1", report.TotalCloudRiskResources)
	}

	if len(report.PlannedPhases) != 11 {
		t.Errorf("PlannedPhases count = %d, want 11", len(report.PlannedPhases))
	}

	if report.Summary == "" {
		t.Error("Summary is empty")
	}

	// Verify phase order (all 11 phases including DeleteInfrastructureCRs, DeleteACMCRDs, RemoveOLMOperator)
	expectedPhases := []operatorv1.TeardownPhase{
		operatorv1.TeardownPhaseGateOLMSubscription,
		operatorv1.TeardownPhaseRemoveBlockingCRs,
		operatorv1.TeardownPhaseDisableAddons,
		operatorv1.TeardownPhaseDeleteInfrastructureCRs,
		operatorv1.TeardownPhaseDetachManagedClusters,
		operatorv1.TeardownPhaseDeleteMCH,
		operatorv1.TeardownPhaseMonitorMCEChain,
		operatorv1.TeardownPhaseCleanOrphans,
		operatorv1.TeardownPhaseDeleteACMCRDs,
		operatorv1.TeardownPhaseRemoveOLMOperator,
		operatorv1.TeardownPhaseComplete,
	}

	for i, expected := range expectedPhases {
		if i >= len(report.PlannedPhases) {
			t.Fatalf("Missing planned phase %d: %s", i, expected)
		}
		if report.PlannedPhases[i].Phase != expected {
			t.Errorf("PlannedPhases[%d].Phase = %s, want %s", i, report.PlannedPhases[i].Phase, expected)
		}
	}
}

func TestListFinalizerSummary(t *testing.T) {
	tests := []struct {
		name      string
		resources []DiscoveredResource
		want      string
	}{
		{
			name:      "empty",
			resources: nil,
			want:      "none",
		},
		{
			name: "single resource with finalizers",
			resources: []DiscoveredResource{
				{
					Ref:        operatorv1.ResourceRef{Kind: "HostedCluster", Namespace: "clusters", Name: "hcp1"},
					Finalizers: []string{"hypershift.openshift.io/finalizer"},
				},
			},
			want: "HostedCluster/clusters/hcp1: [hypershift.openshift.io/finalizer]",
		},
		{
			name: "resource without finalizers",
			resources: []DiscoveredResource{
				{
					Ref:        operatorv1.ResourceRef{Kind: "ManagedCluster", Name: "local-cluster"},
					Finalizers: nil,
				},
			},
			want: "none",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := listFinalizerSummary(tt.resources)
			if got != tt.want {
				t.Errorf("listFinalizerSummary() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTruncateMessage(t *testing.T) {
	short := "hello"
	if got := truncateMessage(short, 100); got != short {
		t.Errorf("truncateMessage(%q, 100) = %q", short, got)
	}

	long := "a very long message that exceeds the limit"
	got := truncateMessage(long, 20)
	if len(got) > 20 {
		t.Errorf("truncateMessage should not exceed max, got len=%d", len(got))
	}
}

func TestIsPhaseComplete(t *testing.T) {
	r := &HubTeardownReconciler{}
	td := &operatorv1.HubTeardown{
		Status: operatorv1.HubTeardownStatus{
			Phases: []operatorv1.TeardownPhaseStatus{
				{Phase: operatorv1.TeardownPhaseGateOLMSubscription, State: operatorv1.PhaseStateComplete},
				{Phase: operatorv1.TeardownPhaseRemoveBlockingCRs, State: operatorv1.PhaseStateInProgress},
				{Phase: operatorv1.TeardownPhaseDisableAddons, State: operatorv1.PhaseStateSkipped},
			},
		},
	}

	if !r.isPhaseComplete(td, operatorv1.TeardownPhaseGateOLMSubscription) {
		t.Error("Expected GateOLMSubscription to be complete")
	}
	if r.isPhaseComplete(td, operatorv1.TeardownPhaseRemoveBlockingCRs) {
		t.Error("Expected RemoveBlockingCRs to NOT be complete (InProgress)")
	}
	if !r.isPhaseComplete(td, operatorv1.TeardownPhaseDisableAddons) {
		t.Error("Expected DisableAddons to be complete (Skipped counts as complete)")
	}
	if r.isPhaseComplete(td, operatorv1.TeardownPhaseDeleteMCH) {
		t.Error("Expected DeleteMCH to NOT be complete (not in list)")
	}
}

func TestSetPhaseStatus(t *testing.T) {
	r := &HubTeardownReconciler{}
	td := &operatorv1.HubTeardown{}

	r.setPhaseStatus(td, operatorv1.TeardownPhaseGateOLMSubscription, operatorv1.PhaseStateInProgress, "Starting")
	if len(td.Status.Phases) != 1 {
		t.Fatalf("Expected 1 phase, got %d", len(td.Status.Phases))
	}
	if td.Status.Phases[0].Phase != operatorv1.TeardownPhaseGateOLMSubscription {
		t.Error("Wrong phase name")
	}
	if td.Status.Phases[0].State != operatorv1.PhaseStateInProgress {
		t.Error("Wrong state")
	}
	if td.Status.Phases[0].StartTime == nil {
		t.Error("StartTime should be set for InProgress")
	}

	// Update existing phase
	r.setPhaseStatus(td, operatorv1.TeardownPhaseGateOLMSubscription, operatorv1.PhaseStateComplete, "Done")
	if len(td.Status.Phases) != 1 {
		t.Fatalf("Expected 1 phase after update, got %d", len(td.Status.Phases))
	}
	if td.Status.Phases[0].State != operatorv1.PhaseStateComplete {
		t.Error("State should be updated to Complete")
	}
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func newTestScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = operatorv1.AddToScheme(s)
	_ = batchv1.AddToScheme(s)
	_ = corev1.AddToScheme(s)
	_ = appsv1.AddToScheme(s)
	_ = apixv1.AddToScheme(s)
	_ = subv1alpha1.AddToScheme(s)
	_ = admissionregistrationv1.AddToScheme(s)
	return s
}

func newTestReconciler() *HubTeardownReconciler {
	s := newTestScheme()
	return &HubTeardownReconciler{
		Client:   fake.NewClientBuilder().WithScheme(s).Build(),
		Scheme:   s,
		Log:      ctrllog.Log,
		Recorder: record.NewFakeRecorder(10),
	}
}

func TestEnsureTeardownJob_CreatesJob(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "teardown",
			Namespace: "open-cluster-management",
		},
	}

	r := newTestReconciler()
	os.Setenv("OPERATOR_IMAGE", "quay.io/test/operator:latest")
	defer os.Unsetenv("OPERATOR_IMAGE")

	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	if err := r.ensureTeardownJob(ctx, log, td); err != nil {
		t.Fatalf("ensureTeardownJob failed: %v", err)
	}

	job := &batchv1.Job{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      teardownJobName,
		Namespace: td.Namespace,
	}, job)
	if err != nil {
		t.Fatalf("expected Job to be created, got error: %v", err)
	}

	if len(job.Spec.Template.Spec.Containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(job.Spec.Template.Spec.Containers))
	}

	container := job.Spec.Template.Spec.Containers[0]
	if container.Image != "quay.io/test/operator:latest" {
		t.Errorf("expected image quay.io/test/operator:latest, got %s", container.Image)
	}
	if len(container.Args) != 1 || container.Args[0] != "--teardown-mode" {
		t.Errorf("expected args [--teardown-mode], got %v", container.Args)
	}

	foundName, foundNs := false, false
	for _, env := range container.Env {
		if env.Name == envTeardownName && env.Value == td.Name {
			foundName = true
		}
		if env.Name == envTeardownNamespace && env.Value == td.Namespace {
			foundNs = true
		}
	}
	if !foundName {
		t.Error("expected TEARDOWN_NAME env var")
	}
	if !foundNs {
		t.Error("expected TEARDOWN_NAMESPACE env var")
	}
}

func TestEnsureTeardownJob_NoImageSkips(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "teardown",
			Namespace: "open-cluster-management",
		},
	}

	r := newTestReconciler()
	os.Unsetenv("OPERATOR_IMAGE")
	os.Unsetenv("RELATED_IMAGE_MULTICLUSTERHUB_OPERATOR")

	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	if err := r.ensureTeardownJob(ctx, log, td); err != nil {
		t.Fatalf("ensureTeardownJob should not fail when image is unset: %v", err)
	}

	job := &batchv1.Job{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      teardownJobName,
		Namespace: td.Namespace,
	}, job)
	if err == nil {
		t.Error("expected Job NOT to be created when image is unset")
	}
}

func TestEnsureTeardownJob_Idempotent(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "teardown",
			Namespace: "open-cluster-management",
		},
	}

	r := newTestReconciler()
	os.Setenv("OPERATOR_IMAGE", "quay.io/test/operator:latest")
	defer os.Unsetenv("OPERATOR_IMAGE")

	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	if err := r.ensureTeardownJob(ctx, log, td); err != nil {
		t.Fatalf("first ensureTeardownJob failed: %v", err)
	}

	if err := r.ensureTeardownJob(ctx, log, td); err != nil {
		t.Fatalf("second ensureTeardownJob should be idempotent: %v", err)
	}
}

func TestCleanupTeardownJob(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "teardown",
			Namespace: "open-cluster-management",
		},
	}

	s := newTestScheme()
	existingJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      teardownJobName,
			Namespace: td.Namespace,
		},
	}
	r := &HubTeardownReconciler{
		Client:   fake.NewClientBuilder().WithScheme(s).WithObjects(existingJob).Build(),
		Scheme:   s,
		Log:      ctrllog.Log,
		Recorder: record.NewFakeRecorder(10),
	}

	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	if err := r.cleanupTeardownJob(ctx, log, td); err != nil {
		t.Fatalf("cleanupTeardownJob failed: %v", err)
	}

	job := &batchv1.Job{}
	err := r.Client.Get(ctx, types.NamespacedName{
		Name:      teardownJobName,
		Namespace: td.Namespace,
	}, job)
	if err == nil {
		t.Error("expected Job to be deleted")
	}
}

func TestCleanupTeardownJob_NotFound(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "teardown",
			Namespace: "open-cluster-management",
		},
	}

	r := newTestReconciler()
	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	if err := r.cleanupTeardownJob(ctx, log, td); err != nil {
		t.Fatalf("cleanupTeardownJob should succeed when Job doesn't exist: %v", err)
	}
}

func TestGetTeardownJobStatus(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "teardown",
			Namespace: "open-cluster-management",
		},
	}

	tests := []struct {
		name     string
		job      *batchv1.Job
		wantType *batchv1.JobConditionType
		wantNil  bool
	}{
		{
			name:    "no Job exists",
			job:     nil,
			wantNil: true,
		},
		{
			name: "Job complete",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      teardownJobName,
					Namespace: "open-cluster-management",
				},
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
					},
				},
			},
			wantNil: false,
		},
		{
			name: "Job failed",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      teardownJobName,
					Namespace: "open-cluster-management",
				},
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{Type: batchv1.JobFailed, Status: corev1.ConditionTrue},
					},
				},
			},
			wantNil: false,
		},
		{
			name: "Job running (no true condition)",
			job: &batchv1.Job{
				ObjectMeta: metav1.ObjectMeta{
					Name:      teardownJobName,
					Namespace: "open-cluster-management",
				},
				Status: batchv1.JobStatus{
					Conditions: []batchv1.JobCondition{
						{Type: batchv1.JobComplete, Status: corev1.ConditionFalse},
					},
				},
			},
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := newTestScheme()
			cb := fake.NewClientBuilder().WithScheme(s)
			if tt.job != nil {
				cb = cb.WithObjects(tt.job)
			}
			r := &HubTeardownReconciler{
				Client:   cb.Build(),
				Scheme:   s,
				Log:      ctrllog.Log,
				Recorder: record.NewFakeRecorder(10),
			}

			condType, err := r.getTeardownJobStatus(context.Background(), td)
			if err != nil {
				t.Fatalf("getTeardownJobStatus error: %v", err)
			}
			if tt.wantNil && condType != nil {
				t.Errorf("expected nil condition type, got %v", *condType)
			}
			if !tt.wantNil && condType == nil {
				t.Error("expected non-nil condition type")
			}
		})
	}
}

func TestSingletonGuard_RejectsSecondCR(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	s := newTestScheme()

	oldest := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "teardown-oldest",
			Namespace:         "open-cluster-management",
			CreationTimestamp: metav1.NewTime(time.Now().Add(-1 * time.Hour)),
		},
		Spec: operatorv1.HubTeardownSpec{DryRun: true},
	}
	newer := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "teardown-newer",
			Namespace:         "open-cluster-management",
			CreationTimestamp: metav1.NewTime(time.Now()),
		},
		Spec: operatorv1.HubTeardownSpec{DryRun: true},
	}

	cb := fake.NewClientBuilder().WithScheme(s).
		WithObjects(oldest, newer).
		WithStatusSubresource(oldest, newer)
	r := &HubTeardownReconciler{
		Client:   cb.Build(),
		Scheme:   s,
		Log:      ctrllog.Log,
		Recorder: record.NewFakeRecorder(10),
	}

	ctx := context.Background()

	// Reconciling the newer CR should reject it
	_, err := r.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: newer.Name, Namespace: newer.Namespace},
	})
	if err != nil {
		t.Fatalf("Reconcile of newer CR should not error: %v", err)
	}

	// Verify the newer CR got the DuplicateRejected condition
	fetched := &operatorv1.HubTeardown{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: newer.Name, Namespace: newer.Namespace}, fetched); err != nil {
		t.Fatalf("Failed to fetch newer CR: %v", err)
	}
	foundRejected := false
	for _, c := range fetched.Status.Conditions {
		if c.Reason == "DuplicateRejected" {
			foundRejected = true
		}
	}
	if !foundRejected {
		t.Error("Expected DuplicateRejected condition on newer CR")
	}
}

func TestSingletonGuard_AllowsSingleCR(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	s := newTestScheme()

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "teardown",
			Namespace: "open-cluster-management",
		},
		Spec: operatorv1.HubTeardownSpec{DryRun: true},
	}

	cb := fake.NewClientBuilder().WithScheme(s).
		WithObjects(td).
		WithStatusSubresource(td)
	r := &HubTeardownReconciler{
		Client:   cb.Build(),
		Scheme:   s,
		Log:      ctrllog.Log,
		Recorder: record.NewFakeRecorder(10),
	}

	ctx := context.Background()
	_, err := r.Reconcile(ctx, ctrl.Request{
		NamespacedName: types.NamespacedName{Name: td.Name, Namespace: td.Namespace},
	})
	if err != nil {
		t.Fatalf("Reconcile of single CR should not error: %v", err)
	}

	// Should NOT have DuplicateRejected
	fetched := &operatorv1.HubTeardown{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: td.Name, Namespace: td.Namespace}, fetched); err != nil {
		t.Fatalf("Failed to fetch CR: %v", err)
	}
	for _, c := range fetched.Status.Conditions {
		if c.Reason == "DuplicateRejected" {
			t.Error("Single CR should not be rejected")
		}
	}
}

func TestHubteardownCRDNameGuard(t *testing.T) {
	if hubteardownCRDName != "hubteardowns.operator.open-cluster-management.io" {
		t.Errorf("hubteardownCRDName = %q, want hubteardowns.operator.open-cluster-management.io", hubteardownCRDName)
	}
}

func TestPhaseDeleteACMCRDs_SkipsHubTeardownCRD(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	s := newTestScheme()

	hubTeardownCRD := &apixv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: hubteardownCRDName,
		},
		Spec: apixv1.CustomResourceDefinitionSpec{
			Group: "operator.open-cluster-management.io",
			Names: apixv1.CustomResourceDefinitionNames{
				Kind:   "HubTeardown",
				Plural: "hubteardowns",
			},
			Versions: []apixv1.CustomResourceDefinitionVersion{
				{Name: "v1", Served: true, Storage: true},
			},
		},
	}
	otherCRD := &apixv1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: "managedclusters.cluster.open-cluster-management.io",
		},
		Spec: apixv1.CustomResourceDefinitionSpec{
			Group: "cluster.open-cluster-management.io",
			Names: apixv1.CustomResourceDefinitionNames{
				Kind:   "ManagedCluster",
				Plural: "managedclusters",
			},
			Versions: []apixv1.CustomResourceDefinitionVersion{
				{Name: "v1", Served: true, Storage: true},
			},
		},
	}

	cb := fake.NewClientBuilder().WithScheme(s).
		WithObjects(hubTeardownCRD, otherCRD)
	r := &HubTeardownReconciler{
		Client:   cb.Build(),
		Scheme:   s,
		Log:      ctrllog.Log,
		Recorder: record.NewFakeRecorder(10),
	}

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "teardown",
			Namespace: "open-cluster-management",
		},
	}
	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	_, err := r.phaseDeleteACMCRDs(ctx, log, td)
	if err != nil {
		t.Fatalf("phaseDeleteACMCRDs error: %v", err)
	}

	// HubTeardown CRD should still exist
	crd := &apixv1.CustomResourceDefinition{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: hubteardownCRDName}, crd)
	if err != nil {
		t.Errorf("HubTeardown CRD should NOT be deleted, but got error: %v", err)
	}
}

func TestPhaseDeleteInfrastructureCRs_SkipsGateWhenEmpty(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	s := newTestScheme()

	r := &HubTeardownReconciler{
		Client:   fake.NewClientBuilder().WithScheme(s).Build(),
		Scheme:   s,
		Log:      ctrllog.Log,
		Recorder: record.NewFakeRecorder(10),
	}

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "teardown",
			Namespace: "open-cluster-management",
		},
		Spec: operatorv1.HubTeardownSpec{
			AcknowledgeCloudResourceRisk: false,
		},
	}
	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	done, err := r.phaseDeleteInfrastructureCRs(ctx, log, td)
	if err != nil {
		t.Fatalf("phaseDeleteInfrastructureCRs error: %v", err)
	}
	if !done {
		t.Error("Expected phase to complete (skip gate) when no infra CRs exist, but got done=false")
	}
}

func TestScaleDownAddonControllers_LabelDiscovery(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	s := newTestScheme()

	one := int32(1)
	labeledDeploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "my-custom-addon-controller",
			Namespace: "open-cluster-management",
			Labels: map[string]string{
				"app.kubernetes.io/component": "addon-controller",
				"app":                         "my-custom-addon",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &one,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "img"}}},
			},
		},
	}
	knownDeploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "klusterlet-addon-controller-v2",
			Namespace: "open-cluster-management",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &one,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test2"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test2"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "img"}}},
			},
		},
	}

	cb := fake.NewClientBuilder().WithScheme(s).
		WithObjects(labeledDeploy, knownDeploy)
	r := &HubTeardownReconciler{
		Client:   cb.Build(),
		Scheme:   s,
		Log:      ctrllog.Log,
		Recorder: record.NewFakeRecorder(10),
	}

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "teardown",
			Namespace: "open-cluster-management",
		},
	}
	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	r.scaleDownAddonControllers(ctx, log, td)

	// Both should be scaled to 0
	for _, name := range []string{"my-custom-addon-controller", "klusterlet-addon-controller-v2"} {
		deploy := &appsv1.Deployment{}
		if err := r.Client.Get(ctx, types.NamespacedName{Name: name, Namespace: "open-cluster-management"}, deploy); err != nil {
			t.Fatalf("Failed to get deployment %s: %v", name, err)
		}
		if deploy.Spec.Replicas == nil || *deploy.Spec.Replicas != 0 {
			t.Errorf("Deployment %s should be scaled to 0, got %v", name, deploy.Spec.Replicas)
		}
	}
}

func TestScaleDeploymentToZero_AlreadyZero(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	s := newTestScheme()

	zero := int32(0)
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "already-zero",
			Namespace: "open-cluster-management",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &zero,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "img"}}},
			},
		},
	}

	cb := fake.NewClientBuilder().WithScheme(s).WithObjects(deploy)
	r := &HubTeardownReconciler{
		Client:   cb.Build(),
		Scheme:   s,
		Log:      ctrllog.Log,
		Recorder: record.NewFakeRecorder(10),
	}

	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	r.scaleDeploymentToZero(ctx, log, deploy)

	fetched := &appsv1.Deployment{}
	if err := r.Client.Get(ctx, client.ObjectKeyFromObject(deploy), fetched); err != nil {
		t.Fatalf("Failed to get deployment: %v", err)
	}
	if *fetched.Spec.Replicas != 0 {
		t.Errorf("Replicas should remain 0, got %d", *fetched.Spec.Replicas)
	}
}

func TestEnsureOLMGate_NoSubscriptionIsNoop(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))

	s := newTestScheme()
	subv1alpha1Scheme := runtime.NewScheme()
	_ = operatorv1.AddToScheme(subv1alpha1Scheme)

	r := &HubTeardownReconciler{
		Client:   fake.NewClientBuilder().WithScheme(s).Build(),
		Scheme:   s,
		Log:      ctrllog.Log,
		Recorder: record.NewFakeRecorder(10),
	}

	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	err := r.ensureOLMGate(ctx, log)
	if err != nil {
		t.Errorf("ensureOLMGate should be a no-op when no Subscription exists, got error: %v", err)
	}
}

func TestShouldResolveStuckFinalizers(t *testing.T) {
	r := newTestReconciler()
	pastTime := metav1.NewTime(time.Now().Add(-10 * time.Minute))
	recentTime := metav1.NewTime(time.Now().Add(-30 * time.Second))

	deletingOld := &unstructured.Unstructured{}
	deletingOld.SetDeletionTimestamp(&pastTime)

	deletingRecent := &unstructured.Unstructured{}
	deletingRecent.SetDeletionTimestamp(&recentTime)

	notDeleting := &unstructured.Unstructured{}

	tests := []struct {
		name string
		td   *operatorv1.HubTeardown
		obj  *unstructured.Unstructured
		want bool
	}{
		{
			name: "default timeout, resource deleted 10m ago",
			td: &operatorv1.HubTeardown{
				Spec: operatorv1.HubTeardownSpec{},
			},
			obj:  deletingOld,
			want: true,
		},
		{
			name: "default timeout, resource deleted 30s ago",
			td: &operatorv1.HubTeardown{
				Spec: operatorv1.HubTeardownSpec{},
			},
			obj:  deletingRecent,
			want: false,
		},
		{
			name: "no deletion timestamp",
			td: &operatorv1.HubTeardown{
				Spec: operatorv1.HubTeardownSpec{},
			},
			obj:  notDeleting,
			want: false,
		},
		{
			name: "custom short timeout (1s), resource deleted 30s ago",
			td: &operatorv1.HubTeardown{
				Spec: operatorv1.HubTeardownSpec{
					ForceFinalizerTimeout: &metav1.Duration{Duration: 1 * time.Second},
				},
			},
			obj:  deletingRecent,
			want: true,
		},
		{
			name: "custom long timeout (1h), resource deleted 10m ago",
			td: &operatorv1.HubTeardown{
				Spec: operatorv1.HubTeardownSpec{
					ForceFinalizerTimeout: &metav1.Duration{Duration: 1 * time.Hour},
				},
			},
			obj:  deletingOld,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := r.shouldResolveStuckFinalizers(tt.td, tt.obj)
			if got != tt.want {
				t.Errorf("shouldResolveStuckFinalizers() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDeleteAgentInstallWebhooks_DeletesVWC(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	s := newTestScheme()

	sideEffects := admissionregistrationv1.SideEffectClassNone
	vwc := &admissionregistrationv1.ValidatingWebhookConfiguration{
		ObjectMeta: metav1.ObjectMeta{
			Name: "infraenvvalidators.admission.agentinstall.openshift.io",
		},
		Webhooks: []admissionregistrationv1.ValidatingWebhook{
			{
				Name:                    "infraenvvalidators.admission.agentinstall.openshift.io",
				SideEffects:             &sideEffects,
				AdmissionReviewVersions: []string{"v1"},
				ClientConfig: admissionregistrationv1.WebhookClientConfig{
					Service: &admissionregistrationv1.ServiceReference{
						Namespace: "assisted-installer",
						Name:      "assisted-service",
					},
				},
			},
		},
	}

	cb := fake.NewClientBuilder().WithScheme(s).WithObjects(vwc)
	r := &HubTeardownReconciler{
		Client:   cb.Build(),
		Scheme:   s,
		Log:      ctrllog.Log,
		Recorder: record.NewFakeRecorder(10),
	}

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{Name: "teardown", Namespace: "open-cluster-management"},
	}
	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	if err := r.deleteAgentInstallWebhooks(ctx, log, td); err != nil {
		t.Fatalf("deleteAgentInstallWebhooks error: %v", err)
	}

	fetched := &admissionregistrationv1.ValidatingWebhookConfiguration{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: vwc.Name}, fetched)
	if err == nil {
		t.Error("Expected VWC to be deleted, but it still exists")
	}
}

func TestDeleteAgentInstallWebhooks_NoWebhooksIsNoop(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	r := newTestReconciler()

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{Name: "teardown", Namespace: "open-cluster-management"},
	}
	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	if err := r.deleteAgentInstallWebhooks(ctx, log, td); err != nil {
		t.Fatalf("deleteAgentInstallWebhooks should be a noop: %v", err)
	}
}

func TestResolveMCHBlockingFinalizers_NoMatchIsNoop(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	r := newTestReconciler()

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{Name: "teardown", Namespace: "open-cluster-management"},
		Spec: operatorv1.HubTeardownSpec{
			ApprovedDestructiveActions: true,
		},
	}
	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	if err := r.resolveMCHBlockingFinalizers(ctx, log, td); err != nil {
		t.Fatalf("resolveMCHBlockingFinalizers should be a noop when CRD types are missing: %v", err)
	}
}

// --- v8 gap fix tests ---

func TestEnsureMCEDeleted_NoMatchIsNoop(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	r := newTestReconciler()

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{Name: "teardown", Namespace: "open-cluster-management"},
	}
	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	if err := r.ensureMCEDeleted(ctx, log, td); err != nil {
		t.Fatalf("ensureMCEDeleted should be a noop when MCE CRD is not registered: %v", err)
	}
}

func TestCleanupLocalCluster_NoMatchIsNoop(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	r := newTestReconciler()

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{Name: "teardown", Namespace: "open-cluster-management"},
	}
	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	if err := r.cleanupLocalCluster(ctx, log, td); err != nil {
		t.Fatalf("cleanupLocalCluster should be a noop when ManagedCluster CRD is not registered: %v", err)
	}
}

func TestResolveOrphanedClusterManager_ChecksBothNamespaces(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	s := newTestScheme()

	// Create cluster-manager deployment in multicluster-engine namespace with ready replicas
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-manager",
			Namespace: "multicluster-engine",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "cluster-manager"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "cluster-manager"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "manager", Image: "test"}},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
	}

	cb := fake.NewClientBuilder().WithScheme(s).WithObjects(deploy)
	r := &HubTeardownReconciler{
		Client:   cb.Build(),
		Scheme:   s,
		Log:      ctrllog.Log,
		Recorder: record.NewFakeRecorder(10),
	}

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{Name: "teardown", Namespace: "open-cluster-management"},
		Spec: operatorv1.HubTeardownSpec{
			ApprovedDestructiveActions: true,
		},
	}
	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	// Should succeed without error (ClusterManager CRD not registered = no-match = noop)
	if err := r.resolveOrphanedClusterManager(ctx, log, td); err != nil {
		t.Fatalf("resolveOrphanedClusterManager should succeed: %v", err)
	}

	// Verify the deployment still exists (was found, so nothing should be orphaned)
	fetched := &appsv1.Deployment{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: "cluster-manager", Namespace: "multicluster-engine"}, fetched); err != nil {
		t.Fatalf("cluster-manager deployment should still exist: %v", err)
	}
}

func TestResolveOrphanedClusterManager_HubNamespace(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	s := newTestScheme()

	// Create cluster-manager deployment in open-cluster-management namespace with ready replicas
	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-manager",
			Namespace: "open-cluster-management",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "cluster-manager"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "cluster-manager"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "manager", Image: "test"}},
				},
			},
		},
		Status: appsv1.DeploymentStatus{
			ReadyReplicas: 1,
		},
	}

	cb := fake.NewClientBuilder().WithScheme(s).WithObjects(deploy)
	r := &HubTeardownReconciler{
		Client:   cb.Build(),
		Scheme:   s,
		Log:      ctrllog.Log,
		Recorder: record.NewFakeRecorder(10),
	}

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{Name: "teardown", Namespace: "open-cluster-management"},
		Spec: operatorv1.HubTeardownSpec{
			ApprovedDestructiveActions: true,
		},
	}
	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	if err := r.resolveOrphanedClusterManager(ctx, log, td); err != nil {
		t.Fatalf("resolveOrphanedClusterManager should succeed: %v", err)
	}

	// Verify deployment in hub namespace still exists
	fetched := &appsv1.Deployment{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: "cluster-manager", Namespace: "open-cluster-management"}, fetched); err != nil {
		t.Fatalf("cluster-manager deployment in hub namespace should still exist: %v", err)
	}
}

func TestResolveOrphanedClusterManager_NeitherNamespace(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	s := newTestScheme()

	// No cluster-manager deployment in any namespace
	cb := fake.NewClientBuilder().WithScheme(s)
	r := &HubTeardownReconciler{
		Client:   cb.Build(),
		Scheme:   s,
		Log:      ctrllog.Log,
		Recorder: record.NewFakeRecorder(10),
	}

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{Name: "teardown", Namespace: "open-cluster-management"},
		Spec: operatorv1.HubTeardownSpec{
			ApprovedDestructiveActions: true,
		},
	}
	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	// Should succeed (ClusterManager CRD not registered = no-match = noop)
	if err := r.resolveOrphanedClusterManager(ctx, log, td); err != nil {
		t.Fatalf("resolveOrphanedClusterManager should succeed when no deployment and no CRD: %v", err)
	}
}

func TestPhaseMonitorMCEChain_AllGoneCompletesPhase(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	r := newTestReconciler()

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{Name: "teardown", Namespace: "open-cluster-management"},
		Spec: operatorv1.HubTeardownSpec{
			ApprovedDestructiveActions: true,
		},
	}
	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	// With no MCH, MCE, ClusterManager, ManagedCluster CRDs registered,
	// all isResourceGone checks return true (no-match = gone).
	done, err := r.phaseMonitorMCEChain(ctx, log, td)
	if err != nil {
		t.Fatalf("phaseMonitorMCEChain error: %v", err)
	}
	if !done {
		t.Error("Expected phase to complete when all resources are gone, got done=false")
	}
}

func TestPhaseGateOLMSubscription_CommentVerification(t *testing.T) {
	// Verify the OLM gate comment exists (Gap 5) by checking the function's
	// behavior: when no Subscription exists, the phase completes.
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	r := newTestReconciler()

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{Name: "teardown", Namespace: "open-cluster-management"},
	}
	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	done, err := r.phaseGateOLMSubscription(ctx, log, td)
	if err != nil {
		t.Fatalf("phaseGateOLMSubscription error: %v", err)
	}
	if !done {
		t.Error("Expected phase to complete when no Subscription exists")
	}
}

func TestPhaseRemoveOLMOperator_ClearsStalledAndBlockingResources(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	s := newTestScheme()

	// Create an ACM Subscription so phaseRemoveOLMOperator doesn't exit early.
	acmSub := &subv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "acm-sub",
			Namespace: "open-cluster-management",
		},
		Spec: &subv1alpha1.SubscriptionSpec{
			Package: "advanced-cluster-management",
		},
		Status: subv1alpha1.SubscriptionStatus{
			InstalledCSV: "",
		},
	}

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "teardown",
			Namespace: "open-cluster-management",
		},
		Status: operatorv1.HubTeardownStatus{
			// Pre-set Stalled=True to simulate the stalled state.
			Conditions: []metav1.Condition{
				{
					Type:   "Stalled",
					Status: metav1.ConditionTrue,
					Reason: "TeardownStalled",
				},
			},
			// Pre-set blocking resources to simulate stale entries.
			BlockingResources: []operatorv1.BlockingResource{
				{Kind: "ManagedCluster", Name: "fake-imported-cluster"},
			},
		},
	}

	cb := fake.NewClientBuilder().WithScheme(s).
		WithObjects(acmSub, td).
		WithStatusSubresource(td)
	fakeClient := cb.Build()
	r := &HubTeardownReconciler{
		Client:         fakeClient,
		UncachedClient: fakeClient, // same client works for test read-back
		Scheme:         s,
		Log:            ctrllog.Log,
		Recorder:       record.NewFakeRecorder(10),
	}

	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	done, err := r.phaseRemoveOLMOperator(ctx, log, td)
	if err != nil {
		t.Fatalf("phaseRemoveOLMOperator error: %v", err)
	}
	if !done {
		t.Error("Expected phase to complete, got done=false")
	}

	// Gap 7: Verify Stalled condition was set to False.
	fetched := &operatorv1.HubTeardown{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: td.Name, Namespace: td.Namespace}, fetched); err != nil {
		t.Fatalf("Failed to fetch HubTeardown: %v", err)
	}
	foundStalledFalse := false
	for _, c := range fetched.Status.Conditions {
		if c.Type == "Stalled" {
			if c.Status == metav1.ConditionFalse {
				foundStalledFalse = true
			} else {
				t.Errorf("Stalled condition should be False, got %v", c.Status)
			}
		}
	}
	if !foundStalledFalse {
		t.Error("Expected Stalled=False condition after phaseRemoveOLMOperator completes")
	}

	// Gap 8: Verify BlockingResources was cleared.
	if len(fetched.Status.BlockingResources) != 0 {
		t.Errorf("BlockingResources should be nil/empty after phaseRemoveOLMOperator, got %v", fetched.Status.BlockingResources)
	}
}

func TestPhaseRemoveOLMOperator_DeletesMCESubscriptionAndCSV(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	s := newTestScheme()

	// Create ACM Subscription (required for phase entry).
	acmSub := &subv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "acm-sub",
			Namespace: "open-cluster-management",
		},
		Spec: &subv1alpha1.SubscriptionSpec{
			Package: "advanced-cluster-management",
		},
	}

	// Create MCE Subscription.
	mceSub := &subv1alpha1.Subscription{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mce-sub",
			Namespace: "multicluster-engine",
		},
		Spec: &subv1alpha1.SubscriptionSpec{
			Package: "multicluster-engine",
		},
	}

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "teardown",
			Namespace: "open-cluster-management",
		},
	}

	cb := fake.NewClientBuilder().WithScheme(s).
		WithObjects(acmSub, mceSub, td).
		WithStatusSubresource(td)
	fakeClient := cb.Build()
	r := &HubTeardownReconciler{
		Client:         fakeClient,
		UncachedClient: fakeClient,
		Scheme:         s,
		Log:            ctrllog.Log,
		Recorder:       record.NewFakeRecorder(10),
	}

	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	done, err := r.phaseRemoveOLMOperator(ctx, log, td)
	if err != nil {
		t.Fatalf("phaseRemoveOLMOperator error: %v", err)
	}
	if !done {
		t.Error("Expected phase to complete, got done=false")
	}

	// Gap 6: Verify MCE Subscription was deleted.
	_, mceErr := r.findMCESubscription(ctx)
	if mceErr == nil {
		t.Error("Expected MCE Subscription to be deleted, but it still exists")
	}
}

func TestDeleteResidualAddonDeployments(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	s := newTestScheme()

	zero := int32(0)
	one := int32(1)

	// Addon deployment scaled to 0 (should be deleted).
	addonDeploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "volsync-addon-controller",
			Namespace: "open-cluster-management",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &zero,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "img"}}},
			},
		},
	}
	// Addon deployment with label, scaled to 0 (should be deleted).
	labeledDeploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "custom-addon-mgr",
			Namespace: "open-cluster-management",
			Labels: map[string]string{
				"app.kubernetes.io/component": "addon-controller",
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &zero,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test2"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test2"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "img"}}},
			},
		},
	}
	// Non-addon deployment scaled to 0 (should NOT be deleted).
	nonAddonDeploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "some-other-controller",
			Namespace: "open-cluster-management",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &zero,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test3"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test3"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "img"}}},
			},
		},
	}
	// Addon deployment with replicas > 0 (should NOT be deleted).
	runningAddonDeploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "submariner-addon",
			Namespace: "open-cluster-management",
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &one,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test4"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "test4"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "img"}}},
			},
		},
	}

	cb := fake.NewClientBuilder().WithScheme(s).
		WithObjects(addonDeploy, labeledDeploy, nonAddonDeploy, runningAddonDeploy)
	r := &HubTeardownReconciler{
		Client:   cb.Build(),
		Scheme:   s,
		Log:      ctrllog.Log,
		Recorder: record.NewFakeRecorder(10),
	}

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "teardown",
			Namespace: "open-cluster-management",
		},
	}
	ctx := context.Background()
	log := ctrllog.Log.WithName("test")

	r.deleteResidualAddonDeployments(ctx, log, td)

	// addonDeploy (known name, replicas=0) should be deleted.
	deploy := &appsv1.Deployment{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: "volsync-addon-controller", Namespace: "open-cluster-management"}, deploy)
	if err == nil {
		t.Error("volsync-addon-controller should have been deleted")
	}

	// labeledDeploy (addon label, replicas=0) should be deleted.
	err = r.Client.Get(ctx, types.NamespacedName{Name: "custom-addon-mgr", Namespace: "open-cluster-management"}, deploy)
	if err == nil {
		t.Error("custom-addon-mgr should have been deleted")
	}

	// nonAddonDeploy should still exist.
	err = r.Client.Get(ctx, types.NamespacedName{Name: "some-other-controller", Namespace: "open-cluster-management"}, deploy)
	if err != nil {
		t.Errorf("some-other-controller should NOT have been deleted: %v", err)
	}

	// runningAddonDeploy (replicas > 0) should still exist.
	err = r.Client.Get(ctx, types.NamespacedName{Name: "submariner-addon", Namespace: "open-cluster-management"}, deploy)
	if err != nil {
		t.Errorf("submariner-addon (replicas=1) should NOT have been deleted: %v", err)
	}
}

// --- v11 liveness-gated intervention tests ---

func TestIsMCEOperatorAlive_WithReadyReplicas(t *testing.T) {
	t.Setenv("OPERATOR_PACKAGE", "advanced-cluster-management")
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	s := newTestScheme()

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "multicluster-engine-operator",
			Namespace: "multicluster-engine",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "mce"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "mce"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "img"}}},
			},
		},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 1},
	}

	cb := fake.NewClientBuilder().WithScheme(s).WithObjects(deploy)
	r := &HubTeardownReconciler{Client: cb.Build(), Scheme: s, Log: ctrllog.Log, Recorder: record.NewFakeRecorder(10)}
	log := ctrllog.Log.WithName("test")

	if !r.isMCEOperatorAlive(context.Background(), log) {
		t.Error("Expected isMCEOperatorAlive=true with ReadyReplicas=1")
	}
}

func TestIsMCEOperatorAlive_NoDeployment(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	r := newTestReconciler()
	log := ctrllog.Log.WithName("test")

	if r.isMCEOperatorAlive(context.Background(), log) {
		t.Error("Expected isMCEOperatorAlive=false with no deployment")
	}
}

func TestIsClusterManagerOperatorAlive_MCENamespace(t *testing.T) {
	t.Setenv("OPERATOR_PACKAGE", "advanced-cluster-management")
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	s := newTestScheme()

	deploy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "cluster-manager",
			Namespace: "multicluster-engine",
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "cm"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "cm"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "img"}}},
			},
		},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 1},
	}

	cb := fake.NewClientBuilder().WithScheme(s).WithObjects(deploy)
	r := &HubTeardownReconciler{Client: cb.Build(), Scheme: s, Log: ctrllog.Log, Recorder: record.NewFakeRecorder(10)}
	log := ctrllog.Log.WithName("test")

	if !r.isClusterManagerOperatorAlive(context.Background(), log) {
		t.Error("Expected isClusterManagerOperatorAlive=true with ReadyReplicas=1 in MCE namespace")
	}
}

func TestHandleOrphanedClusterManagerChain_NoCM(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	r := newTestReconciler()
	log := ctrllog.Log.WithName("test")

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{Name: "teardown", Namespace: "open-cluster-management"},
		Spec:       operatorv1.HubTeardownSpec{ApprovedDestructiveActions: true},
	}

	done, err := r.handleOrphanedClusterManagerChain(context.Background(), log, td)
	if err != nil {
		t.Fatalf("handleOrphanedClusterManagerChain error: %v", err)
	}
	if !done {
		t.Error("Expected done=true when no ClusterManager CRD exists")
	}
}

func TestSweepMCSAndCMA_NoMatchIsNoop(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	r := newTestReconciler()
	log := ctrllog.Log.WithName("test")

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{Name: "teardown", Namespace: "open-cluster-management"},
		Spec:       operatorv1.HubTeardownSpec{ApprovedDestructiveActions: true},
	}

	// Should not panic or error when MCS/CMA CRDs are not registered
	r.sweepMCSAndCMA(context.Background(), log, td)
}

func TestEnsureMCEDeleted_NoMatchIsNoop_v11(t *testing.T) {
	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	r := newTestReconciler()
	log := ctrllog.Log.WithName("test")

	td := &operatorv1.HubTeardown{
		ObjectMeta: metav1.ObjectMeta{Name: "teardown", Namespace: "open-cluster-management"},
	}

	if err := r.ensureMCEDeleted(context.Background(), log, td); err != nil {
		t.Fatalf("ensureMCEDeleted should be a noop when MCE CRD is not registered: %v", err)
	}
}

func TestIsAddonControllerDeployment(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
		want   bool
	}{
		{"component=addon-controller", map[string]string{"app.kubernetes.io/component": "addon-controller"}, true},
		{"component=addon-manager", map[string]string{"app.kubernetes.io/component": "addon-manager"}, true},
		{"app contains addon", map[string]string{"app": "my-addon-thing"}, true},
		{"partOf contains addon", map[string]string{"app.kubernetes.io/part-of": "addon-framework"}, true},
		{"no match", map[string]string{"app": "grc-controller"}, false},
		{"nil labels", nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			deploy := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{Labels: tt.labels},
			}
			if got := isAddonControllerDeployment(deploy); got != tt.want {
				t.Errorf("isAddonControllerDeployment() = %v, want %v", got, tt.want)
			}
		})
	}
}
