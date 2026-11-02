// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"reflect"

	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	ocv1 "github.com/operator-framework/operator-controller/api/v1"
	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// mceImageRepositoryAnnotation is the only MCE annotation the operator manages.
const mceImageRepositoryAnnotation = "imageRepository"

// mceNeedsUpdate reports whether the desired MultiClusterEngine differs from the live one in
// the fields the operator owns: its spec and the image repository annotation.
func mceNeedsUpdate(desired, live *mcev1.MultiClusterEngine) bool {
	if !equality.Semantic.DeepEqual(desired.Spec, live.Spec) {
		return true
	}
	return desired.GetAnnotations()[mceImageRepositoryAnnotation] !=
		live.GetAnnotations()[mceImageRepositoryAnnotation]
}

// subscriptionNeedsUpdate reports whether the desired MCE Subscription spec differs from the
// live one.
func subscriptionNeedsUpdate(desired, live *subv1alpha1.Subscription) bool {
	return !equality.Semantic.DeepEqual(desired.Spec, live.Spec)
}

// clusterExtensionNeedsUpdate reports whether the desired MCE ClusterExtension spec differs
// from the live one.
func clusterExtensionNeedsUpdate(desired, live *ocv1.ClusterExtension) bool {
	return !equality.Semantic.DeepEqual(desired.Spec, live.Spec)
}

// unstructuredContains reports whether every field present in desired is also present in live
// with an equal value. Fields that exist only on the live object (server-defaulted values,
// status, or fields written by other controllers) are ignored.
func unstructuredContains(live, desired interface{}) bool {
	switch desiredValue := desired.(type) {
	case map[string]interface{}:
		liveValue, ok := live.(map[string]interface{})
		if !ok {
			return false
		}
		for key, value := range desiredValue {
			liveField, found := liveValue[key]
			if !found || !unstructuredContains(liveField, value) {
				return false
			}
		}
		return true
	case []interface{}:
		liveValue, ok := live.([]interface{})
		if !ok || len(liveValue) != len(desiredValue) {
			return false
		}
		for i := range desiredValue {
			if !unstructuredContains(liveValue[i], desiredValue[i]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(live, desired)
	}
}

// desiredStateMatches reports whether the live object already contains every field declared
// in the desired object. It is used to skip writes for resources rendered from templates:
// Kubernetes defaults fields the template does not set, so a strict comparison would always
// report a difference.
func desiredStateMatches(live, desired *unstructured.Unstructured) bool {
	return unstructuredContains(live.Object, desired.Object)
}
