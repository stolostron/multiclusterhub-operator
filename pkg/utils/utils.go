// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// WebhookServiceName ...
	WebhookServiceName = "multiclusterhub-operator-webhook"

	// CertManagerNamespace ...
	CertManagerNamespace = "cert-manager"

	podNamespaceEnvVar = "POD_NAMESPACE"
	rsaKeySize         = 2048
	duration365d       = time.Hour * 24 * 365

	// DefaultRepository ...
	DefaultRepository = "quay.io/stolostron"

	// UnitTestEnvVar ...
	UnitTestEnvVar = "UNIT_TEST"

	// MCHOperatorName is the name of this operator deployment
	MCHOperatorName = "multiclusterhub-operator"

	// SubscriptionOperatorName is the name of the operator deployment managing application subscriptions
	SubscriptionOperatorName = "multicluster-operators-standalone-subscription"

	MCESubscriptionName          = "multicluster-engine"
	MCESubscriptionNamespace     = "multicluster-engine"
	ClusterSubscriptionNamespace = "open-cluster-management-backup"

	MCEManagedByLabel = "multiclusterhubs.operator.open-cluster-management.io/managed-by"
)

var (
	// DefaultSSLCiphers defines the default cipher configuration used by management ingress
	DefaultSSLCiphers = []string{
		"ECDHE-ECDSA-AES256-GCM-SHA384",
		"ECDHE-RSA-AES256-GCM-SHA384",
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

// AddDeploymentLabels ...
func AddDeploymentLabels(d *appsv1.Deployment, labels map[string]string) bool {
	updated := false
	if d.Labels == nil {
		d.Labels = labels
		return true
	}

	for k, v := range labels {
		if d.Labels[k] != v {
			d.Labels[k] = v
			updated = true
		}
	}

	return updated
}

// AddPodLabels ...
func AddPodLabels(d *appsv1.Deployment, labels map[string]string) bool {
	updated := false
	if d.Spec.Template.Labels == nil {
		d.Spec.Template.Labels = labels
		return true
	}

	for k, v := range labels {
		if d.Spec.Template.Labels[k] != v {
			d.Spec.Template.Labels[k] = v
			updated = true
		}
	}

	return updated
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
	if m.Spec.Overrides == nil || m.Spec.Overrides.ImagePullPolicy == "" {
		return corev1.PullIfNotPresent
	}
	return m.Spec.Overrides.ImagePullPolicy
}

// GetContainerArgs return arguments forfirst container in deployment
func GetContainerArgs(dep *appsv1.Deployment) []string {
	return dep.Spec.Template.Spec.Containers[0].Args
}

// GetContainerEnvVars returns environment variables for first container in deployment
func GetContainerEnvVars(dep *appsv1.Deployment) []v1.EnvVar {
	return dep.Spec.Template.Spec.Containers[0].Env
}

// GetContainerVolumeMounts returns volume mount for first container in deployment
func GetContainerVolumeMounts(dep *appsv1.Deployment) []corev1.VolumeMount {
	return dep.Spec.Template.Spec.Containers[0].VolumeMounts
}

// GetDeploymentVolumes returns volumes in deployment
func GetContainerVolumes(dep *appsv1.Deployment) []corev1.Volume {
	return dep.Spec.Template.Spec.Volumes
}

//GetContainerRequestResources returns Request Requirements for first container in deployment
func GetContainerRequestResources(dep *appsv1.Deployment) corev1.ResourceList {
	return dep.Spec.Template.Spec.Containers[0].Resources.Requests
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

// TrackedNamespaces returns the list of namespaces we deploy components to and should track
func TrackedNamespaces(m *operatorsv1.MultiClusterHub) []string {
	trackedNamespaces := []string{m.Namespace}
	if m.Spec.SeparateCertificateManagement {
		trackedNamespaces = append(trackedNamespaces, CertManagerNamespace)
	}
	if m.Enabled(operatorsv1.ClusterBackup) {
		trackedNamespaces = append(trackedNamespaces, ClusterSubscriptionNamespace)
	}
	return trackedNamespaces
}

// GetDisableClusterImageSets returns true or false for whether auto update for clusterImageSets should be disabled
func GetDisableClusterImageSets(m *operatorsv1.MultiClusterHub) string {
	if m.Spec.DisableUpdateClusterImageSets {
		return "true"
	}
	return "false"
}

// ProxyEnvVarIsSet ...
// OLM handles these environment variables as a unit;
// if at least one of them is set, all three are considered overridden
// and the cluster-wide defaults are not used for the deployments of the subscribed Operator.
// https://docs.openshift.com/container-platform/4.6/operators/admin/olm-configuring-proxy-support.html
// GetProxyEnvVars
func ProxyEnvVarsAreSet() bool {
	if os.Getenv("HTTP_PROXY") != "" || os.Getenv("HTTPS_PROXY") != "" || os.Getenv("NO_PROXY") != "" {
		return true
	}
	return false
}

// FindNamespace
func FindNamespace() (string, error) {
	ns, found := os.LookupEnv(podNamespaceEnvVar)
	if !found {
		return "", fmt.Errorf("%s envvar is not set", podNamespaceEnvVar)
	}
	return ns, nil
}

func GetDeployments(m *operatorsv1.MultiClusterHub) []types.NamespacedName {
	return []types.NamespacedName{
		{Name: "multiclusterhub-repo", Namespace: m.Namespace},
	}
}

func GetAppsubs(m *operatorsv1.MultiClusterHub) []types.NamespacedName {
	appsubs := []types.NamespacedName{
		{Name: "console-chart-sub", Namespace: m.Namespace},
		{Name: "policyreport-sub", Namespace: m.Namespace},
		{Name: "grc-sub", Namespace: m.Namespace},
		{Name: "management-ingress-sub", Namespace: m.Namespace},
		{Name: "cluster-lifecycle-sub", Namespace: m.Namespace},
		{Name: "search-prod-sub", Namespace: m.Namespace},
		{Name: "volsync-addon-controller-sub", Namespace: m.Namespace},
	}
	if m.Enabled(operatorsv1.ClusterBackup) {
		appsubs = append(appsubs, types.NamespacedName{Name: "cluster-backup-chart-sub", Namespace: ClusterSubscriptionNamespace})
	}
	if m.Enabled(operatorsv1.ClusterProxyAddon) {
		appsubs = append(appsubs, types.NamespacedName{Name: "cluster-proxy-addon-sub", Namespace: m.Namespace})
	}
	return appsubs
}

func GetCustomResources(m *operatorsv1.MultiClusterHub) []types.NamespacedName {
	return []types.NamespacedName{
		{Name: "multicluster-engine-sub", Namespace: MCESubscriptionNamespace},
		{Name: "multicluster-engine-csv", Namespace: MCESubscriptionNamespace},
		{Name: "multicluster-engine"},
	}
}

func GetDeploymentsForStatus(m *operatorsv1.MultiClusterHub) []types.NamespacedName {
	nn := []types.NamespacedName{}
	if m.Enabled("multiclusterhub-repo") {
		nn = append(nn, types.NamespacedName{Name: "multiclusterhub-repo", Namespace: m.Namespace})
	}
	return nn
}

func GetAppsubsForStatus(m *operatorsv1.MultiClusterHub) []types.NamespacedName {
	nn := []types.NamespacedName{}
	if m.Enabled(operatorsv1.Console) {
		nn = append(nn, types.NamespacedName{Name: "console-chart-sub", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.Insights) {
		nn = append(nn, types.NamespacedName{Name: "policyreport-sub", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.GRC) {
		nn = append(nn, types.NamespacedName{Name: "grc-sub", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.ManagementIngress) {
		nn = append(nn, types.NamespacedName{Name: "management-ingress-sub", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.ClusterLifecycle) {
		nn = append(nn, types.NamespacedName{Name: "cluster-lifecycle-sub", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.Search) {
		nn = append(nn, types.NamespacedName{Name: "search-prod-sub", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.ClusterBackup) {
		nn = append(nn, types.NamespacedName{Name: "cluster-backup-chart-sub", Namespace: ClusterSubscriptionNamespace})
	}
	if m.Enabled(operatorsv1.ClusterProxyAddon) {
		nn = append(nn, types.NamespacedName{Name: "cluster-proxy-addon-sub", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.Volsync) {
		nn = append(nn, types.NamespacedName{Name: "volsync-addon-controller-sub", Namespace: m.Namespace})
	}
	return nn
}

func GetCustomResourcesForStatus(m *operatorsv1.MultiClusterHub) []types.NamespacedName {
	if m.Enabled(operatorsv1.MultiClusterEngine) {
		return []types.NamespacedName{
			{Name: "multicluster-engine-sub", Namespace: MCESubscriptionNamespace},
			{Name: "multicluster-engine-csv", Namespace: MCESubscriptionNamespace},
			{Name: "multicluster-engine"},
		}
	}
	return []types.NamespacedName{}
}

func GetTolerations(m *operatorsv1.MultiClusterHub) []corev1.Toleration {
	if len(m.Spec.Tolerations) == 0 {
		return []corev1.Toleration{
			{
				Effect:   "NoSchedule",
				Key:      "node-role.kubernetes.io/infra",
				Operator: "Exists",
			},
		}
	}
	return m.Spec.Tolerations
}

func RemoveString(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

func AppendProxyVariables(existing []corev1.EnvVar, added []corev1.EnvVar) []corev1.EnvVar {

	for i := 0; i < len(added); i++ {
		existing = appendIfMissing(existing, added[i])
	}
	return existing
}

func appendIfMissing(slice []corev1.EnvVar, s corev1.EnvVar) []corev1.EnvVar {
	for i := 0; i < len(slice); i++ {
		if slice[i].Name == s.Name {
			slice[i].Value = s.Value
			return slice
		}
	}
	return append(slice, s)
}

// SetDefaultComponents returns true if changes are made
func SetDefaultComponents(m *operatorsv1.MultiClusterHub) bool {
	updated := false
	for _, c := range operatorsv1.DefaultEnabledComponents {
		if !m.ComponentPresent(c) {
			m.Enable(c)
			updated = true
		}
	}
	for _, c := range operatorsv1.DefaultDisabledComponents {
		if !m.ComponentPresent(c) {
			m.Disable(c)
			updated = true
		}
	}
	return updated
}

// DeduplicateComponents removes duplicate componentconfigs by name, keeping the config of the last
// componentconfig in the list. Returns true if changes are made.
func DeduplicateComponents(m *operatorsv1.MultiClusterHub) bool {
	config := m.Spec.Overrides.Components
	newConfig := deduplicate(m.Spec.Overrides.Components)
	if len(newConfig) != len(config) {
		m.Spec.Overrides.Components = newConfig
		return true
	}
	return false
}

// deduplicate removes duplicate componentconfigs by name, keeping the config of the last
// componentconfig in the list
func deduplicate(config []operatorsv1.ComponentConfig) []operatorsv1.ComponentConfig {
	newConfig := []operatorsv1.ComponentConfig{}
	for _, cc := range config {
		duplicate := false
		// if name in newConfig update newConfig at existing index
		for i, ncc := range newConfig {
			if cc.Name == ncc.Name {
				duplicate = true
				newConfig[i] = cc
				break
			}
		}
		if !duplicate {
			newConfig = append(newConfig, cc)
		}
	}
	return newConfig
}

// MigrateToggles returns true if the hub needs to be modified to handle the change of toggle logic
// from 2.4 to 2.5.
func MigrateToggles(m *operatorsv1.MultiClusterHub) bool {
	// If cluster proxy was enabled with the deprecated flag it should continue to be enabled
	// in the components field
	if m.Spec.EnableClusterProxyAddon {
		m.Enable(operatorsv1.ClusterProxyAddon)
		m.Spec.EnableClusterProxyAddon = false
		return true
	}
	return false
}

// getMCEComponents returns mce components that are present in mch
func GetMCEComponents(mch *operatorsv1.MultiClusterHub) []mcev1.ComponentConfig {
	config := []mcev1.ComponentConfig{}
	for _, n := range operatorsv1.MCEComponents {
		if mch.ComponentPresent(n) {
			config = append(config, mcev1.ComponentConfig{Name: n, Enabled: mch.Enabled(n)})
		}
	}
	return config
}

// UpdateMCEOverrides adds MCE componenets that are present in mch
func UpdateMCEOverrides(mce *mcev1.MultiClusterEngine, mch *operatorsv1.MultiClusterHub) {
	mceComponents := GetMCEComponents(mch)
	for _, c := range mceComponents {
		if c.Enabled {
			mce.Enable(c.Name)
		} else {
			mce.Disable(c.Name)
		}
	}
	return
}
