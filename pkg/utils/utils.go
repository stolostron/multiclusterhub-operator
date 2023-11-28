// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package utils

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
)

const (
	// WebhookServiceName is the name of the webhook service.
	WebhookServiceName = "multiclusterhub-operator-webhook"

	// CertManagerNamespace is the namespace where CertManager is deployed.
	CertManagerNamespace = "cert-manager"

	// podNamespaceEnvVar is the environment variable name for the pod's namespace.
	podNamespaceEnvVar = "POD_NAMESPACE"

	// DefaultRepository is the default repository for images.
	DefaultRepository = "quay.io/stolostron"

	// UnitTestEnvVar is the environment variable for unit testing.
	UnitTestEnvVar = "UNIT_TEST"

	// MCHOperatorName is the name of the Multicluster Hub operator deployment.
	MCHOperatorName = "multiclusterhub-operator"

	// SubscriptionOperatorName is the name of the operator deployment managing application subscriptions.
	SubscriptionOperatorName = "multicluster-operators-standalone-subscription"

	// MCESubscriptionName is the name of the Multicluster Engine subscription.
	MCESubscriptionName = "multicluster-engine"

	// MCESubscriptionNamespace is the namespace for the Multicluster Engine subscription.
	MCESubscriptionNamespace = "multicluster-engine"

	// ClusterSubscriptionNamespace is the namespace for the open-cluster-management-backup subscription.
	ClusterSubscriptionNamespace = "open-cluster-management-backup"

	// MCEManagedByLabel is the label used to mark resources managed by Multicluster Hub.
	MCEManagedByLabel = "multiclusterhubs.operator.open-cluster-management.io/managed-by"

	// OpenShiftClusterMonitoringLabel is the label for OpenShift cluster monitoring.
	OpenShiftClusterMonitoringLabel = "openshift.io/cluster-monitoring"

	// AppsubChartLocation is the location of the App Subscription chart.
	AppsubChartLocation = "/charts/toggle/multicloud-operators-subscription"

	// CLCChartLocation is the location of the Cluster Lifecycle chart.
	CLCChartLocation = "/charts/toggle/cluster-lifecycle"

	// ClusterBackupChartLocation is the location of the Cluster Backup chart.
	ClusterBackupChartLocation = "/charts/toggle/cluster-backup"

	// ClusterPermissionChartLocation is the location of the Cluster Permission chart.
	ClusterPermissionChartLocation = "/charts/toggle/cluster-permission"

	// ConsoleChartLocation is the location of the Console chart.
	ConsoleChartLocation = "/charts/toggle/console"

	// GRCChartLocation is the location of the GRC chart.
	GRCChartLocation = "/charts/toggle/grc"

	// InsightsChartLocation is the location of the Insights chart.
	InsightsChartLocation = "/charts/toggle/insights"

	// MCOChartLocation is the location of the Multicluster Observability Operator chart.
	MCOChartLocation = "/charts/toggle/multicluster-observability-operator"

	// SearchV2ChartLocation is the location of the Search V2 Operator chart.
	SearchV2ChartLocation = "/charts/toggle/search-v2-operator"

	// SubmarinerAddonChartLocation is the location of the Submariner Addon chart.
	SubmarinerAddonChartLocation = "/charts/toggle/submariner-addon"

	// VolsyncChartLocation is the location of the Volsync Controller chart.
	VolsyncChartLocation = "/charts/toggle/volsync-controller"
)

const (
	/*
	   MCHOperatorMetricsServiceName is the name of the service used to expose the metrics
	   endpoint for the multiclusterhub-operator.
	*/
	MCHOperatorMetricsServiceName = "multiclusterhub-operator-metrics"

	/*
	   MCHOperatorMetricsServiceMonitorName is the name of the service monitor used to expose
	   the metrics for the multiclusterhub-operator.
	*/
	MCHOperatorMetricsServiceMonitorName = "multiclusterhub-operator-metrics"
)

var (
	/*
		(Deprecated) DefaultSSLCiphers is an array of default SSL ciphers used by the management ingress.
		SSL ciphers are encryption algorithms used to secure communication over HTTPS.
		These ciphers define the encryption strength and method used for securing data in transit.
	*/
	DefaultSSLCiphers = []string{
		"ECDHE-ECDSA-AES256-GCM-SHA384",
		"ECDHE-RSA-AES256-GCM-SHA384",
		"ECDHE-ECDSA-AES128-GCM-SHA256",
		"ECDHE-RSA-AES128-GCM-SHA256",
	}
)

