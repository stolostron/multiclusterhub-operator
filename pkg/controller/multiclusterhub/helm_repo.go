package multiclusterhub

import (
	"fmt"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const repoName = "multiclusterhub-repo"
const repoVersion = "1.0.0"
const repoPort = 3000

func labels() map[string]string {
	return map[string]string{
		"app": repoName,
	}
}

func repoImageName(m *operatorsv1alpha1.MultiClusterHub) string {
	imageName := fmt.Sprintf("%s/%s:%s", m.Spec.ImageRepository, repoName, repoVersion)
	if m.Spec.ImageTagSuffix == "" {
		return imageName
	}
	return imageName + "-" + m.Spec.ImageTagSuffix
}

func (r *ReconcileMultiClusterHub) helmRepoDeployment(m *operatorsv1alpha1.MultiClusterHub) *appsv1.Deployment {
	labels := labels()
	replicas := int32(1)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      repoName,
			Namespace: m.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image:           repoImageName(m),
						ImagePullPolicy: m.Spec.ImagePullPolicy,
						Name:            repoName,
						Ports: []corev1.ContainerPort{{
							ContainerPort: repoPort,
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
									Port:   intstr.FromInt(repoPort),
									Scheme: v1.URISchemeHTTP,
								},
							},
						},
						ReadinessProbe: &v1.Probe{
							Handler: v1.Handler{
								HTTPGet: &v1.HTTPGetAction{
									Path:   "/readiness",
									Port:   intstr.FromInt(repoPort),
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
					// ServiceAccountName: "default",
				},
			},
		},
	}

	if err := controllerutil.SetControllerReference(m, dep, r.scheme); err != nil {
		log.Error(err, "Failed to set controller reference", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
	}
	return dep
}

func (r *ReconcileMultiClusterHub) repoService(m *operatorsv1alpha1.MultiClusterHub) *corev1.Service {
	labels := labels()

	s := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      repoName,
			Namespace: m.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: labels,
			Ports: []corev1.ServicePort{{
				Protocol:   corev1.ProtocolTCP,
				Port:       repoPort,
				TargetPort: intstr.FromInt(repoPort),
			}},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	if err := controllerutil.SetControllerReference(m, s, r.scheme); err != nil {
		log.Error(err, "Failed to set controller reference", "Service.Namespace", s.Namespace, "Service.Name", s.Name)
	}
	return s
}
