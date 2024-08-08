// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package utils

import (
	"fmt"
	"strings"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
)

var (
	/*
		AnnotationConfiguration is an annotation used in a resource's metadata to identify the configuration
		last used to create it.
	*/
	AnnotationConfiguration = "installer.open-cluster-management.io/last-applied-configuration"

	/*
		AnnotationIgnoreOCPVersion is an annotation used to indicate the operator should not check the OpenShift
		Container Platform (OCP) version before proceeding when set.
	*/
	AnnotationIgnoreOCPVersion           = "installer.open-cluster-management.io/ignore-ocp-version"
	DeprecatedAnnotationIgnoreOCPVersion = "ignoreOCPVersion"

	/*
		AnnotationImageOverridesCM is an annotation used in multiclusterhub to specify a custom ConfigMap containing
		image overrides.
	*/
	AnnotationImageOverridesCM           = "installer.open-cluster-management.io/image-overrides-configmap"
	DeprecatedAnnotationImageOverridesCM = "mch-imageOverridesCM"

	/*
		AnnotationImageRepo is an annotation used in multiclusterhub to specify a custom image repository to use.
	*/
	AnnotationImageRepo           = "installer.open-cluster-management.io/image-repository"
	DeprecatedAnnotationImageRepo = "mch-imageRepository"

	/*
		AnnotationKubeconfig is an annotation used to specify the secret name residing in target containing the
		kubeconfig to access the remote cluster.
	*/
	AnnotationKubeconfig           = "installer.open-cluster-management.io/kubeconfig"
	DeprecatedAnnotationKubeconfig = "mch-kubeconfig"

	/*
		AnnotationMCHPause is an annotation used in multiclusterhub to identify if the multiclusterhub is paused or not.
	*/
	AnnotationMCHPause           = "installer.open-cluster-management.io/pause"
	DeprecatedAnnotationMCHPause = "mch-pause"

	/*
		AnnotationMCESubscriptionSpec is an annotation used in multiclusterhub to identify the subscription spec
		last used to create the multiclustengine.
	*/
	AnnotationMCESubscriptionSpec = "installer.open-cluster-management.io/mce-subscription-spec"

	/*
		AnnotationOADPSubscriptionSpec is an annotation used to override the OADP subscription used in cluster-backup.
	*/
	AnnotationOADPSubscriptionSpec = "installer.open-cluster-management.io/oadp-subscription-spec"

	/*
		AnnotationReleaseVersion is an annotation used to indicate the release version that should be applied to all
		resources managed by the MCH operator.
	*/
	AnnotationReleaseVersion = "installer.open-cluster-management.io/release-version"

	/*
		AnnotationTemplateOverridesCM is an annotation used in multiclusterhub to specify a custom ConfigMap
		containing resource template overrides.
	*/
	AnnotationTemplateOverridesCM = "installer.open-cluster-management.io/template-override-configmap"

	/*
		AnnotationHubSize is an annotation used in multiclusterhub to specify a hub size that can be
		used by other components
	*/
	AnnotationHubSize = "installer.open-cluster-management.io/hub-size"
)

/*
IsPaused checks if the MultiClusterHub instance is labeled as paused.
It returns true if the instance is paused, otherwise false.
*/
func IsPaused(instance *operatorsv1.MultiClusterHub) bool {
	return IsAnnotationTrue(instance, AnnotationMCHPause) || IsAnnotationTrue(instance, DeprecatedAnnotationMCHPause)
}

/*
IsComponentPaused checks if the MultiClusterHub instance components is labeled as paused.
It returns true if the instance is paused, otherwise false.
*/
func IsComponentPaused(instance *operatorsv1.MultiClusterHub) bool {
	if len(instance.Spec.Overrides.Components) == 0 {
		return false
	}

	for _, c := range instance.Spec.Overrides.Components {
		if c.Paused {
			return true
		}
	}

	return false
}

/*
GetHubSize gets the current hubsize, returning "Small" as default if the annotation is not found.
*/
func GetHubSize(instance *operatorsv1.MultiClusterHub) operatorsv1.HubSize {
	hubsize := getAnnotation(instance, AnnotationHubSize)
	if hubsize != "" {
		return operatorsv1.HubSize(hubsize)
	}
	return operatorsv1.Small
}

/*
IsAnnotationTrue checks if a specific annotation key in the given instance is set to "true".
*/
func IsAnnotationTrue(instance *operatorsv1.MultiClusterHub, annotationKey string) bool {
	a := instance.GetAnnotations()
	if a == nil {
		return false
	}

	value := strings.EqualFold(a[annotationKey], "true")
	return value
}