/*
(Deprecated) CertManagerNS returns the namespace in which to deploy Cert Manager objects.
Cert Manager is a Kubernetes add-on for managing X.509 certificates.
The function checks if separate certificate management is enabled in the MultiClusterHub (m) and returns the
appropriate namespace based on the configuration.
*/
func CertManagerNS(m *operatorsv1.MultiClusterHub) string {
	if m.Spec.SeparateCertificateManagement {
		return CertManagerNamespace
	}
	return m.Namespace
}

/*
ContainsPullSecret check whether a given pull secret is present in a list of pull secrets.
Pull secrets are used to authenticate with container registries.
The function iterates through the list of pull secrets and returns `true` if the provided pull secret (ps) is found.
*/
func ContainsPullSecret(pullSecrets []corev1.LocalObjectReference, ps corev1.LocalObjectReference) bool {
	for _, v := range pullSecrets {
		if v == ps {
			return true
		}
	}
	return false
}

/*
ContainsMap checks if a map (`all`) contains all the key-value pairs specified in another map (`expected`).
It iterates through the key-value pairs in the `expected` map and verifies their presence and equality in the `all` map.
If all expected key-value pairs are found in the `all` map, the function returns `true`.
*/
func ContainsMap(all map[string]string, expected map[string]string) bool {
	for key, exval := range expected {
		allval, ok := all[key]
		if !ok || allval != exval {
			return false
		}

	}
	return true
}

/*
AddInstallerLabel adds installer labels to a Kubernetes resource represented by an unstructured object (`u`).
Installer labels are used to identify the installer's name and namespace. The function retrieves the existing
labels from the unstructured object, adds the installer labels, and sets them back to the object.
*/
func AddInstallerLabel(u *unstructured.Unstructured, name string, ns string) {
	labels := make(map[string]string)
	for key, value := range u.GetLabels() {
		labels[key] = value
	}
	labels["installer.name"] = name
	labels["installer.namespace"] = ns

	u.SetLabels(labels)
}

/*
AddInstallerLabels adds installer labels to a map of labels (`l`). It returns a new map of labels that includes
the installer labels with the specified installer name and namespace. The original labels in the input map are
preserved, and the installer labels are added to the new map.
*/
func AddInstallerLabels(l map[string]string, name string, ns string) map[string]string {
	labels := make(map[string]string)
	for key, value := range l {
		labels[key] = value
	}
	labels["installer.name"] = name
	labels["installer.namespace"] = ns

	return labels
}

/*
AddDeploymentLabels adds labels to a Kubernetes Deployment (`d`). It checks if the Deployment already has labels.
If not, it sets the provided labels. If the Deployment already has labels, the function updates them with the provided
labels and returns `true` if any changes are made.
*/
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

/*
AddPodLabels adds labels to the Pods managed by a Kubernetes Deployment (`d`). It checks if the Pods already have
labels. If not, it sets the provided labels. If the Pods already have labels, the function updates them with the
provided labels and returns `true` if any changes are made.
*/
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

/*
CoreToUnstructured converts a core Kubernetes resource (represented as a `runtime.Object`) into an unstructured
resource (`unstructured.Unstructured`). This conversion is useful for working with Kubernetes resources in a more
generic and flexible way.
*/
func CoreToUnstructured(obj runtime.Object) (*unstructured.Unstructured, error) {
	content, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	u := &unstructured.Unstructured{}
	err = u.UnmarshalJSON(content)
	return u, err
}

/*
MchIsValid checks if the optional default parameters need to be set in a MultiClusterHub (m). It verifies whether the
SSL ciphers and availability configuration in the MultiClusterHub are valid.
If the SSL ciphers are empty or the availability configuration is invalid, the function returns `false`.
*/
func MchIsValid(m *operatorsv1.MultiClusterHub) bool {
	invalid := len(m.Spec.Ingress.SSLCiphers) == 0 || !operatorsv1.AvailabilityConfigIsValid(m.Spec.AvailabilityConfig)
	return !invalid
}

/*
DefaultReplicaCount returns the default number of replicas for a MultiClusterHub (mch). The number of replicas is
determined based on the availability configuration in the MultiClusterHub. For High Availability (HA) configurations,
it returns 2 replicas; otherwise, it returns 1.
*/
func DefaultReplicaCount(mch *operatorsv1.MultiClusterHub) int {
	if mch.Spec.AvailabilityConfig == operatorsv1.HABasic {
		return 1
	}
	return 2
}

