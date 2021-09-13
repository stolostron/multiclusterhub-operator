// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"encoding/json"
	"fmt"

	operatorsv1 "github.com/open-cluster-management/multiclusterhub-operator/api/v1"
	utils "github.com/open-cluster-management/multiclusterhub-operator/pkg/utils"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/version"
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
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: ManagedClusterName,
		},
	}
	return ns
}

func getManagedCluster() *unstructured.Unstructured {
	managedCluster := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "cluster.open-cluster-management.io/v1",
			"kind":       "ManagedCluster",
			"metadata": map[string]interface{}{
				"name": ManagedClusterName,
				"labels": map[string]interface{}{
					"local-cluster": "true",
					"cloud":         "auto-detect",
					"vendor":        "auto-detect",
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

func (r *MultiClusterHubReconciler) ensureHubIsImported(m *operatorsv1.MultiClusterHub) (ctrl.Result, error) {
	if !r.ComponentsAreRunning(m) {
		r.Log.Info("Waiting for mch phase to be 'running' before importing hub cluster")
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	// resume klusterletaddonconfig ignore error
	if err := ensureKlusterletAddonConfigPausedStatus(
		r.Client,
		KlusterletAddonConfigName,
		ManagedClusterName,
		false,
	); err != nil && !errors.IsNotFound(err) {
		r.Log.Error(err, "failed to resume klusterletaddonconfig")
	}

	result, err := r.ensureManagedCluster(m)
	if result != (ctrl.Result{}) {
		return result, err
	}

	result, err = r.ensureKlusterletAddonConfig(m)
	if result != (ctrl.Result{}) {
		return result, err
	}
	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureHubIsExported(m *operatorsv1.MultiClusterHub) (ctrl.Result, error) {
	r.Log.Info("Ensuring managed cluster hub resources are removed")

	result, err := r.removeManagedCluster(m)
	if result != (ctrl.Result{}) {
		waiting := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, ManagedClusterTerminatingReason, "Waiting for local managed cluster to terminate.")
		SetHubCondition(&m.Status, *waiting)
		return result, err
	}

	// Removed by rcm-controller
	result, err = r.ensureHubNamespaceIsRemoved(m)
	if result != (ctrl.Result{}) {
		waiting := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, NamespaceTerminatingReason, "Waiting for the local managed cluster's namespace to terminate.")
		SetHubCondition(&m.Status, *waiting)
		return result, err
	}
	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureHubNamespaceIsRemoved(m *operatorsv1.MultiClusterHub) (ctrl.Result, error) {
	HubNamespace := getHubNamespace()
	HubNamespace.SetLabels(getInstallerLabels(m))

	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: HubNamespace.GetName()}, HubNamespace)
	if err != nil && errors.IsNotFound(err) {
		// Namespace is removed
		return ctrl.Result{}, nil
	}
	r.Log.Info(fmt.Sprintf("Waiting on namespace: %s to be removed", HubNamespace.GetName()))
	return ctrl.Result{RequeueAfter: resyncPeriod}, fmt.Errorf("Waiting on namespace: %s to be removed", HubNamespace.GetName())
}

func (r *MultiClusterHubReconciler) ensureManagedCluster(m *operatorsv1.MultiClusterHub) (ctrl.Result, error) {
	managedCluster := getManagedCluster()

	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: ManagedClusterName}, managedCluster)
	if err != nil && errors.IsNotFound(err) {
		localNS := getHubNamespace()
		localNS.SetLabels(getInstallerLabels(m))
		err := r.Client.Get(context.TODO(), types.NamespacedName{Name: localNS.GetName()}, localNS)
		if err == nil {
			// Wait for local-cluster ns to be deleted before creating managedCluster
			r.Log.Info("Waiting on namespace to be removed before creating managedCluster", "Namespace", localNS.GetName())
			return ctrl.Result{RequeueAfter: resyncPeriod}, nil
		} else if errors.IsNotFound(err) {
			// Namespace is removed. Creating new managedCluster
			newManagedCluster := getManagedCluster()
			utils.AddInstallerLabel(newManagedCluster, m.GetName(), m.GetNamespace())

			err = r.Client.Create(context.TODO(), newManagedCluster)
			if err != nil {
				r.Log.Error(err, "Failed to create managedcluster resource")
				return ctrl.Result{}, err
			}
			r.Log.Info("Created a new ManagedCluster")
			return ctrl.Result{}, nil
		} else {
			r.Log.Error(err, "Failed to get local-cluster namespace")
			return ctrl.Result{}, err
		}
	} else if err != nil {
		// Error that isn't due to the managedcluster not existing
		r.Log.Error(err, "Failed to get ManagedCluster")
		return ctrl.Result{}, err
	}

	// Ensure labels set
	labels := getInstallerLabels(m)
	labels["local-cluster"] = "true"
	labels["cloud"] = "auto-detect"
	labels["vendor"] = "auto-detect"

	// Overwrite with existing labels
	for k, v := range managedCluster.GetLabels() {
		labels[k] = v
	}
	managedCluster.SetLabels(labels)

	annotations := managedCluster.GetAnnotations()
	if len(m.Spec.NodeSelector) != 0 {
		nodeSelectors, err := json.Marshal(m.Spec.NodeSelector)
		if err != nil {
			r.Log.Error(err, "Failed to marshal nodeSelector")
			return ctrl.Result{}, err
		}
		annotations[AnnotationNodeSelector] = string(nodeSelectors)
		managedCluster.SetAnnotations(annotations)
	} else if _, ok := annotations[AnnotationNodeSelector]; ok {
		delete(annotations, AnnotationNodeSelector)
		managedCluster.SetAnnotations(annotations)
	}

	err = r.Client.Update(context.TODO(), managedCluster)
	if err != nil {
		r.Log.Error(err, "Failed to update managedcluster resource")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) removeManagedCluster(m *operatorsv1.MultiClusterHub) (ctrl.Result, error) {
	managedCluster := getManagedCluster()
	labels := getInstallerLabels(m)
	labels["local-cluster"] = "true"
	managedCluster.SetLabels(labels)

	// Wait for managedcluster to be removed
	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: ManagedClusterName}, managedCluster)
	if err != nil {
		// ManagedCluster is removed
		return ctrl.Result{}, nil
	}

	err = r.Client.Delete(context.TODO(), getManagedCluster())
	if err != nil && !errors.IsNotFound(err) {
		r.Log.Error(err, "Error deleting managedcluster")
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureKlusterletAddonConfig(m *operatorsv1.MultiClusterHub) (ctrl.Result, error) {
	klusterletaddonconfig := getKlusterletAddonConfig()

	err := r.Client.Get(context.TODO(), types.NamespacedName{Name: KlusterletAddonConfigName, Namespace: ManagedClusterName}, klusterletaddonconfig)
	if err != nil && errors.IsNotFound(err) {
		// Creating new klusterletAddonConfig
		newKlusterletaddonconfig := getKlusterletAddonConfig()
		utils.AddInstallerLabel(newKlusterletaddonconfig, m.GetName(), m.GetNamespace())

		err = r.Client.Create(context.TODO(), newKlusterletaddonconfig)
		if err != nil {
			r.Log.Error(err, "Failed to create klusterletaddonconfig resource")
			return ctrl.Result{}, err
		}
		// KlusterletAddonConfig was successful
		r.Log.Info("Created a new KlusterletAddonConfig")
		return ctrl.Result{}, nil
	}

	utils.AddInstallerLabel(klusterletaddonconfig, m.GetName(), m.GetNamespace())

	err = r.Client.Update(context.TODO(), klusterletaddonconfig)
	if err != nil {
		r.Log.Error(err, "Failed to update klusterletaddonconfig resource")
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureManagedClusterIsRunning(m *operatorsv1.MultiClusterHub) ([]interface{}, error) {
	if m.Spec.DisableHubSelfManagement {
		return nil, nil
	}
	if !r.ComponentsAreRunning(m) {
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
