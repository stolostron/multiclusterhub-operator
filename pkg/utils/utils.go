package utils

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
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
	if m.Spec.Version == "" {
		return false
	}

	if m.Spec.ImageRepository == "" {
		return false
	}

	if m.Spec.ImagePullPolicy == "" {
		return false
	}

	if m.Spec.Mongo.Storage == "" {
		return false
	}

	if m.Spec.Mongo.StorageClass == "" {
		return false
	}

	if m.Spec.Etcd.Storage == "" {
		return false
	}

	if m.Spec.Etcd.StorageClass == "" {
		return false
	}

	if m.Spec.ReplicaCount <= 0 {
		return false
	}

	if m.Spec.Mongo.ReplicaCount <= 0 {
		return false
	}

	return true
}

func GenerateNodeSelectorNotation(mch *operatorsv1alpha1.MultiClusterHub) string {
	nodeSelectorOptions := mch.Spec.NodeSelector
	if nodeSelectorOptions == nil {
		return ""
	}

	selectormap := map[string]string{}
	if nodeSelectorOptions.OS != "" {
		selectormap["beta.kubernetes.io/os"] = nodeSelectorOptions.OS
	}
	if nodeSelectorOptions.CustomLabelSelector != "" && nodeSelectorOptions.CustomLabelValue != "" {
		selectormap[nodeSelectorOptions.CustomLabelSelector] = nodeSelectorOptions.CustomLabelValue
	}
	if len(selectormap) == 0 {
		return ""
	}
	selectors := []string{}
	for k, v := range selectormap {
		selectors = append(selectors, fmt.Sprintf("\"%s\":\"%s\"", k, v))
	}

	return strings.Join(selectors, ",")
}
