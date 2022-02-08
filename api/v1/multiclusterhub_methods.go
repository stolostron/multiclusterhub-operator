package v1

const (
	searchDisableDefault = false
)

func (mch *MultiClusterHub) hasComponentConfig() bool {
	return mch.Spec.ComponentConfig != nil
}

func (mch *MultiClusterHub) hasSearchConfig() bool {
	if !mch.hasComponentConfig() {
		return false
	}
	return mch.Spec.ComponentConfig.Search != nil
}

func (mch *MultiClusterHub) SearchDisabled() bool {
	if !mch.hasSearchConfig() {
		return searchDisableDefault
	}
	return mch.Spec.ComponentConfig.Search.Disable
}
