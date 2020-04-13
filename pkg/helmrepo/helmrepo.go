package helmrepo

import (
	"fmt"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// Name of helm repo
const Name = "multiclusterhub-repo"

// Port of helm repo service
const Port = 3000

// Version of helm repo image
const Version = "1.0.0"

func labels() map[string]string {
	return map[string]string{
		"app": Name,
	}
}

func Image(m *operatorsv1alpha1.MultiClusterHub) string {
	imageName := fmt.Sprintf("%s/%s:%s", m.Spec.ImageRepository, Name, Version)
	if m.Spec.ImageTagSuffix == "" {
		return imageName
	}
	return imageName + "-" + m.Spec.ImageTagSuffix
}

// Deployment for the helm repo serving charts
func Deployment(m *operatorsv1alpha1.MultiClusterHub) *appsv1.Deployment {
	replicas := int32(m.Spec.ReplicaCount)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Name,
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
						Image:           Image(m),
						ImagePullPolicy: m.Spec.ImagePullPolicy,
						Name:            Name,
						Ports: []corev1.ContainerPort{{
							ContainerPort: Port,
							Name:          "helmrepo",
						}},
						Resources: v1.ResourceRequirements{
							Limits: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("50m"),
								v1.ResourceMemory: resource.MustParse("100Mi"),
							},
							Requests: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("50m"),
								v1.ResourceMemory: resource.MustParse("50Mi"),
							},
						},
						LivenessProbe: &v1.Probe{
							Handler: v1.Handler{
								HTTPGet: &v1.HTTPGetAction{
									Path:   "/liveness",
									Port:   intstr.FromInt(Port),
									Scheme: v1.URISchemeHTTP,
								},
							},
						},
						ReadinessProbe: &v1.Probe{
							Handler: v1.Handler{
								HTTPGet: &v1.HTTPGetAction{
									Path:   "/readiness",
									Port:   intstr.FromInt(Port),
									Scheme: v1.URISchemeHTTP,
								},
							},
						},
						Env: []v1.EnvVar{
							{
								Name:      "POD_NAMESPACE",
								ValueFrom: &v1.EnvVarSource{FieldRef: &v1.ObjectFieldSelector{FieldPath: "metadata.namespace"}},
							},
						},
					}},
					ImagePullSecrets: []corev1.LocalObjectReference{{Name: m.Spec.ImagePullSecret}},
					NodeSelector:     utils.NodeSelectors(m),
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
func Service(m *operatorsv1alpha1.MultiClusterHub) *corev1.Service {
	labels := labels()

	s := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Name,
			Namespace: m.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{{
				Protocol:   corev1.ProtocolTCP,
				Port:       Port,
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
