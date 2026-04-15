// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package utils

import (
	"context"
	"crypto/tls"
	"fmt"
	"os"

	configv1 "github.com/openshift/api/config/v1"
	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

	// MTVIntegrationsChartLocation is the location of the MTV Integrations charts.
	MTVIntegrationsChartLocation = "/charts/toggle/mtv-integrations"

	// SiteConfigChartLocation is the location of the SiteConfig Operator chart.
	SiteConfigChartLocation = "/charts/toggle/siteconfig-operator"

	// SubmarinerAddonChartLocation is the location of the Submariner Addon chart.
	SubmarinerAddonChartLocation = "/charts/toggle/submariner-addon"

	// VolsyncChartLocation is the location of the Volsync Controller chart.
	VolsyncChartLocation = "/charts/toggle/volsync-controller"

	// FineGrainedRbacChartLocation is the location of the Fine Grained RBAC chart.
	FineGrainedRbacChartLocation = "/charts/toggle/fine-grained-rbac"
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

// MchIsValid Checks if the optional default parameters need to be set
func MchIsValid(m *operatorsv1.MultiClusterHub) bool {
	return operatorsv1.AvailabilityConfigIsValid(m.Spec.AvailabilityConfig)
}

// DefaultReplicaCount returns an integer corresponding to the default number of replicas
// for HA or non-HA modes
func DefaultReplicaCount(mch *operatorsv1.MultiClusterHub) int {
	if mch.Spec.AvailabilityConfig == operatorsv1.HABasic {
		return 1
	}
	return 2
}

// GetImagePullPolicy returns either pull policy from CR overrides or default of Always
func GetImagePullPolicy(m *operatorsv1.MultiClusterHub) corev1.PullPolicy {
	if m.Spec.Overrides == nil || m.Spec.Overrides.ImagePullPolicy == "" {
		return corev1.PullIfNotPresent
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

func GetTestImages() []string {
	return []string{
		"LIFECYCLE_BACKEND_E2E", "BAILER", "CERT_POLICY_CONTROLLER", "CLUSTER_BACKUP_CONTROLLER",
		"CLUSTER_LIFECYCLE_E2E", "CLUSTER_PROXY", "CLUSTER_PROXY_ADDON", "CONSOLE", "ENDPOINT_MONITORING_OPERATOR",
		"GRAFANA", "GRAFANA_DASHBOARD_LOADER", "GRC_POLICY_FRAMEWORK_TESTS", "HELLOPROW_GO", "HELLOWORLD",
		"HYPERSHIFT_DEPLOYMENT_CONTROLLER", "INSIGHTS_CLIENT", "INSIGHTS_METRICS",
		"KLUSTERLET_ADDON_CONTROLLER", "KLUSTERLET_ADDON_OPERATOR", "KUBE_RBAC_PROXY", "KUBE_STATE_METRICS",
		"METRICS_COLLECTOR", "MULTICLOUD_INTEGRATIONS", "MULTICLUSTER_OBSERVABILITY_OPERATOR",
		"MULTICLUSTER_OPERATORS_APPLICATION", "MULTICLUSTER_OPERATORS_CHANNEL", "MULTICLUSTER_OPERATORS_SUBSCRIPTION",
		"MULTICLUSTERHUB_OPERATOR", "MULTICLUSTERHUB_OPERATOR_TESTS", "MULTICLUSTERHUB_REPO", "MUST_GATHER",
		"NODE_EXPORTER", "OBSERVABILITY_E2E_TEST", "OBSERVATORIUM", "OBSERVATORIUM_OPERATOR",
		"POSTGRESQL_12", "POSTGRESQL_13", "PROMETHEUS", "PROMETHEUS_ALERTMANAGER",
		"PROMETHEUS_CONFIG_RELOADER", "PROMETHEUS_OPERATOR", "RBAC_QUERY_PROXY", "REDISGRAPH_TLS",
		"SEARCH_AGGREGATOR", "SEARCH_API", "SEARCH_COLLECTOR", "SEARCH_E2E", "SEARCH_INDEXER", "SEARCH_OPERATOR",
		"SEARCH_V2_API", "SITECONFIG_OPERATOR", "SUBMARINER_ADDON", "THANOS", "VOLSYNC", "VOLSYNC_ADDON_CONTROLLER", "VOLSYNC_MOVER_RCLONE",
		"VOLSYNC_MOVER_RESTIC", "VOLSYNC_MOVER_RSYNC", "CLUSTER_PERMISSION", "kube_rbac_proxy", "insights_metrics",
		"insights_client", "search_collector", "search_indexer", "search_v2_api", "postgresql_13", "search_v2_operator",
		"klusterlet_addon_controller", "governance_policy_propagator", "governance_policy_addon_controller",
		"cert_policy_controller", "config_policy_controller", "governance_policy_framework_addon",
		"cluster_backup_controller", "console", "volsync_addon_controller", "multicluster_operators_application",
		"multicloud_integrations", "mtv_integrations", "multicluster_operators_channel", "multicluster_operators_subscription",
		"multicluster_observability_operator", "cluster_permission", "siteconfig_operator", "submariner_addon", "acm_cli",
		"multicluster_role_assignment", "postgresql_16",
	}
}

// TrackedNamespaces returns the list of namespaces we deploy components to and should track
func TrackedNamespaces(m *operatorsv1.MultiClusterHub) []string {
	trackedNamespaces := []string{m.Namespace}
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

// OperatorNamespace returns the namespace where the MultiClusterHub operator is registered or deployed.
func OperatorNamespace() (string, error) {
	ns, found := os.LookupEnv(podNamespaceEnvVar)
	if !found {
		return "", fmt.Errorf("%s envvar is not set", podNamespaceEnvVar)
	}
	return ns, nil
}

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
		nn = append(nn, types.NamespacedName{Name: "cluster-backup-chart-clusterbackup", Namespace: ClusterSubscriptionNamespace})
		nn = append(nn, types.NamespacedName{Name: "openshift-adp-controller-manager", Namespace: ClusterSubscriptionNamespace})
	}
	return nn
}

func GetDeploymentsForStatus(m *operatorsv1.MultiClusterHub, ocpConsole, isSTSEnabled bool) []types.NamespacedName {
	nn := []types.NamespacedName{}
	if m.Enabled(operatorsv1.Insights) {
		nn = append(nn, types.NamespacedName{Name: "insights-client", Namespace: m.Namespace})
		nn = append(nn, types.NamespacedName{Name: "insights-metrics", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.SiteConfig) {
		nn = append(nn, types.NamespacedName{Name: "siteconfig-controller-manager", Namespace: m.Namespace})
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
		nn = append(nn, types.NamespacedName{Name: "multicluster-operators-standalone-subscription", Namespace: m.Namespace})
		nn = append(nn, types.NamespacedName{Name: "multicluster-operators-subscription-report", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.ClusterLifecycle) {
		nn = append(nn, types.NamespacedName{Name: "klusterlet-addon-controller-v2", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.ClusterBackup) {
		nn = append(nn, types.NamespacedName{Name: "cluster-backup-chart-clusterbackup", Namespace: ClusterSubscriptionNamespace})

		if !isSTSEnabled {
			nn = append(nn, types.NamespacedName{Name: "openshift-adp-controller-manager", Namespace: ClusterSubscriptionNamespace})
		}
	}
	if m.Enabled(operatorsv1.GRC) {
		nn = append(nn, types.NamespacedName{Name: "grc-policy-addon-controller", Namespace: m.Namespace})
		nn = append(nn, types.NamespacedName{Name: "grc-policy-propagator", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.Console) && ocpConsole {
		nn = append(nn, types.NamespacedName{Name: "console-chart-console-v2", Namespace: m.Namespace})
		nn = append(nn, types.NamespacedName{Name: "acm-cli-downloads", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.Volsync) {
		nn = append(nn, types.NamespacedName{Name: "volsync-addon-controller", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.MultiClusterObservability) {
		nn = append(nn, types.NamespacedName{Name: "multicluster-observability-operator", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.FineGrainedRbac) {
		nn = append(nn, types.NamespacedName{Name: "multicluster-role-assignment-controller", Namespace: m.Namespace})
	}
	if m.Enabled(operatorsv1.MTVIntegrations) {
		nn = append(nn, types.NamespacedName{Name: "mtv-integrations-controller", Namespace: m.Namespace})
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

// SetDefaultComponents returns true if changes are made
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

// GetMCEComponents returns mce components that are present in mch
func GetMCEComponents(mch *operatorsv1.MultiClusterHub) []mcev1.ComponentConfig {
	config := []mcev1.ComponentConfig{}

	for _, n := range operatorsv1.MCEComponents {
		/*
			In MCE, some components have migrated from ACM to MCE for example, the Cluster Permission component
			in ACM 2.17. If such a component is present in MCH, it should be added to the MCE config with the same
			enabled status in order to ensure a smooth transition for users who have been using these components in ACM
			and are now moving to MCE. By checking if the component is present in MCH and adding it to the MCE config,
			we can ensure that users can continue to use these components without any disruption, even as they
			transition to MCE.
		*/
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

// IsCommunityMode returns true if operator is running in community mode
func IsCommunityMode() bool {
	packageName := os.Getenv("OPERATOR_PACKAGE")
	if packageName == "advanced-cluster-management" {
		return false
	} else {
		// other option is "stolostron"
		return true
	}
}

// GetAPIServerTLSProfile retrieves the TLS security profile from the OpenShift APIServer resource.
// Returns the TLSProfileSpec containing minTLSVersion and ciphers.
// If no profile is set, returns the default Intermediate profile.
func GetAPIServerTLSProfile(ctx context.Context, cl client.Client) (*configv1.TLSProfileSpec, error) {
	// If running in unit test mode, return default Intermediate profile
	if val, ok := os.LookupEnv(UnitTestEnvVar); ok && val == "true" {
		return configv1.TLSProfiles[configv1.TLSProfileIntermediateType], nil
	}

	apiServer := &configv1.APIServer{}
	err := cl.Get(ctx, types.NamespacedName{Name: "cluster"}, apiServer)
	if err != nil {
		return nil, fmt.Errorf("failed to get APIServer resource: %w", err)
	}

	// If no TLS profile is set, use the default (Intermediate)
	if apiServer.Spec.TLSSecurityProfile == nil {
		return configv1.TLSProfiles[configv1.TLSProfileIntermediateType], nil
	}

	profile := apiServer.Spec.TLSSecurityProfile

	// For predefined profiles (Old, Intermediate, Modern), use the map
	if profileSpec, ok := configv1.TLSProfiles[profile.Type]; ok {
		return profileSpec, nil
	}

	// For custom profile, return the inline spec
	if profile.Type == configv1.TLSProfileCustomType && profile.Custom != nil {
		return &profile.Custom.TLSProfileSpec, nil
	}

	// Fallback to Intermediate if something unexpected
	return configv1.TLSProfiles[configv1.TLSProfileIntermediateType], nil
}

// ConvertTLSVersion converts OpenShift TLSProtocolVersion string to crypto/tls uint16 constant.
// Returns tls.VersionTLS12 as default if the version string is not recognized.
func ConvertTLSVersion(version configv1.TLSProtocolVersion) uint16 {
	switch version {
	case configv1.VersionTLS10:
		return tls.VersionTLS10
	case configv1.VersionTLS11:
		return tls.VersionTLS11
	case configv1.VersionTLS12:
		return tls.VersionTLS12
	case configv1.VersionTLS13:
		return tls.VersionTLS13
	default:
		// Default to TLS 1.2 for safety
		return tls.VersionTLS12
	}
}

// ConvertCipherSuites converts OpenShift cipher suite names (OpenSSL format) to crypto/tls uint16 constants.
// TLS 1.3 cipher suites are managed automatically by Go and cannot be configured, so they are filtered out.
// Only returns cipher suites applicable to TLS ≤ 1.2.
func ConvertCipherSuites(cipherNames []string) []uint16 {
	// Mapping from OpenSSL cipher names to crypto/tls constants
	// Only includes cipher suites that exist in Go's crypto/tls package
	cipherMap := map[string]uint16{
		// TLS 1.2 ECDHE ciphers (GCM and ChaCha20)
		"ECDHE-RSA-AES128-GCM-SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		"ECDHE-RSA-AES256-GCM-SHA384":   tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
		"ECDHE-ECDSA-AES128-GCM-SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
		"ECDHE-ECDSA-AES256-GCM-SHA384": tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
		"ECDHE-RSA-CHACHA20-POLY1305":   tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
		"ECDHE-ECDSA-CHACHA20-POLY1305": tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,

		// TLS 1.2 ECDHE ciphers (CBC)
		"ECDHE-RSA-AES128-SHA256":   tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA256,
		"ECDHE-RSA-AES128-SHA":      tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
		"ECDHE-ECDSA-AES128-SHA256": tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA256,
		"ECDHE-ECDSA-AES128-SHA":    tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
		"ECDHE-RSA-AES256-SHA":      tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		"ECDHE-ECDSA-AES256-SHA":    tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,

		// RSA ciphers
		"AES128-GCM-SHA256": tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
		"AES256-GCM-SHA384": tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
		"AES128-SHA256":     tls.TLS_RSA_WITH_AES_128_CBC_SHA256,
		"AES128-SHA":        tls.TLS_RSA_WITH_AES_128_CBC_SHA,
		"AES256-SHA":        tls.TLS_RSA_WITH_AES_256_CBC_SHA,
		"DES-CBC3-SHA":      tls.TLS_RSA_WITH_3DES_EDE_CBC_SHA,
	}

	var result []uint16
	for _, name := range cipherNames {
		// Skip TLS 1.3 cipher suites (they start with TLS_ prefix and are auto-managed)
		if len(name) > 4 && name[:4] == "TLS_" {
			continue
		}

		if cipher, ok := cipherMap[name]; ok {
			result = append(result, cipher)
		}
		// Silently skip unsupported cipher suites - Go may not support all OpenSSL ciphers
	}

	return result
}
