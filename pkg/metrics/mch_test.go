// Copyright (c) 2026 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package metrics

import (
	"testing"
	"time"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"

	"github.com/prometheus/client_golang/prometheus/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

func phaseValue(namespace, name string, phase operatorsv1.HubPhaseType) float64 {
	return testutil.ToFloat64(mchPhase.WithLabelValues(namespace, name, string(phase)))
}

func TestSetMCHPhaseReportsExactlyOneActivePhase(t *testing.T) {
	const namespace, name = "phase-active", "multiclusterhub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	for _, active := range allHubPhases {
		SetMCHPhase(namespace, name, active)

		for _, candidate := range allHubPhases {
			want := 0.0
			if candidate == active {
				want = 1
			}
			if got := phaseValue(namespace, name, candidate); got != want {
				t.Fatalf("phase %q active, phase %q = %v, want %v", active, candidate, got, want)
			}
		}
	}
}

func TestSetMCHPhaseReportsChangeOnlyOnTransition(t *testing.T) {
	const namespace, name = "phase-changed", "multiclusterhub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	if changed := SetMCHPhase(namespace, name, operatorsv1.HubInstalling); !changed {
		t.Fatal("first publish reported no change, want change")
	}
	if changed := SetMCHPhase(namespace, name, operatorsv1.HubInstalling); changed {
		t.Fatal("republishing the same phase reported a change")
	}
	if changed := SetMCHPhase(namespace, name, operatorsv1.HubRunning); !changed {
		t.Fatal("transition reported no change, want change")
	}

	DeleteMCHMetrics(namespace, name)
	if changed := SetMCHPhase(namespace, name, operatorsv1.HubRunning); !changed {
		t.Fatal("publish after delete reported no change, want change")
	}
}

func TestSetMCHPhaseFollowsTransitions(t *testing.T) {
	const namespace, name = "phase-transition", "multiclusterhub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	SetMCHPhase(namespace, name, operatorsv1.HubInstalling)
	if got := phaseValue(namespace, name, operatorsv1.HubInstalling); got != 1 {
		t.Fatalf("Installing = %v, want 1", got)
	}

	SetMCHPhase(namespace, name, operatorsv1.HubError)
	if got := phaseValue(namespace, name, operatorsv1.HubInstalling); got != 0 {
		t.Fatalf("Installing = %v after transition, want 0", got)
	}
	if got := phaseValue(namespace, name, operatorsv1.HubError); got != 1 {
		t.Fatalf("Error = %v after transition, want 1", got)
	}
}

func TestSetMCHPhaseReportsUnknownPhase(t *testing.T) {
	const namespace, name = "phase-unknown", "multiclusterhub"
	const unknown = operatorsv1.HubPhaseType("SomeFuturePhase")
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	SetMCHPhase(namespace, name, unknown)

	if got := phaseValue(namespace, name, unknown); got != 1 {
		t.Fatalf("unknown phase = %v, want 1", got)
	}
	if got := phaseValue(namespace, name, operatorsv1.HubRunning); got != 0 {
		t.Fatalf("Running = %v, want 0", got)
	}

	SetMCHPhase(namespace, name, operatorsv1.HubRunning)
	if got := phaseValue(namespace, name, unknown); got != 0 {
		t.Fatalf("unknown phase = %v after transition, want 0", got)
	}
	if got := phaseValue(namespace, name, operatorsv1.HubRunning); got != 1 {
		t.Fatalf("Running = %v after transition, want 1", got)
	}
}

func TestSetMCHPhaseReplacesUnknownPhase(t *testing.T) {
	const namespace, name = "phase-unknown-replace", "multiclusterhub"
	const firstUnknown = operatorsv1.HubPhaseType("SomeFuturePhase")
	const secondUnknown = operatorsv1.HubPhaseType("AnotherFuturePhase")
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	SetMCHPhase(namespace, name, firstUnknown)
	SetMCHPhase(namespace, name, secondUnknown)

	if got := phaseValue(namespace, name, firstUnknown); got != 0 {
		t.Fatalf("first unknown phase = %v after transition, want 0", got)
	}
	if got := phaseValue(namespace, name, secondUnknown); got != 1 {
		t.Fatalf("second unknown phase = %v, want 1", got)
	}
}

