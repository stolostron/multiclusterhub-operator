package multiclusterhub

import (
	"context"
	"fmt"

	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// ManagedClusterName name of the hub cluster managedcluster resource
	ManagedClusterName = "ocm-hub-cluster"

	// KlusterletAddonConfigName name of the hub cluster managedcluster resource
	KlusterletAddonConfigName = "ocm-hub-cluster"
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
			},
			"spec": map[string]interface{}{
				"hubAcceptsClient": true,
			},
		},
	}
	return managedCluster
}

func getKlusterletAddonConfig(version string) *unstructured.Unstructured {
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
					"enabled": true,
				},
				"certPolicyController": map[string]interface{}{
					"enabled": true,
				},
				"iamPolicyController": map[string]interface{}{
					"enabled": true,
				},
				"version": version,
			},
		},
	}
	return klusterletaddonconfig
}

func (r *ReconcileMultiClusterHub) ensureHubIsImported(m *operatorsv1.MultiClusterHub) (*reconcile.Result, error) {
	if m.Status.Phase != operatorsv1.HubRunning {
		log.Info("Waiting for mch phase to be 'running' before importing hub cluster")
		return &reconcile.Result{}, fmt.Errorf("Waiting for mch phase to be 'running' before importing hub cluster")
	}

	result, err := r.ensureNamespace(m)
	if result != nil {
		return result, err
	}

	result, err = r.ensureManagedCluster(m)
	if result != nil {
		return result, err
	}

	result, err = r.ensureKlusterletAddonConfig(m)
	if result != nil {
		return result, err
	}
	return nil, nil
}

func (r *ReconcileMultiClusterHub) ensureHubIsExported(m *operatorsv1.MultiClusterHub) (*reconcile.Result, error) {
	log.Info("Ensuring managed cluster hub resources are removed")

	result, err := r.removeManagedCluster(m)
	if result != nil {
		return result, err
	}

	// Removed by rcm-controller
	result, err = r.ensureKlusterletAddonConfigIsRemoved(m)
	if result != nil {
		return result, err
	}

	// Removed by rcm-controller
	result, err = r.ensureHubNamespaceIsRemoved(m)
	if result != nil {
		return result, err
	}
	return nil, nil
}

func (r *ReconcileMultiClusterHub) ensureNamespace(m *operatorsv1.MultiClusterHub) (*reconcile.Result, error) {
	namespace := getHubNamespace()

	err := r.client.Get(context.TODO(), types.NamespacedName{Name: ManagedClusterName}, namespace)
	if err != nil && errors.IsNotFound(err) {
		namespace = getHubNamespace()
		namespace.SetLabels(getInstallerLabels(m))

		err = r.client.Create(context.TODO(), namespace)
		if err != nil {
			log.Error(err, "Failed to create managedcluster namespace resource")
			return &reconcile.Result{}, err
		}
	}

	return nil, nil
}

func (r *ReconcileMultiClusterHub) ensureHubNamespaceIsRemoved(m *operatorsv1.MultiClusterHub) (*reconcile.Result, error) {
	HubNamespace := getHubNamespace()
	HubNamespace.SetLabels(getInstallerLabels(m))

	err := r.client.Get(context.TODO(), types.NamespacedName{Name: HubNamespace.GetName()}, HubNamespace)
	if err != nil && errors.IsNotFound(err) {
		// Namespace is removed
		return nil, nil
	}
	return &reconcile.Result{}, fmt.Errorf("Waiting on namespace: %s to be removed", HubNamespace.GetName())
}

func (r *ReconcileMultiClusterHub) ensureManagedCluster(m *operatorsv1.MultiClusterHub) (*reconcile.Result, error) {
	managedCluster := getManagedCluster()

	err := r.client.Get(context.TODO(), types.NamespacedName{Name: ManagedClusterName}, managedCluster)
	if err != nil && errors.IsNotFound(err) {
		managedCluster = getManagedCluster()

		labels := getInstallerLabels(m)
		labels["local-cluster"] = "true"
		managedCluster.SetLabels(labels)

		managedCluster.SetOwnerReferences([]metav1.OwnerReference{
			*metav1.NewControllerRef(m, m.GetObjectKind().GroupVersionKind()),
		})

		err = r.client.Create(context.TODO(), managedCluster)
		if err != nil {
			log.Error(err, "Failed to create managedcluster resource")
			return &reconcile.Result{}, err
		}
	}

	return nil, nil
}

func (r *ReconcileMultiClusterHub) removeManagedCluster(m *operatorsv1.MultiClusterHub) (*reconcile.Result, error) {
	managedCluster := getManagedCluster()
	labels := getInstallerLabels(m)
	labels["local-cluster"] = "true"
	managedCluster.SetLabels(labels)

	// Wait for managedcluster to be removed
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: ManagedClusterName}, managedCluster)
	if err != nil && errors.IsNotFound(err) {
		fmt.Println("Managed Cluster Gone")

		// ManagedCluster is removed
		return nil, nil
	}

	fmt.Println("Deleting Managed Cluster")
	err = r.client.Delete(context.TODO(), getManagedCluster())
	if err != nil && !errors.IsNotFound(err) {
		log.Error(err, "Error deleting managedcluster")
		return &reconcile.Result{}, err
	}
	return &reconcile.Result{}, fmt.Errorf("Waiting on managedcluster: %s to be removed", ManagedClusterName)
}

func (r *ReconcileMultiClusterHub) ensureKlusterletAddonConfig(m *operatorsv1.MultiClusterHub) (*reconcile.Result, error) {
	klusterletaddonconfig := getKlusterletAddonConfig(m.Status.CurrentVersion)

	err := r.client.Get(context.TODO(), types.NamespacedName{Name: KlusterletAddonConfigName, Namespace: ManagedClusterName}, klusterletaddonconfig)
	if err != nil && errors.IsNotFound(err) {
		klusterletaddonconfig = getKlusterletAddonConfig(m.Status.CurrentVersion)
		klusterletaddonconfig.SetLabels(getInstallerLabels(m))

		err = r.client.Create(context.TODO(), klusterletaddonconfig)
		if err != nil {
			log.Error(err, "Failed to create klusterletaddonconfig resource")
			return &reconcile.Result{}, err
		}
	}
	return nil, nil
}

func (r *ReconcileMultiClusterHub) ensureKlusterletAddonConfigIsRemoved(m *operatorsv1.MultiClusterHub) (*reconcile.Result, error) {
	klusterletaddonconfig := getKlusterletAddonConfig(m.Status.CurrentVersion)
	klusterletaddonconfig.SetLabels(getInstallerLabels(m))

	err := r.client.Get(context.TODO(), types.NamespacedName{Name: KlusterletAddonConfigName, Namespace: ManagedClusterName}, klusterletaddonconfig)
	if err != nil && errors.IsNotFound(err) {
		// KlusterletAddonConfig is removed
		return nil, nil
	}
	return &reconcile.Result{}, fmt.Errorf("Waiting on klusterletaddonconfig: %s/%s to be removed", ManagedClusterName, KlusterletAddonConfigName)
}
