// Copyright (c) 2020 Red Hat, Inc.

package utils

import (
	"strings"

	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1"
)

var (
	// AnnotationMCHPause sits in multiclusterhub annotations to identify if the multiclusterhub is paused or not
	AnnotationMCHPause = "mch-pause"
	// AnnotationImageRepo sits in multiclusterhub annotations to identify a custom image repository to use
	AnnotationImageRepo = "mch-imageRepository"
	// AnnotationSuffix sits in multiclusterhub annotations to identify a custom image tag suffix to use
	AnnotationSuffix = "mch-imageTagSuffix"
)

// IsPaused returns true if the multiclusterhub instance is labeled as paused, and false otherwise
func IsPaused(instance *operatorsv1.MultiClusterHub) bool {
	a := instance.GetAnnotations()
	if a == nil {
		return false
	}

	if a[AnnotationMCHPause] != "" && strings.EqualFold(a[AnnotationMCHPause], "true") {
		return true
	}

	return false
}

// AnnotationsMatch returns true if all annotation values used by the operator match
func AnnotationsMatch(old, new map[string]string) bool {
	return old[AnnotationMCHPause] == new[AnnotationMCHPause] &&
		old[AnnotationImageRepo] == new[AnnotationImageRepo] &&
		old[AnnotationSuffix] == new[AnnotationSuffix]
}

// getAnnotation returns the annotation value for a given key, or an empty string if not set
func getAnnotation(instance *operatorsv1.MultiClusterHub, key string) string {
	a := instance.GetAnnotations()
	if a == nil {
		return ""
	}
	return a[key]
}

// GetImageRepository returns the image repo annotation, or an empty string if not set
func GetImageRepository(instance *operatorsv1.MultiClusterHub) string {
	return getAnnotation(instance, AnnotationImageRepo)
}

// GetImageSuffix returns the image tag suffix annotation, or an empty string if not set
func GetImageSuffix(instance *operatorsv1.MultiClusterHub) string {
	return getAnnotation(instance, AnnotationSuffix)
}
