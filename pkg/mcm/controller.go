// Copyright (c) 2020 Red Hat, Inc.

package mcm

import (
	operatorsv11 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ControllerName is the name of the mcm controller deployment
const ControllerName string = "mcm-controller"

// ControllerDeployment creates the deployment for the mcm controller
func ControllerDeployment(m *operatorsv1.MultiClusterHub, overrides map[string]string) *appsv1.Deployment {
	replicas := getReplicaCount(m)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ControllerName,
			Namespace: m.Namespace,
			Labels:    defaultLabels(ControllerName),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: defaultLabels(ControllerName),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: defaultLabels(ControllerName),
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets:   []corev1.LocalObjectReference{{Name: m.Spec.ImagePullSecret}},
					ServiceAccountName: ServiceAccount,
					NodeSelector:       m.Spec.NodeSelector,
					Affinity:           utils.DistributePods("app", ControllerName),
					Containers: []corev1.Container{{
						Image:           Image(overrides),
						ImagePullPolicy: utils.GetImagePullPolicy(m),
						Name:            ControllerName,
						Args: []string{
							"/mcm-controller",
							"--leader-elect=true",
							"--enable-rbac=true",
							"--enable-service-registry=true",
							"--enable-inventory=true",
						},
						Resources: v1.ResourceRequirements{
							Requests: v1.ResourceList{
								v1.ResourceCPU:    resource.MustParse("100m"),
								v1.ResourceMemory: resource.MustParse("256Mi"),
							},
							Limits: v1.ResourceList{
								v1.ResourceMemory: resource.MustParse("2048Mi"),
							},
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
