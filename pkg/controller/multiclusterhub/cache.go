// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import "github.com/open-cluster-management/multicloudhub-operator/pkg/manifest"

// CacheSpec ...
type CacheSpec struct {
	IngressDomain     string
	ImageOverrides    map[string]string
	ImageOverrideType manifest.OverrideType
	ManifestVersion   string
}
