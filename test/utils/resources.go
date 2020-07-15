// Copyright (c) 2020 Red Hat, Inc.
package utils

import (
	"os"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// NewMultiClusterHub ...
func NewMultiClusterHub(name, namespace string) *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "operator.open-cluster-management.io/v1",
			"kind":       "MultiClusterHub",
			"metadata": map[string]interface{}{
				"name":      name,
				"namespace": namespace,
			},
			"spec": map[string]interface{}{
				"imagePullSecret": "multiclusterhub-operator-pull-secret",
			},
		},
	}
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
			"spec": map[string]interface{}{
				"sourceNamespace":     os.Getenv("sourceNamespace"),
				"source":              os.Getenv("source"),
				"channel":             os.Getenv("channel"),
				"installPlanApproval": "Automatic",
				"name":                os.Getenv("name"),
			},
		},
	}
}