func TestSetMCHPhaseIsolatesHubInstances(t *testing.T) {
	const name = "multiclusterhub"
	t.Cleanup(func() {
		DeleteMCHMetrics("hub-one", name)
		DeleteMCHMetrics("hub-two", name)
	})

	SetMCHPhase("hub-one", name, operatorsv1.HubRunning)
	SetMCHPhase("hub-two", name, operatorsv1.HubError)

	if got := phaseValue("hub-one", name, operatorsv1.HubRunning); got != 1 {
		t.Fatalf("hub-one Running = %v, want 1", got)
	}
	if got := phaseValue("hub-two", name, operatorsv1.HubRunning); got != 0 {
		t.Fatalf("hub-two Running = %v, want 0", got)
	}

	DeleteMCHMetrics("hub-one", name)

	if got := phaseValue("hub-two", name, operatorsv1.HubError); got != 1 {
		t.Fatalf("hub-two Error = %v after deleting hub-one, want 1", got)
	}
}

func TestDeleteMCHMetricsRemovesSeries(t *testing.T) {
	const namespace, name = "phase-delete", "multiclusterhub"

	SetMCHPhase(namespace, name, operatorsv1.HubRunning)
	if count := testutil.CollectAndCount(mchPhase, PhaseMetricName); count != len(allHubPhases) {
		t.Fatalf("series count = %d, want %d", count, len(allHubPhases))
	}

	DeleteMCHMetrics(namespace, name)
	if count := testutil.CollectAndCount(mchPhase, PhaseMetricName); count != 0 {
		t.Fatalf("series count = %d after delete, want 0", count)
	}
}

func TestDeleteMCHMetricsReportsWhetherSeriesExisted(t *testing.T) {
	const namespace, name = "phase-delete-reported", "multiclusterhub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	if removed := DeleteMCHMetrics(namespace, name); removed {
		t.Fatal("delete reported a removal before anything was published")
	}

	SetMCHPhase(namespace, name, operatorsv1.HubRunning)
	if removed := DeleteMCHMetrics(namespace, name); !removed {
		t.Fatal("delete reported no removal after a phase was published")
	}
	if removed := DeleteMCHMetrics(namespace, name); removed {
		t.Fatal("repeated delete reported a removal, want false")
	}
}

// ---------------------------------------------------------------------------
// conditionMatchesPhase tests
// ---------------------------------------------------------------------------

