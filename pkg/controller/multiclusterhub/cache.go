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
	ImageSuffix       string
	ManifestVersion   string
	CRName            string
}

// Determines whether the cache has become out of date. Returns true if a change to the
// multiclusterhub CR that would alter the cache contents occurs
func (c CacheSpec) isStale(m *operatorsv1beta1.MultiClusterHub) bool {
	// A change in override type invalidates cache
	if oType := manifest.GetImageOverrideType(m); oType != c.ImageOverrideType {
		return true
	}
	// A change in suffix invalidates cache
	if s := m.Spec.Overrides.ImageTagSuffix; s != c.ImageSuffix {
		return true
	}
	// A change to image repository invalidates cache
	if repo := m.Spec.Overrides.ImageRepository; repo != c.ImageRepository {
		return true
	}

	return false
}
