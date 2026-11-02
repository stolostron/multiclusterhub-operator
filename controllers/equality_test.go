// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"testing"

	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	ocv1 "github.com/operator-framework/operator-controller/api/v1"
	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// --- unstructuredContains ---

func Test_unstructuredContains_MapSubset(t *testing.T) {
	live := map[string]interface{}{
		"a": "1",
		"b": "2",
		"c": "3",
	}
	desired := map[string]interface{}{
		"a": "1",
		"b": "2",
	}
	if !unstructuredContains(live, desired) {
		t.Error("expected live to contain desired subset")
	}
}

func Test_unstructuredContains_MissingKey(t *testing.T) {
	live := map[string]interface{}{
		"a": "1",
	}
	desired := map[string]interface{}{
		"a": "1",
		"b": "2",
	}
	if unstructuredContains(live, desired) {
		t.Error("expected mismatch when desired has key live lacks")
	}
}

func Test_unstructuredContains_SliceLengthMismatch(t *testing.T) {
	live := []interface{}{"a", "b"}
	desired := []interface{}{"a", "b", "c"}
	if unstructuredContains(live, desired) {
		t.Error("expected mismatch on slice length difference")
	}
}

func Test_unstructuredContains_NestedMatch(t *testing.T) {
	live := map[string]interface{}{
		"spec": map[string]interface{}{
			"foo": "bar",
			"baz": int64(42),
		},
		"status": map[string]interface{}{"phase": "Running"},
	}
	desired := map[string]interface{}{
		"spec": map[string]interface{}{
			"foo": "bar",
		},
	}
	if !unstructuredContains(live, desired) {
		t.Error("expected nested subset match")
	}
}

func Test_unstructuredContains_ScalarTypes(t *testing.T) {
	tests := []struct {
		name    string
		live    interface{}
		desired interface{}
		want    bool
	}{
		{"equal strings", "hello", "hello", true},
		{"different strings", "hello", "world", false},
		{"equal int64", int64(42), int64(42), true},
		{"different int64", int64(42), int64(99), false},
		{"equal bool", true, true, true},
		{"different bool", true, false, false},
		{"equal float64", float64(3.14), float64(3.14), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := unstructuredContains(tt.live, tt.desired)
			if got != tt.want {
				t.Errorf("unstructuredContains(%v, %v) = %v, want %v", tt.live, tt.desired, got, tt.want)
			}
		})
	}
}

// --- desiredStateMatches ---

func Test_desiredStateMatches_LiveHasExtraFields(t *testing.T) {
	live := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":            "test",
				"resourceVersion": "12345",
			},
			"data": map[string]interface{}{
				"key": "value",
			},
		},
	}
	desired := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name": "test",
			},
			"data": map[string]interface{}{
				"key": "value",
			},
		},
	}
	if !desiredStateMatches(live, desired) {
		t.Error("expected match when live has extra fields not in desired")
	}
}

func Test_desiredStateMatches_DesiredHasFieldLiveLacks(t *testing.T) {
	live := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name": "test",
			},
		},
	}
	desired := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name": "test",
			},
			"data": map[string]interface{}{
				"key": "value",
			},
		},
	}
	if desiredStateMatches(live, desired) {
		t.Error("expected no match when desired has field live lacks")
	}
}

// --- mceNeedsUpdate ---

func Test_mceNeedsUpdate_IdenticalSpecAndAnnotations(t *testing.T) {
	a := &mcev1.MultiClusterEngine{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{mceImageRepositoryAnnotation: "quay.io/test"},
		},
		Spec: mcev1.MultiClusterEngineSpec{
			TargetNamespace:    "ns",
			AvailabilityConfig: mcev1.HAHigh,
		},
	}
	b := a.DeepCopy()
	if mceNeedsUpdate(a, b) {
		t.Error("identical MCE should not need update")
	}
}

func Test_mceNeedsUpdate_ChangedSpec(t *testing.T) {
	desired := &mcev1.MultiClusterEngine{
		Spec: mcev1.MultiClusterEngineSpec{
			AvailabilityConfig: mcev1.HABasic,
		},
	}
	live := &mcev1.MultiClusterEngine{
		Spec: mcev1.MultiClusterEngineSpec{
			AvailabilityConfig: mcev1.HAHigh,
		},
	}
	if !mceNeedsUpdate(desired, live) {
		t.Error("spec change should require update")
	}
}

func Test_mceNeedsUpdate_ChangedImageRepositoryAnnotation(t *testing.T) {
	desired := &mcev1.MultiClusterEngine{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{mceImageRepositoryAnnotation: "quay.io/new"},
		},
	}
	live := &mcev1.MultiClusterEngine{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{mceImageRepositoryAnnotation: "quay.io/old"},
		},
	}
	if !mceNeedsUpdate(desired, live) {
		t.Error("changed imageRepository annotation should require update")
	}
}

// --- subscriptionNeedsUpdate ---

func Test_subscriptionNeedsUpdate_IdenticalSpec(t *testing.T) {
	spec := &subv1alpha1.SubscriptionSpec{
		Channel: "stable-2.8",
		Package: "multicluster-engine",
	}
	a := &subv1alpha1.Subscription{Spec: spec}
	b := &subv1alpha1.Subscription{Spec: spec.DeepCopy()}
	if subscriptionNeedsUpdate(a, b) {
		t.Error("identical subscription specs should not need update")
	}
}

func Test_subscriptionNeedsUpdate_ChangedChannel(t *testing.T) {
	desired := &subv1alpha1.Subscription{
		Spec: &subv1alpha1.SubscriptionSpec{Channel: "stable-2.9"},
	}
	live := &subv1alpha1.Subscription{
		Spec: &subv1alpha1.SubscriptionSpec{Channel: "stable-2.8"},
	}
	if !subscriptionNeedsUpdate(desired, live) {
		t.Error("changed channel should require update")
	}
}

// --- clusterExtensionNeedsUpdate ---

func Test_clusterExtensionNeedsUpdate_IdenticalSpec(t *testing.T) {
	spec := ocv1.ClusterExtensionSpec{
		Namespace: "multicluster-engine",
		Source: ocv1.SourceConfig{
			SourceType: "Catalog",
			Catalog: &ocv1.CatalogFilter{
				PackageName: "multicluster-engine",
				Channels:    []string{"stable-2.8"},
			},
		},
	}
	a := &ocv1.ClusterExtension{Spec: spec}
	b := &ocv1.ClusterExtension{Spec: *spec.DeepCopy()}
	if clusterExtensionNeedsUpdate(a, b) {
		t.Error("identical ClusterExtension specs should not need update")
	}
}

func Test_clusterExtensionNeedsUpdate_ChangedChannels(t *testing.T) {
	desired := &ocv1.ClusterExtension{
		Spec: ocv1.ClusterExtensionSpec{
			Source: ocv1.SourceConfig{
				SourceType: "Catalog",
				Catalog: &ocv1.CatalogFilter{
					Channels: []string{"stable-2.9"},
				},
			},
		},
	}
	live := &ocv1.ClusterExtension{
		Spec: ocv1.ClusterExtensionSpec{
			Source: ocv1.SourceConfig{
				SourceType: "Catalog",
				Catalog: &ocv1.CatalogFilter{
					Channels: []string{"stable-2.8"},
				},
			},
		},
	}
	if !clusterExtensionNeedsUpdate(desired, live) {
		t.Error("changed channels should require update")
	}
}
