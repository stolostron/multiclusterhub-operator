// Copyright (c) 2026 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

// Package metrics registers and populates the Prometheus metrics that expose
// MultiClusterHub health. The metrics are served on the operator's existing
// metrics endpoint (:8383) through the controller-runtime registry, so they are
// scrapeable by the OpenShift built-in monitoring stack without requiring ACM
// Observability.
package metrics

import (
	"maps"
	"sync"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"

	"github.com/prometheus/client_golang/prometheus"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	// PhaseMetricName is the name of the MultiClusterHub phase gauge. It is exported
	// so callers can log the metric they publish.
	PhaseMetricName = "acm_mch_phase"

	// ComponentConditionMetricName is the name of the per-component health gauge. It
	// mirrors the shape of the OpenShift cluster_operator_conditions metric.
	ComponentConditionMetricName = "acm_mch_component_condition"

	// ComponentEnabledMetricName is the name of the per-component enabled gauge, which
	// distinguishes a disabled component from one that is missing from status.
	ComponentEnabledMetricName = "acm_mch_component_enabled"
)

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
		Name: PhaseMetricName,
		Help: "Current phase of the MultiClusterHub. The active phase reports 1; all other phases report 0.",
	},
	[]string{"namespace", "name", "phase"},
)

// mchComponentCondition is the acm_mch_component_condition gauge. Each component
// in the MultiClusterHub status contributes one series, labelled with the raw
// Kubernetes kind so it can be joined against kube-state-metrics.
var mchComponentCondition = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: ComponentConditionMetricName,
		Help: "Condition of each MultiClusterHub component. 1 = the condition status is True, 0 = False or Unknown.",
	},
	[]string{"namespace", "name", "component", "kind", "condition"},
)

// mchComponentEnabled is the acm_mch_component_enabled gauge. It is published for
// every known MultiClusterHub component so a disabled component reports 0 instead
// of disappearing, which keeps it distinguishable from a component that is missing
// from the status.
var mchComponentEnabled = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: ComponentEnabledMetricName,
		Help: "Whether each MultiClusterHub component is enabled. 1 = enabled, 0 = disabled.",
	},
	[]string{"namespace", "name", "component"},
)

// componentConditionSeries identifies a single acm_mch_component_condition series
// for a hub. Tracking the label set lets a series be deleted when its component is
// disabled, paused, or removed, since the status only lists enabled components.
type componentConditionSeries struct {
	component string
	kind      string
	condition string
}

// mchMetricsMu serializes updates to every MultiClusterHub metric family and to the
// tracking maps below, so a published series always matches the tracked state.
var mchMetricsMu sync.Mutex

// unknownPhases tracks the last unknown phase published for each hub
// (namespace/name), so it can be removed on the next phase change.
var unknownPhases = map[string]string{}

// publishedPhases tracks the last phase published for each hub (namespace/name)
// so SetMCHPhase can report whether the gauge actually changed. Callers use this
// to log transitions (including the first publish after an operator restart)
// without logging on every reconcile.
var publishedPhases = map[string]string{}

// publishedComponentConditions tracks the condition series published for each hub
// so series belonging to components that are no longer reported can be removed.
var publishedComponentConditions = map[string]map[componentConditionSeries]float64{}

// publishedComponentEnabled tracks the enabled state published for each hub so
// components that are no longer known can be removed.
var publishedComponentEnabled = map[string]map[string]bool{}

func init() {
	ctrlmetrics.Registry.MustRegister(mchPhase)
	ctrlmetrics.Registry.MustRegister(mchComponentCondition)
	ctrlmetrics.Registry.MustRegister(mchComponentEnabled)
}

// SetMCHPhase publishes the phase of the given MultiClusterHub, reporting 1 for
// the active phase and 0 for every other known phase. It reports whether the
// active phase changed, so callers can log transitions instead of every update.
func SetMCHPhase(namespace, name string, phase operatorsv1.HubPhaseType) bool {
	mchMetricsMu.Lock()
	defer mchMetricsMu.Unlock()

	key := hubMetricKey(namespace, name)
	changed := publishedPhases[key] != string(phase)
	publishedPhases[key] = string(phase)

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
		return changed
	}

	// A phase added to the API but not to allHubPhases would otherwise leave every
	// series at 0. Report it as active instead of losing the signal, replacing any
	// different unknown phase previously reported for this hub.
	if previous, ok := unknownPhases[key]; ok && previous != string(phase) {
		mchPhase.DeleteLabelValues(namespace, name, previous)
	}
	unknownPhases[key] = string(phase)
	mchPhase.WithLabelValues(namespace, name, string(phase)).Set(1)
	return changed
}

