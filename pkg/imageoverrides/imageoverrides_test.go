// Copyright (c) 2020 Red Hat, Inc.

package imageoverrides

import (
	"os"
	"testing"
)

func TestGetImageOverrides(t *testing.T) {
	os.Setenv("RELATED_IMAGE_APPLICATION_UI", "quay.io/open-cluster-management/application-ui:test-image")
	os.Setenv("RELATED_IMAGE_CERT_POLICY_CONTROLLER", "quay.io/open-cluster-management/cert-policy-controller:test-image")

	if len(GetImageOverrides()) == 0 {
		t.Fatal("Expected image overrides")
	}

	os.Unsetenv("RELATED_IMAGE_APPLICATION_UI")
	os.Unsetenv("RELATED_IMAGE_CERT_POLICY_CONTROLLER")

	if len(GetImageOverrides()) != 0 {
		t.Fatal("Expected no image overrides")
	}
}
