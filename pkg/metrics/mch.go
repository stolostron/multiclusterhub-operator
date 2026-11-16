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
	"strings"
	"sync"
	"time"

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

	// PhaseTransitionTimestampMetricName is the name of the gauge that reports the
	// Unix epoch of the last phase transition.
	PhaseTransitionTimestampMetricName = "acm_mch_phase_transition_timestamp"

	// LastReconcileTimestampMetricName is the name of the gauge that reports the
	// Unix epoch of the last successful reconcile that reached syncHubStatus.
	LastReconcileTimestampMetricName = "acm_mch_last_reconcile_timestamp"

	// VersionInfoMetricName is the name of the info-style gauge (always 1) that
	// exposes the current and desired version as labels.
	VersionInfoMetricName = "acm_mch_version_info"

	// UpgradeAvailableMetricName is the name of the gauge that reports whether an
	// upgrade is available (CurrentVersion != DesiredVersion and both are non-empty).
	UpgradeAvailableMetricName = "acm_mch_upgrade_available"

	// MCEVersionCompliantMetricName is the name of the gauge that reports whether
	// the MCE version is compliant with the required channel.
	MCEVersionCompliantMetricName = "acm_mch_mce_version_compliant"
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

// mchPhaseTransitionTimestamp is the acm_mch_phase_transition_timestamp gauge.
// It reports the Unix epoch (float64) of the last phase transition for each hub.
var mchPhaseTransitionTimestamp = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: PhaseTransitionTimestampMetricName,
		Help: "Unix timestamp of the last phase transition for the MultiClusterHub.",
	},
	[]string{"namespace", "name"},
)

// mchLastReconcileTimestamp is the acm_mch_last_reconcile_timestamp gauge.
// It reports the Unix epoch (float64) of the last successful reconcile that
// reached syncHubStatus.
var mchLastReconcileTimestamp = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: LastReconcileTimestampMetricName,
		Help: "Unix timestamp of the last successful reconcile that reached syncHubStatus.",
	},
	[]string{"namespace", "name"},
)

// mchVersionInfo is the acm_mch_version_info gauge. It is an info-style gauge
// (always value 1) with version labels for dashboards.
var mchVersionInfo = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: VersionInfoMetricName,
		Help: "Info-style gauge (always 1) exposing current and desired version as labels.",
	},
	[]string{"namespace", "name", "current_version", "desired_version"},
)

// mchUpgradeAvailable is the acm_mch_upgrade_available gauge.
// 1 when CurrentVersion != "" && DesiredVersion != "" && CurrentVersion != DesiredVersion, 0 otherwise.
var mchUpgradeAvailable = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: UpgradeAvailableMetricName,
		Help: "1 when the hub's current version differs from the desired (operator) version, indicating an upgrade or rollout is in progress",
	},
	[]string{"namespace", "name"},
)

// mchMCEVersionCompliant is the acm_mch_mce_version_compliant gauge.
// 1 when MCEVersionCompliance != nil && IsCompliant, 0 otherwise.
var mchMCEVersionCompliant = prometheus.NewGaugeVec(
	prometheus.GaugeOpts{
		Name: MCEVersionCompliantMetricName,
		Help: "Whether the MCE version is compliant with the required channel. 1 = compliant, 0 = non-compliant or unknown.",
	},
	[]string{"namespace", "name", "required_channel", "current_mce_version"},
)

// versionInfoLabels tracks the previously published version-info label set so
// stale series with old version labels can be cleaned up when versions change.
type versionInfoLabels struct {
	currentVersion string
	desiredVersion string
}

// mceCompliantLabels tracks the previously published MCE-compliant label set so
// stale series with old channel/version labels can be cleaned up.
type mceCompliantLabels struct {
	requiredChannel   string
	currentMCEVersion string
}

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

// phaseTransitionRecord stores the previously published phase and timestamp so
// repeated reconciles of an unmatched phase (HubPaused, HubPending,
// HubWaitingForMCE) reuse the original timestamp instead of overwriting it
// with time.Now() on every call.
type phaseTransitionRecord struct {
	phase operatorsv1.HubPhaseType
	ts    float64
}

// publishedPhaseTransitionTS tracks the previously published phase-transition
// timestamp for each hub (namespace/name), so stale series can be removed and
// unchanged phases keep their original timestamp.
var publishedPhaseTransitionTS = map[string]phaseTransitionRecord{}

// publishedLastReconcileTS tracks whether a last-reconcile timestamp has been
// published for each hub (namespace/name), so stale series can be removed.
var publishedLastReconcileTS = map[string]bool{}

// publishedVersionInfo tracks the previous version-info label set for each hub
// so stale series with old version labels can be cleaned up.
var publishedVersionInfo = map[string]*versionInfoLabels{}

// publishedUpgradeAvailable tracks whether an upgrade-available gauge has been
// published for each hub (namespace/name), so stale series can be removed.
var publishedUpgradeAvailable = map[string]bool{}

