package v1

import (
	"errors"
	"os"
)

const (
	// MCH
	Appsub                    string = "app-lifecycle"
	ClusterBackup             string = "cluster-backup"
	ClusterLifecycle          string = "cluster-lifecycle"
	ClusterPermission         string = "cluster-permission"
	Console                   string = "console"
	Insights                  string = "insights"
	GRC                       string = "grc"
	ManagementIngress         string = "management-ingress"
	MultiClusterEngine        string = "multicluster-engine"
	MultiClusterObservability string = "multicluster-observability"
	Repo                      string = "multiclusterhub-repo"
	Search                    string = "search"
	SubmarinerAddon           string = "submariner-addon"
	Volsync                   string = "volsync"

	// MCE
	MCEAssistedService        string = "assisted-service"
	MCEClusterLifecycle       string = "cluster-lifecycle-mce"
	MCEClusterManager         string = "cluster-manager"
	MCEClusterProxyAddon      string = "cluster-proxy-addon"
	MCEConsole                string = "console-mce"
	MCEDiscovery              string = "discovery"
	MCEHive                   string = "hive"
	MCEHypershift             string = "hypershift"
	MCEHypershiftLocalHosting string = "hypershift-local-hosting"
	MCEHypershiftPreview      string = "hypershift-preview"
	MCELocalCluster           string = "local-cluster"
	MCEManagedServiceAccount  string = "managedserviceaccount-preview"
	MCEServerFoundation       string = "server-foundation"
)

var allComponents = []string{
	// MCH
	Appsub,
	ClusterBackup,
	ClusterLifecycle,
	ClusterPermission,
	Console,
	GRC,
	Insights,
	ManagementIngress,
	MultiClusterEngine,
	MultiClusterObservability,
	Repo,
	Search,
	SubmarinerAddon,
	Volsync,

	// MCE
	MCEAssistedService,
	MCEClusterLifecycle,
	MCEClusterManager,
	MCEClusterProxyAddon,
	MCEConsole,
	MCEDiscovery,
	MCEHive,
	MCEHypershift,
	MCEHypershiftLocalHosting,
	MCEHypershiftPreview,
	MCEManagedServiceAccount,
	MCEServerFoundation,
}

var MCHComponents = []string{
	Appsub,
	ClusterBackup,
	ClusterLifecycle,
	ClusterPermission,
	Console,
	GRC,
	Insights,
	MultiClusterObservability,
	Search,
	SubmarinerAddon,
	Volsync,
}

var MCEComponents = []string{
	MCEAssistedService,
	MCEClusterLifecycle,
	MCEClusterManager,
	MCEConsole,
	MCEDiscovery,
	MCEHive,
	MCEHypershift,
	MCEHypershiftLocalHosting,
	MCEHypershiftPreview,
	MCEManagedServiceAccount,
	MCEServerFoundation,
}

func GetDefaultEnabledComponents() ([]string, error) {
	var defaultEnabledComponents = []string{
		Appsub,
		ClusterLifecycle,
		ClusterPermission,
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

func GetDefaultDisabledComponents() ([]string, error) {
	var defaultDisabledComponents = []string{
		ClusterBackup,
	}
	return defaultDisabledComponents, nil
}

func GetDefaultHostedComponents() []string {
	var defaultHostedComponents = []string{
		MultiClusterEngine,
		//Add other components here when added to hostedmode
	}

	return defaultHostedComponents
}

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
		Name:    s,
		Enabled: true,
	})
}

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
		Name:    s,
		Enabled: false,
	})
}

// Prune removes the component from the component list. Returns true if changes are made
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

// a component is valid if its name matches a known component
func ValidComponent(c ComponentConfig) bool {
	for _, name := range allComponents {
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
		err := errors.New("There is an illegal value set for OPERATOR_PACKAGE")
		return true, err
	}
}

// IsInHostedMode returns true if mch is configured for hosted mode
func (mch *MultiClusterHub) IsInHostedMode() bool {
	a := mch.GetAnnotations()
	if a == nil {
		return false
	}
	if a["deploymentmode"] == string(ModeHosted) {
		return true
	}
	return false
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
