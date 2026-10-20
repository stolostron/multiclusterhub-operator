// Copyright (c) 2026 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

// Package metrics registers and populates the Prometheus metrics that expose
// MultiClusterHub health. The metrics are served on the operator's existing
// metrics endpoint (:8383) through the controller-runtime registry, so they are
// scrapeable by the OpenShift built-in monitoring stack without requiring ACM
// Observability.
package metrics

import (
	"sync"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"

	"github.com/prometheus/client_golang/prometheus"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

// mchPhaseMetricName is the name of the MultiClusterHub phase gauge.
const mchPhaseMetricName = "acm_mch_phase"

// allHubPhases lists every phase a MultiClusterHub can report. Publishing a
// series for each phase (0 for inactive phases) lets consumers alert on a
// specific phase without first testing for series presence.
var allHubPhases = []operatorsv1.HubPhaseType{
	operatorsv1.HubPending,
	operatorsv1.HubPaused,
	operatorsv1.HubRunning,
	operatorsv1.HubInstalling,
	operatorsv1.HubUpdating,
	operatorsv1.HubUninstalling,
	operatorsv1.HubUpdatingBlocked,
	operatorsv1.HubError,
	operatorsv1.HubWaitingForMCE,
}

// mchPhase is the acm_mch_phase gauge. Each MultiClusterHub has one series per
// entry in allHubPhases, identified by the namespace, name, and phase labels;
// exactly one of those series is 1 (the current phase) and the rest are 0.
var mchPhase = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: mchPhaseMetricName,
		Help: "Current phase of the MultiClusterHub. The active phase reports 1; all other phases report 0.",
	},
	[]string{"namespace", "name", "phase"},
)

// mchPhaseMu serializes updates to mchPhase and unknownPhases so the tracked
// unknown phase always matches the published series.
var mchPhaseMu sync.Mutex

// unknownPhases tracks the last unknown phase published for each hub
// (namespace/name), so it can be removed on the next phase change.
var unknownPhases = map[string]string{}

func init() {
	ctrlmetrics.Registry.MustRegister(mchPhase)
}

// SetMCHPhase publishes the phase of the given MultiClusterHub, reporting 1 for
// the active phase and 0 for every other known phase.
func SetMCHPhase(namespace, name string, phase operatorsv1.HubPhaseType) {
	mchPhaseMu.Lock()
	defer mchPhaseMu.Unlock()

	key := hubMetricKey(namespace, name)
	known := false
	for _, candidate := range allHubPhases {
		if candidate == phase {
			known = true
		}
		mchPhase.WithLabelValues(namespace, name, string(candidate)).Set(boolToFloat(candidate == phase))
	}

	if known {
		// Drop any previously reported unknown phase so it cannot stay active
		// alongside the known phase.
		if previous, ok := unknownPhases[key]; ok {
			mchPhase.DeleteLabelValues(namespace, name, previous)
			delete(unknownPhases, key)
		}
		return
	}

	// A phase added to the API but not to allHubPhases would otherwise leave every
	// series at 0. Report it as active instead of losing the signal, replacing any
	// different unknown phase previously reported for this hub.
	if previous, ok := unknownPhases[key]; ok && previous != string(phase) {
		mchPhase.DeleteLabelValues(namespace, name, previous)
	}
	unknownPhases[key] = string(phase)
	mchPhase.WithLabelValues(namespace, name, string(phase)).Set(1)
}

// DeleteMCHMetrics removes every metric series belonging to the given
// MultiClusterHub so that a deleted hub does not leave stale series behind.
// All MultiClusterHub-scoped metrics must be cleaned up here so callers only
// need a single entry point.
func DeleteMCHMetrics(namespace, name string) {
	mchPhaseMu.Lock()
	defer mchPhaseMu.Unlock()

	delete(unknownPhases, hubMetricKey(namespace, name))
	mchPhase.DeletePartialMatch(prometheus.Labels{
		"namespace": namespace,
		"name":      name,
	})
}

func hubMetricKey(namespace, name string) string {
	return namespace + "/" + name
}

func boolToFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
