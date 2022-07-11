package v1

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

var DefaultEnabledComponents = []string{
	Repo,
	Search,
	ManagementIngress,
	Console,
	Insights,
	GRC,
	ClusterLifecycle,
	Volsync,
	MultiClusterEngine,
}

var DefaultDisabledComponents = []string{
	ClusterBackup,
	SearchV2,
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
