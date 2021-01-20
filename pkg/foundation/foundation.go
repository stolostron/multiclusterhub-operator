// Copyright (c) 2020 Red Hat, Inc.

package foundation

import (
	"reflect"

	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// ImageKey used by multicoud manager deployments
const ImageKey = "multicloud_manager"

// RegistrationImageKey used by registration deployments
const RegistrationImageKey = "registration"

// WorkImageKey used by work deployments
const WorkImageKey = "work"

// ServiceAccount used by ocm deployments
const ServiceAccount = "ocm-foundation-sa"

// Image returns image reference for multicloud-manager
func Image(overrides map[string]string) string {
	return overrides[ImageKey]
}

// RegistrationImage ...
func RegistrationImage(overrides map[string]string) string {
	return overrides[RegistrationImageKey]
}

// WorkImage ...
func WorkImage(overrides map[string]string) string {
	return overrides[WorkImageKey]
}

func defaultLabels(app string) map[string]string {
	return map[string]string{
		"app":                       app,
		"ocm-antiaffinity-selector": app,
	}
}

func getReplicaCount(mch *operatorsv1.MultiClusterHub) int32 {
	if mch.Spec.AvailabilityConfig == operatorsv1.HABasic {
		return 1
	}
	return 2
}

// ValidateDeployment returns a deep copy of the deployment with the desired spec based on the MultiClusterHub spec.
// Returns true if an update is needed to reconcile differences with the current spec.
func ValidateDeployment(m *operatorsv1.MultiClusterHub, overrides map[string]string, expected, dep *appsv1.Deployment) (*appsv1.Deployment, bool) {
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
	if container.Image != Image(overrides) {
		log.Info("Enforcing image repo and suffix from CR spec")
		container.Image = Image(overrides)
		needsUpdate = true
	}

	// verify image pull policy
	if container.ImagePullPolicy != utils.GetImagePullPolicy(m) {
		log.Info("Enforcing imagePullPolicy from CR spec")
		container.ImagePullPolicy = utils.GetImagePullPolicy(m)
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

	if !reflect.DeepEqual(container.Args, utils.GetContainerArgs(expected)) {
		log.Info("Enforcing container arguments")
		args := utils.GetContainerArgs(expected)
		container.Args = args
		needsUpdate = true
	}

	if !reflect.DeepEqual(container.Env, utils.GetContainerEnvVars(expected)) {
		log.Info("Enforcing container environment variables")
		envs := utils.GetContainerEnvVars(expected)
		container.Env = envs
		needsUpdate = true
	}

	if !reflect.DeepEqual(container.VolumeMounts, utils.GetContainerVolumeMounts(expected)) {
		log.Info("Enforcing container volume mounts")
		vms := utils.GetContainerVolumeMounts(expected)
		container.VolumeMounts = vms
		needsUpdate = true
	}

	return found, needsUpdate
}
