// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package utils

import (
	"fmt"
	"strings"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"k8s.io/apimachinery/pkg/types"
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
	// AnnotationMCESubscriptionSpec sits in multiclusterhub annotations to identify the subscription spec last used to create the multiclustengine
	AnnotationMCESubscriptionSpec = "installer.open-cluster-management.io/mce-subscription-spec"
	// AnnotationOADPSubscriptionSpec overrides the OADP subscription used in cluster-backup
	AnnotationOADPSubscriptionSpec = "installer.open-cluster-management.io/oadp-subscription-spec"
	// AnnotationIgnoreOCPVersion indicates the operator should not check the OCP version before proceeding when set
	AnnotationIgnoreOCPVersion = "ignoreOCPVersion"

	// AnnotationKubeconfig is the secret name residing in targetcontaining the kubeconfig to access the remote cluster
	AnnotationKubeconfig = "mch-kubeconfig"
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
		old[AnnotationImageOverridesCM] == new[AnnotationImageOverridesCM] &&
		old[AnnotationMCESubscriptionSpec] == new[AnnotationMCESubscriptionSpec] &&
		old[AnnotationOADPSubscriptionSpec] == new[AnnotationOADPSubscriptionSpec]
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

func GetMCEAnnotationOverrides(instance *operatorsv1.MultiClusterHub) string {
	return getAnnotation(instance, AnnotationMCESubscriptionSpec)
}

func GetOADPAnnotationOverrides(instance *operatorsv1.MultiClusterHub) string {
	return getAnnotation(instance, AnnotationOADPSubscriptionSpec)
}

// ShouldIgnoreOCPVersion returns true if the instance is annotated to skip
// the minimum OCP version requirement
func ShouldIgnoreOCPVersion(instance *operatorsv1.MultiClusterHub) bool {
	a := instance.GetAnnotations()
	if a == nil {
		return false
	}

	if _, ok := a[AnnotationIgnoreOCPVersion]; ok {
		return true
	}
	return false
}

// GetHostedCredentialsSecret returns the secret namespacedName containing the kubeconfig
// to access the hosted cluster
func GetHostedCredentialsSecret(mch *operatorsv1.MultiClusterHub) (types.NamespacedName, error) {
	nn := types.NamespacedName{}
	if mch.Annotations == nil || mch.Annotations[AnnotationKubeconfig] == "" {
		return nn, fmt.Errorf("no kubeconfig secret annotation defined in %s", mch.Name)
	}
	nn.Name = mch.Annotations[AnnotationKubeconfig]
	nn.Namespace = mch.Namespace
	return nn, nil
}
