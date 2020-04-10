package mcm

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

// APIServerName is the name of the mcm apiserver deployment
const APIServerName string = "mcm-apiserver"

// APIServerDeployment creates the deployment for the mcm apiserver
func APIServerDeployment(m *operatorsv1alpha1.MultiClusterHub) *appsv1.Deployment {
	replicas := int32(m.Spec.ReplicaCount)
	mode := int32(420)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      APIServerName,
			Namespace: m.Namespace,
			Labels:    defaultLabels(APIServerName),
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: defaultLabels(APIServerName),
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: defaultLabels(APIServerName),
				},
				Spec: corev1.PodSpec{
					ImagePullSecrets:   []corev1.LocalObjectReference{{Name: m.Spec.ImagePullSecret}},
					ServiceAccountName: ServiceAccount,
					NodeSelector:       nodeSelectors(m),
					Volumes: []corev1.Volume{
						{
							Name: "apiserver-certs",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{SecretName: utils.APIServerSecretName},
							},
						},
						{
							Name: "klusterlet-certs",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{SecretName: utils.KlusterletSecretName},
							},
						},
						corev1.Volume{
							Name: "mongodb-ca-cert",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{DefaultMode: &mode, SecretName: utils.MongoCaSecret},
							},
						},
						corev1.Volume{
							Name: "mongodb-client-cert",
							VolumeSource: corev1.VolumeSource{
								Secret: &corev1.SecretVolumeSource{DefaultMode: &mode, SecretName: utils.MongoTLSSecret},
							},
						},
					},
					Containers: []corev1.Container{{
						Image:           mcmImage(m),
						ImagePullPolicy: m.Spec.ImagePullPolicy,
						Name:            APIServerName,
						Args: []string{
							"/mcm-apiserver",
							"--mongo-database=mcm",
							"--enable-admission-plugins=HCMUserIdentity,KlusterletCA,NamespaceLifecycle",
							"--secure-port=6443",
							"--tls-cert-file=/var/run/apiserver/tls.crt",
							"--tls-private-key-file=/var/run/apiserver/tls.key",
							"--klusterlet-cafile=/var/run/klusterlet/ca.crt",
							"--klusterlet-certfile=/var/run/klusterlet/tls.crt",
							"--klusterlet-keyfile=/var/run/klusterlet/tls.key",
							"--http2-max-streams-per-connection=1000",
							"--etcd-servers=" + fmt.Sprintf("http://etcd-cluster.%s.svc.cluster.local:2379", m.Namespace),
							"--mongo-host=" + utils.MongoEndpoints,
							"--mongo-replicaset=" + utils.MongoReplicaSet,
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
								v1.ResourceCPU:    resource.MustParse("200m"),
								v1.ResourceMemory: resource.MustParse("256Mi"),
							},
							Limits: v1.ResourceList{
								v1.ResourceMemory: resource.MustParse("2048Mi"),
							},
						},
						VolumeMounts: []corev1.VolumeMount{
							{Name: "apiserver-certs", MountPath: "/var/run/apiserver"},
							{Name: "klusterlet-certs", MountPath: "/var/run/klusterlet"},
							{Name: "mongodb-ca-cert", MountPath: "/certs/mongodb-ca"},
							{Name: "mongodb-client-cert", MountPath: "/certs/mongodb-client"},
						},
						Env: []v1.EnvVar{
							{
								Name: "MONGO_USERNAME",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										Key: "user",
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "mongodb-admin",
										},
									},
								},
							},
							{
								Name: "MONGO_PASSWORD",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										Key: "password",
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "mongodb-admin",
										},
									},
								},
							},
							{Name: "MONGO_SSLCA", Value: "/certs/mongodb-ca/tls.crt"},
							{Name: "MONGO_SSLCERT", Value: "/certs/mongodb-client/tls.crt"},
							{Name: "MONGO_SSLKEY", Value: "/certs/mongodb-client/tls.key"},
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

// APIServerService creates a service object for the mcm apiserver
func APIServerService(m *operatorsv1alpha1.MultiClusterHub) *corev1.Service {
	s := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      APIServerName,
			Namespace: m.Namespace,
			Labels:    defaultLabels(APIServerName),
		},
		Spec: corev1.ServiceSpec{
			Selector: defaultLabels(APIServerName),
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
