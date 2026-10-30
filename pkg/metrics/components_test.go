// Copyright (c) 2026 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package metrics

import (
	"testing"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"

	"github.com/prometheus/client_golang/prometheus/testutil"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func componentConditionValue(namespace, name, component, kind, condition string) float64 {
	return testutil.ToFloat64(mchComponentCondition.WithLabelValues(namespace, name, component, kind, condition))
}

func componentEnabledValue(namespace, name, component string) float64 {
	return testutil.ToFloat64(mchComponentEnabled.WithLabelValues(namespace, name, component))
}

func healthyComponent(name, kind string) operatorsv1.StatusCondition {
	return operatorsv1.StatusCondition{
		Name:   name,
		Kind:   kind,
		Type:   "Available",
		Status: metav1.ConditionTrue,
	}
}

func TestSetMCHComponentMetricsPublishesHealthAndEnabledState(t *testing.T) {
	const namespace, name = "components-publish", "multiclusterhub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	components := map[string]operatorsv1.StatusCondition{
		"console": healthyComponent("console", "Deployment"),
	}
	enabled := map[string]bool{"console": true, "search": false}

	if changed := SetMCHComponentMetrics(namespace, name, components, enabled); !changed {
		t.Fatal("first publish reported no change, want change")
	}

	if got := componentConditionValue(namespace, name, "console", "Deployment", "Available"); got != 1 {
		t.Fatalf("console Available = %v, want 1", got)
	}
	if got := componentEnabledValue(namespace, name, "console"); got != 1 {
		t.Fatalf("console enabled = %v, want 1", got)
	}
	if got := componentEnabledValue(namespace, name, "search"); got != 0 {
		t.Fatalf("search enabled = %v, want 0", got)
	}
	if count := testutil.CollectAndCount(mchComponentCondition, ComponentConditionMetricName); count != 1 {
		t.Fatalf("condition series = %d, want 1", count)
	}
	if count := testutil.CollectAndCount(mchComponentEnabled, ComponentEnabledMetricName); count != 2 {
		t.Fatalf("enabled series = %d, want 2", count)
	}
}

func TestSetMCHComponentMetricsReportsChangeOnlyOnTransition(t *testing.T) {
	const namespace, name = "components-changed", "multiclusterhub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	components := map[string]operatorsv1.StatusCondition{
		"console": healthyComponent("console", "Deployment"),
	}
	enabled := map[string]bool{"console": true}

	if changed := SetMCHComponentMetrics(namespace, name, components, enabled); !changed {
		t.Fatal("first publish reported no change, want change")
	}
	if changed := SetMCHComponentMetrics(namespace, name, components, enabled); changed {
		t.Fatal("republishing identical state reported a change")
	}

	unhealthy := map[string]operatorsv1.StatusCondition{
		"console": {
			Name:   "console",
			Kind:   "Deployment",
			Type:   "Available",
			Status: metav1.ConditionFalse,
			Reason: "MinimumReplicasUnavailable",
		},
	}
	if changed := SetMCHComponentMetrics(namespace, name, unhealthy, enabled); !changed {
		t.Fatal("health transition reported no change, want change")
	}
	if got := componentConditionValue(namespace, name, "console", "Deployment", "Available"); got != 0 {
		t.Fatalf("console Available = %v after transition, want 0", got)
	}
	if count := testutil.CollectAndCount(mchComponentCondition, ComponentConditionMetricName); count != 1 {
		t.Fatalf("condition series = %d after transition, want 1", count)
	}
}

func TestSetMCHComponentMetricsRemovesHealthSeriesWhenDisabled(t *testing.T) {
	const namespace, name = "components-disabled", "multiclusterhub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	components := map[string]operatorsv1.StatusCondition{
		"console": healthyComponent("console", "Deployment"),
		"search":  healthyComponent("search", "Deployment"),
	}
	enabled := map[string]bool{"console": true, "search": true}
	SetMCHComponentMetrics(namespace, name, components, enabled)

	// Disabling search removes it from the status components but keeps it in the
	// known component list, so only its health series goes away.
	components = map[string]operatorsv1.StatusCondition{
		"console": healthyComponent("console", "Deployment"),
	}
	enabled = map[string]bool{"console": true, "search": false}
	SetMCHComponentMetrics(namespace, name, components, enabled)

	if count := testutil.CollectAndCount(mchComponentCondition, ComponentConditionMetricName); count != 1 {
		t.Fatalf("condition series = %d after disabling a component, want 1", count)
	}
	if got := componentConditionValue(namespace, name, "console", "Deployment", "Available"); got != 1 {
		t.Fatalf("console Available = %v, want 1", got)
	}
	if got := componentEnabledValue(namespace, name, "search"); got != 0 {
		t.Fatalf("search enabled = %v after disabling, want 0", got)
	}
}