/*
DistributePods returns an anti-affinity rule for Kubernetes Pods. The rule specifies a preference for Pod replicas
with a matching label key-value pair to run across different nodes and zones. It helps distribute Pods to
ensure redundancy and availability.
*/
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

/*
GetImagePullPolicy determines the image pull policy for Pods within the MultiClusterHub based on the specified overrides
in the MultiClusterHub (m). If no image pull policy is specified in the overrides, it returns the default
pull policy of "PullIfNotPresent."
*/
func GetImagePullPolicy(m *operatorsv1.MultiClusterHub) corev1.PullPolicy {
	if m.Spec.Overrides == nil || m.Spec.Overrides.ImagePullPolicy == "" {
		return corev1.PullIfNotPresent
	}
	return m.Spec.Overrides.ImagePullPolicy
}

/*
IsUnitTest` checks whether the current environment is running in a unit test context. It reads the value of an
environment variable and returns `true` if the variable is set to "true."
*/
func IsUnitTest() bool {
	if unitTest, found := os.LookupEnv(UnitTestEnvVar); found {
		if unitTest == "true" {
			return true
		}
	}
	return false
}

/*
GetTestImages returns a list of test images used in the MultiClusterHub. The function provides a list of image names
that are relevant for testing and verification purposes.
*/
func GetTestImages() []string {
	return []string{
		"BAILER", "CERT_POLICY_CONTROLLER", "CLUSTER_BACKUP_CONTROLLER", "CLUSTER_LIFECYCLE_E2E", "CLUSTER_PERMISSION",
		"CLUSTER_PROXY", "CLUSTER_PROXY_ADDON", "CONSOLE", "ENDPOINT_MONITORING_OPERATOR", "GRAFANA",
		"GRAFANA_DASHBOARD_LOADER", "GRC_POLICY_FRAMEWORK_TESTS", "HELLOPROW_GO", "HELLOWORLD",
		"HYPERSHIFT_DEPLOYMENT_CONTROLLER", "IAM_POLICY_CONTROLLER", "INSIGHTS_CLIENT", "INSIGHTS_METRICS",
		"KLUSTERLET_ADDON_CONTROLLER", "KLUSTERLET_ADDON_OPERATOR", "KUBE_RBAC_PROXY", "KUBE_STATE_METRICS",
		"LIFECYCLE_BACKEND_E2E", "METRICS_COLLECTOR", "MULTICLOUD_INTEGRATIONS", "MULTICLUSTERHUB_OPERATOR",
		"MULTICLUSTERHUB_OPERATOR_TESTS", "MULTICLUSTERHUB_REPO", "MULTICLUSTER_OBSERVABILITY_OPERATOR",
		"MULTICLUSTER_OPERATORS_APPLICATION", "MULTICLUSTER_OPERATORS_CHANNEL", "MULTICLUSTER_OPERATORS_SUBSCRIPTION",
		"MUST_GATHER", "NODE_EXPORTER", "OAUTH_PROXY", "OAUTH_PROXY_48", "OAUTH_PROXY_49_AND_UP",
		"OBSERVABILITY_E2E_TEST", "OBSERVATORIUM", "OBSERVATORIUM_OPERATOR", "POSTGRESQL_12", "POSTGRESQL_13",
		"PROMETHEUS", "PROMETHEUS_ALERTMANAGER", "PROMETHEUS_CONFIG_RELOADER", "PROMETHEUS_OPERATOR",
		"RBAC_QUERY_PROXY", "REDISGRAPH_TLS", "SEARCH_AGGREGATOR", "SEARCH_API", "SEARCH_COLLECTOR", "SEARCH_E2E",
		"SEARCH_INDEXER", "SEARCH_OPERATOR", "SEARCH_V2_API", "SUBMARINER_ADDON", "THANOS", "VOLSYNC",
		"VOLSYNC_ADDON_CONTROLLER", "VOLSYNC_MOVER_RCLONE", "VOLSYNC_MOVER_RESTIC", "VOLSYNC_MOVER_RSYNC",
		"cert_policy_controller", "cluster_backup_controller", "cluster_permission", "config_policy_controller",
		"console", "governance_policy_addon_controller", "governance_policy_framework_addon",
		"governance_policy_propagator", "iam_policy_controller", "insights_client", "insights_metrics",
		"klusterlet_addon_controller", "kube_rbac_proxy", "multicloud_integrations",
		"multicluster_observability_operator", "multicluster_operators_application", "multicluster_operators_channel",
		"multicluster_operators_subscription", "postgresql_13", "search_collector", "search_indexer", "search_v2_api",
		"search_v2_operator", "submariner_addon", "volsync_addon_controller",
	}
}

