// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package channel

import (
	"fmt"

	"github.com/open-cluster-management/multiclusterhub-operator/version"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	operatorsv1 "github.com/open-cluster-management/multiclusterhub-operator/pkg/apis/operator/v1"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/helmrepo"
)

// ChannelName is the name of the open-cluster-management.io channel
var ChannelName = "charts-v1"

// Schema is the GVK for an application subscription channel
var Schema = schema.GroupVersionResource{Group: "apps.open-cluster-management.io", Version: "v1", Resource: "channels"}

// custom annotation to reduce reconcilation rate to once per hour (default is 15 minutes)
var AnnotationRateLow = map[string]string{
	"apps.open-cluster-management.io/reconcile-rate": "low",
}

// custom annotation to increase reconcilation rate to once every two minutes (default is 15 minutes)
var AnnotationRateHigh = map[string]string{
	"apps.open-cluster-management.io/reconcile-rate": "high",
}

// build Helm pathname from repo name and por
func channelURL(m *operatorsv1.MultiClusterHub) string {
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d/charts", helmrepo.HelmRepoName, m.Namespace, helmrepo.Port)
}

// Channel returns an unstructured Channel object to watch the helm repository
func Channel(m *operatorsv1.MultiClusterHub) *unstructured.Unstructured {
	ch := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps.open-cluster-management.io/v1",
			"kind":       "Channel",
			"metadata": map[string]interface{}{
				"name":      ChannelName,
				"namespace": m.Namespace,
			},
			"spec": map[string]interface{}{
				"type":     "HelmRepo",
				"pathname": channelURL(m),
			},
		},
	}
	if m.Status.CurrentVersion != version.Version {
		ch.SetAnnotations(AnnotationRateHigh)
	} else {
		ch.SetAnnotations(AnnotationRateLow)
	}

	ch.SetOwnerReferences([]metav1.OwnerReference{
		*metav1.NewControllerRef(m, m.GetObjectKind().GroupVersionKind()),
	})
	return ch
}

// Validate returns true if an update is needed to reconcile differences with the current spec. If an update
// is needed it returns the object with the new spec to update with.
func Validate(m *operatorsv1.MultiClusterHub, found *unstructured.Unstructured) (*unstructured.Unstructured, bool) {
	updateNeeded := false

	// Verify reconcile-rate annotation is set
	if annotationsCorrect(m, found) == false {
		setAnnotation(m, found)
		updateNeeded = true
	}
	return found, updateNeeded
}

func annotationsCorrect(m *operatorsv1.MultiClusterHub, u *unstructured.Unstructured) bool {
	a := u.GetAnnotations()
	if a == nil || a["apps.open-cluster-management.io/reconcile-rate"] != desiredRate(m) {
		return false
	}
	return true
}

func setAnnotation(m *operatorsv1.MultiClusterHub, u *unstructured.Unstructured) {
	a := u.GetAnnotations()
	if a == nil {
		u.SetAnnotations(desiredAnnotation(m))
	} else {
		a["apps.open-cluster-management.io/reconcile-rate"] = desiredRate(m)
		u.SetAnnotations(a)
	}
}

func desiredAnnotation(m *operatorsv1.MultiClusterHub) map[string]string {
	if m.Status.CurrentVersion != version.Version {
		return AnnotationRateHigh
	} else {
		return AnnotationRateLow
	}
}

func desiredRate(m *operatorsv1.MultiClusterHub) string {
	if m.Status.CurrentVersion != version.Version {
		return "high"
	} else {
		return "low"
	}
}
