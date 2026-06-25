// Copyright (c) 2024 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package overrides

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stolostron/multiclusterhub-operator/pkg/manifest"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func Test_GetOverridesFromEnv(t *testing.T) {
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

func Test_ConvertImageOverrides(t *testing.T) {
	t.Run("Convert image overrides with no ImageKey", func(t *testing.T) {
		overrides := map[string]string{}
		manifestImages := []manifest.ManifestImage{{
			ImageName:   "foo",
			ImageRemote: "quay.io",
			ImageTag:    "83b7f8",
		}}

		if got := ConvertImageOverrides(overrides, manifestImages); got == nil {
			t.Fatal("Expected conversion to fail due to missing ImageKey")
		}
	})

	t.Run("Convert image overrides with ImageDigest", func(t *testing.T) {
		overrides := map[string]string{}
		manifestImages := []manifest.ManifestImage{{
			ImageDigest: "sha256:4e1a295760c9f2fc7b2b143e6933a625892fda6fe2b3c597d4318d1b1ab3b276",
			ImageKey:    "foo",
			ImageName:   "foo",
			ImageRemote: "quay.io",
		}}

		if got := ConvertImageOverrides(overrides, manifestImages); got != nil {
			t.Fatalf("Expected conversion to pass, got: %v", got)
		}
	})

	t.Run("Convert image overrides with ImageTag", func(t *testing.T) {
		overrides := map[string]string{}
		manifestImages := []manifest.ManifestImage{{
			ImageKey:    "foo",
			ImageName:   "foo",
			ImageRemote: "quay.io",
			ImageTag:    "83b7f8",
		}}

		if got := ConvertImageOverrides(overrides, manifestImages); got != nil {
			t.Fatalf("Expected conversion to pass, got: %v", got)
		}
	})

	t.Run("Convert image overrides with preexisting values", func(t *testing.T) {
		overrides := map[string]string{"foo": "quay.io/foo:123"}
		manifestImages := []manifest.ManifestImage{{
			ImageKey:    "foo",
			ImageName:   "foo",
			ImageRemote: "quay.io",
			ImageTag:    "83b7f8",
		}}

		if got := ConvertImageOverrides(overrides, manifestImages); got != nil {
			t.Fatalf("Expected conversion to pass, got: %v", got)
		}

		if ok := strings.Contains(overrides["foo"], "83b7f8"); !ok {
			t.Fatalf("Expected conversion to override preexisting value, got: %v", ok)
		}
	})
}

func Test_ConvertTemplateOverrides(t *testing.T) {
	t.Run("Convert template overrides with preexisting values", func(t *testing.T) {
		overrides := map[string]string{
			"foo_limit_cpu": "3m",
		}
		manifestTemplates := manifest.ManifestTemplate{
			TemplateOverrides: map[string]interface{}{
				"foo_limit_cpu":    "1m",
				"foo_limit_memory": "40Mi",
			},
		}

		if got := ConvertTemplateOverrides(overrides, manifestTemplates); got != nil {
			t.Fatalf("Expected conversion to pass, got: %v", got)
		}

		if ok := overrides["foo_limit_cpu"]; ok != "3m" {
			t.Fatalf("Expected conversion to skip override due to preexisting value, got: %v", ok)
		}
	})

	t.Run("Convert template overrides", func(t *testing.T) {
		overrides := map[string]string{}
		manifestTemplates := manifest.ManifestTemplate{
			TemplateOverrides: map[string]interface{}{
				"foo_limit_cpu":    "3m",
				"foo_limit_memory": "40Mi",
			},
		}

		if got := ConvertTemplateOverrides(overrides, manifestTemplates); got != nil {
			t.Fatalf("Expected conversion to pass, got: %v", got)
		}

		if ok := overrides["foo_limit_cpu"]; ok != "3m" {
			t.Fatalf("Expected conversion to pass, got: %v", ok)
		}
	})
}

func Test_ConvertToString(t *testing.T) {
	t.Run("Convert int to string", func(t *testing.T) {
		var v interface{}

		v, err := ConvertToString(1)
		if err != nil {
			t.Fatalf("Expected int to be converted to string, got %v", err)
		}

		if _, ok := v.(string); !ok {
			t.Fatalf("Expected int to be converted to string, got %v", ok)
		}
	})

	t.Run("Convert float to string", func(t *testing.T) {
		var v interface{}

		v, err := ConvertToString(1.5)
		if err != nil {
			t.Fatalf("Expected float to be converted to string, got %v", err)
		}

		if _, ok := v.(string); !ok {
			t.Fatalf("Expected float to be converted to string, got %v", ok)
		}
	})

	t.Run("Convert bool to string", func(t *testing.T) {
		var v interface{}

		v, err := ConvertToString(true)
		if err != nil {
			t.Fatalf("Expected bool to be converted to string, got %v", err)
		}

		if _, ok := v.(string); !ok {
			t.Fatalf("Expected bool to be converted to string, got %v", ok)
		}
	})

	t.Run("Convert string", func(t *testing.T) {
		var v interface{}

		v, err := ConvertToString("foo")
		if err != nil {
			t.Fatalf("Expected conversion to return string, got %v", err)
		}

		if _, ok := v.(string); !ok {
			t.Fatalf("Expected conversion to return to string, got %v", ok)
		}
	})
}

func Test_GetOverridesFromConfigmap(t *testing.T) {
	// Create a fake clientset
	fakeclient := fake.NewClientBuilder().Build()

	t.Run("Get overrides from template configmap", func(t *testing.T) {
		cm := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "configmapName",
				Namespace: "namespace",
			},
			Data: map[string]string{
				"template-override.json": `
					{
						"templateOverrides": {
							"console_limit_cpu": "30Mi"
						}
					}
				`,
			},
		}

		err := fakeclient.Create(context.TODO(), &cm, &client.CreateOptions{})
		if err != nil {
			fmt.Println("Error creating ConfigMap:", err)
		}

		overrides := map[string]string{}
		overrides, err = GetOverridesFromConfigmap(fakeclient, overrides, "namespace", "configmapName", true)
		if err != nil {
			t.Errorf("Failed to get overrides from configmap: %v", err)
		}

		if overrides["console_limit_cpu"] == "" {
			t.Errorf("Failed to get correct override for console_limit_cpu: %v", overrides["console_limit_cpu"])
		}

		fakeclient.Delete(context.TODO(), &cm, &client.DeleteOptions{})
	})

	t.Run("Get overrides from image configmap", func(t *testing.T) {
		cm := corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "configmapName",
				Namespace: "namespace",
			},
			Data: map[string]string{
				"image-override.json": `
					[
						{
						"image-name": "bar",
						"image-tag": "0.0.1",
						"image-remote": "quay.io/foo",
						"image-key": "bar"
						}
				  	]
				`,
			},
		}

		err := fakeclient.Create(context.TODO(), &cm, &client.CreateOptions{})
		if err != nil {
			fmt.Println("Error creating ConfigMap:", err)
		}

		overrides := map[string]string{}
		overrides, err = GetOverridesFromConfigmap(fakeclient, overrides, "namespace", "configmapName", false)
		if err != nil {
			t.Errorf("Failed to get overrides from configmap: %v", err)
		}

		if overrides["bar"] == "" {
			t.Errorf("Failed to get correct override for bar: %v", overrides["bar"])
		}

		// fakeclient.Delete(context.TODO(), &cm, &client.DeleteOptions{})
	})
}