func TestConditionMatchesPhase(t *testing.T) {
	tests := []struct {
		condType operatorsv1.HubConditionType
		phase    operatorsv1.HubPhaseType
		want     bool
	}{
		{operatorsv1.Complete, operatorsv1.HubRunning, true},
		{operatorsv1.Progressing, operatorsv1.HubRunning, false},
		{operatorsv1.Progressing, operatorsv1.HubInstalling, true},
		{operatorsv1.Progressing, operatorsv1.HubUpdating, true},
		{operatorsv1.Complete, operatorsv1.HubInstalling, false},
		{operatorsv1.Terminating, operatorsv1.HubUninstalling, true},
		{operatorsv1.Complete, operatorsv1.HubUninstalling, false},
		// HubUpdatingBlocked maps to Blocked (not Progressing).
		{operatorsv1.Blocked, operatorsv1.HubUpdatingBlocked, true},
		{operatorsv1.Progressing, operatorsv1.HubUpdatingBlocked, false},
		// ComponentFailure uses substring match for component-qualified types.
		{operatorsv1.ComponentFailure, operatorsv1.HubError, true},
		{"ComponentFailure: console", operatorsv1.HubError, true},
		{"ComponentFailure: cluster-backup", operatorsv1.HubError, true},
		{operatorsv1.Complete, operatorsv1.HubError, false},
		// Unmatched phases
		{operatorsv1.Complete, operatorsv1.HubPaused, false},
		{operatorsv1.Progressing, operatorsv1.HubPaused, false},
		{operatorsv1.Complete, operatorsv1.HubPending, false},
		{operatorsv1.Complete, operatorsv1.HubWaitingForMCE, false},
	}
	for _, tt := range tests {
		if got := conditionMatchesPhase(tt.condType, tt.phase); got != tt.want {
			t.Errorf("conditionMatchesPhase(%s, %s) = %v, want %v", tt.condType, tt.phase, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// SetMCHPhaseTransitionTimestamp tests
// ---------------------------------------------------------------------------

func tsGaugeCount() int {
	return testutil.CollectAndCount(mchPhaseTransitionTimestamp, PhaseTransitionTimestampMetricName)
}

func tsGaugeValue(namespace, name string) float64 {
	return testutil.ToFloat64(mchPhaseTransitionTimestamp.WithLabelValues(namespace, name))
}

func TestSetMCHPhaseTransitionTimestamp_MatchedPhase(t *testing.T) {
	const namespace, name = "ts-matched", "hub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	ts := time.Date(2025, 6, 15, 12, 0, 0, 0, time.UTC)
	conditions := []operatorsv1.HubCondition{
		{
			Type:               operatorsv1.Complete,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(ts),
		},
	}

	changed := SetMCHPhaseTransitionTimestamp(namespace, name, conditions, operatorsv1.HubRunning)
	if !changed {
		t.Fatal("first publish should report changed=true")
	}

	got := tsGaugeValue(namespace, name)
	want := float64(ts.Unix())
	if got != want {
		t.Fatalf("gauge = %v, want %v", got, want)
	}
}

func TestSetMCHPhaseTransitionTimestamp_UnmatchedFirstObservation(t *testing.T) {
	const namespace, name = "ts-unmatched-first", "hub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	// HubPaused has no matching condition type. On first observation (no prior
	// record), the function should skip publishing and return false.
	changed := SetMCHPhaseTransitionTimestamp(namespace, name, nil, operatorsv1.HubPaused)
	if changed {
		t.Fatal("unmatched phase with no prior record should return false")
	}

	// The metric should not exist at all for this hub.
	count := tsGaugeCount()
	if count != 0 {
		t.Fatalf("expected 0 series, got %d", count)
	}
}

func TestSetMCHPhaseTransitionTimestamp_RepublishSamePhase(t *testing.T) {
	const namespace, name = "ts-republish", "hub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	ts := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	conditions := []operatorsv1.HubCondition{
		{
			Type:               operatorsv1.Complete,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(ts),
		},
	}

	SetMCHPhaseTransitionTimestamp(namespace, name, conditions, operatorsv1.HubRunning)
	changed := SetMCHPhaseTransitionTimestamp(namespace, name, conditions, operatorsv1.HubRunning)
	if changed {
		t.Fatal("republishing same phase/timestamp should return false")
	}
}

func TestSetMCHPhaseTransitionTimestamp_MatchedToUnmatched(t *testing.T) {
	const namespace, name = "ts-matched-to-unmatched", "hub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	// Seed with a matched phase.
	conditions := []operatorsv1.HubCondition{
		{
			Type:               operatorsv1.Complete,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
		},
	}
	SetMCHPhaseTransitionTimestamp(namespace, name, conditions, operatorsv1.HubRunning)

	// Transition to unmatched phase: since a prior record exists, it should use
	// time.Now() and report changed=true.
	before := float64(time.Now().Unix())
	changed := SetMCHPhaseTransitionTimestamp(namespace, name, nil, operatorsv1.HubPaused)
	after := float64(time.Now().Unix())
	if !changed {
		t.Fatal("transition to unmatched phase should return true")
	}

	got := tsGaugeValue(namespace, name)
	if got < before || got > after {
		t.Fatalf("gauge = %v, want between %v and %v", got, before, after)
	}
}

func TestSetMCHPhaseTransitionTimestamp_UnmatchedToMatched(t *testing.T) {
	const namespace, name = "ts-unmatched-to-matched", "hub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	// Seed with a matched phase then transition to unmatched.
	seedCond := []operatorsv1.HubCondition{
		{
			Type:               operatorsv1.Progressing,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
		},
	}
	SetMCHPhaseTransitionTimestamp(namespace, name, seedCond, operatorsv1.HubInstalling)
	SetMCHPhaseTransitionTimestamp(namespace, name, nil, operatorsv1.HubPaused)

	// Now transition back to a matched phase. Use a condition time after the
	// HubPaused timestamp (which was time.Now()) so it is not considered stale.
	ts := time.Now().Add(time.Minute)
	conditions := []operatorsv1.HubCondition{
		{
			Type:               operatorsv1.Complete,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(ts),
		},
	}
	changed := SetMCHPhaseTransitionTimestamp(namespace, name, conditions, operatorsv1.HubRunning)
	if !changed {
		t.Fatal("transition to matched phase should return true")
	}

	got := tsGaugeValue(namespace, name)
	want := float64(ts.Unix())
	if got != want {
		t.Fatalf("gauge = %v, want %v", got, want)
	}
}

func TestSetMCHPhaseTransitionTimestamp_IsolatesHubs(t *testing.T) {
	const name = "hub"
	t.Cleanup(func() {
		DeleteMCHMetrics("ts-iso-a", name)
		DeleteMCHMetrics("ts-iso-b", name)
	})

	ts1 := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	ts2 := time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC)

	SetMCHPhaseTransitionTimestamp("ts-iso-a", name, []operatorsv1.HubCondition{
		{Type: operatorsv1.Complete, LastTransitionTime: metav1.NewTime(ts1)},
	}, operatorsv1.HubRunning)

	SetMCHPhaseTransitionTimestamp("ts-iso-b", name, []operatorsv1.HubCondition{
		{Type: operatorsv1.Complete, LastTransitionTime: metav1.NewTime(ts2)},
	}, operatorsv1.HubRunning)

	if got := tsGaugeValue("ts-iso-a", name); got != float64(ts1.Unix()) {
		t.Errorf("hub-a = %v, want %v", got, float64(ts1.Unix()))
	}
	if got := tsGaugeValue("ts-iso-b", name); got != float64(ts2.Unix()) {
		t.Errorf("hub-b = %v, want %v", got, float64(ts2.Unix()))
	}

	DeleteMCHMetrics("ts-iso-a", name)
	// hub-b must not be affected.
	if got := tsGaugeValue("ts-iso-b", name); got != float64(ts2.Unix()) {
		t.Errorf("hub-b after deleting hub-a = %v, want %v", got, float64(ts2.Unix()))
	}
}

func TestSetMCHPhaseTransitionTimestamp_ZeroTimestampCondition(t *testing.T) {
	const namespace, name = "ts-zero-cond", "hub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	// A matching condition with zero timestamp should behave like no match.
	conditions := []operatorsv1.HubCondition{
		{
			Type:               operatorsv1.Complete,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Time{}, // zero
		},
	}
	changed := SetMCHPhaseTransitionTimestamp(namespace, name, conditions, operatorsv1.HubRunning)
	// No prior record + zero-timestamp condition = skip (no match found).
	if changed {
		t.Fatal("zero-timestamp condition with no prior record should return false")
	}
}

// ---------------------------------------------------------------------------
// SetMCHLastReconcileTimestamp tests
// ---------------------------------------------------------------------------

func TestSetMCHLastReconcileTimestamp_FirstPublish(t *testing.T) {
	const namespace, name = "reconcile-first", "hub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	ts := float64(time.Now().Unix())
	changed := SetMCHLastReconcileTimestamp(namespace, name, ts)
	if !changed {
		t.Fatal("first publish should return true")
	}

	got := testutil.ToFloat64(mchLastReconcileTimestamp.WithLabelValues(namespace, name))
	if got != ts {
		t.Fatalf("gauge = %v, want %v", got, ts)
	}
}

func TestSetMCHLastReconcileTimestamp_SecondPublish(t *testing.T) {
	const namespace, name = "reconcile-second", "hub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	SetMCHLastReconcileTimestamp(namespace, name, 1000)
	changed := SetMCHLastReconcileTimestamp(namespace, name, 2000)
	if changed {
		t.Fatal("second publish should return false")
	}

	// Value should still update.
	got := testutil.ToFloat64(mchLastReconcileTimestamp.WithLabelValues(namespace, name))
	if got != 2000 {
		t.Fatalf("gauge = %v, want 2000", got)
	}
}

func TestSetMCHLastReconcileTimestamp_IsolatesHubs(t *testing.T) {
	const name = "hub"
	t.Cleanup(func() {
		DeleteMCHMetrics("reconcile-a", name)
		DeleteMCHMetrics("reconcile-b", name)
	})

	SetMCHLastReconcileTimestamp("reconcile-a", name, 1000)
	SetMCHLastReconcileTimestamp("reconcile-b", name, 2000)

	if got := testutil.ToFloat64(mchLastReconcileTimestamp.WithLabelValues("reconcile-a", name)); got != 1000 {
		t.Errorf("hub-a = %v, want 1000", got)
	}
	if got := testutil.ToFloat64(mchLastReconcileTimestamp.WithLabelValues("reconcile-b", name)); got != 2000 {
		t.Errorf("hub-b = %v, want 2000", got)
	}
}

func TestSetMCHLastReconcileTimestamp_AfterDelete(t *testing.T) {
	const namespace, name = "reconcile-del", "hub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	SetMCHLastReconcileTimestamp(namespace, name, 1000)
	DeleteMCHMetrics(namespace, name)

	// After delete, next publish should be "first" again.
	changed := SetMCHLastReconcileTimestamp(namespace, name, 2000)
	if !changed {
		t.Fatal("publish after delete should return true")
	}
}

// ---------------------------------------------------------------------------
// SetMCHVersionInfo tests
// ---------------------------------------------------------------------------

func TestSetMCHVersionInfo_FirstPublish(t *testing.T) {
	const namespace, name = "ver-first", "hub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	changed := SetMCHVersionInfo(namespace, name, "2.14.0", "2.14.0")
	if !changed {
		t.Fatal("first publish should return true")
	}

	got := testutil.ToFloat64(mchVersionInfo.WithLabelValues(namespace, name, "2.14.0", "2.14.0"))
	if got != 1 {
		t.Fatalf("version_info = %v, want 1", got)
	}
}

func TestSetMCHVersionInfo_RepublishSame(t *testing.T) {
	const namespace, name = "ver-same", "hub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	SetMCHVersionInfo(namespace, name, "2.14.0", "2.14.0")
	changed := SetMCHVersionInfo(namespace, name, "2.14.0", "2.14.0")
	if changed {
		t.Fatal("republishing same versions should return false")
	}
}

func TestSetMCHVersionInfo_VersionChange(t *testing.T) {
	const namespace, name = "ver-change", "hub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	SetMCHVersionInfo(namespace, name, "2.14.0", "2.14.0")
	changed := SetMCHVersionInfo(namespace, name, "2.14.0", "2.15.0")
	if !changed {
		t.Fatal("version label change should return true")
	}

	// New series should exist.
	got := testutil.ToFloat64(mchVersionInfo.WithLabelValues(namespace, name, "2.14.0", "2.15.0"))
	if got != 1 {
		t.Fatalf("new version_info = %v, want 1", got)
	}

	// Verify only the new label set exists — old series must be removed.
	families, err := ctrlmetrics.Registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}
	for _, fam := range families {
		if fam.GetName() != VersionInfoMetricName {
			continue
		}
		for _, m := range fam.GetMetric() {
			labels := map[string]string{}
			for _, lp := range m.GetLabel() {
				labels[lp.GetName()] = lp.GetValue()
			}
			if labels["namespace"] != namespace || labels["name"] != name {
				continue
			}
			if labels["current_version"] == "2.14.0" && labels["desired_version"] == "2.14.0" {
				t.Fatal("old version_info series still exists after label change")
			}
		}
	}
}

func TestSetMCHVersionInfo_UpgradeAvailable(t *testing.T) {
	const namespace, name = "ver-upgrade", "hub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	// Same versions → no upgrade.
	SetMCHVersionInfo(namespace, name, "2.14.0", "2.14.0")
	got := testutil.ToFloat64(mchUpgradeAvailable.WithLabelValues(namespace, name))
	if got != 0 {
		t.Fatalf("upgrade_available (same versions) = %v, want 0", got)
	}

	// Different versions → upgrade available.
	SetMCHVersionInfo(namespace, name, "2.14.0", "2.15.0")
	got = testutil.ToFloat64(mchUpgradeAvailable.WithLabelValues(namespace, name))
	if got != 1 {
		t.Fatalf("upgrade_available (diff versions) = %v, want 1", got)
	}
}

func TestSetMCHVersionInfo_EmptyStrings(t *testing.T) {
	const namespace, name = "ver-empty", "hub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	SetMCHVersionInfo(namespace, name, "", "")
	got := testutil.ToFloat64(mchUpgradeAvailable.WithLabelValues(namespace, name))
	if got != 0 {
		t.Fatalf("upgrade_available (empty strings) = %v, want 0", got)
	}

	// One empty, one set → no upgrade (requires both non-empty).
	SetMCHVersionInfo(namespace, name, "", "2.15.0")
	got = testutil.ToFloat64(mchUpgradeAvailable.WithLabelValues(namespace, name))
	if got != 0 {
		t.Fatalf("upgrade_available (one empty) = %v, want 0", got)
	}
}

func TestSetMCHVersionInfo_IsolatesHubs(t *testing.T) {
	const name = "hub"
	t.Cleanup(func() {
		DeleteMCHMetrics("ver-a", name)
		DeleteMCHMetrics("ver-b", name)
	})

	SetMCHVersionInfo("ver-a", name, "2.14.0", "2.14.0")
	SetMCHVersionInfo("ver-b", name, "2.14.0", "2.15.0")

	if got := testutil.ToFloat64(mchUpgradeAvailable.WithLabelValues("ver-a", name)); got != 0 {
		t.Errorf("hub-a upgrade = %v, want 0", got)
	}
	if got := testutil.ToFloat64(mchUpgradeAvailable.WithLabelValues("ver-b", name)); got != 1 {
		t.Errorf("hub-b upgrade = %v, want 1", got)
	}
}

func TestSetMCHVersionInfo_CleanupOnDelete(t *testing.T) {
	const namespace, name = "ver-cleanup", "hub"

	SetMCHVersionInfo(namespace, name, "2.14.0", "2.14.0")
	DeleteMCHMetrics(namespace, name)

	count := testutil.CollectAndCount(mchVersionInfo, VersionInfoMetricName)
	if count != 0 {
		t.Fatalf("version_info series after delete = %d, want 0", count)
	}
	count = testutil.CollectAndCount(mchUpgradeAvailable, UpgradeAvailableMetricName)
	if count != 0 {
		t.Fatalf("upgrade_available series after delete = %d, want 0", count)
	}
}

// ---------------------------------------------------------------------------
// SetMCHMCEVersionCompliant tests
// ---------------------------------------------------------------------------

func TestSetMCHMCEVersionCompliant_Compliant(t *testing.T) {
	const namespace, name = "mce-compliant", "hub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	compliance := &operatorsv1.MCEVersionComplianceStatus{
		RequiredChannel: "stable-2.8",
		CurrentVersion:  "2.8.1",
		IsCompliant:     true,
	}
	changed := SetMCHMCEVersionCompliant(namespace, name, compliance)
	if !changed {
		t.Fatal("first publish should return true")
	}

	got := testutil.ToFloat64(mchMCEVersionCompliant.WithLabelValues(namespace, name, "stable-2.8", "2.8.1"))
	if got != 1 {
		t.Fatalf("compliant gauge = %v, want 1", got)
	}
}

