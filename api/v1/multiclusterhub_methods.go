package v1

const (
	searchDisableDefault                = false
	managedServiceAccountEnabledDefault = false
)

func (mch *MultiClusterHub) ComponentEnabled(c ComponentEnabled) bool {
	if c == Search {
		return !mch.searchDisabled()
	}
	if c == ManagedServiceAccount {
		return mch.managedServiceAccountEnabled()
	}
	return false
}

func (mch *MultiClusterHub) hasComponentConfig() bool {
	return mch.Spec.ComponentConfig != nil
}

func (mch *MultiClusterHub) hasSearchConfig() bool {
	if !mch.hasComponentConfig() {
		return false
	}
	return mch.Spec.ComponentConfig.Search != nil
}

func (mch *MultiClusterHub) searchDisabled() bool {
	if !mch.hasSearchConfig() {
		return searchDisableDefault
	}
	return mch.Spec.ComponentConfig.Search.Disable
}

func (mch *MultiClusterHub) hasManagedServiceAccountConfig() bool {
	if !mch.hasComponentConfig() {
		return false
	}
	return mch.Spec.ComponentConfig.ManagedServiceAccount != nil
}

func (mch *MultiClusterHub) managedServiceAccountEnabled() bool {
	if !mch.hasManagedServiceAccountConfig() {
		return managedServiceAccountEnabledDefault
	}
	return mch.Spec.ComponentConfig.ManagedServiceAccount.Enable
}
