// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package foundation

import (
	operatorsv1 "github.com/open-cluster-management/multiclusterhub-operator/pkg/apis/operator/v1"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
)

const (
	// OCMProxyServerName is the name of the ocm proxy server deployment
	OCMProxyServerName string = "ocm-proxyserver"

	OCMProxyAPIServiceName               string = "v1beta1.proxy.open-cluster-management.io"
	OCMClusterViewV1APIServiceName       string = "v1.clusterview.open-cluster-management.io"
	OCMClusterViewV1alpha1APIServiceName string = "v1alpha1.clusterview.open-cluster-management.io"
	OCMProxyGroup                        string = "proxy.open-cluster-management.io"
	OCMClusterViewGroup                  string = "clusterview.open-cluster-management.io"
)

// OCMProxyServerDeployment creates the deployment for the ocm proxy server
func OCMProxyServerDeployment(m *operatorsv1.MultiClusterHub, overrides map[string]string) *appsv1.Deployment {
	replicas := getReplicaCount(m)
	mode := int32(420)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OCMProxyServerName,
			Namespace: m.Namespace,
			Labels:    defaultLabels(OCMProxyServerName),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: defaultLabels(OCMProxyServerName),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: defaultLabels(OCMProxyServerName),
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets:   []corev1.LocalObjectReference{{Name: m.Spec.ImagePullSecret}},
					ServiceAccountName: ServiceAccount,
					Tolerations:        defaultTolerations(),
					NodeSelector:       m.Spec.NodeSelector,
					Affinity:           utils.DistributePods("ocm-antiaffinity-selector", OCMProxyServerName),
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
						Name:            OCMProxyServerName,
						Args: []string{
							"/proxyserver",
							"--secure-port=6443",
							"--cert-dir=/tmp",
							"--agent-cafile=/var/run/klusterlet/ca.crt",
							"--agent-certfile=/var/run/klusterlet/tls.crt",
							"--agent-keyfile=/var/run/klusterlet/tls.key",
						},
						LivenessProbe: &v1.Probe{
							Handler: v1.Handler{
								HTTPGet: &v1.HTTPGetAction{
									Path:   "/healthz",
									Port:   intstr.FromInt(6443),
									Scheme: v1.URISchemeHTTPS,
								},
							},
							InitialDelaySeconds: 2,
							PeriodSeconds:       10,
						},
						ReadinessProbe: &v1.Probe{
							Handler: v1.Handler{
								HTTPGet: &v1.HTTPGetAction{
									Path:   "/healthz",
									Port:   intstr.FromInt(6443),
									Scheme: v1.URISchemeHTTPS,
								},
							},
							InitialDelaySeconds: 2,
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

// OCMProxyServerService creates a service object for the ocm proxy server
func OCMProxyServerService(m *operatorsv1.MultiClusterHub) *corev1.Service {
	s := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      OCMProxyServerName,
			Namespace: m.Namespace,
			Labels:    defaultLabels(OCMProxyServerName),
		},
		Spec: corev1.ServiceSpec{
			Selector: defaultLabels(OCMProxyServerName),
			Ports: []corev1.ServicePort{{
				Name:       "secure",
				Protocol:   corev1.ProtocolTCP,
				Port:       443,
				TargetPort: intstr.FromInt(6443),
			}},
		},
	}

	s.SetOwnerReferences([]metav1.OwnerReference{
		*metav1.NewControllerRef(m, m.GetObjectKind().GroupVersionKind()),
	})
	return s
}

// OCMProxyAPIService creates an apiservice object for the ocm proxy api
func OCMProxyAPIService(m *operatorsv1.MultiClusterHub) *apiregistrationv1.APIService {
	s := &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name: OCMProxyAPIServiceName,
		},
		Spec: apiregistrationv1.APIServiceSpec{
			Service: &apiregistrationv1.ServiceReference{
				Namespace: m.Namespace,
				Name:      OCMProxyServerName,
			},
			Group:                 OCMProxyGroup,
			Version:               "v1beta1",
			InsecureSkipTLSVerify: true,
			GroupPriorityMinimum:  10000,
			VersionPriority:       20,
		},
	}

	return s
}

// OCMClusterViewV1APIService creates an apiservice object for the ocm clusterview api v1
func OCMClusterViewV1APIService(m *operatorsv1.MultiClusterHub) *apiregistrationv1.APIService {
	s := &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name: OCMClusterViewV1APIServiceName,
		},
		Spec: apiregistrationv1.APIServiceSpec{
			Service: &apiregistrationv1.ServiceReference{
				Namespace: m.Namespace,
				Name:      OCMProxyServerName,
			},
			Group:                 OCMClusterViewGroup,
			Version:               "v1",
			InsecureSkipTLSVerify: true,
			GroupPriorityMinimum:  10,
			VersionPriority:       20,
		},
	}

	return s
}

// OCMClusterViewV1alpha1APIService creates an apiservice object for the ocm clusterview api V1alpha1
func OCMClusterViewV1alpha1APIService(m *operatorsv1.MultiClusterHub) *apiregistrationv1.APIService {
	s := &apiregistrationv1.APIService{
		ObjectMeta: metav1.ObjectMeta{
			Name: OCMClusterViewV1alpha1APIServiceName,
		},
		Spec: apiregistrationv1.APIServiceSpec{
			Service: &apiregistrationv1.ServiceReference{
				Namespace: m.Namespace,
				Name:      OCMProxyServerName,
			},
			Group:                 OCMClusterViewGroup,
			Version:               "v1alpha1",
			InsecureSkipTLSVerify: true,
			GroupPriorityMinimum:  10,
			VersionPriority:       20,
		},
	}

	return s
}