func TestSetMCHMCEVersionCompliant_NonCompliant(t *testing.T) {
	const namespace, name = "mce-noncompliant", "hub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	compliance := &operatorsv1.MCEVersionComplianceStatus{
		RequiredChannel: "stable-2.9",
		CurrentVersion:  "2.8.0",
		IsCompliant:     false,
	}
	SetMCHMCEVersionCompliant(namespace, name, compliance)

	// Use Gather() to verify the series actually exists with value 0, rather
	// than WithLabelValues which auto-creates a zero-valued gauge.
	families, err := ctrlmetrics.Registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}
	var found bool
	for _, fam := range families {
		if fam.GetName() != MCEVersionCompliantMetricName {
			continue
		}
		for _, m := range fam.GetMetric() {
			labels := map[string]string{}
			for _, lp := range m.GetLabel() {
				labels[lp.GetName()] = lp.GetValue()
			}
			if labels["namespace"] == namespace && labels["name"] == name &&
				labels["required_channel"] == "stable-2.9" && labels["current_mce_version"] == "2.8.0" {
				found = true
				if v := m.GetGauge().GetValue(); v != 0 {
					t.Fatalf("non-compliant gauge = %v, want 0", v)
				}
			}
		}
	}
	if !found {
		t.Fatal("expected mce_version_compliant series for non-compliant status, but none found")
	}
}

