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

	/*
		AnnotationMCHPause is a key that is utilized within the annotations of a multiclusterhub resource.
		It serves the purpose of identifying whether the multiclusterhub is currently in a paused state or not.
		The presence of this annotation, if set to "mch-pause," indicates that the multiclusterhub has been
		intentionally paused, possibly for maintenance or operational reasons. Pausing a multiclusterhub
		typically involves suspending various processes and controllers that might otherwise be active.
	*/
	AnnotationMCHPause = "mch-pause"

	/*
		AnnotationImageRepo is a key used within the annotations of a multiclusterhub resource.
		Its primary role is to specify a custom image repository that should be utilized by the multiclusterhub.
		By setting this annotation to "mch-imageRepository," administrators can configure the multiclusterhub to pull
		its container images from a specific repository of their choice. This customization allows for greater
		flexibility in managing the sources of container images used by the multiclusterhub.
	*/
	AnnotationImageRepo = "mch-imageRepository"

	/*
		AnnotationImageOverridesCM is employed within the annotations of a multiclusterhub resource.
		Its primary function is to point to a custom ConfigMap that contains image overrides. Image overrides typically
		provide instructions on which container images should be used for specific components or features within the
		multiclusterhub. By referencing this annotation, administrators can ensure that the multiclusterhub uses the
		specified images as configured in the associated ConfigMap.
	*/
	AnnotationImageOverridesCM = "mch-imageOverridesCM"

	/*
		AnnotationConfiguration is a key that can be found within the annotations of various resources.
		Its primary purpose is to identify the last applied configuration used to create or modify a particular
		resource. This annotation helps to keep track of the configuration changes and settings that were used during
		the creation or update of a resource, aiding in auditing and understanding the history of
		resource modifications.
	*/
	AnnotationConfiguration = "installer.open-cluster-management.io/last-applied-configuration"

	/*
		AnnotationMCESubscriptionSpec is used within the annotations of multiclusterhub resources.
		It serves to specify the subscription specification that was last used to create the multiclustengine.
		A multiclustengine typically represents a component responsible for managing and coordinating operations
		across multiple clusters. This annotation helps in tracking and referencing the specific subscription
		specification that governs the behavior of the multiclustengine.
	*/
	AnnotationMCESubscriptionSpec = "installer.open-cluster-management.io/mce-subscription-spec"

	/*
		AnnotationOADPSubscriptionSpec is utilized to override the subscription used in the context of cluster backup
		operations, particularly for Open Application Data Platform (OADP) subscription management. By applying this
		annotation, administrators can specify an alternative subscription specification that should be used for the
		cluster backup process. This is useful for customizing the backup mechanism according
		to specific requirements.
	*/
	AnnotationOADPSubscriptionSpec = "installer.open-cluster-management.io/oadp-subscription-spec"

	/*
		AnnotationIgnoreOCPVersion is employed as an indicator within resource annotations. When this annotation is set,
		it instructs the associated operator to bypass the standard OCP (OpenShift Container Platform) version checks
		before proceeding with its operations. This can be valuable in situations where a specific operator behavior
		needs to be maintained, regardless of the underlying OCP version, possibly to ensure compatibility or to
		address specific operational needs.
	*/
	AnnotationIgnoreOCPVersion = "ignoreOCPVersion"
	// AnnotationReleaseVersion indicates the release version that should be applied to all resources managed by MCH operator
	AnnotationReleaseVersion = "installer.open-cluster-management.io/release-version"

	/*
		AnnotationKubeconfig is used to specify the secret name within a target resource.
		This secret name typically contains the kubeconfig file required to access and interact with a remote cluster.
		By utilizing this annotation, it's possible to identify the location of the kubeconfig information within the
		resource, making it easier for administrators and components to locate and use the necessary configuration
		details for remote cluster access.
	*/
	AnnotationKubeconfig = "mch-kubeconfig"
)

/*
IsPaused returns true if the MultiClusterHub instance is labeled as paused, and false otherwise.
It takes a MultiClusterHub instance (instance) as input and checks its annotations for the "paused" label.
If the label is present and set to "true" (case-insensitive), the function returns true; otherwise, it returns false.
*/
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

