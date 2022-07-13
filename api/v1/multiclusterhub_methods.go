package v1

import (
	"errors"
	"os"
)

const (
	Search             string = "search"
	SearchV2           string = "search-v2"
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
	MCEManagedServiceAccount string = "managedserviceaccount-preview"
	MCEConsole               string = "console-mce"
	MCEDiscovery             string = "discovery"
	MCEHive                  string = "hive"
	MCEAssistedService       string = "assisted-service"
	MCEClusterLifecycle      string = "cluster-lifecycle-mce"
	MCEClusterManager        string = "cluster-manager"
	MCEServerFoundation      string = "server-foundation"
	MCEHypershift            string = "hypershift-preview"
	MCEClusterProxyAddon     string = "cluster-proxy-addon"
)

var allComponents = []string{
	// MCH
	Repo,
	Search,
	SearchV2,
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
}

func GetDefaultEnabledComponents() ([]string, error) {
	var defaultEnabledComponents = []string{
		Repo,
		ManagementIngress,
		Console,
		Insights,
		GRC,
		ClusterLifecycle,
		Volsync,
		MultiClusterEngine,
	}
	community, err := IsCommunity()

	if err != nil {
		return defaultEnabledComponents, err
	}

	if community {
		defaultEnabledComponents = append(defaultEnabledComponents, SearchV2)
	} else {
		defaultEnabledComponents = append(defaultEnabledComponents, Search)
	}
	return defaultEnabledComponents, err
}

func GetDefaultDisabledComponents() ([]string, error) {
	var defaultDisabledComponents = []string{
		ClusterBackup,
	}
	community, err := IsCommunity()

	if err != nil {
		return defaultDisabledComponents, err
	}

	if community {
		defaultDisabledComponents = append(defaultDisabledComponents, Search)
	} else {
		defaultDisabledComponents = append(defaultDisabledComponents, SearchV2)
	}
	return defaultDisabledComponents, err
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
