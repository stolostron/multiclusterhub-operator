package imageoverrides

import (
	"os"
	"testing"
)

func TestGetImageOverrides(t *testing.T) {
	os.Setenv("RELEASES_IMAGE_APPLICATION_UI", "quay.io/open-cluster-management/application-ui:test-image")
	os.Setenv("RELEASES_IMAGE_CERT_POLICY_CONTROLLER", "quay.io/open-cluster-management/cert-policy-controller:test-image")
	defer os.Unsetenv("RELEASES_IMAGE_APPLICATION_UI")
	defer os.Unsetenv("RELEASES_IMAGE_CERT_POLICY_CONTROLLER")

	imageOverrides := GetImageOverrides()
	if len(imageOverrides) == 0 {
		t.Fatal("Expected Image overrides")
	}
}
