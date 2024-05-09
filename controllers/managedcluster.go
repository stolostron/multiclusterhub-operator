// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"reflect"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"

	ctrl "sigs.k8s.io/controller-runtime"
)

const (
	// ManagedClusterName name of the hub cluster managedcluster resource
	ManagedClusterName = "local-cluster"

	// KlusterletAddonConfigName name of the hub cluster managedcluster resource
	KlusterletAddonConfigName = "local-cluster"

	// AnnotationNodeSelector key name of nodeSelector annotation synced from mch
	AnnotationNodeSelector = "open-cluster-management/nodeSelector"
)

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
				"name":      KlusterletAddonConfigName,
				"namespace": ManagedClusterName,
			},
			"spec": map[string]interface{}{
				"applicationManager": map[string]interface{}{
					"enabled": true,
				},
				"connectionManager": map[string]interface{}{
					"enabledGlobalView": false,
				},
				"policyController": map[string]interface{}{
					"enabled": grcEnabled,
				},
				"prometheusIntegration": map[string]interface{}{
					"enabled": true,
				},
				"searchCollector": map[string]interface{}{
					"enabled": false,
				},
				"certPolicyController": map[string]interface{}{
					"enabled": grcEnabled,
				},
			},
		},
	}
	return klusterletaddonconfig
}

func equivalentKlusterletAddonConfig(desiredKlusterletaddonconfig, klusterletaddonconfig *unstructured.Unstructured,
	m *operatorsv1.MultiClusterHub,
) (bool, map[string]interface{}, error) {
	newSpec, _, err := unstructured.NestedMap(desiredKlusterletaddonconfig.Object, "spec")
	if err != nil {
		return false, nil, err
	}

	currentSpec, _, err := unstructured.NestedMap(klusterletaddonconfig.Object, "spec")
	if err != nil {
		return false, nil, err
	}

	labels := klusterletaddonconfig.GetLabels()

	hasLabels := labels["installer.name"] == m.Name && labels["installer.namespace"] == m.Namespace

	return reflect.DeepEqual(newSpec, currentSpec) && hasLabels, newSpec, nil
}

func (r *MultiClusterHubReconciler) ensureKlusterletAddonConfig(m *operatorsv1.MultiClusterHub) (ctrl.Result, error) {
	ctx := context.Background()

	r.Log.Info("Checking for local-cluster namespace")
	ns := &corev1.Namespace{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: ManagedClusterName}, ns)
	if err != nil && errors.IsNotFound(err) {
		r.Log.Info("Waiting for local-cluster namespace to be created")
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	} else if err != nil {
		r.Log.Error(err, "Failed to check for local-cluster namespace")
		return ctrl.Result{}, err
	}

	desiredKlusterletaddonconfig := getKlusterletAddonConfig(m)
	klusterletaddonconfig := desiredKlusterletaddonconfig.DeepCopy()
	nsn := types.NamespacedName{
		Name:      KlusterletAddonConfigName,
		Namespace: ManagedClusterName,
	}

	err = r.Client.Get(ctx, nsn, klusterletaddonconfig)
	if err != nil {
		if errors.IsNotFound(err) {
			// Creating new klusterletAddonConfig
			utils.AddInstallerLabel(desiredKlusterletaddonconfig, m.GetName(), m.GetNamespace())

			err = r.Client.Create(ctx, desiredKlusterletaddonconfig)
			if err != nil {
				r.Log.Error(err, "Failed to create klusterletaddonconfig resource")
				return ctrl.Result{}, err
			}
			// KlusterletAddonConfig was successful
			r.Log.Info("Created a new KlusterletAddonConfig")
			return ctrl.Result{}, nil
		}

		r.Log.Error(err, "Failed to get klusterletaddonconfig resource")
		return ctrl.Result{}, err
	}

	isEquivalent, newSpec, err := equivalentKlusterletAddonConfig(desiredKlusterletaddonconfig, klusterletaddonconfig, m)
	if err != nil {
		return ctrl.Result{}, err
	}

	// Don't need to update klusterletaddonconfig when isEquivalent is true
	if isEquivalent {
		return ctrl.Result{}, nil
	}

	utils.AddInstallerLabel(klusterletaddonconfig, m.GetName(), m.GetNamespace())

	err = unstructured.SetNestedMap(klusterletaddonconfig.Object, newSpec, "spec")
	if err != nil {
		r.Log.Error(err, "Failed to set the spec of the KlusterletAddonConfig")
		return ctrl.Result{}, err
	}

	err = r.Client.Update(ctx, klusterletaddonconfig)
	if err != nil {
		r.Log.Error(err, "Failed to update klusterletaddonconfig resource")
		return ctrl.Result{}, err
	}

	r.Log.Info("Updated the KlusterletAddonConfig")

	return ctrl.Result{}, nil
}
