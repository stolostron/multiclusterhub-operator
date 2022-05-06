// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package helmrepo

import (
	"reflect"
	"strconv"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	"github.com/stolostron/multiclusterhub-operator/pkg/version"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// ImageKey used by mch repo
var ImageKey = "multiclusterhub_repo"

// HelmRepoName for labels, service name, and deployment name
var HelmRepoName = "multiclusterhub-repo"

// Port of helm repo service
var Port = 3000

// Version of helm repo image

func labels() map[string]string {
	return map[string]string{
		"app":                       HelmRepoName,
		"ocm-antiaffinity-selector": HelmRepoName,
	}
}

func selectorLabels() map[string]string {
	return map[string]string{
		"app": HelmRepoName,
	}
}

// Image returns image reference for multiclusterhub-repo
func Image(overrides map[string]string) string {
	return overrides[ImageKey]
}

// Deployment for the helm repo serving charts
func Deployment(m *operatorsv1.MultiClusterHub, overrides map[string]string) *appsv1.Deployment {
	replicas := int32(1)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HelmRepoName,
			Namespace: m.Namespace,
			Labels:    labels(),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels(),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels(),
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image:           Image(overrides),
						ImagePullPolicy: utils.GetImagePullPolicy(m),
						Name:            HelmRepoName,
						Ports: []corev1.ContainerPort{{
							ContainerPort: int32(Port),
							Name:          "helmrepo",
						}},
						Resources: corev1.ResourceRequirements{
							Requests: corev1.ResourceList{
								corev1.ResourceCPU:    resource.MustParse("50m"),
								corev1.ResourceMemory: resource.MustParse("50Mi"),
							},
							Limits: corev1.ResourceList{
								corev1.ResourceMemory: resource.MustParse("100Mi"),
							},
						},
						LivenessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/liveness",
									Port:   intstr.FromInt(Port),
									Scheme: corev1.URISchemeHTTP,
								},
							},
						},
						ReadinessProbe: &corev1.Probe{
							ProbeHandler: corev1.ProbeHandler{
								HTTPGet: &corev1.HTTPGetAction{
									Path:   "/readiness",
									Port:   intstr.FromInt(Port),
									Scheme: corev1.URISchemeHTTP,
								},
							},
						},
						Env: []corev1.EnvVar{
							{
								Name: "POD_NAMESPACE",
								ValueFrom: &corev1.EnvVarSource{
									FieldRef: &corev1.ObjectFieldSelector{
										APIVersion: "v1",
										FieldPath:  "metadata.namespace",
									},
								},
							},
							{
								Name:  "MCH_REPO_PORT",
								Value: strconv.Itoa(Port),
							},
							{
								Name:  "MCH_REPO_SERVICE",
								Value: HelmRepoName,
							},
							{
								Name:  "CHART_VERSION",
								Value: version.Version,
							},
							{
								Name:  "REPO_DIR",
								Value: "/repo/charts",
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "repo-volume",
								MountPath: "/repo/charts",
							},
						},
					}},
					ImagePullSecrets: []corev1.LocalObjectReference{{Name: m.Spec.ImagePullSecret}},
					NodeSelector:     m.Spec.NodeSelector,
					Tolerations:      utils.GetTolerations(m),
					Affinity:         utils.DistributePods("ocm-antiaffinity-selector", HelmRepoName),
					Volumes: []corev1.Volume{
						{
							Name: "repo-volume",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{},
							},
						},
					},
					// ServiceAccountName: "default",
				},
			},
		},
	}

	dep.SetOwnerReferences([]metav1.OwnerReference{
		*metav1.NewControllerRef(m, m.GetObjectKind().GroupVersionKind()),
	})
	return dep
}

// Service for the helm repo serving charts
func Service(m *operatorsv1.MultiClusterHub) *corev1.Service {
	labels := selectorLabels()

	s := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      HelmRepoName,
			Namespace: m.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{{
				Protocol:   corev1.ProtocolTCP,
				Port:       int32(Port),
				TargetPort: intstr.FromInt(Port),
			}},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	s.SetOwnerReferences([]metav1.OwnerReference{
		*metav1.NewControllerRef(m, m.GetObjectKind().GroupVersionKind()),
	})
	return s
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
		log.Info("Enforcing imagePullPolicy")
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

	// verify image pull policy
	if container.ImagePullPolicy != utils.GetImagePullPolicy(m) {
		log.Info("Enforcing imagePullPolicy")
		container.ImagePullPolicy = utils.GetImagePullPolicy(m)
		needsUpdate = true
	}

	// add missing labels to deployment
	if utils.AddDeploymentLabels(found, expected.Labels) {
		log.Info("Enforcing deployment labels")
		needsUpdate = true
	}

	// add missing pod labels
	if utils.AddPodLabels(found, expected.Spec.Template.Labels) {
		log.Info("Enforcing pod labels")
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

	if !reflect.DeepEqual(pod.Tolerations, utils.GetTolerations(m)) {
		log.Info("Enforcing spec tolerations")
		pod.Tolerations = utils.GetTolerations(m)
		needsUpdate = true
	}

	if !reflect.DeepEqual(pod.Volumes, utils.GetContainerVolumes(expected)) {
		log.Info("Enforcing container volumes")
		vms := utils.GetContainerVolumes(expected)
		pod.Volumes = vms
		needsUpdate = true
	}

	return found, needsUpdate
}
