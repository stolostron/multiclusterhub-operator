// Copyright (c) 2024 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package overrides

import (
	"os"
	"testing"
)

func TestGetOverridesFromEnv(t *testing.T) {
	os.Setenv("OPERAND_IMAGE_APPLICATION_UI", "quay.io/stolostron/application-ui:test-image")
	os.Setenv("OPERAND_IMAGE_CERT_POLICY_CONTROLLER", "quay.io/stolostron/cert-policy-controller:test-image")

	if len(GetOverridesFromEnv(OperandImagePrefix)) != 2 {
		t.Fatal("Expected image overrides")
	}

	os.Unsetenv("OPERAND_IMAGE_APPLICATION_UI")
	os.Unsetenv("OPERAND_IMAGE_CERT_POLICY_CONTROLLER")

	if len(GetOverridesFromEnv(OperandImagePrefix)) != 0 {
		t.Fatal("Expected no image overrides")
	}

	os.Setenv("RELATED_IMAGE_APPLICATION_UI", "quay.io/stolostron/application-ui:test-image")
	os.Setenv("RELATED_IMAGE_CERT_POLICY_CONTROLLER", "quay.io/stolostron/cert-policy-controller:test-image")

	if len(GetOverridesFromEnv(OSBSImagePrefix)) != 2 {
		t.Fatal("Expected related image overrides")
	}

	os.Unsetenv("RELATED_IMAGE_APPLICATION_UI")
	os.Unsetenv("RELATED_IMAGE_CERT_POLICY_CONTROLLER")

	if len(GetOverridesFromEnv(OSBSImagePrefix)) != 0 {
		t.Fatal("Expected no related image overrides")
	}

	os.Setenv("TEMPLATE_OVERRIDE_FOO_LIMIT_CPU", "3m")
	os.Setenv("TEMPLATE_OVERRIDE_FOO_LIMIT_MEMORY", "40Mi")

	if len(GetOverridesFromEnv(TemplateOverridePrefix)) != 2 {
		t.Fatal("Expected template overrides")
	}

	os.Unsetenv("TEMPLATE_OVERRIDE_FOO_LIMIT_CPU")
	os.Unsetenv("TEMPLATE_OVERRIDE_FOO_LIMIT_MEMORY")

	if len(GetOverridesFromEnv(TemplateOverridePrefix)) != 0 {
		t.Fatal("Expected no template overrides")
	}
}