func TestSetMCHMCEVersionCompliant_Nil(t *testing.T) {
	const namespace, name = "mce-nil", "hub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	SetMCHMCEVersionCompliant(namespace, name, nil)

	// Use Gather() to verify the series actually exists with value 0, rather
	// than WithLabelValues which auto-creates a zero-valued gauge.
	families, err := ctrlmetrics.Registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}
	var found bool
	for _, fam := range families {
		if fam.GetName() != MCEVersionCompliantMetricName {
			continue
		}
		for _, m := range fam.GetMetric() {
			labels := map[string]string{}
			for _, lp := range m.GetLabel() {
				labels[lp.GetName()] = lp.GetValue()
			}
			if labels["namespace"] == namespace && labels["name"] == name &&
				labels["required_channel"] == "" && labels["current_mce_version"] == "" {
				found = true
				if v := m.GetGauge().GetValue(); v != 0 {
					t.Fatalf("nil compliance gauge = %v, want 0", v)
				}
			}
		}
	}
	if !found {
		t.Fatal("expected mce_version_compliant series for nil compliance, but none found")
	}
}

func TestSetMCHMCEVersionCompliant_LabelChange(t *testing.T) {
	const namespace, name = "mce-label-change", "hub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	comp1 := &operatorsv1.MCEVersionComplianceStatus{
		RequiredChannel: "stable-2.8",
		CurrentVersion:  "2.8.1",
		IsCompliant:     true,
	}
	SetMCHMCEVersionCompliant(namespace, name, comp1)

	comp2 := &operatorsv1.MCEVersionComplianceStatus{
		RequiredChannel: "stable-2.9",
		CurrentVersion:  "2.9.0",
		IsCompliant:     true,
	}
	changed := SetMCHMCEVersionCompliant(namespace, name, comp2)
	if !changed {
		t.Fatal("label change should return true")
	}

	// New series should exist with value 1.
	got := testutil.ToFloat64(mchMCEVersionCompliant.WithLabelValues(namespace, name, "stable-2.9", "2.9.0"))
	if got != 1 {
		t.Fatalf("new compliant gauge = %v, want 1", got)
	}

	// Verify only the new label set exists — old series must be removed.
	families, err := ctrlmetrics.Registry.Gather()
	if err != nil {
		t.Fatalf("Gather() error: %v", err)
	}
	for _, fam := range families {
		if fam.GetName() != MCEVersionCompliantMetricName {
			continue
		}
		for _, m := range fam.GetMetric() {
			labels := map[string]string{}
			for _, lp := range m.GetLabel() {
				labels[lp.GetName()] = lp.GetValue()
			}
			if labels["namespace"] != namespace || labels["name"] != name {
				continue
			}
			if labels["required_channel"] == "stable-2.8" && labels["current_mce_version"] == "2.8.1" {
				t.Fatal("old mce_version_compliant series still exists after label change")
			}
		}
	}
}

