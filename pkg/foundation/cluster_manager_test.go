// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package foundation

import (
	"testing"

	operatorsv1 "github.com/open-cluster-management/multiclusterhub-operator/api/v1"
)

func TestClusterManager(t *testing.T) {

	empty := &operatorsv1.MultiClusterHub{}

	imageOverrides := map[string]string{
		"registration": "quay.io/open-cluster-management/registration@sha256:fe95bca419976ca8ffe608bc66afcead6ef333b863f22be55df57c89ded75dda",
		"work":         "quay.io/open-cluster-management/work@sha256:856d2151423f020952d9b9253676c1c4d462fab6722c8af4885fe2b19ccd1be0",
		"placement":    "quay.io/open-cluster-management/placement@sha256:8d69eb89ee008bf95c2b877887e66cc1541c2407c9d7339fff8a9a973200660f",
	}

	t.Run("Create Cluster Manager", func(t *testing.T) {
		c := ClusterManager(empty, imageOverrides)
		expectedRegistrationImage := "quay.io/open-cluster-management/registration@sha256:fe95bca419976ca8ffe608bc66afcead6ef333b863f22be55df57c89ded75dda"
		expectedWorkImage := "quay.io/open-cluster-management/work@sha256:856d2151423f020952d9b9253676c1c4d462fab6722c8af4885fe2b19ccd1be0"
		expectedPlacementImage := "quay.io/open-cluster-management/placement@sha256:8d69eb89ee008bf95c2b877887e66cc1541c2407c9d7339fff8a9a973200660f"

		spec, ok := c.Object["spec"].(map[string]interface{})
		if !ok {
			t.Errorf("expected cluster manager spec not found")
		}

		registrationImage, ok := spec["registrationImagePullSpec"]
		if !ok {
			t.Errorf("expected cluster manager registrationImagePullSpec not found")
		}
		if registrationImage != expectedRegistrationImage {
			t.Errorf("expected registrationImagePullSpec %s, got %s", registrationImage, expectedRegistrationImage)
		}

		workImage, ok := spec["workImagePullSpec"]
		if !ok {
			t.Errorf("expected cluster manager workImagePullSpec not found")
		}
		if workImage != expectedWorkImage {
			t.Errorf("expected workImagePullSpec %s, got %s", workImage, expectedWorkImage)
		}

		placementImage, ok := spec["placementImagePullSpec"]
		if !ok {
			t.Errorf("expected cluster manager placementImagePullSpec not found")
		}
		if placementImage != expectedPlacementImage {
			t.Errorf("expected placementImagePullSpec %s, got %s", placementImage, expectedPlacementImage)
		}
	})

}
