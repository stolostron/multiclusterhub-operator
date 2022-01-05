// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package utils

import (
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// getMCHImageRepository...
func getMCHImageRepository() string {
	return os.Getenv("mchImageRepository")
}

// NewMultiClusterHub ...
func NewMultiClusterHub(name, namespace, imageOverridesConfigmapName string, disableHubSelfManagement bool) *unstructured.Unstructured {

	metadata := map[string]interface{}{
		"name":      name,
		"namespace": namespace,
	}

	annotations := map[string]interface{}{}

	if imageOverridesConfigmapName != "" {
		annotations["mch-imageOverridesCM"] = imageOverridesConfigmapName
	}

	if getMCHImageRepository() != "" {
		annotations["mch-imageRepository"] = getMCHImageRepository()
	}

	if len(annotations) > 0 {
		metadata["annotations"] = annotations
	}

	spec := map[string]interface{}{
		"imagePullSecret": "multiclusterhub-operator-pull-secret",
	}

	if disableHubSelfManagement {
		spec["disableHubSelfManagement"] = true
	}

	if os.Getenv("MOCK") == "true" {
		spec["availabilityConfig"] = "Basic"
	}

	mch := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "operator.open-cluster-management.io/v1",
			"kind":       "MultiClusterHub",
			"metadata":   metadata,
			"spec":       spec,
		},
	}

	return mch
}

// NewOCMSubscription ...
func NewOCMSubscription(namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "operators.coreos.com/v1alpha1",
			"kind":       "Subscription",
			"metadata": map[string]interface{}{
				"name":      os.Getenv("name"),
				"namespace": namespace,
			},
			"spec": GetSubscriptionSpec(),
		},
	}
}

// NewImageOverridesConfigmapBadImageRef ...
func NewImageOverridesConfigmapBadImageRef(name, namespace string) *corev1.ConfigMap {

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Data: map[string]string{
			"single-bad-image-reference.json": `[
				{
					"image-name": "multiclusterhub-repo",
					"image-version": "2.5",
					"git-sha256": "8b551bb18e4d89529f9b07c61b49a1dd67b5435a",
					"git-repository": "stolostron/multiclusterhub-repo",
					"image-remote": "quay.io/stolostron",
					"image-digest": "sha256:bad-image-sha",
					"image-key": "multiclusterhub_repo"
				}
			  ]`,
		},
	}
}