func TestSetMCHMCEVersionCompliant_RepublishSame(t *testing.T) {
	const namespace, name = "mce-same", "hub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	comp := &operatorsv1.MCEVersionComplianceStatus{
		RequiredChannel: "stable-2.8",
		CurrentVersion:  "2.8.1",
		IsCompliant:     true,
	}
	SetMCHMCEVersionCompliant(namespace, name, comp)
	changed := SetMCHMCEVersionCompliant(namespace, name, comp)
	if changed {
		t.Fatal("republishing same values should return false")
	}
}

func TestSetMCHMCEVersionCompliant_IsolatesHubs(t *testing.T) {
	const name = "hub"
	t.Cleanup(func() {
		DeleteMCHMetrics("mce-a", name)
		DeleteMCHMetrics("mce-b", name)
	})

	SetMCHMCEVersionCompliant("mce-a", name, &operatorsv1.MCEVersionComplianceStatus{
		RequiredChannel: "stable-2.8",
		CurrentVersion:  "2.8.0",
		IsCompliant:     true,
	})
	SetMCHMCEVersionCompliant("mce-b", name, &operatorsv1.MCEVersionComplianceStatus{
		RequiredChannel: "stable-2.9",
		CurrentVersion:  "2.8.0",
		IsCompliant:     false,
	})

	if got := testutil.ToFloat64(mchMCEVersionCompliant.WithLabelValues("mce-a", name, "stable-2.8", "2.8.0")); got != 1 {
		t.Errorf("hub-a = %v, want 1", got)
	}
	if got := testutil.ToFloat64(mchMCEVersionCompliant.WithLabelValues("mce-b", name, "stable-2.9", "2.8.0")); got != 0 {
		t.Errorf("hub-b = %v, want 0", got)
	}
}

