package v1

import (
	"errors"
	"os"
)

const (
	Appsub             string = "app-lifecycle"
	Search             string = "search"
	ManagementIngress  string = "management-ingress"
	Console            string = "console"
	Insights           string = "insights"
	GRC                string = "grc"
	ClusterLifecycle   string = "cluster-lifecycle"
	ClusterBackup      string = "cluster-backup"
	Repo               string = "multiclusterhub-repo"
	MultiClusterEngine string = "multicluster-engine"
	Volsync            string = "volsync"

	// MCE
	MCEManagedServiceAccount  string = "managedserviceaccount-preview"
	MCEConsole                string = "console-mce"
	MCEDiscovery              string = "discovery"
	MCEHive                   string = "hive"
	MCEAssistedService        string = "assisted-service"
	MCEClusterLifecycle       string = "cluster-lifecycle-mce"
	MCEClusterManager         string = "cluster-manager"
	MCEServerFoundation       string = "server-foundation"
	MCEHypershift             string = "hypershift-preview"
	MCEHypershiftLocalHosting string = "hypershift-local-hosting"
	MCEClusterProxyAddon      string = "cluster-proxy-addon"
	MCELocalCluster           string = "local-cluster"
)

var allComponents = []string{
	// MCH
	Repo,
	Search,
	Appsub,
	ManagementIngress,
	Console,
	Insights,
	GRC,
	ClusterLifecycle,
	ClusterBackup,
	Volsync,
	MultiClusterEngine,
	// MCE
	MCEAssistedService,
	MCEClusterLifecycle,
	MCEClusterManager,
	MCEDiscovery,
	MCEHive,
	MCEServerFoundation,
	MCEConsole,
	MCEManagedServiceAccount,
	MCEHypershift,
	MCEHypershiftLocalHosting,
	MCEClusterProxyAddon,
}

var MCEComponents = []string{
	MCEAssistedService,
	MCEClusterLifecycle,
	MCEClusterManager,
	MCEDiscovery,
	MCEHive,
	MCEServerFoundation,
	MCEConsole,
	MCEManagedServiceAccount,
	MCEHypershift,
	MCEHypershiftLocalHosting,
}

func GetDefaultEnabledComponents() ([]string, error) {
	var defaultEnabledComponents = []string{
		//Repo,
		Console,
		Insights,
		GRC,
		ClusterLifecycle,
		Volsync,
		MultiClusterEngine,
		Search,
		Appsub,
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
