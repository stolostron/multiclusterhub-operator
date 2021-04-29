// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import (
	operatorsv1 "github.com/open-cluster-management/multiclusterhub-operator/pkg/apis/operator/v1"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/manifest"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/utils"
)

// CacheSpec ...
type CacheSpec struct {
	IngressDomain     string
	ImageOverrides    map[string]string
	ImageOverrideType manifest.OverrideType
	ImageRepository   string
	ImageSuffix       string
	ManifestVersion   string
}

// Determines whether the cache has become out of date. Returns true if a change to the
// multiclusterhub CR that would alter the cache contents occurs
func (c CacheSpec) isStale(m *operatorsv1.MultiClusterHub) bool {
	// A change in override type invalidates cache
	if oType := manifest.GetImageOverrideType(m); oType != c.ImageOverrideType {
		return true
	}
	// A change in suffix invalidates cache
	if s := utils.GetImageSuffix(m); s != c.ImageSuffix {
		return true
	}
	// A change to image repository invalidates cache
	if repo := utils.GetImageRepository(m); repo != c.ImageRepository {
		return true
	}

	return false
}
