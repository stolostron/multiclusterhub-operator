package apiserver

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
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// Name of mcm apiserver
const Name = "mcm-apiserver"

// ImageName of container
const ImageName = "multicloud-manager"

// ImageVersion of container
const ImageVersion = "0.0.1"

var labels = map[string]string{
	"app": Name,
}

func setArgs(m *operatorsv1alpha1.MultiClusterHub) []string {
	return []string{
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
	}
}

// Deployment for the mcm apiserver
func Deployment(m *operatorsv1alpha1.MultiClusterHub) *appsv1.Deployment {
	replicas := int32(1)
	mode := int32(420)

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      Name,
			Namespace: m.Namespace,
			Labels:    labels,
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
					ImagePullSecrets:   []corev1.LocalObjectReference{{Name: m.Spec.ImagePullSecret}},
					ServiceAccountName: "hub-sa",
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
						Image:           image(m),
						ImagePullPolicy: m.Spec.ImagePullPolicy,
						Name:            Name,
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

func image(mch *operatorsv1alpha1.MultiClusterHub) string {
	image := fmt.Sprintf("%s/%s:%s", mch.Spec.ImageRepository, ImageName, ImageVersion)
	if mch.Spec.ImageTagSuffix == "" {
		return image
	}
	return image + "-" + mch.Spec.ImageTagSuffix
}

func nodeSelectors(mch *operatorsv1alpha1.MultiClusterHub) map[string]string {
	selectors := map[string]string{}
	if mch.Spec.NodeSelector == nil {
		return nil
	}

	if mch.Spec.NodeSelector.OS != "" {
		selectors["kubernetes.io/os"] = mch.Spec.NodeSelector.OS
	}
	if mch.Spec.NodeSelector.CustomLabelSelector != "" && mch.Spec.NodeSelector.CustomLabelValue != "" {
		selectors[mch.Spec.NodeSelector.CustomLabelSelector] = mch.Spec.NodeSelector.CustomLabelValue
	}
	return selectors
}

// ValidateDeployment returns a deep copy of the deployment with the desired spec based on the MultiClusterHub spec.
// Returns true if an update is needed to reconcile differences with the current spec.
func ValidateDeployment(m *operatorsv1alpha1.MultiClusterHub, dep *appsv1.Deployment) (*appsv1.Deployment, bool) {
	var log = logf.Log.WithValues("Deployment.Namespace", dep.GetNamespace(), "Deployment.Name", dep.GetName())
	found := dep.DeepCopy()

	pod := &found.Spec.Template.Spec.Containers[0]
	needsUpdate := false

	// verify image pull secret
	if m.Spec.ImagePullSecret != "" {
		ps := corev1.LocalObjectReference{Name: m.Spec.ImagePullSecret}
		if !utils.ContainsPullSecret(found.Spec.Template.Spec.ImagePullSecrets, ps) {
			log.Info("Enforcing imagePullSecret from CR spec")
			found.Spec.Template.Spec.ImagePullSecrets = append(found.Spec.Template.Spec.ImagePullSecrets, ps)
			needsUpdate = true
		}
	}

	// verify image repository and suffix
	image := image(m)
	if pod.Image != image {
		log.Info("Enforcing image repo and suffix from CR spec")
		found.Spec.Template.Spec.Containers[0].Image = image
		needsUpdate = true
	}

	// verify image pull policy
	if pod.ImagePullPolicy != m.Spec.ImagePullPolicy {
		log.Info("Enforcing imagePullPolicy from CR spec")
		pod.ImagePullPolicy = m.Spec.ImagePullPolicy
		needsUpdate = true
	}

	return found, needsUpdate
}
