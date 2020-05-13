// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import (
	"testing"

	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/manifest"
)

func TestCacheSpec_isStale(t *testing.T) {
	c := CacheSpec{
		IngressDomain:     "test.com",
		ImageOverrides:    map[string]string{"hello": "world"},
		ImageOverrideType: manifest.Manifest,
		ImageRepository:   "quay.io",
		ManifestVersion:   "0.0.0",
	}

	t.Run("No change to cache", func(t *testing.T) {
		mch := &operatorsv1beta1.MultiClusterHub{
			Spec: operatorsv1beta1.MultiClusterHubSpec{
				Overrides: operatorsv1beta1.Overrides{
					ImageRepository: "quay.io",
				},
			},
		}
		want := false
		if got := c.isStale(mch); got != want {
			t.Errorf("CacheSpec.isStale() = %v, want %v", got, want)
		}
	})

	t.Run("Change override type", func(t *testing.T) {
		mch := &operatorsv1beta1.MultiClusterHub{
			Spec: operatorsv1beta1.MultiClusterHubSpec{
				Overrides: operatorsv1beta1.Overrides{
					ImageRepository: "quay.io",
					ImageTagSuffix:  "foo",
				},
			},
		}
		want := true
		if got := c.isStale(mch); got != want {
			t.Errorf("CacheSpec.isStale() = %v, want %v. Override type should invalidate cache.", got, want)
		}
	})

	t.Run("Change image repository", func(t *testing.T) {
		mch := &operatorsv1beta1.MultiClusterHub{
			Spec: operatorsv1beta1.MultiClusterHubSpec{
				Overrides: operatorsv1beta1.Overrides{
					ImageRepository: "artifactory",
				},
			},
		}
		want := true
		if got := c.isStale(mch); got != want {
			t.Errorf("CacheSpec.isStale() = %v, want %v. Image repo should invalidate cache.", got, want)
		}
	})
}
