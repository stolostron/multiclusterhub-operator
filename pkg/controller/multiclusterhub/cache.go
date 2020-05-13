// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import (
	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/manifest"
)

// CacheSpec ...
type CacheSpec struct {
	IngressDomain     string
	ImageOverrides    map[string]string
	ImageOverrideType manifest.OverrideType
	ImageRepository   string
	ManifestVersion   string
}

// Determines whether the cache has become out of date. Returns true if a change to the
// multiclusterhub CR that would alter the cache contents occurs
func (c CacheSpec) isStale(m *operatorsv1beta1.MultiClusterHub) bool {
	// Override type invalidates cache
	if oType := manifest.GetImageOverrideType(m); oType != c.ImageOverrideType {
		return true
	}
	// Repository change invalidates cache
	if repo := m.Spec.Overrides.ImageRepository; repo != c.ImageRepository {
		return true
	}

	return false
}
