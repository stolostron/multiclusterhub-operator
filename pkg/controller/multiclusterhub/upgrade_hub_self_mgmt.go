package multiclusterhub

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver"
	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// UpgradeHubSelfMgmtHackRequired checks the the current version and if hub self management is enabled
// to determine if special upgrade logic is required
func (r *ReconcileMultiClusterHub) UpgradeHubSelfMgmtHackRequired(mch *operatorsv1.MultiClusterHub) (bool, error) {
	c, err := semver.NewConstraint("< 2.1.2, >= 2.1.0")
	if err != nil {
		return false, fmt.Errorf("Error setting semver constraint < 2.1.2, >=2.1.0")
	}

	if mch.Status.CurrentVersion == "" {
		// Current Version is not available yet
		return false, nil
	}

	currentVersion, err := semver.NewVersion(mch.Status.CurrentVersion)
	if err != nil {
		return false, fmt.Errorf("Error setting semver currentversion: %s", mch.Status.CurrentVersion)
	}

	versionValidation := c.Check(currentVersion)
	if versionValidation && !mch.Spec.DisableHubSelfManagement {
		return true, nil
	}
	return false, nil
}

// BeginEnsuringHubIsUpgradeable - beginning hook for ensuring the hub is upgradeable
func (r *ReconcileMultiClusterHub) BeginEnsuringHubIsUpgradeable(mch *operatorsv1.MultiClusterHub) (*reconcile.Result, error) {
	log.Info("Beginning Upgrade Specific Logic!")

	// Example of how to retrieve an image from the manifest configmap by key
	image, err := r.getImageFromManifestByKey(mch, "multicluster_operators_subscription")
	if err != nil {
		return nil, err
	}
	log.Info(fmt.Sprintf("Image: %s", image))
	return nil, nil
}

// EndEnsuringHubIsUpgradeable - end hook for ensuring the hub is upgradeable
func (r *ReconcileMultiClusterHub) EndEnsuringHubIsUpgradeable(mch *operatorsv1.MultiClusterHub) (*reconcile.Result, error) {
	log.Info("Ending Upgrade Specific Logic!")
	return nil, nil
}

// getImageFromManifestByKey - Returns image associated with key for desiredVersion of MCH (retrieves new image)
func (r *ReconcileMultiClusterHub) getImageFromManifestByKey(mch *operatorsv1.MultiClusterHub, key string) (string, error) {
	log.Info(fmt.Sprintf("Checking for image associated with key: %s", key))
	configmap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("mch-image-manifest-%s", mch.Status.DesiredVersion),
			Namespace: mch.Namespace,
		},
	}

	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      configmap.Name,
		Namespace: configmap.Namespace,
	}, configmap)
	if err != nil {
		return "", err
	}

	if val, ok := configmap.Data[key]; ok {
		return val, nil
	}
	return "", fmt.Errorf("No image exists associated with key: %s", key)
}