func TestSetMCHComponentMetricsReportsUnknownAsZero(t *testing.T) {
	const namespace, name = "components-unknown", "multiclusterhub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	components := map[string]operatorsv1.StatusCondition{
		"console": {
			Name:   "console",
			Kind:   "Deployment",
			Type:   "Available",
			Status: metav1.ConditionUnknown,
		},
	}
	SetMCHComponentMetrics(namespace, name, components, map[string]bool{"console": true})

	if got := componentConditionValue(namespace, name, "console", "Deployment", "Available"); got != 0 {
		t.Fatalf("unknown condition = %v, want 0", got)
	}
}

func TestSetMCHComponentMetricsHandlesMultipleComponents(t *testing.T) {
	const namespace, name = "components-multiple", "multiclusterhub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	components := map[string]operatorsv1.StatusCondition{
		"console":                    healthyComponent("console", "Deployment"),
		"multicluster-engine-sub":    healthyComponent("multicluster-engine", "Subscription"),
		"multicluster-engine":        healthyComponent("multicluster-engine", "MultiClusterEngine"),
		"search-v2-operator":         healthyComponent("search-v2-operator", "Deployment"),
		"multicluster-engine-csv":    healthyComponent("multicluster-engine", "ClusterServiceVersion"),
		"multicluster-engine-cr-ext": healthyComponent("multicluster-engine", "ClusterExtension"),
	}
	SetMCHComponentMetrics(namespace, name, components, map[string]bool{
		"console": true, "multicluster-engine": true, "search": true,
	})

	if count := testutil.CollectAndCount(mchComponentCondition, ComponentConditionMetricName); count != len(components) {
		t.Fatalf("condition series = %d, want %d", count, len(components))
	}
	if got := componentConditionValue(namespace, name, "multicluster-engine", "Subscription", "Available"); got != 1 {
		t.Fatalf("subscription Available = %v, want 1", got)
	}
	if got := componentConditionValue(namespace, name, "multicluster-engine", "MultiClusterEngine", "Available"); got != 1 {
		t.Fatalf("MultiClusterEngine Available = %v, want 1", got)
	}
}

func TestSetMCHComponentMetricsFallsBackToKeyAndAvailableCondition(t *testing.T) {
	const namespace, name = "components-fallback", "multiclusterhub"
	t.Cleanup(func() { DeleteMCHMetrics(namespace, name) })

	components := map[string]operatorsv1.StatusCondition{
		"console": {
			Kind:   "Deployment",
			Status: metav1.ConditionTrue,
		},
	}
	SetMCHComponentMetrics(namespace, name, components, nil)

	if got := componentConditionValue(namespace, name, "console", "Deployment", "Available"); got != 1 {
		t.Fatalf("fallback series = %v, want 1", got)
	}
}

func TestSetMCHComponentMetricsRemovesSeriesOnDelete(t *testing.T) {
	const namespace, name = "components-delete", "multiclusterhub"

	SetMCHComponentMetrics(namespace, name,
		map[string]operatorsv1.StatusCondition{"console": healthyComponent("console", "Deployment")},
		map[string]bool{"console": true})

	if removed := DeleteMCHMetrics(namespace, name); !removed {
		t.Fatal("delete reported no removal after component metrics were published")
	}
	if count := testutil.CollectAndCount(mchComponentCondition, ComponentConditionMetricName); count != 0 {
		t.Fatalf("condition series = %d after delete, want 0", count)
	}
	if count := testutil.CollectAndCount(mchComponentEnabled, ComponentEnabledMetricName); count != 0 {
		t.Fatalf("enabled series = %d after delete, want 0", count)
	}
	if removed := DeleteMCHMetrics(namespace, name); removed {
		t.Fatal("repeated delete reported a removal, want false")
	}
}

func TestSetMCHComponentMetricsIsolatesHubInstances(t *testing.T) {
	const name = "multiclusterhub"
	t.Cleanup(func() {
		DeleteMCHMetrics("hub-one", name)
		DeleteMCHMetrics("hub-two", name)
	})

	SetMCHComponentMetrics("hub-one", name,
		map[string]operatorsv1.StatusCondition{"console": healthyComponent("console", "Deployment")},
		map[string]bool{"console": true})
	SetMCHComponentMetrics("hub-two", name,
		map[string]operatorsv1.StatusCondition{"console": {
			Name: "console", Kind: "Deployment", Type: "Available", Status: metav1.ConditionFalse,
		}},
		map[string]bool{"console": true})

	if got := componentConditionValue("hub-one", name, "console", "Deployment", "Available"); got != 1 {
		t.Fatalf("hub-one console Available = %v, want 1", got)
	}
	if got := componentConditionValue("hub-two", name, "console", "Deployment", "Available"); got != 0 {
		t.Fatalf("hub-two console Available = %v, want 0", got)
	}

	DeleteMCHMetrics("hub-one", name)

	if got := componentConditionValue("hub-two", name, "console", "Deployment", "Available"); got != 0 {
		t.Fatalf("hub-two console Available = %v after deleting hub-one, want 0", got)
	}
}
