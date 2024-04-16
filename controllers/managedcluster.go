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
	// ManagedClusterName name of the hub cluster managedcluster resource
	ManagedClusterName = "local-cluster"

	// KlusterletAddonConfigName name of the hub cluster managedcluster resource
	KlusterletAddonConfigName = "local-cluster"

	// AnnotationNodeSelector key name of nodeSelector annotation synced from mch
	AnnotationNodeSelector = "open-cluster-management/nodeSelector"
)

func getKlusterletAddonConfig() *unstructured.Unstructured {
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
					"enabled": true,
				},
				"prometheusIntegration": map[string]interface{}{
					"enabled": true,
				},
				"searchCollector": map[string]interface{}{
					"enabled": false,
				},
				"certPolicyController": map[string]interface{}{
					"enabled": true,
				},
				"iamPolicyController": map[string]interface{}{
					"enabled": true,
				},
			},
		},
	}
	return klusterletaddonconfig
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

	klusterletaddonconfig := getKlusterletAddonConfig()
	nsn := types.NamespacedName{
		Name:      KlusterletAddonConfigName,
		Namespace: ManagedClusterName,
	}
	err = r.Client.Get(ctx, nsn, klusterletaddonconfig)
	if err != nil && errors.IsNotFound(err) {
		// Creating new klusterletAddonConfig
		newKlusterletaddonconfig := getKlusterletAddonConfig()
		utils.AddInstallerLabel(newKlusterletaddonconfig, m.GetName(), m.GetNamespace())

		err = r.Client.Create(ctx, newKlusterletaddonconfig)
		if err != nil {
			r.Log.Error(err, "Failed to create klusterletaddonconfig resource")
			return ctrl.Result{}, err
		}
		// KlusterletAddonConfig was successful
		r.Log.Info("Created a new KlusterletAddonConfig")
		return ctrl.Result{}, nil
	}

	utils.AddInstallerLabel(klusterletaddonconfig, m.GetName(), m.GetNamespace())

	err = r.Client.Update(ctx, klusterletaddonconfig)
	if err != nil {
		r.Log.Error(err, "Failed to update klusterletaddonconfig resource")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}
