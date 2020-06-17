// Copyright (c) 2020 Red Hat, Inc.

package mcm

import (
	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// WebhookName is the name of the mcm apiserver deployment
const WebhookName string = "mcm-webhook"

// WebhookDeployment creates the deployment for the mcm webhook
func WebhookDeployment(m *operatorsv1.MultiClusterHub, overrides map[string]string) *appsv1.Deployment {
	replicas := getReplicaCount(m)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WebhookName,
			Namespace: m.Namespace,
			Labels:    defaultLabels(WebhookName),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: defaultLabels(WebhookName),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: defaultLabels(WebhookName),
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets:   []corev1.LocalObjectReference{{Name: m.Spec.ImagePullSecret}},
					ServiceAccountName: ServiceAccount,
					NodeSelector:       m.Spec.NodeSelector,
					Affinity:           utils.DistributePods("app", WebhookName),
					Volumes: []corev1.Volume{
						{
							Name: "webhook-cert",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{SecretName: "mcm-webhook-secret"},
							},
						},
					},
					Containers: []corev1.Container{{
						Image:           Image(overrides),
						ImagePullPolicy: utils.GetImagePullPolicy(m),
						Name:            WebhookName,
						Args: []string{
							"/mcm-webhook",
							"--tls-cert-file=/var/run/mcm-webhook/tls.crt",
							"--tls-private-key-file=/var/run/mcm-webhook/tls.key",
						},
						Ports: []v1.ContainerPort{{ContainerPort: 8000}},
						LivenessProbe: &v1.Probe{
							Handler: v1.Handler{
								Exec: &v1.ExecAction{
									Command: []string{"ls"},
								},
							},
							InitialDelaySeconds: 15,
							PeriodSeconds:       15,
						},
						ReadinessProbe: &v1.Probe{
							Handler: v1.Handler{
								Exec: &v1.ExecAction{
									Command: []string{"ls"},
								},
							},
							InitialDelaySeconds: 15,
							PeriodSeconds:       15,
						},
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceMemory: resource.MustParse("128Mi"),
								v1.ResourceCPU:    resource.MustParse("100m"),
							},
							Limits: v1.ResourceList{
								v1.ResourceMemory: resource.MustParse("256Mi"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "webhook-cert", MountPath: "/var/run/mcm-webhook"},
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

// WebhookService creates a service object for the mcm webhook
func WebhookService(m *operatorsv1.MultiClusterHub) *corev1.Service {
	s := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      WebhookName,
			Namespace: m.Namespace,
			Labels:    defaultLabels(WebhookName),
		},
		Spec: corev1.ServiceSpec{
			Selector: defaultLabels(WebhookName),
			Ports: []corev1.ServicePort{{
				Port:       443,
				TargetPort: intstr.FromInt(8000),
			}},
		},
	}

	s.SetOwnerReferences([]metav1.OwnerReference{
		*metav1.NewControllerRef(m, m.GetObjectKind().GroupVersionKind()),
	})
	return s
}
