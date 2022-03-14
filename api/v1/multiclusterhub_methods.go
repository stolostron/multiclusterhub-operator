package v1

import (
	"errors"
	"fmt"
)

const (
	Search             string = "search"
	ManagementIngress  string = "management-ingress"
	Console            string = "console"
	Insights           string = "insights"
	GRC                string = "grc"
	ClusterLifecycle   string = "cluster-lifecycle"
	ClusterBackup      string = "cluster-backup"
	ClusterProxyAddon  string = "cluster-proxy-addon"
	Repo               string = "multiclusterhub-repo"
	MultiClusterEngine string = "multicluster-engine"
	Volsync            string = "volsync"

	// MCE
	MCEManagedServiceAccount string = "managed-service-account"
	MCEConsole               string = "console-mce"
	MCEDiscovery             string = "discovery"
	MCEHive                  string = "hive"
	MCEAssistedService       string = "assisted-service"
	MCEClusterLifecycle      string = "cluster-lifecycle-mce"
	MCEClusterManager        string = "cluster-manager"
	MCEServerFoundation      string = "server-foundation"
)

var allComponents = []string{
	// MCH
	Repo,
	Search,
	ManagementIngress,
	Console,
	Insights,
	GRC,
	ClusterLifecycle,
	ClusterBackup,
	ClusterProxyAddon,
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
}

var mceComponents = []string{
	MCEAssistedService,
	MCEClusterLifecycle,
	MCEClusterManager,
	MCEDiscovery,
	MCEHive,
	MCEServerFoundation,
	MCEConsole,
	MCEManagedServiceAccount,
}

var requiredComponents = []string{
	Repo,
	ManagementIngress,
	Console,
	Insights,
	GRC,
	ClusterLifecycle,
	MultiClusterEngine,
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
	ClusterProxyAddon,
}

func (mch *MultiClusterHub) ComponentPresent(s string) bool {
	for _, c := range mch.Spec.Components {
		if c.Name == s {
			return true
		}
	}
	return false
}

func (mch *MultiClusterHub) Enabled(s string) bool {
	for _, c := range mch.Spec.Components {
		if c.Name == s {
			return c.Enabled
		}
	}

	return false
}

func (mch *MultiClusterHub) Enable(s string) {
	for i, c := range mch.Spec.Components {
		if c.Name == s {
			mch.Spec.Components[i].Enabled = true
			return
		}
	}
	mch.Spec.Components = append(mch.Spec.Components, ComponentConfig{
		Name:    s,
		Enabled: true,
	})
}

func (mch *MultiClusterHub) Disable(s string) {
	for i, c := range mch.Spec.Components {
		if c.Name == s {
			mch.Spec.Components[i].Enabled = false
			return
		}
	}
	mch.Spec.Components = append(mch.Spec.Components, ComponentConfig{
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

func RequiredComponentsPresentCheck(mch *MultiClusterHub) error {
	for _, req := range requiredComponents {
		if mch.ComponentPresent(req) && !mch.Enabled(req) {
			return errors.New(fmt.Sprintf("invalid component config: %s can not be disabled", req))
		}
	}
	return nil
}

func (mch *MultiClusterHub) GetMCEComponents() []ComponentConfig {
	config := []ComponentConfig{}
	for _, n := range mceComponents {
		if mch.ComponentPresent(n) {
			config = append(config, ComponentConfig{Name: n, Enabled: mch.Enabled(n)})
		}
	}
	return config
}
