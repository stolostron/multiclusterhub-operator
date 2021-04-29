// Copyright (c) 2020 Red Hat, Inc.

package foundation

import (
	"testing"

	operatorsv1 "github.com/open-cluster-management/multiclusterhub-operator/pkg/apis/operator/v1"
)

func TestClusterManager(t *testing.T) {

	empty := &operatorsv1.MultiClusterHub{}

	imageOverrides := map[string]string{
		"registration": "quay.io/open-cluster-management/registration@sha256:fe95bca419976ca8ffe608bc66afcead6ef333b863f22be55df57c89ded75dda",
	}

	t.Run("Create Cluster Manager", func(t *testing.T) {
		c := ClusterManager(empty, imageOverrides)
		expectedImage := "quay.io/open-cluster-management/registration@sha256:fe95bca419976ca8ffe608bc66afcead6ef333b863f22be55df57c89ded75dda"

		spec, ok := c.Object["spec"].(map[string]interface{})
		if !ok {
			t.Errorf("expected cluster manager spec not found")
		}

		registrationImage, ok := spec["registrationImagePullSpec"]
		if !ok {
			t.Errorf("expected cluster manager registrationImagePullSpec not found")
		}
		if registrationImage != expectedImage {
			t.Errorf("expected registrationImagePullSpec %s, got %s", registrationImage, expectedImage)
		}
	})

}
