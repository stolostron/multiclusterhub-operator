// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"
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

	r.Log.Info(fmt.Sprintf("Checking for ManagedCluster %v namespace", m.Spec.LocalClusterName))
	ns := &corev1.Namespace{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: m.Spec.LocalClusterName}, ns)
	if err != nil && errors.IsNotFound(err) {
		r.Log.Info(fmt.Sprintf("Waiting for %v namespace to be created", m.Spec.LocalClusterName))
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	} else if err != nil {
		r.Log.Error(err, fmt.Sprintf("Failed to check for %v namespace", m.Spec.LocalClusterName))
		return ctrl.Result{}, err
	}

	klusterletaddonconfig := getKlusterletAddonConfig(m)
	nsn := types.NamespacedName{
		Name:      m.Spec.LocalClusterName,
		Namespace: m.Spec.LocalClusterName,
	}
	err = r.Client.Get(ctx, nsn, klusterletaddonconfig)
	if err != nil {
		if errors.IsNotFound(err) {
			// Creating new klusterletAddonConfig
			utils.AddInstallerLabel(klusterletaddonconfig, m.GetName(), m.GetNamespace())

			err = r.Client.Create(ctx, klusterletaddonconfig)
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

	utils.AddInstallerLabel(klusterletaddonconfig, m.GetName(), m.GetNamespace())

	err = r.Client.Update(ctx, klusterletaddonconfig)
	if err != nil {
		r.Log.Error(err, "Failed to update klusterletaddonconfig resource")
		return ctrl.Result{}, err
	}

	r.Log.Info("Updated the KlusterletAddonConfig")

	return ctrl.Result{}, nil
}
