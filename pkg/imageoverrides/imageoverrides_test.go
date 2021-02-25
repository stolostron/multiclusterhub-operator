// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project


package imageoverrides

import (
	"os"
	"testing"
)

func TestGetImageOverridesRelatedImage(t *testing.T) {
	os.Setenv("RELATED_IMAGE_APPLICATION_UI", "quay.io/open-cluster-management/application-ui:test-image")
	os.Setenv("RELATED_IMAGE_CERT_POLICY_CONTROLLER", "quay.io/open-cluster-management/cert-policy-controller:test-image")

	if len(GetImageOverrides()) != 2 {
		t.Fatal("Expected image overrides")
	}

	os.Unsetenv("RELATED_IMAGE_APPLICATION_UI")
	os.Unsetenv("RELATED_IMAGE_CERT_POLICY_CONTROLLER")

	if len(GetImageOverrides()) != 0 {
		t.Fatal("Expected no image overrides")
	}
}

func TestGetImageOverridesOperandImage(t *testing.T) {
	os.Setenv("OPERAND_IMAGE_APPLICATION_UI", "quay.io/open-cluster-management/application-ui:test-image")
	os.Setenv("OPERAND_IMAGE_CERT_POLICY_CONTROLLER", "quay.io/open-cluster-management/cert-policy-controller:test-image")

	if len(GetImageOverrides()) != 2 {
		t.Fatal("Expected image overrides")
	}

	os.Unsetenv("OPERAND_IMAGE_APPLICATION_UI")
	os.Unsetenv("OPERAND_IMAGE_CERT_POLICY_CONTROLLER")

	if len(GetImageOverrides()) != 0 {
		t.Fatal("Expected no image overrides")
	}
}

func TestGetImageOverridesBothEnvVars(t *testing.T) {
	os.Setenv("RELATED_IMAGE_APPLICATION_UI", "quay.io/open-cluster-management/application-ui:test-image")
	os.Setenv("OPERAND_IMAGE_CERT_POLICY_CONTROLLER", "quay.io/open-cluster-management/cert-policy-controller:test-image")

	if len(GetImageOverrides()) != 1 {
		t.Fatal("Expected image overrides")
	}

	os.Unsetenv("OPERAND_IMAGE_CERT_POLICY_CONTROLLER")

	if len(GetImageOverrides()) != 1 {
		t.Fatal("Expected no image overrides")
	}
}
