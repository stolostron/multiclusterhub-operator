// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package overrides

import (
	"os"
	"testing"
)

func TestGetTemplateOverrides(t *testing.T) {
	os.Setenv("TEMPLATE_OVERRIDE_FOO", "")
	os.Setenv("TEMPLATE_OVERRIDE_BAR", "")

	if len(GetTemplateOverrides()) != 2 {
		t.Fatal("Expected template overrides")
	}

	os.Unsetenv("TEMPLATE_OVERRIDE_FOO")
	os.Unsetenv("TEMPLATE_OVERRIDE_BAR")

	if len(GetImageOverrides()) != 0 {
		t.Fatal("Expected no template overrides")
	}
}
