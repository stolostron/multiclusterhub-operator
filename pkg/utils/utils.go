// Copyright (c) 2020 Red Hat, Inc.

package utils

import (
	"encoding/json"
	"os"
	"strings"
	"time"

	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	// WebhookServiceName ...
	WebhookServiceName = "multiclusterhub-operator-webhook"

	// KlusterletSecretName ...
	KlusterletSecretName = "ocm-klusterlet-self-signed-secrets" // #nosec G101 (no confidential credentials)

	// CertManagerNamespace ...
	CertManagerNamespace = "cert-manager"

	podNamespaceEnvVar = "POD_NAMESPACE"
	rsaKeySize         = 2048
	duration365d       = time.Hour * 24 * 365

	// DefaultRepository ...
	DefaultRepository = "quay.io/open-cluster-management"

	// UnitTestEnvVar ...
	UnitTestEnvVar = "UNIT_TEST"
)

var (
	// DefaultSSLCiphers defines the default cipher configuration used by management ingress
	DefaultSSLCiphers = []string{
		"ECDHE-ECDSA-AES256-GCM-SHA384",
		"ECDHE-RSA-AES256-GCM-SHA384",
		"ECDHE-ECDSA-CHACHA20-POLY1305",
		"ECDHE-RSA-CHACHA20-POLY1305",
		"ECDHE-ECDSA-AES128-GCM-SHA256",
		"ECDHE-RSA-AES128-GCM-SHA256",
	}
)

// CertManagerNS returns the namespace to deploy cert manager objects
func CertManagerNS(m *operatorsv1.MultiClusterHub) string {
	if m.Spec.SeparateCertificateManagement {
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
func MchIsValid(m *operatorsv1.MultiClusterHub) bool {
	invalid := len(m.Spec.Ingress.SSLCiphers) == 0 || !AvailabilityConfigIsValid(m.Spec.AvailabilityConfig)
	return !invalid
}

// DefaultReplicaCount returns an integer corresponding to the default number of replicas
// for HA or non-HA modes
func DefaultReplicaCount(mch *operatorsv1.MultiClusterHub) int {
	if mch.Spec.AvailabilityConfig == operatorsv1.HABasic {
		return 1
	}
	return 2
}

//AvailabilityConfigIsValid ...
func AvailabilityConfigIsValid(config operatorsv1.AvailabilityType) bool {
	switch config {
	case operatorsv1.HAHigh, operatorsv1.HABasic:
		return true
	default:
		return false
	}
}

// DistributePods returns a anti-affinity rule that specifies a preference for pod replicas with
// the matching key-value label to run across different nodes and zones
func DistributePods(key string, value string) *corev1.Affinity {
	return &corev1.Affinity{
		PodAntiAffinity: &corev1.PodAntiAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
				{
					PodAffinityTerm: corev1.PodAffinityTerm{
						TopologyKey: "kubernetes.io/hostname",
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      key,
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{value},
								},
							},
						},
					},
					Weight: 35,
				},
				{
					PodAffinityTerm: corev1.PodAffinityTerm{
						TopologyKey: "failure-domain.beta.kubernetes.io/zone",
						LabelSelector: &metav1.LabelSelector{
							MatchExpressions: []metav1.LabelSelectorRequirement{
								{
									Key:      key,
									Operator: metav1.LabelSelectorOpIn,
									Values:   []string{value},
								},
							},
						},
					},
					Weight: 70,
				},
			},
		},
	}
}

//GetImagePullPolicy returns either pull policy from CR overrides or default of Always
func GetImagePullPolicy(m *operatorsv1.MultiClusterHub) v1.PullPolicy {
	if m.Spec.Overrides.ImagePullPolicy == "" {
		return corev1.PullAlways
	}
	return m.Spec.Overrides.ImagePullPolicy
}

func IsUnitTest() bool {
	if unitTest, found := os.LookupEnv(UnitTestEnvVar); found {
		if unitTest == "true" {
			return true
		}
	}
	return false
}

// FormatSSLCiphers converts an array of ciphers into a string consumed by the management
// ingress chart
func FormatSSLCiphers(ciphers []string) string {
	return strings.Join(ciphers, ":")
}
