package v1

import (
	"errors"
	"fmt"
	"os"
)

type ResourceGVK struct {
	Group   string `json:"group"`
	Kind    string `json:"kind"`
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Name of the MultiClusterHub (MCH) operator.
const MCH = "multiclusterhub-operator"

// Component related to MultiClusterHub (MCH)
const (
	Appsub                    string = "app-lifecycle"
	ClusterBackup             string = "cluster-backup"
	ClusterLifecycle          string = "cluster-lifecycle"
	ClusterPermission         string = "cluster-permission" // Deprecated in ACM 2.17, moved to MCE 2.17.
	Console                   string = "console"
	MTVIntegrationsPreview    string = "cnv-mtv-integrations-preview"
	MTVIntegrations           string = "cnv-mtv-integrations"
	FineGrainedRbac           string = "fine-grained-rbac"
	FineGrainedRbacPreview    string = "fine-grained-rbac-preview"
	GRC                       string = "grc"
	Insights                  string = "insights"
	ManagementIngress         string = "management-ingress"
	MultiClusterEngine        string = "multicluster-engine"
	MultiClusterObservability string = "multicluster-observability"
	Repo                      string = "multiclusterhub-repo"
	Search                    string = "search"
	SiteConfig                string = "siteconfig"
	SubmarinerAddon           string = "submariner-addon"
	Volsync                   string = "volsync"
)

// Component related to MultiCluster Engine (MCE)
const (
	MCEAssistedService                  string = "assisted-service"
	MCEClusterAPI                       string = "cluster-api"
	MCEClusterAPIPreview                string = "cluster-api-preview"
	MCEClusterAPIProviderAWS            string = "cluster-api-provider-aws"
	MCEClusterAPIProviderAWSPreview     string = "cluster-api-provider-aws-preview"
	MCEClusterLifecycle                 string = "cluster-lifecycle-mce"
	MCEClusterManager                   string = "cluster-manager"
	MCEClusterPermission                string = "cluster-permission"
	MCEClusterProxyAddon                string = "cluster-proxy-addon"
	MCEConsole                          string = "console-mce"
	MCEDiscovery                        string = "discovery"
	MCEHive                             string = "hive"
	MCEHypershiftLocalHosting           string = "hypershift-local-hosting"
	MCEHypershiftPreview                string = "hypershift-preview"
	MCEHypershift                       string = "hypershift"
	MCEImageBasedInstallOperator        string = "image-based-install-operator"
	MCEImageBasedInstallOperatorPreview string = "image-based-install-operator-preview"
	MCELocalCluster                     string = "local-cluster"
	MCEManagedServiceAccount            string = "managedserviceaccount"
	MCEManagedServiceAccountPreview     string = "managedserviceaccount-preview"
	MCEServerFoundation                 string = "server-foundation"
	IamPolicyController                 string = "iam-policy-controller"
)

// MCHComponents is a slice containing component names specific to the "MCH" category.
var MCHComponents = []string{
	Appsub,
	ClusterBackup,
	ClusterLifecycle,
	ClusterPermission, // Migrated to MCE in 2.17, but must remain here for webhook validation (ValidateCreate/ValidateUpdate).
	Console,
	FineGrainedRbac,
	MTVIntegrations,
	GRC,
	Insights,
	MultiClusterEngine, // Adding MCE component to ensure that the component is validated by the webhook.
	MCH,                // Adding MCH component to ensure legacy resources are cleaned up properly.
	MultiClusterObservability,
	Search,
	SiteConfig,
	SubmarinerAddon,
	Volsync,
}

// MCEComponents is a slice containing component names specific to the "MCE" category.
var MCEComponents = []string{
	MCEAssistedService,
	MCEClusterAPI,
	MCEClusterAPIPreview,
	MCEClusterAPIProviderAWS,
	MCEClusterAPIProviderAWSPreview,
	MCEClusterLifecycle,
	MCEClusterManager,
	MCEClusterPermission,
	MCEClusterProxyAddon,
	MCEConsole,
	MCEDiscovery,
	MCEHive,
	MCEHypershift,
	MCEHypershiftLocalHosting,
	MCEHypershiftPreview,
	MCEImageBasedInstallOperator,
	MCEImageBasedInstallOperatorPreview,
	MCEManagedServiceAccount,
	MCEManagedServiceAccountPreview,
	MCEServerFoundation,
}

var MCECRDs = []ResourceGVK{
	{
		Group:   "addon.open-cluster-management.io",
		Version: "v1alpha1",
		Kind:    "ClusterManagementAddOn",
		Name:    "clustermanagementaddons.addon.open-cluster-management.io",
	},
	{
		Group:   "addon.open-cluster-management.io",
		Version: "v1alpha1",
		Kind:    "AddOnTemplate",
		Name:    "addontemplates.addon.open-cluster-management.io",
	},
}

// resources to check for sts enabled or not
var RequiredSTSCRDs = []ResourceGVK{
	{
		Group:   "config.openshift.io",
		Version: "v1",
		Kind:    "Infrastructure",
		Name:    "infrastructures.config.openshift.io",
	},
	{
		Group:   "config.openshift.io",
		Version: "v1",
		Kind:    "Authentication",
		Name:    "authentications.config.openshift.io",
	},
	{
		Group:   "operator.openshift.io",
		Version: "v1",
		Kind:    "CloudCredential",
		Name:    "cloudcredentials.operator.openshift.io",
	},
}

// ClusterManagementAddOns is a map that associates certain component names with their corresponding add-ons.
var ClusterManagementAddOns = map[string]string{
	IamPolicyController: "iam-policy-controller",
	SubmarinerAddon:     "submariner",
	// Add other components here when ClusterManagementAddOns is required.
}

/*
GetDefaultEnabledComponents returns a slice of default enabled component names.
It is expected to be used to get a list of components that are enabled by default.

Removed components:
- Repo: Removed in ACM 2.7.0
- ClusterPermission: Removed in ACM 2.17 (migrated to MCE)
*/
func GetDefaultEnabledComponents() ([]string, error) {
	defaultEnabledComponents := []string{
		Appsub,
		ClusterLifecycle,
		Console,
		GRC,
		Insights,
		MultiClusterEngine,
		MultiClusterObservability,
		Search,
		SubmarinerAddon,
		Volsync,
	}

	return defaultEnabledComponents, nil
}

/*
GetDefaultDisabledComponents returns a slice of default disabled component names.
It is expected to be used to get a list of components that are disabled by default.
*/
func GetDefaultDisabledComponents() ([]string, error) {
	defaultDisabledComponents := []string{
		ClusterBackup,
		FineGrainedRbac,
		SiteConfig,
		MTVIntegrations,
	}
	return defaultDisabledComponents, nil
}

// GetClusterManagementAddonName returns the name of the ClusterManagementAddOn based on the provided component name.
func GetClusterManagementAddonName(component string) (string, error) {
	if val, ok := ClusterManagementAddOns[component]; !ok {
		return val, fmt.Errorf("failed to find ClusterManagementAddOn name for: %s component", component)
	} else {
		return val, nil
	}
}

/*
ComponentPresent checks if a specific component is present based on the provided component name in the
MultiClusterHub struct.
*/
func (mch *MultiClusterHub) ComponentPresent(s string) bool {
	if mch.Spec.Overrides == nil {
		return false
	}
	for _, c := range mch.Spec.Overrides.Components {
		if c.Name == s {
			return true
		}
	}
	return false
}

// Enabled checks if a specific component is enabled based on the provided component name in the MultiClusterHub struct.
func (mch *MultiClusterHub) Enabled(s string) bool {
	if mch.Spec.Overrides == nil {
		return false
	}
	for _, c := range mch.Spec.Overrides.Components {
		if c.Name == s {
			return c.Enabled
		}
	}

	return false
}

// Enable enables a specific component based on the provided component name in the MultiClusterHub struct.
func (mch *MultiClusterHub) Enable(s string) {
	if mch.Spec.Overrides == nil {
		mch.Spec.Overrides = &Overrides{}
	}
	for i, c := range mch.Spec.Overrides.Components {
		if c.Name == s {
			mch.Spec.Overrides.Components[i].Enabled = true
			return
		}
	}

	mch.Spec.Overrides.Components = append(mch.Spec.Overrides.Components, ComponentConfig{
		Enabled:         true,
		Name:            s,
		ConfigOverrides: ConfigOverride{},
	})
}

// Disable disables a specific component based on the provided component name in the MultiClusterHub struct.
func (mch *MultiClusterHub) Disable(s string) {
	if mch.Spec.Overrides == nil {
		mch.Spec.Overrides = &Overrides{}
	}

	for i, c := range mch.Spec.Overrides.Components {
		if c.Name == s {
			mch.Spec.Overrides.Components[i].Enabled = false
			return
		}
	}

	mch.Spec.Overrides.Components = append(mch.Spec.Overrides.Components, ComponentConfig{
		Enabled:         false,
		Name:            s,
		ConfigOverrides: ConfigOverride{},
	})
}

/*
Prune removes a specific component from the component list in the MultiClusterHub struct.
It returns true if changes were made.
*/
func (mch *MultiClusterHub) Prune(s string) bool {
	if mch.Spec.Overrides == nil {
		return false
	}
	pruned := false
	prunedList := []ComponentConfig{}
	for _, c := range mch.Spec.Overrides.Components {
		if c.Name == s {
			pruned = true
		} else {
			prunedList = append(prunedList, c)
		}
	}

	if pruned {
		mch.Spec.Overrides.Components = prunedList
		return true
	}
	return false
}

// ValidComponent checks if a given component configuration is valid by comparing its name to the known component names.
func ValidComponent(c ComponentConfig, validComponents []string) bool {
	for _, name := range validComponents {
		if c.Name == name {
			return true
		}
	}
	return false
}

// IsCommunity checks to see if the operator is running in community mode
func IsCommunity() (bool, error) {
	packageName := os.Getenv("OPERATOR_PACKAGE")
	if packageName == "advanced-cluster-management" {
		return false, nil
	} else if (packageName == "stolostron") || (packageName == "") {
		return true, nil
	} else {
		err := errors.New("there is an illegal value set for OPERATOR_PACKAGE")
		return true, err
	}
}

// AvailabilityConfigIsValid returns true is the availability type is a recognized value
func AvailabilityConfigIsValid(config AvailabilityType) bool {
	switch config {
	case HAHigh, HABasic:
		return true
	default:
		return false
	}
}
