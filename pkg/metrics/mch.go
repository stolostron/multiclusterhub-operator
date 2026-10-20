// Copyright (c) 2026 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

// Package metrics registers and populates the Prometheus metrics that expose
// MultiClusterHub health. The metrics are served on the operator's existing
// metrics endpoint (:8383) through the controller-runtime registry, so they are
// scrapeable by the OpenShift built-in monitoring stack without requiring ACM
// Observability.
package metrics

import (
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

var mchPhase = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: mchPhaseMetricName,
		Help: "Current phase of the MultiClusterHub. The active phase reports 1; all other phases report 0.",
	},
	[]string{"namespace", "name", "phase"},
)

func init() {
	ctrlmetrics.Registry.MustRegister(mchPhase)
}

// SetMCHPhase publishes the phase of the given MultiClusterHub, reporting 1 for
// the active phase and 0 for every other known phase.
func SetMCHPhase(namespace, name string, phase operatorsv1.HubPhaseType) {
	known := false
	for _, candidate := range allHubPhases {
		if candidate == phase {
			known = true
		}
		mchPhase.WithLabelValues(namespace, name, string(candidate)).Set(boolToFloat(candidate == phase))
	}

	// A phase added to the API but not to allHubPhases would otherwise leave every
	// series at 0. Report it as active instead of losing the signal.
	if !known {
		mchPhase.WithLabelValues(namespace, name, string(phase)).Set(1)
	}
}

// DeleteMCHPhase removes every phase series belonging to the given
// MultiClusterHub so that a deleted hub does not leave stale series behind.
func DeleteMCHPhase(namespace, name string) {
	mchPhase.DeletePartialMatch(prometheus.Labels{
		"namespace": namespace,
		"name":      name,
	})
}

func boolToFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
