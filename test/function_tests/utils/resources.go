// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package utils

import (
	"fmt"
	"os"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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

	annotations := map[string]interface{}{
		"installer.open-cluster-management.io/mce-subscription-spec": `{"channel": "stable-2.4","installPlanApproval": "Automatic","name": "multicluster-engine","source": "multiclusterengine-catalog","sourceNamespace": "openshift-marketplace"}`,
	}

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

// NewMCHTolerations ...
func NewMCHTolerations(name, namespace, imageOverridesConfigmapName string, disableHubSelfManagement bool) *unstructured.Unstructured {

	metadata := map[string]interface{}{
		"name":      name,
		"namespace": namespace,
	}

	annotations := map[string]interface{}{
		"installer.open-cluster-management.io/mce-subscription-spec": `{"channel": "stable-2.4","installPlanApproval": "Automatic","name": "multicluster-engine","source": "multiclusterengine-catalog","sourceNamespace": "openshift-marketplace"}`,
	}

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
		"tolerations":     UnstructuredTestTolerations(),
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

func TestTolerations() []corev1.Toleration {
	return []corev1.Toleration{
		corev1.Toleration{
			Key:      "node-role.kubernetes.io/infra",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		},
		corev1.Toleration{
			Key:      "tolerations.test.key",
			Operator: corev1.TolerationOpExists,
			Effect:   corev1.TaintEffectNoSchedule,
		},
	}
}

func UnstructuredTestTolerations() []interface{} {
	tolerations := TestTolerations()
	response := make([]interface{}, 0, len(tolerations))
	for _, toleration := range tolerations {
		t, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&toleration)
		if err != nil {
			panic(fmt.Errorf("FATAL: Could not convert toleration to unstructured: %s\n\n%#v\n\n", err, toleration))
		}
		response = append(response, t)
	}
	return response
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
