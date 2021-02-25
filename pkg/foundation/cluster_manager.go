// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project


package foundation

import (
	"bytes"
	"context"

	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/yaml"
)

func ClusterManager(m *operatorsv1.MultiClusterHub, overrides map[string]string) *unstructured.Unstructured {

	cm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "operator.open-cluster-management.io/v1",
			"kind":       "ClusterManager",
			"metadata": map[string]interface{}{
				"name": "cluster-manager",
			},
			"spec": map[string]interface{}{
				"registrationImagePullSpec": RegistrationImage(overrides),
				"workImagePullSpec":         WorkImage(overrides),
			},
		},
	}

	utils.AddInstallerLabel(cm, m.GetName(), m.GetNamespace())

	return cm
}

// ValidateClusterManager returns true if an update is needed to reconcile differences with the current spec. If an update
// is needed it returns the object with the new spec to update with.
func ValidateClusterManager(found *unstructured.Unstructured, want *unstructured.Unstructured) (*unstructured.Unstructured, bool) {
	var log = logf.Log.WithValues("Namespace", found.GetNamespace(), "Name", found.GetName(), "Kind", found.GetKind())

	desired, err := yaml.Marshal(want.Object["spec"])
	if err != nil {
		log.Error(err, "issue parsing desired cluster manager values")
	}
	current, err := yaml.Marshal(found.Object["spec"])
	if err != nil {
		log.Error(err, "issue parsing current cluster manager values")
	}

	if res := bytes.Compare(desired, current); res != 0 {
		// Return current object with adjusted spec, preserving metadata
		log.V(1).Info("Cluster Manager doesn't match spec", "Want", want.Object["spec"], "Have", found.Object["spec"])
		found.Object["spec"] = want.Object["spec"]
		return found, true
	}

	return nil, false
}

// GetClusterManager returns the cluster-manager instance found on the cluster
func GetClusterManager(client client.Client) (*unstructured.Unstructured, error) {
	cm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "operator.open-cluster-management.io/v1",
			"kind":       "ClusterManager",
			"metadata": map[string]interface{}{
				"name":      "cluster-manager",
				"namespace": "",
			},
		},
	}

	err := client.Get(context.TODO(), types.NamespacedName{
		Name:      cm.GetName(),
		Namespace: cm.GetNamespace(),
	}, cm)
	if err != nil {
		if errors.IsNotFound(err) {
			// Error due to cluster-manager not existing
			return cm, err
		}
		// Error likely due to cluster-manager not existing
		return cm, err
	}
	return cm, nil
}