/*
(Deprecated) FormatSSLCiphers is a function that converts an array of SSL ciphers (`ciphers`) into a string format that
can be consumed by the management ingress chart. It joins the individual ciphers into a single string with delimiters.
*/
func FormatSSLCiphers(ciphers []string) string {
	return strings.Join(ciphers, ":")
}

/*
TrackedNamespaces returns a list of namespaces that the MultiClusterHub should track. The function constructs this list
based on the MultiClusterHub's configuration, including the primary namespace and additional namespaces related to
specific features like certificate management and cluster backup.
*/
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

/*
GetDisableClusterImageSets determines whether auto-updates for cluster image sets should be disabled.
It checks the MultiClusterHub's configuration and returns "true" if the auto-update is disabled, or "false"
if it is not.
*/
func GetDisableClusterImageSets(m *operatorsv1.MultiClusterHub) string {
	if m.Spec.DisableUpdateClusterImageSets {
		return "true"
	}
	return "false"
}

/*
ProxyEnvVarsAreSet checks if proxy environment variables, such as `HTTP_PROXY`, `HTTPS_PROXY`, and `NO_PROXY`, are set.
It returns `true` if at least one of these proxy environment variables is defined, indicating that the system is
configured to use a proxy for network connections.

OLM handles these environment variables as a unit; if at least one of them is set, all three are considered overridden
and the cluster-wide defaults are not used for the deployments of the subscribed Operator.
https://docs.openshift.com/container-platform/4.6/operators/admin/olm-configuring-proxy-support.html
*/
func ProxyEnvVarsAreSet() bool {
	if os.Getenv("HTTP_PROXY") != "" || os.Getenv("HTTPS_PROXY") != "" || os.Getenv("NO_PROXY") != "" {
		return true
	}
	return false
}

/*
OperatorNamespace returns the namespace in which the MultiClusterHub operator is registered or deployed. It retrieves
the namespace from the `POD_NAMESPACE` environment variable and returns it as a string. If the environment variable is
not set, the function returns an error.
*/
func OperatorNamespace() (string, error) {
	ns, found := os.LookupEnv(podNamespaceEnvVar)
	if !found {
		return "", fmt.Errorf("%s envvar is not set", podNamespaceEnvVar)
	}
	return ns, nil
}

