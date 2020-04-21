package mcm

import (
	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// ImageName used by mcm deployments
const ImageName = "multicloud-manager"

// ImageVersion used by mcm deployments
const ImageVersion = "0.0.1"

// ServiceAccount used by mcm deployments
const ServiceAccount = "hub-sa"

// Image returns image reference for multicloud-manager
func Image(mch *operatorsv1beta1.MultiClusterHub, cache utils.CacheSpec) string {
	return utils.GetImageReference(mch, ImageName, ImageVersion, cache)
}

func defaultLabels(app string) map[string]string {
	return map[string]string{
		"app": app,
	}
}

func getReplicaCount(mch *operatorsv1beta1.MultiClusterHub) int32 {
	if mch.Spec.Failover {
		return 3
	}
	return 1
}

// ValidateDeployment returns a deep copy of the deployment with the desired spec based on the MultiClusterHub spec.
// Returns true if an update is needed to reconcile differences with the current spec.
func ValidateDeployment(m *operatorsv1beta1.MultiClusterHub, cache utils.CacheSpec, dep *appsv1.Deployment) (*appsv1.Deployment, bool) {
	var log = logf.Log.WithValues("Deployment.Namespace", dep.GetNamespace(), "Deployment.Name", dep.GetName())
	found := dep.DeepCopy()

	pod := &found.Spec.Template.Spec
	container := &found.Spec.Template.Spec.Containers[0]
	needsUpdate := false

	// verify image pull secret
	if m.Spec.ImagePullSecret != "" {
		ps := corev1.LocalObjectReference{Name: m.Spec.ImagePullSecret}
		if !utils.ContainsPullSecret(pod.ImagePullSecrets, ps) {
			log.Info("Enforcing imagePullSecret from CR spec")
			pod.ImagePullSecrets = append(pod.ImagePullSecrets, ps)
			needsUpdate = true
		}
	}

	// verify image repository and suffix
	if container.Image != Image(m, cache) {
		log.Info("Enforcing image repo and suffix from CR spec")
		container.Image = Image(m, cache)
		needsUpdate = true
	}

	// verify image pull policy
	if container.ImagePullPolicy != m.Spec.ImagePullPolicy {
		log.Info("Enforcing imagePullPolicy from CR spec")
		container.ImagePullPolicy = m.Spec.ImagePullPolicy
		needsUpdate = true
	}

	// verify node selectors
	desiredSelectors := m.Spec.NodeSelector
	if !utils.ContainsMap(pod.NodeSelector, desiredSelectors) {
		log.Info("Enforcing node selectors from CR spec")
		pod.NodeSelector = desiredSelectors
		needsUpdate = true
	}

	// verify replica count
	if *found.Spec.Replicas != getReplicaCount(m) {
		log.Info("Enforcing number of replicas")
		replicas := getReplicaCount(m)
		found.Spec.Replicas = &replicas
		needsUpdate = true
	}

	return found, needsUpdate
}
