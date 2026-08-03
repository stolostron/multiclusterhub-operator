// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// AnnotationNodeSelector key name of nodeSelector annotation synced from mch
	AnnotationNodeSelector = "open-cluster-management/nodeSelector"
)

// getKlusterletAddonConfig builds a KlusterletAddonConfig for the local-cluster ManagedCluster.
// Enables applicationManager and conditionally enables certPolicyController and policyController
// based on whether the GRC component is enabled in the MultiClusterHub spec.
// searchCollector is always disabled as search is handled separately.
func getKlusterletAddonConfig(m *operatorsv1.MultiClusterHub) *unstructured.Unstructured {
	grcEnabled := true

	if m.Spec.Overrides != nil {
		for _, component := range m.Spec.Overrides.Components {
			if component.Name == operatorsv1.GRC {
				grcEnabled = component.Enabled
				break
			}
		}
	}

	klusterletaddonconfig := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "agent.open-cluster-management.io/v1",
			"kind":       "KlusterletAddonConfig",
			"metadata": map[string]interface{}{
				"name":      m.Spec.LocalClusterName,
				"namespace": m.Spec.LocalClusterName,
			},
			"spec": map[string]interface{}{
				"applicationManager": map[string]interface{}{
					"enabled": true,
				},
				"certPolicyController": map[string]interface{}{
					"enabled": grcEnabled,
				},
				"policyController": map[string]interface{}{
					"enabled": grcEnabled,
				},
				"searchCollector": map[string]interface{}{
					"enabled": false,
				},
			},
		},
	}
	return klusterletaddonconfig
}

// ensureKlusterletAddonConfig ensures a KlusterletAddonConfig exists for the local-cluster
// and has the correct installer labels. Creates the resource if the ManagedCluster namespace
// exists but the config does not. Requeues if the namespace hasn't been created yet.
func (r *MultiClusterHubReconciler) ensureKlusterletAddonConfig(m *operatorsv1.MultiClusterHub) (ctrl.Result, error) {
	ctx := context.Background()

	// Check that the ManagedCluster namespace exists
	ns := &corev1.Namespace{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: m.Spec.LocalClusterName}, ns)

	if err != nil && errors.IsNotFound(err) {
		r.Log.Info("Waiting for ManagedCluster namespace to be created", "namespace", m.Spec.LocalClusterName)
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	} else if err != nil {
		r.Log.Error(err, "Failed to get ManagedCluster namespace", "namespace", m.Spec.LocalClusterName)
		return ctrl.Result{}, err
	}

	// Get or create the KlusterletAddonConfig
	klusterletaddonconfig := getKlusterletAddonConfig(m)
	nsn := types.NamespacedName{
		Name:      m.Spec.LocalClusterName,
		Namespace: m.Spec.LocalClusterName,
	}

	err = r.Client.Get(ctx, nsn, klusterletaddonconfig)
	if err != nil {
		if errors.IsNotFound(err) {
			utils.AddInstallerLabel(klusterletaddonconfig, m.GetName(), m.GetNamespace())

			err = r.Client.Create(ctx, klusterletaddonconfig)
			if err != nil {
				r.Log.Error(err, "Failed to create KlusterletAddonConfig",
					"name", nsn.Name, "namespace", nsn.Namespace)
				return ctrl.Result{}, err
			}

			r.Log.Info("Created KlusterletAddonConfig",
				"name", nsn.Name, "namespace", nsn.Namespace)
			return ctrl.Result{}, nil
		}

		r.Log.Error(err, "Failed to get KlusterletAddonConfig",
			"name", nsn.Name, "namespace", nsn.Namespace)
		return ctrl.Result{}, err
	}

	// Update installer labels
	utils.AddInstallerLabel(klusterletaddonconfig, m.GetName(), m.GetNamespace())

	err = r.Client.Update(ctx, klusterletaddonconfig)
	if err != nil {
		r.Log.Error(err, "Failed to update KlusterletAddonConfig",
			"name", nsn.Name, "namespace", nsn.Namespace)
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