/*
GetDeployments returns a list of Kubernetes Deployments relevant to the MultiClusterHub. The list includes the names and
namespaces of Deployments associated with enabled components based on the MultiClusterHub's configuration.
*/
func GetDeployments(m *operatorsv1.MultiClusterHub) []types.NamespacedName {
	nn := []types.NamespacedName{}
	if m.Enabled(operatorsv1.Volsync) {
		nn = append(nn, types.NamespacedName{Name: "volsync-addon-controller", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.Insights) {
		nn = append(nn, types.NamespacedName{Name: "insights-client", Namespace: m.Namespace})
		nn = append(nn, types.NamespacedName{Name: "insights-metrics", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.ClusterBackup) {
		nn = append(nn, types.NamespacedName{Name: "cluster-backup-chart-clusterbackup",
			Namespace: ClusterSubscriptionNamespace})

		nn = append(nn, types.NamespacedName{Name: "openshift-adp-controller-manager",
			Namespace: ClusterSubscriptionNamespace})
	}
	return nn
}

/*
GetCustomResources returns a list of custom resources relevant to the MultiClusterHub. The list includes the names and
namespaces of custom resources that are used in the MultiClusterHub's setup and operation.
*/
func GetCustomResources(m *operatorsv1.MultiClusterHub) []types.NamespacedName {
	return []types.NamespacedName{
		{Name: "multicluster-engine-sub", Namespace: MCESubscriptionNamespace},
		{Name: "multicluster-engine-csv", Namespace: MCESubscriptionNamespace},
		{Name: "multicluster-engine"},
	}
}

/*
GetDeploymentsForStatus returns a list of Kubernetes Deployments that are relevant for status updates in the
MultiClusterHub. The function takes into account the MultiClusterHub's configuration and an additional flag
(`ocpConsole`) to include Deployments specific to the OpenShift Console.
*/
func GetDeploymentsForStatus(m *operatorsv1.MultiClusterHub, ocpConsole bool) []types.NamespacedName {
	nn := []types.NamespacedName{}
	if m.Enabled(operatorsv1.Insights) {
		nn = append(nn, types.NamespacedName{Name: "insights-client", Namespace: m.Namespace})
		nn = append(nn, types.NamespacedName{Name: "insights-metrics", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.Search) {
		nn = append(nn, types.NamespacedName{Name: "search-v2-operator-controller-manager", Namespace: m.Namespace})
		nn = append(nn, types.NamespacedName{Name: "search-api", Namespace: m.Namespace})
		nn = append(nn, types.NamespacedName{Name: "search-collector", Namespace: m.Namespace})
		nn = append(nn, types.NamespacedName{Name: "search-indexer", Namespace: m.Namespace})
		nn = append(nn, types.NamespacedName{Name: "search-postgres", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.Appsub) {
		nn = append(nn, types.NamespacedName{Name: "multicluster-operators-application", Namespace: m.Namespace})
		nn = append(nn, types.NamespacedName{Name: "multicluster-operators-channel", Namespace: m.Namespace})
		nn = append(nn, types.NamespacedName{Name: "multicluster-operators-hub-subscription", Namespace: m.Namespace})
		nn = append(nn, types.NamespacedName{Name: "multicluster-operators-standalone-subscription",
			Namespace: m.Namespace})
		nn = append(nn, types.NamespacedName{Name: "multicluster-operators-subscription-report", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.ClusterLifecycle) {
		nn = append(nn, types.NamespacedName{Name: "klusterlet-addon-controller-v2", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.ClusterBackup) {
		nn = append(nn, types.NamespacedName{Name: "cluster-backup-chart-clusterbackup",
			Namespace: ClusterSubscriptionNamespace})
		nn = append(nn, types.NamespacedName{Name: "openshift-adp-controller-manager",
			Namespace: ClusterSubscriptionNamespace})
	}
	if m.Enabled(operatorsv1.GRC) {
		nn = append(nn, types.NamespacedName{Name: "grc-policy-addon-controller", Namespace: m.Namespace})
		nn = append(nn, types.NamespacedName{Name: "grc-policy-propagator", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.Console) && ocpConsole {
		nn = append(nn, types.NamespacedName{Name: "console-chart-console-v2", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.Volsync) {
		nn = append(nn, types.NamespacedName{Name: "volsync-addon-controller", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.MultiClusterObservability) {
		nn = append(nn, types.NamespacedName{Name: "multicluster-observability-operator", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.ClusterPermission) {
		nn = append(nn, types.NamespacedName{Name: "cluster-permission", Namespace: m.Namespace})
	}
	return nn
}

/*
GetCustomResourcesForStatus returns a list of custom resources relevant for status updates in the MultiClusterHub.
The list includes the names and namespaces of custom resources used in the MultiClusterHub's status tracking.
*/
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

/*
GetTolerations retrieves the tolerations to be applied to Pods created by the MultiClusterHub. Tolerations allow Pods
to be scheduled on nodes with specific taints. The function returns a list of `corev1.Toleration` structures
representing toleration rules.
*/
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

/*
RemoveString removes a specified string (`r`) from a slice of strings (`s`). It iterates through the slice and
returns a new slice with the specified string removed, if found.
*/
func RemoveString(s []string, r string) []string {
	for i, v := range s {
		if v == r {
			return append(s[:i], s[i+1:]...)
		}
	}
	return s
}

/*
Contains checks if a slice of strings (`s`) contains a specific string (`e`). It iterates through the slice and returns
`true` if the string is found, indicating its presence in the slice.
*/
func Contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

/*
AppendProxyVariables appends environment variables (`added`) to an existing list of environment variables (`existing`).
It checks if an environment variable with the same name already exists in the list and updates its value if found.
This function is used to manage proxy-related environment variables.
*/
func AppendProxyVariables(existing []corev1.EnvVar, added []corev1.EnvVar) []corev1.EnvVar {

	for i := 0; i < len(added); i++ {
		existing = appendIfMissing(existing, added[i])
	}
	return existing
}

/*
appendIfMissing helper function used by `AppendProxyVariables`. It appends a new environment variable (`s`) to an
existing slice of environment variables (`slice`) if an environment variable with the same name is not already present
in the slice. If a matching environment variable is found, its value is updated.
*/
func appendIfMissing(slice []corev1.EnvVar, s corev1.EnvVar) []corev1.EnvVar {
	for i := 0; i < len(slice); i++ {
		if slice[i].Name == s.Name {
			slice[i].Value = s.Value
			return slice
		}
	}
	return append(slice, s)
}

/*
SetDefaultComponents sets the default enabled and disabled components in the MultiClusterHub. It reads the default
component configuration, and if any changes are made to the MultiClusterHub's component configuration, it returns `true`
to indicate that updates were performed. It can also return an error if there's an issue with reading
the default configurations.
*/
func SetDefaultComponents(m *operatorsv1.MultiClusterHub) (bool, error) {
	updated := false
	defaultEnabledComponents, err := operatorsv1.GetDefaultEnabledComponents()
	if err != nil {
		return updated, err
	}
	defaultDisabledComponents, err := operatorsv1.GetDefaultDisabledComponents()
	if err != nil {
		return true, err
	}
	for _, c := range defaultEnabledComponents {
		if !m.ComponentPresent(c) {
			m.Enable(c)
			updated = true
		}
	}
	for _, c := range defaultDisabledComponents {
		if !m.ComponentPresent(c) {
			m.Disable(c)
			updated = true
		}
	}
	return updated, nil
}

/*
SetHostedDefaultComponents sets the default components specific to hosted environments in the MultiClusterHub's
configuration. It checks if any changes are made and returns `true` if updates are performed.
*/
func SetHostedDefaultComponents(m *operatorsv1.MultiClusterHub) bool {
	updated := false
	components := operatorsv1.GetDefaultHostedComponents()
	for _, c := range components {
		if !m.ComponentPresent(c) {
			m.Enable(c)
			updated = true
		}
	}
	return updated
}

/*
DeduplicateComponents removes duplicate component configurations by name in the MultiClusterHub's configuration.
If any duplicates are found and removed, the function returns `true` to indicate that changes were made.
*/
func DeduplicateComponents(m *operatorsv1.MultiClusterHub) bool {
	config := m.Spec.Overrides.Components
	newConfig := deduplicate(m.Spec.Overrides.Components)
	if len(newConfig) != len(config) {
		m.Spec.Overrides.Components = newConfig
		return true
	}
	return false
}

/*
deduplicate a helper function used by `DeduplicateComponents`. It removes duplicate component configurations by name
from the input slice of `operatorsv1.ComponentConfig` and returns a new slice with duplicates removed.
*/
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

/*
getMCEComponents returns MultiClusterEngine (MCE) components present in the MultiClusterHub's configuration.
It constructs a list of MCE component configurations based on the MultiClusterHub's component settings and returns it.
*/
func GetMCEComponents(mch *operatorsv1.MultiClusterHub) []mcev1.ComponentConfig {
	config := []mcev1.ComponentConfig{}
	for _, n := range operatorsv1.MCEComponents {
		if mch.ComponentPresent(n) {
			config = append(config, mcev1.ComponentConfig{Name: n, Enabled: mch.Enabled(n)})
		}
	}
	if mch.Spec.DisableHubSelfManagement {
		config = append(config, mcev1.ComponentConfig{Name: operatorsv1.MCELocalCluster, Enabled: false})
	} else {
		config = append(config, mcev1.ComponentConfig{Name: operatorsv1.MCELocalCluster, Enabled: true})
	}
	return config
}

/*
UpdateMCEOverrides updates the MCE component configurations in the MultiClusterEngine (MCE) based on the
MultiClusterHub's configuration. It ensures that MCE components are enabled or disabled according to the
MultiClusterHub's settings.
*/
func UpdateMCEOverrides(mce *mcev1.MultiClusterEngine, mch *operatorsv1.MultiClusterHub) {
	mceComponents := GetMCEComponents(mch)
	for _, c := range mceComponents {
		if c.Enabled {
			mce.Enable(c.Name)
		} else {
			mce.Disable(c.Name)
		}
	}
	if mch.Spec.DisableHubSelfManagement {
		mce.Disable(operatorsv1.MCELocalCluster)
	} else {
		mce.Enable(operatorsv1.MCELocalCluster)
	}
}

/*
IsCommunityMode checks whether the operator is running in community mode or advanced mode. It reads the value of an
environment variable and returns `true` if the variable indicates that the operator is in community mode.
*/
func IsCommunityMode() bool {
	packageName := os.Getenv("OPERATOR_PACKAGE")
	if packageName == "advanced-cluster-management" {
		return false
	} else {
		// other option is "stolostron"
		return true
	}
}
