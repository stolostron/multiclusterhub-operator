// Copyright (c) 2026 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package metrics

import (
	"testing"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"

	"github.com/prometheus/client_golang/prometheus/testutil"
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