// SetMCHComponentMetrics publishes the health of every component reported in the
// MultiClusterHub status together with the enabled state of every known component.
// components is keyed by component name and is expected to hold only the enabled
// components, as Status.Components does; enabled must list every known component
// (enabled or not) so a disabled component reports 0 instead of disappearing.
//
// Health series for components that are no longer reported are removed, so a
// disabled, paused, or removed component leaves no stale series behind. It reports
// whether any published series changed, so callers can log transitions instead of
// every update.
func SetMCHComponentMetrics(namespace, name string, components map[string]operatorsv1.StatusCondition, enabled map[string]bool) bool {
	mchMetricsMu.Lock()
	defer mchMetricsMu.Unlock()

	key := hubMetricKey(namespace, name)

	desiredConditions := make(map[componentConditionSeries]float64, len(components))
	for componentName, component := range components {
		series := componentConditionSeries{
			component: component.Name,
			kind:      component.Kind,
			condition: component.Type,
		}
		// Status entries are keyed by component name; fall back to the key so a
		// partially populated condition still yields a usable series.
		if series.component == "" {
			series.component = componentName
		}
		// Every known condition type is published as-is, but guard against an empty
		// type so the series never carries a blank condition label.
		if series.condition == "" {
			series.condition = "Available"
		}
		desiredConditions[series] = conditionStatusValue(component.Status)
	}

	changed := false
	previousConditions := publishedComponentConditions[key]
	for series := range previousConditions {
		if _, ok := desiredConditions[series]; !ok {
			mchComponentCondition.DeleteLabelValues(namespace, name, series.component, series.kind, series.condition)
		}
	}
	for series, value := range desiredConditions {
		mchComponentCondition.WithLabelValues(namespace, name, series.component, series.kind, series.condition).Set(value)
	}
	if !maps.Equal(previousConditions, desiredConditions) {
		changed = true
	}
	if len(desiredConditions) == 0 {
		delete(publishedComponentConditions, key)
	} else {
		publishedComponentConditions[key] = desiredConditions
	}

	desiredEnabled := make(map[string]bool, len(enabled))
	for component, isEnabled := range enabled {
		desiredEnabled[component] = isEnabled
	}

	previousEnabled := publishedComponentEnabled[key]
	for component := range previousEnabled {
		if _, ok := desiredEnabled[component]; !ok {
			mchComponentEnabled.DeleteLabelValues(namespace, name, component)
		}
	}
	for component, isEnabled := range desiredEnabled {
		mchComponentEnabled.WithLabelValues(namespace, name, component).Set(boolToFloat(isEnabled))
	}
	if !maps.Equal(previousEnabled, desiredEnabled) {
		changed = true
	}
	if len(desiredEnabled) == 0 {
		delete(publishedComponentEnabled, key)
	} else {
		publishedComponentEnabled[key] = desiredEnabled
	}

	return changed
}

// DeleteMCHMetrics removes every metric series belonging to the given
// MultiClusterHub so that a deleted hub does not leave stale series behind.
// All MultiClusterHub-scoped metrics must be cleaned up here so callers only
// need a single entry point. It reports whether any series were removed, so
// callers can log cleanup once instead of on every teardown reconcile.
func DeleteMCHMetrics(namespace, name string) bool {
	mchMetricsMu.Lock()
	defer mchMetricsMu.Unlock()

	key := hubMetricKey(namespace, name)
	_, phaseExisted := publishedPhases[key]
	_, conditionsExisted := publishedComponentConditions[key]
	_, enabledExisted := publishedComponentEnabled[key]
	existed := phaseExisted || conditionsExisted || enabledExisted

	delete(unknownPhases, key)
	delete(publishedPhases, key)
	delete(publishedComponentConditions, key)
	delete(publishedComponentEnabled, key)
	mchPhase.DeletePartialMatch(prometheus.Labels{
		"namespace": namespace,
		"name":      name,
	})
	mchComponentCondition.DeletePartialMatch(prometheus.Labels{
		"namespace": namespace,
		"name":      name,
	})
	mchComponentEnabled.DeletePartialMatch(prometheus.Labels{
		"namespace": namespace,
		"name":      name,
	})
	return existed
}

func hubMetricKey(namespace, name string) string {
	return namespace + "/" + name
}

// conditionStatusValue maps a condition status to the gauge value. Only True
// reports healthy; False and Unknown both report 0, matching the acm_mch_condition
// family so the two metrics never disagree about a condition.
func conditionStatusValue(status metav1.ConditionStatus) float64 {
	return boolToFloat(status == metav1.ConditionTrue)
}

func boolToFloat(value bool) float64 {
	if value {
		return 1
	}
	return 0
}
