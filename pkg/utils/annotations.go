// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package utils

import (
	"fmt"
	"strings"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/pkg/apis/operator/v1"
)

var (
	// AnnotationMCHPause sits in multiclusterhub annotations to identify if the multiclusterhub is paused or not
	AnnotationMCHPause = "mch-pause"
	// AnnotationImageRepo sits in multiclusterhub annotations to identify a custom image repository to use
	AnnotationImageRepo = "mch-imageRepository"
	// AnnotationImageOverridesCM sits in multiclusterhub annotations to identify a custom configmap containing image overrides
	AnnotationImageOverridesCM = "mch-imageOverridesCM"
	// AnnotationConfiguration sits in a resource's annotations to identify the configuration last used to create it
	AnnotationConfiguration = "installer.open-cluster-management.io/last-applied-configuration"
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
		old[AnnotationImageOverridesCM] == new[AnnotationImageOverridesCM]
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

// GetImageOverridesConfigmap returns the images override configmap annotation, or an empty string if not set
func GetImageOverridesConfigmap(instance *operatorsv1.MultiClusterHub) string {
	return getAnnotation(instance, AnnotationImageOverridesCM)
}

func OverrideImageRepository(imageOverrides map[string]string, imageRepo string) map[string]string {
	for imageKey, imageRef := range imageOverrides {
		image := strings.LastIndex(imageRef, "/")
		imageOverrides[imageKey] = fmt.Sprintf("%s%s", imageRepo, imageRef[image:])
	}
	return imageOverrides
}
