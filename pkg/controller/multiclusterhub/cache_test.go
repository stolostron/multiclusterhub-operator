// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import (
	"testing"

	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/manifest"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCacheSpec_IsStale(t *testing.T) {
	cs := CacheSpec{
		IngressDomain:     "test.com",
		ImageOverrides:    map[string]string{"hello": "world"},
		ImageOverrideType: manifest.Suffix,
		ImageRepository:   "quay.io",
		ImageSuffix:       "foo",
		ManifestVersion:   "0.0.0",
	}
	cm := CacheSpec{
		IngressDomain:     "test.com",
		ImageOverrides:    map[string]string{"hello": "world"},
		ImageOverrideType: manifest.Manifest,
		ImageRepository:   "quay.io",
		ImageSuffix:       "",
		ManifestVersion:   "0.0.0",
	}

	t.Run("No change to suffix cache", func(t *testing.T) {
		mch := &operatorsv1beta1.MultiClusterHub{
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					utils.AnnotationImageRepo: "quay.io",
					utils.AnnotationSuffix:    "foo",
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
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					utils.AnnotationImageRepo: "quay.io",
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
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					utils.AnnotationImageRepo: "quay.io",
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
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					utils.AnnotationImageRepo: "quay.io",
					utils.AnnotationSuffix:    "foo",
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
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					utils.AnnotationImageRepo: "quay.io",
					utils.AnnotationSuffix:    "bar",
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
			ObjectMeta: metav1.ObjectMeta{
				Annotations: map[string]string{
					utils.AnnotationImageRepo: "artifactory",
					utils.AnnotationSuffix:    "foo",
				},
			},
		}
		want := true
		if got := cs.isStale(mch); got != want {
			t.Errorf("CacheSpec.isStale() = %v, want %v. Image repo should invalidate cache.", got, want)
		}
	})
}
