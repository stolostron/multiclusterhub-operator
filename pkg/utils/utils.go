package utils

import (
	"encoding/json"
	"time"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	// WebhookServiceName ...
	WebhookServiceName = "multiclusterhub-operator-webhook"
	// APIServerSecretName ...
	APIServerSecretName = "mcm-apiserver-self-signed-secrets" // #nosec G101 (no confidential credentials)
	// KlusterletSecretName ...
	KlusterletSecretName = "mcm-klusterlet-self-signed-secrets" // #nosec G101 (no confidential credentials)

	// CertManagerNamespace ...
	CertManagerNamespace = "cert-manager"

	// MongoEndpoints ...
	MongoEndpoints = "multicluster-mongodb"
	// MongoReplicaSet ...
	MongoReplicaSet = "rs0"
	// MongoTLSSecret ...
	MongoTLSSecret = "multicluster-mongodb-client-cert"
	// MongoCaSecret ...
	MongoCaSecret = "multicloud-ca-cert" // #nosec G101 (no confidential credentials)

	podNamespaceEnvVar = "POD_NAMESPACE"
	apiserviceName     = "mcm-apiserver"
	rsaKeySize         = 2048
	duration365d       = time.Hour * 24 * 365

	// DefaultRepository ...
	DefaultRepository = "quay.io/open-cluster-management"
	// LatestVerison ...
	LatestVerison = "latest"
)

// CacheSpec ...
type CacheSpec struct {
	IngressDomain string
}

// CertManagerNS returns the namespace to deploy cert manager objects
func CertManagerNS(m *operatorsv1alpha1.MultiClusterHub) string {
	if m.Spec.CloudPakCompatibility {
		return CertManagerNamespace
	}
	return m.Namespace
}

// ContainsPullSecret returns whether a list of pullSecrets contains a given pull secret
func ContainsPullSecret(pullSecrets []corev1.LocalObjectReference, ps corev1.LocalObjectReference) bool {
	for _, v := range pullSecrets {
		if v == ps {
			return true
		}
	}
	return false
}

// ContainsMap returns whether the expected map entries are included in the map
func ContainsMap(all map[string]string, expected map[string]string) bool {
	for key, exval := range expected {
		allval, ok := all[key]
		if !ok || allval != exval {
			return false
		}

	}
	return true
}

// NodeSelectors returns a (v1.PodSpec).NodeSelector map
func NodeSelectors(mch *operatorsv1alpha1.MultiClusterHub) map[string]string {
	selectors := map[string]string{}
	if mch.Spec.NodeSelector == nil {
		return nil
	}

	if mch.Spec.NodeSelector.OS != "" {
		selectors["kubernetes.io/os"] = mch.Spec.NodeSelector.OS
	}
	if mch.Spec.NodeSelector.CustomLabelSelector != "" {
		selectors[mch.Spec.NodeSelector.CustomLabelSelector] = mch.Spec.NodeSelector.CustomLabelValue
	}
	return selectors
}

// AddInstallerLabel adds Installer Labels ...
func AddInstallerLabel(u *unstructured.Unstructured, name string, ns string) {
	labels := make(map[string]string)
	for key, value := range u.GetLabels() {
		labels[key] = value
	}
	labels["installer.name"] = name
	labels["installer.namespace"] = ns

	u.SetLabels(labels)
}

// CoreToUnstructured converts a Core Kube resource to unstructured
func CoreToUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	content, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{}
	err = u.UnmarshalJSON(content)
	return u, err
}

// MchIsValid Checks if the optional default parameters need to be set
func MchIsValid(m *operatorsv1alpha1.MultiClusterHub) bool {
	invalid := m.Spec.Version == "" ||
		m.Spec.ImageRepository == "" ||
		m.Spec.ImagePullPolicy == "" ||
		m.Spec.Mongo.Storage == "" ||
		m.Spec.Mongo.StorageClass == "" ||
		m.Spec.Etcd.Storage == "" ||
		m.Spec.Etcd.StorageClass == "" ||
		m.Spec.ReplicaCount <= 0 ||
		m.Spec.Mongo.ReplicaCount <= 0

	return !invalid
}

// ValidateDeployment returns a deep copy of the deployment with the desired spec based on the MultiClusterHub spec.
// Returns true if an update is needed to reconcile differences with the current spec.
func ValidateDeployment(m *operatorsv1alpha1.MultiClusterHub, dep *appsv1.Deployment, image string) (*appsv1.Deployment, bool) {
	var log = logf.Log.WithValues("Deployment.Namespace", dep.GetNamespace(), "Deployment.Name", dep.GetName())
	found := dep.DeepCopy()

	pod := &found.Spec.Template.Spec
	container := &found.Spec.Template.Spec.Containers[0]
	needsUpdate := false

	// verify image pull secret
	if m.Spec.ImagePullSecret != "" {
		ps := corev1.LocalObjectReference{Name: m.Spec.ImagePullSecret}
		if !ContainsPullSecret(pod.ImagePullSecrets, ps) {
			log.Info("Enforcing imagePullSecret from CR spec")
			pod.ImagePullSecrets = append(pod.ImagePullSecrets, ps)
			needsUpdate = true
		}
	}

	// verify image repository and suffix
	if container.Image != image {
		log.Info("Enforcing image repo and suffix from CR spec")
		container.Image = image
		needsUpdate = true
	}

	// verify image pull policy
	if container.ImagePullPolicy != m.Spec.ImagePullPolicy {
		log.Info("Enforcing imagePullPolicy from CR spec")
		container.ImagePullPolicy = m.Spec.ImagePullPolicy
		needsUpdate = true
	}

	// verify node selectors
	desiredSelectors := NodeSelectors(m)
	if !ContainsMap(pod.NodeSelector, desiredSelectors) {
		log.Info("Enforcing node selectors from CR spec")
		pod.NodeSelector = desiredSelectors
		needsUpdate = true
	}

	// verify replica count
	if *found.Spec.Replicas != int32(m.Spec.ReplicaCount) {
		log.Info("Enforcing replicaCount from CR spec")
		replicas := int32(m.Spec.ReplicaCount)
		found.Spec.Replicas = &replicas
		needsUpdate = true
	}

	return found, needsUpdate
}
