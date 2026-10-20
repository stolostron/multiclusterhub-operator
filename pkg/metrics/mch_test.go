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
	t.Cleanup(func() { DeleteMCHPhase(namespace, name) })

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

func TestSetMCHPhaseFollowsTransitions(t *testing.T) {
	const namespace, name = "phase-transition", "multiclusterhub"
	t.Cleanup(func() { DeleteMCHPhase(namespace, name) })

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
	t.Cleanup(func() { DeleteMCHPhase(namespace, name) })

	SetMCHPhase(namespace, name, unknown)

	if got := phaseValue(namespace, name, unknown); got != 1 {
		t.Fatalf("unknown phase = %v, want 1", got)
	}
	if got := phaseValue(namespace, name, operatorsv1.HubRunning); got != 0 {
		t.Fatalf("Running = %v, want 0", got)
	}
}

func TestSetMCHPhaseIsolatesHubInstances(t *testing.T) {
	const name = "multiclusterhub"
	t.Cleanup(func() {
		DeleteMCHPhase("hub-one", name)
		DeleteMCHPhase("hub-two", name)
	})

	SetMCHPhase("hub-one", name, operatorsv1.HubRunning)
	SetMCHPhase("hub-two", name, operatorsv1.HubError)

	if got := phaseValue("hub-one", name, operatorsv1.HubRunning); got != 1 {
		t.Fatalf("hub-one Running = %v, want 1", got)
	}
	if got := phaseValue("hub-two", name, operatorsv1.HubRunning); got != 0 {
		t.Fatalf("hub-two Running = %v, want 0", got)
	}

	DeleteMCHPhase("hub-one", name)

	if got := phaseValue("hub-two", name, operatorsv1.HubError); got != 1 {
		t.Fatalf("hub-two Error = %v after deleting hub-one, want 1", got)
	}
}

func TestDeleteMCHPhaseRemovesSeries(t *testing.T) {
	const namespace, name = "phase-delete", "multiclusterhub"

	SetMCHPhase(namespace, name, operatorsv1.HubRunning)
	if count := testutil.CollectAndCount(mchPhase, mchPhaseMetricName); count != len(allHubPhases) {
		t.Fatalf("series count = %d, want %d", count, len(allHubPhases))
	}

	DeleteMCHPhase(namespace, name)
	if count := testutil.CollectAndCount(mchPhase, mchPhaseMetricName); count != 0 {
		t.Fatalf("series count = %d after delete, want 0", count)
	}
}
