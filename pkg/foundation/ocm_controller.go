// Copyright (c) 2020 Red Hat, Inc.

package foundation

import (
	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// OCMControllerName is the name of the ocm controller deployment
const OCMControllerName string = "ocm-controller"

// OCMControllerDeployment creates the deployment for the ocm controller
func OCMControllerDeployment(m *operatorsv1.MultiClusterHub, overrides map[string]string) *appsv1.Deployment {
	replicas := getReplicaCount(m)
	mode := int32(420)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OCMControllerName,
			Namespace: m.Namespace,
			Labels:    defaultLabels(OCMControllerName),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: defaultLabels(OCMControllerName),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: defaultLabels(OCMControllerName),
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets:   []corev1.LocalObjectReference{{Name: m.Spec.ImagePullSecret}},
					ServiceAccountName: ServiceAccount,
					NodeSelector:       m.Spec.NodeSelector,
					Tolerations:        defaultTolerations(),
					Affinity:           utils.DistributePods("ocm-antiaffinity-selector", OCMControllerName),
					Volumes: []corev1.Volume{
						{
							Name: "klusterlet-certs",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{DefaultMode: &mode, SecretName: utils.KlusterletSecretName},
							},
						},
					},
					Containers: []corev1.Container{{
						Image:           Image(overrides),
						ImagePullPolicy: utils.GetImagePullPolicy(m),
						Name:            OCMControllerName,
						Args: []string{
							"/controller",
							"--agent-cafile=/var/run/klusterlet/ca.crt",
						},
						LivenessProbe: &v1.Probe{
							Handler: v1.Handler{
								HTTPGet: &v1.HTTPGetAction{
									Path: "/healthz",
									Port: intstr.FromInt(8000),
								},
							},
							PeriodSeconds: 10,
						},
						ReadinessProbe: &v1.Probe{
							Handler: v1.Handler{
								HTTPGet: &v1.HTTPGetAction{
									Path: "/readyz",
									Port: intstr.FromInt(8000),
								},
							},
							PeriodSeconds: 10,
						},
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("200m"),
								v1.ResourceMemory: resource.MustParse("256Mi"),
							},
							Limits: v1.ResourceList{
								v1.ResourceMemory: resource.MustParse("2048Mi"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "klusterlet-certs", MountPath: "/var/run/klusterlet"},
						},
					}},
				},
			},
		},
	}

	dep.SetOwnerReferences([]metav1.OwnerReference{
		*metav1.NewControllerRef(m, m.GetObjectKind().GroupVersionKind()),
	})
	return dep
}
