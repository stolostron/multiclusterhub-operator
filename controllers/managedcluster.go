// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"
	"github.com/stolostron/multiclusterhub-operator/pkg/version"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func getInstallerLabels(m *operatorsv1.MultiClusterHub) map[string]string {
	labels := make(map[string]string)
	labels["installer.name"] = m.GetName()
	labels["installer.namespace"] = m.GetNamespace()
	return labels
}

func getHubNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ManagedClusterName,
		},
	}
}

func getManagedCluster() *unstructured.Unstructured {
	managedCluster := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cluster.open-cluster-management.io/v1",
			"kind":       "ManagedCluster",
			"metadata": map[string]interface{}{
				"name": ManagedClusterName,
				"labels": map[string]interface{}{
					"local-cluster":                 "true",
					"cloud":                         "auto-detect",
					"vendor":                        "auto-detect",
					"velero.io/exclude-from-backup": "true",
				},
			},
			"spec": map[string]interface{}{
				"hubAcceptsClient": true,
			},
		},
	}
	return managedCluster
}

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
				"clusterName":      KlusterletAddonConfigName,
				"clusterNamespace": ManagedClusterName,
				"applicationManager": map[string]interface{}{
					"enabled": true,
				},
				"clusterLabels": map[string]interface{}{
					"cloud":  "auto-detect",
					"vendor": "auto-detect",
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
				"version": version.Version,
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
	if err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("get klusterletaddonconfig: %v", err)
	}

	res, _, err := unstructured.NestedString(klusterletaddonconfig.Object, "spec", "version")
	if err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("modify klusterletaddonconfig: %v", err)
	} else if res != version.Version {
		err = unstructured.SetNestedField(klusterletaddonconfig.Object, version.Version, "spec", "version")
		if err != nil {
			return ctrl.Result{Requeue: true}, fmt.Errorf("modify klusterletaddonconfig: %v", err)
		}
	}
	utils.AddInstallerLabel(klusterletaddonconfig, m.GetName(), m.GetNamespace())

	err = r.Client.Update(ctx, klusterletaddonconfig)
	if err != nil {
		r.Log.Error(err, "Failed to update klusterletaddonconfig resource")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureManagedClusterIsRunning(m *operatorsv1.MultiClusterHub, ocpConsole bool) ([]interface{}, error) {
	if m.Spec.DisableHubSelfManagement {
		return nil, nil
	}
	if !r.ComponentsAreRunning(m, ocpConsole) {
		r.Log.Info("Waiting for mch phase to be 'running' before ensuring hub is running")
		return nil, fmt.Errorf("Waiting for mch phase to be 'running' before ensuring hub is running")
	}

	managedCluster := getManagedCluster()
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: ManagedClusterName}, managedCluster)
	if err != nil {
		r.Log.Info("Failed to find managedcluster resource")
		return nil, err
	}

	status, ok := managedCluster.Object["status"].(map[string]interface{})
	if !ok {
		r.Log.Info("Managedcluster status is not present")
		return nil, fmt.Errorf("Managedcluster status is not present")
	}
	conditions, ok := status["conditions"].([]interface{})
	if !ok {
		r.Log.Info("Managedcluster status conditions are not present")
		return nil, fmt.Errorf("Managedcluster status conditions are not present")
	}

	return conditions, nil
}