// publishedMCECompliant tracks the previous MCE-compliant label set for each hub
// so stale series with old channel/version labels can be cleaned up.
var publishedMCECompliant = map[string]*mceCompliantLabels{}

func init() {
	ctrlmetrics.Registry.MustRegister(mchPhase)
	ctrlmetrics.Registry.MustRegister(mchComponentCondition)
	ctrlmetrics.Registry.MustRegister(mchComponentEnabled)
	ctrlmetrics.Registry.MustRegister(mchPhaseTransitionTimestamp)
	ctrlmetrics.Registry.MustRegister(mchLastReconcileTimestamp)
	ctrlmetrics.Registry.MustRegister(mchVersionInfo)
	ctrlmetrics.Registry.MustRegister(mchUpgradeAvailable)
	ctrlmetrics.Registry.MustRegister(mchMCEVersionCompliant)
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

// SetMCHPhaseTransitionTimestamp publishes the Unix epoch of the last phase
// transition for the given MultiClusterHub. The timestamp is sourced from the
// HubCondition whose Type matches the phase, falling back to time.Now() when no
// matching condition has a LastTransitionTime. It reports whether the published
// value changed, so callers can log transitions instead of every update.
func SetMCHPhaseTransitionTimestamp(namespace, name string, conditions []operatorsv1.HubCondition, phase operatorsv1.HubPhaseType) bool {
	mchMetricsMu.Lock()
	defer mchMetricsMu.Unlock()

	key := hubMetricKey(namespace, name)
	prev, prevExists := publishedPhaseTransitionTS[key]

	// If the phase is unchanged, always preserve the recorded timestamp.
	// Condition times may shift (e.g. a different ComponentFailure condition
	// becomes the match, or Progressing is reused across Pending→Updating)
	// but the phase hasn't transitioned, so the original "when did this phase
	// start" timestamp must be kept.
	if prevExists && prev.phase == phase {
		return false
	}

	// Phase changed (or first observation). Try to find a matching
	// condition's LastTransitionTime.
	var ts float64
	found := false
	for i := range conditions {
		if conditionMatchesPhase(conditions[i].Type, phase) && !conditions[i].LastTransitionTime.IsZero() {
			ts = float64(conditions[i].LastTransitionTime.Unix())
			found = true
			break
		}
	}

	if prevExists {
		// Phase changed.
		if found && ts < prev.ts {
			// The condition's LastTransitionTime predates the previous
			// phase's recorded time — it is stale (e.g. Progressing was
			// set True during Pending and hasn't changed for Updating).
			// Use time.Now() as the best approximation for when this
			// transition happened.
			ts = float64(time.Now().Unix())
		} else if !found {
			// Phase changed but no condition matches (e.g. HubPaused →
			// HubWaitingForMCE). Use time.Now() as a reasonable
			// approximation for when this transition happened.
			ts = float64(time.Now().Unix())
		}
	} else {
		// First observation of this hub.
		if !found {
			// No matching condition — skip publishing entirely rather
			// than reporting a misleading time.Now() timestamp (which
			// would be wrong after an operator restart — the hub may
			// have been in this phase for hours). The metric will appear
			// once a transition to a matchable phase occurs.
			return false
		}
	}

	mchPhaseTransitionTimestamp.WithLabelValues(namespace, name).Set(ts)
	publishedPhaseTransitionTS[key] = phaseTransitionRecord{phase: phase, ts: ts}
	return true
}

// conditionMatchesPhase returns true when the HubCondition type is the natural
// match for the given phase. This covers the common mappings; an unmatched phase
// on first observation skips publishing entirely (no misleading time.Now()), and
// on subsequent observations reuses the previously recorded timestamp.
func conditionMatchesPhase(condType operatorsv1.HubConditionType, phase operatorsv1.HubPhaseType) bool {
	switch phase {
	case operatorsv1.HubRunning:
		return condType == operatorsv1.Complete
	case operatorsv1.HubInstalling, operatorsv1.HubUpdating:
		return condType == operatorsv1.Progressing
	case operatorsv1.HubUpdatingBlocked:
		return condType == operatorsv1.Blocked
	case operatorsv1.HubUninstalling:
		return condType == operatorsv1.Terminating
	case operatorsv1.HubError:
		return strings.Contains(string(condType), string(operatorsv1.ComponentFailure))
	default:
		return false
	}
}

// SetMCHLastReconcileTimestamp publishes the Unix epoch of the last successful
// reconcile that reached syncHubStatus. The timestamp is provided by the caller
// (typically time.Now()). It reports whether the gauge was newly created (first
// publish after restart or hub creation), so callers can log that event.
func SetMCHLastReconcileTimestamp(namespace, name string, ts float64) bool {
	mchMetricsMu.Lock()
	defer mchMetricsMu.Unlock()

	key := hubMetricKey(namespace, name)
	prev := publishedLastReconcileTS[key]
	mchLastReconcileTimestamp.WithLabelValues(namespace, name).Set(ts)
	publishedLastReconcileTS[key] = true
	return !prev
}

// SetMCHVersionInfo publishes the version-info gauge and the upgrade-available gauge
// for the given MultiClusterHub. The version-info gauge is always 1 with
// current_version and desired_version labels. The upgrade-available gauge is 1
// when both versions are non-empty and differ, 0 otherwise. It reports whether
// any published series changed.
func SetMCHVersionInfo(namespace, name, currentVersion, desiredVersion string) bool {
	mchMetricsMu.Lock()
	defer mchMetricsMu.Unlock()

	key := hubMetricKey(namespace, name)
	changed := false

	// --- version_info (info-style, always 1) ---
	desired := &versionInfoLabels{currentVersion: currentVersion, desiredVersion: desiredVersion}
	prev := publishedVersionInfo[key]
	if prev != nil && *prev != *desired {
		// Version labels changed — remove the stale series.
		mchVersionInfo.DeleteLabelValues(namespace, name, prev.currentVersion, prev.desiredVersion)
		changed = true
	}
	if prev == nil {
		changed = true
	}
	mchVersionInfo.WithLabelValues(namespace, name, currentVersion, desiredVersion).Set(1)
	publishedVersionInfo[key] = desired

	// --- upgrade_available ---
	upgradeVal := boolToFloat(currentVersion != "" && desiredVersion != "" && currentVersion != desiredVersion)
	if !publishedUpgradeAvailable[key] {
		changed = true
	}
	mchUpgradeAvailable.WithLabelValues(namespace, name).Set(upgradeVal)
	publishedUpgradeAvailable[key] = true

	return changed
}

// SetMCHMCEVersionCompliant publishes the MCE-version-compliant gauge for the
// given MultiClusterHub. When MCEVersionCompliance is nil, the gauge publishes 0
// with empty channel/version labels. It reports whether any published series changed.
func SetMCHMCEVersionCompliant(namespace, name string, compliance *operatorsv1.MCEVersionComplianceStatus) bool {
	mchMetricsMu.Lock()
	defer mchMetricsMu.Unlock()

	key := hubMetricKey(namespace, name)
	changed := false

	var requiredChannel, currentMCEVersion string
	var compliant float64
	if compliance != nil {
		requiredChannel = compliance.RequiredChannel
		currentMCEVersion = compliance.CurrentVersion
		compliant = boolToFloat(compliance.IsCompliant)
	}

	desired := &mceCompliantLabels{requiredChannel: requiredChannel, currentMCEVersion: currentMCEVersion}
	prev := publishedMCECompliant[key]
	if prev != nil && *prev != *desired {
		// Labels changed — remove the stale series.
		mchMCEVersionCompliant.DeleteLabelValues(namespace, name, prev.requiredChannel, prev.currentMCEVersion)
		changed = true
	}
	if prev == nil {
		changed = true
	}
	mchMCEVersionCompliant.WithLabelValues(namespace, name, requiredChannel, currentMCEVersion).Set(compliant)
	publishedMCECompliant[key] = desired

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
	_, phaseTransitionExisted := publishedPhaseTransitionTS[key]
	_, lastReconcileExisted := publishedLastReconcileTS[key]
	_, versionInfoExisted := publishedVersionInfo[key]
	_, upgradeAvailableExisted := publishedUpgradeAvailable[key]
	_, mceCompliantExisted := publishedMCECompliant[key]
	existed := phaseExisted || conditionsExisted || enabledExisted ||
		phaseTransitionExisted || lastReconcileExisted ||
		versionInfoExisted || upgradeAvailableExisted || mceCompliantExisted

	delete(unknownPhases, key)
	delete(publishedPhases, key)
	delete(publishedComponentConditions, key)
	delete(publishedComponentEnabled, key)
	delete(publishedPhaseTransitionTS, key)
	delete(publishedLastReconcileTS, key)
	delete(publishedVersionInfo, key)
	delete(publishedUpgradeAvailable, key)
	delete(publishedMCECompliant, key)
	partialLabels := prometheus.Labels{
		"namespace": namespace,
		"name":      name,
	}
	mchPhase.DeletePartialMatch(partialLabels)
	mchComponentCondition.DeletePartialMatch(partialLabels)
	mchComponentEnabled.DeletePartialMatch(partialLabels)
	mchPhaseTransitionTimestamp.DeletePartialMatch(partialLabels)
	mchLastReconcileTimestamp.DeletePartialMatch(partialLabels)
	mchVersionInfo.DeletePartialMatch(partialLabels)
	mchUpgradeAvailable.DeletePartialMatch(partialLabels)
	mchMCEVersionCompliant.DeletePartialMatch(partialLabels)
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