/*
AnnotationsMatch checks if all specified annotations in the 'old' map match the corresponding ones in the 'new' map.
It returns true if all annotations match, otherwise false.
*/
func AnnotationsMatch(old, new map[string]string) bool {
	return getAnnotationOrDefaultForMap(old, new, AnnotationMCHPause, DeprecatedAnnotationMCHPause) &&
		getAnnotationOrDefaultForMap(old, new, AnnotationImageRepo, DeprecatedAnnotationImageRepo) &&
		getAnnotationOrDefaultForMap(old, new, AnnotationImageOverridesCM, DeprecatedAnnotationImageOverridesCM) &&
		getAnnotationOrDefaultForMap(old, new, AnnotationKubeconfig, DeprecatedAnnotationKubeconfig) &&
		getAnnotationOrDefaultForMap(old, new, AnnotationTemplateOverridesCM, "") &&
		getAnnotationOrDefaultForMap(old, new, AnnotationMCESubscriptionSpec, "") &&
		getAnnotationOrDefaultForMap(old, new, AnnotationOADPSubscriptionSpec, "")
}

/*
GetAnnotation returns the annotation value for a given key from the instance's annotations,
or an empty string if the annotation is not set.
*/
func getAnnotation(instance *operatorsv1.MultiClusterHub, key string) string {
	a := instance.GetAnnotations()
	if a == nil {
		return ""
	}

	return a[key]
}

/*
getAnnotationOrDefault retrieves the value of the primary annotation key,
falling back to the deprecated key if the primary key is not set.
*/
func getAnnotationOrDefault(instance *operatorsv1.MultiClusterHub, primaryKey, deprecatedKey string) string {
	primaryValue := getAnnotation(instance, primaryKey)
	if primaryValue != "" {
		return primaryValue
	}

	return getAnnotation(instance, deprecatedKey)
}

/*
getAnnotationOrDefaultForMap checks if the annotation value from the 'old' map matches the one from the 'new' map,
including deprecated annotations.
*/
func getAnnotationOrDefaultForMap(old, new map[string]string, primaryKey, deprecatedKey string) bool {
	oldValue := old[primaryKey]

	if oldValue == "" {
		oldValue = old[deprecatedKey]
	}

	newValue := new[primaryKey]
	if newValue == "" {
		newValue = new[deprecatedKey]
	}

	return oldValue == newValue
}

/*
GetImageRepository returns the image repository annotation value,
using the primary annotation key and falling back to the deprecated key if not set.
*/
func GetImageRepository(instance *operatorsv1.MultiClusterHub) string {
	return getAnnotationOrDefault(instance, AnnotationImageRepo, DeprecatedAnnotationImageRepo)
}

/*
GetImageOverridesConfigmapName returns the image overrides ConfigMap annotation value,
using the primary annotation key and falling back to the deprecated key if not set.
*/
func GetImageOverridesConfigmapName(instance *operatorsv1.MultiClusterHub) string {
	return getAnnotationOrDefault(instance, AnnotationImageOverridesCM, DeprecatedAnnotationImageOverridesCM)
}

/*
GetMCEAnnotationOverrides returns the MulticlusterEngine subscription spec annotation value,
or an empty string if not set.
*/
func GetMCEAnnotationOverrides(instance *operatorsv1.MultiClusterHub) string {
	return getAnnotation(instance, AnnotationMCESubscriptionSpec)
}

/*
GetOADPAnnotationOverrides returns the OADP subscription spec annotation value,
or an empty string if not set.
*/
func GetOADPAnnotationOverrides(instance *operatorsv1.MultiClusterHub) string {
	return getAnnotation(instance, AnnotationOADPSubscriptionSpec)
}

/*
GetTemplateOverridesConfigmapName returns the template overrides ConfigMap annotation value,
or an empty string if not set.
*/
func GetTemplateOverridesConfigmapName(instance *operatorsv1.MultiClusterHub) string {
	return getAnnotation(instance, AnnotationTemplateOverridesCM)
}

/*
HasAnnotation checks if a specific annotation key exists in the instance's annotations.
*/
func HasAnnotation(instance *operatorsv1.MultiClusterHub, annotationKey string) bool {
	a := instance.GetAnnotations()
	if a == nil {
		return false
	}

	_, exists := a[annotationKey]
	return exists
}

/*
OverrideImageRepository modifies image references in a map to use a specified image repository.
*/
func OverrideImageRepository(imageOverrides map[string]string, imageRepo string) map[string]string {
	for imageKey, imageRef := range imageOverrides {
		image := strings.LastIndex(imageRef, "/")
		imageOverrides[imageKey] = fmt.Sprintf("%s%s", imageRepo, imageRef[image:])
	}
	return imageOverrides
}

/*
ShouldIgnoreOCPVersion checks if the instance is annotated to skip the minimum OCP version requirement.
*/
func ShouldIgnoreOCPVersion(instance *operatorsv1.MultiClusterHub) bool {
	return HasAnnotation(instance, AnnotationIgnoreOCPVersion) ||
		HasAnnotation(instance, DeprecatedAnnotationIgnoreOCPVersion)
}