func TestSetMCHMCEVersionCompliant_CleanupOnDelete(t *testing.T) {
	const namespace, name = "mce-cleanup", "hub"

	SetMCHMCEVersionCompliant(namespace, name, &operatorsv1.MCEVersionComplianceStatus{
		RequiredChannel: "stable-2.8",
		CurrentVersion:  "2.8.1",
		IsCompliant:     true,
	})
	DeleteMCHMetrics(namespace, name)

	count := testutil.CollectAndCount(mchMCEVersionCompliant, MCEVersionCompliantMetricName)
	if count != 0 {
		t.Fatalf("mce_compliant series after delete = %d, want 0", count)
	}
}

// ---------------------------------------------------------------------------
// DeleteMCHMetrics coverage for new metric families
// ---------------------------------------------------------------------------

func TestDeleteMCHMetrics_CleansAllNewMetrics(t *testing.T) {
	const namespace, name = "delete-all-new", "hub"

	// Publish every new metric family.
	conditions := []operatorsv1.HubCondition{
		{
			Type:               operatorsv1.Complete,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.NewTime(time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)),
		},
	}
	SetMCHPhaseTransitionTimestamp(namespace, name, conditions, operatorsv1.HubRunning)
	SetMCHLastReconcileTimestamp(namespace, name, 1000)
	SetMCHVersionInfo(namespace, name, "2.14.0", "2.15.0")
	SetMCHMCEVersionCompliant(namespace, name, &operatorsv1.MCEVersionComplianceStatus{
		RequiredChannel: "stable-2.8",
		CurrentVersion:  "2.8.1",
		IsCompliant:     true,
	})

	removed := DeleteMCHMetrics(namespace, name)
	if !removed {
		t.Fatal("delete should report removal")
	}

	if c := testutil.CollectAndCount(mchPhaseTransitionTimestamp, PhaseTransitionTimestampMetricName); c != 0 {
		t.Errorf("phase_transition_timestamp series = %d, want 0", c)
	}
	if c := testutil.CollectAndCount(mchLastReconcileTimestamp, LastReconcileTimestampMetricName); c != 0 {
		t.Errorf("last_reconcile_timestamp series = %d, want 0", c)
	}
	if c := testutil.CollectAndCount(mchVersionInfo, VersionInfoMetricName); c != 0 {
		t.Errorf("version_info series = %d, want 0", c)
	}
	if c := testutil.CollectAndCount(mchUpgradeAvailable, UpgradeAvailableMetricName); c != 0 {
		t.Errorf("upgrade_available series = %d, want 0", c)
	}
	if c := testutil.CollectAndCount(mchMCEVersionCompliant, MCEVersionCompliantMetricName); c != 0 {
		t.Errorf("mce_version_compliant series = %d, want 0", c)
	}
}