/*
AnnotationsMatch returns true if all annotation values used by the operator match.
It compares two maps of annotation values (old and new) and checks whether they match.
It specifically checks the values of annotations related to the operator's behavior.
*/
func AnnotationsMatch(old, new map[string]string) bool {
	return old[AnnotationMCHPause] == new[AnnotationMCHPause] &&
		old[AnnotationImageRepo] == new[AnnotationImageRepo] &&
		old[AnnotationImageOverridesCM] == new[AnnotationImageOverridesCM] &&
		old[AnnotationMCESubscriptionSpec] == new[AnnotationMCESubscriptionSpec] &&
		old[AnnotationOADPSubscriptionSpec] == new[AnnotationOADPSubscriptionSpec]
}

/*
getAnnotation returns the annotation value for a given key, or an empty string if not set.
It takes a MultiClusterHub instance (instance) and a key for the annotation.
The function retrieves the instance's annotations and returns the value associated with the given key.
If the key is not set in the annotations, it returns an empty string.
*/
func getAnnotation(instance *operatorsv1.MultiClusterHub, key string) string {
	a := instance.GetAnnotations()
	if a == nil {
		return ""
	}
	return a[key]
}

/*
GetImageRepository returns the image repository annotation, or an empty string if not set.
It takes a MultiClusterHub instance (instance) as input and uses the getAnnotation function to
retrieve the value of the image repository annotation.
*/
func GetImageRepository(instance *operatorsv1.MultiClusterHub) string {
	return getAnnotation(instance, AnnotationImageRepo)
}

/*
GetImageOverridesConfigmap returns the image overrides configmap annotation, or an empty string if not set.
It takes a MultiClusterHub instance (instance) as input and uses the getAnnotation function to
retrieve the value of the image overrides configmap annotation.
*/
func GetImageOverridesConfigmap(instance *operatorsv1.MultiClusterHub) string {
	return getAnnotation(instance, AnnotationImageOverridesCM)
}

/*
OverrideImageRepository overrides image references in a map with the specified image repository.
It takes a map of image references (imageOverrides) and an image repository (imageRepo).
The function updates the image references in the map by replacing the image repository part with the provided
image repository. It returns the updated map of image references.
*/
func OverrideImageRepository(imageOverrides map[string]string, imageRepo string) map[string]string {
	for imageKey, imageRef := range imageOverrides {
		image := strings.LastIndex(imageRef, "/")
		imageOverrides[imageKey] = fmt.Sprintf("%s%s", imageRepo, imageRef[image:])
	}
	return imageOverrides
}

/*
GetMCEAnnotationOverrides returns the ManagedClusterEngine (MCE) subscription spec annotation, or an empty string if
not set. It takes a MultiClusterHub instance (instance) as input and uses the getAnnotation function to retrieve the
value of the MCE subscription spec annotation.
*/
func GetMCEAnnotationOverrides(instance *operatorsv1.MultiClusterHub) string {
	return getAnnotation(instance, AnnotationMCESubscriptionSpec)
}

/*
GetOADPAnnotationOverrides returns the OpenShift Application Data Protection (OADP) subscription spec annotation,
or an empty string if not set. It takes a MultiClusterHub instance (instance) as input and uses the getAnnotation
function to retrieve the value of the OADP subscription spec annotation.
*/
func GetOADPAnnotationOverrides(instance *operatorsv1.MultiClusterHub) string {
	return getAnnotation(instance, AnnotationOADPSubscriptionSpec)
}

/*
ShouldIgnoreOCPVersion returns true if the instance is annotated to skip the minimum OpenShift version requirement.
It checks the annotations of a MultiClusterHub instance to see if it has an annotation indicating that the minimum
OpenShift version requirement should be ignored.
*/
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

/*
GetHostedCredentialsSecret returns the NamespacedName of the secret containing the kubeconfig to access the hosted
cluster. It takes a MultiClusterHub instance (mch) as input and retrieves the NamespacedName of the kubeconfig
secret from the instance's annotations. If the annotation is not set or if it's empty, an error is returned
*/
func GetHostedCredentialsSecret(mch *operatorsv1.MultiClusterHub) (types.NamespacedName, error) {
	nn := types.NamespacedName{}
	if mch.Annotations == nil || mch.Annotations[AnnotationKubeconfig] == "" {
		return nn, fmt.Errorf("no kubeconfig secret annotation defined in %s", mch.Name)
	}
	nn.Name = mch.Annotations[AnnotationKubeconfig]
	nn.Namespace = mch.Namespace
	return nn, nil
}
