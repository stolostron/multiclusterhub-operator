// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import (
	"testing"

	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/manifest"
)

func TestCacheSpec_IsStale(t *testing.T) {
	cs := CacheSpec{
		IngressDomain:     "test.com",
		ImageOverrides:    map[string]string{"hello": "world"},
		ImageOverrideType: manifest.Suffix,
		ImageRepository:   "quay.io",
		ImageSuffix:       "foo",
		ManifestVersion:   "0.0.0",
		CRName:            "name",
	}
	cm := CacheSpec{
		IngressDomain:     "test.com",
		ImageOverrides:    map[string]string{"hello": "world"},
		ImageOverrideType: manifest.Manifest,
		ImageRepository:   "quay.io",
		ImageSuffix:       "",
		ManifestVersion:   "0.0.0",
		CRName:            "name",
	}

	t.Run("No change to suffix cache", func(t *testing.T) {
		mch := &operatorsv1beta1.MultiClusterHub{
			Spec: operatorsv1beta1.MultiClusterHubSpec{
				Overrides: operatorsv1beta1.Overrides{
					ImageRepository: "quay.io",
					ImageTagSuffix:  "foo",
				},
			},
		}
		want := false
		if got := cs.isStale(mch); got != want {
			t.Errorf("CacheSpec.isStale() = %v, want %v", got, want)
		}
	})

	t.Run("No change to manifest cache", func(t *testing.T) {
		mch := &operatorsv1beta1.MultiClusterHub{
			Spec: operatorsv1beta1.MultiClusterHubSpec{
				Overrides: operatorsv1beta1.Overrides{
					ImageRepository: "quay.io",
				},
			},
		}
		want := false
		if got := cm.isStale(mch); got != want {
			t.Errorf("CacheSpec.isStale() = %v, want %v", got, want)
		}
	})

	t.Run("Change type to manifest", func(t *testing.T) {
		mch := &operatorsv1beta1.MultiClusterHub{
			Spec: operatorsv1beta1.MultiClusterHubSpec{
				Overrides: operatorsv1beta1.Overrides{
					ImageRepository: "quay.io",
				},
			},
		}
		want := true
		if got := cs.isStale(mch); got != want {
			t.Errorf("CacheSpec.isStale() = %v, want %v. Override type should invalidate cache.", got, want)
		}
	})

	t.Run("Change type to suffix", func(t *testing.T) {
		mch := &operatorsv1beta1.MultiClusterHub{
			Spec: operatorsv1beta1.MultiClusterHubSpec{
				Overrides: operatorsv1beta1.Overrides{
					ImageRepository: "quay.io",
					ImageTagSuffix:  "foo",
				},
			},
		}
		want := true
		if got := cm.isStale(mch); got != want {
			t.Errorf("CacheSpec.isStale() = %v, want %v. Override type should invalidate cache.", got, want)
		}
	})

	t.Run("Change suffix", func(t *testing.T) {
		mch := &operatorsv1beta1.MultiClusterHub{
			Spec: operatorsv1beta1.MultiClusterHubSpec{
				Overrides: operatorsv1beta1.Overrides{
					ImageRepository: "quay.io",
					ImageTagSuffix:  "bar",
				},
			},
		}
		want := true
		if got := cs.isStale(mch); got != want {
			t.Errorf("CacheSpec.isStale() = %v, want %v. Suffix change should invalidate cache.", got, want)
		}
	})

	t.Run("Change image repository", func(t *testing.T) {
		mch := &operatorsv1beta1.MultiClusterHub{
			Spec: operatorsv1beta1.MultiClusterHubSpec{
				Overrides: operatorsv1beta1.Overrides{
					ImageRepository: "artifactory",
					ImageTagSuffix:  "foo",
				},
			},
		}
		want := true
		if got := cs.isStale(mch); got != want {
			t.Errorf("CacheSpec.isStale() = %v, want %v. Image repo should invalidate cache.", got, want)
		}
	})
}
