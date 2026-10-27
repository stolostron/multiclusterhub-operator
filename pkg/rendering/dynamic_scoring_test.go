// Copyright Contributors to the Open Cluster Management project

package renderer

import (
	"testing"

	v1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestDynamicScoringUsesEnvironmentDefaultsAndRetainsUserConfigReference(t *testing.T) {
	t.Setenv("DIRECTORY_OVERRIDE", "../templates")
	t.Setenv("ACM_HUB_OCP_VERSION", "4.18.0")
	t.Setenv("HTTP_PROXY", "")
	t.Setenv("HTTPS_PROXY", "")
	t.Setenv("NO_PROXY", "")

	const (
		addonImage      = "quay.io/test/dynamic-scoring-addon:test"
		controllerImage = "quay.io/test/dynamic-scoring-controller:test"
	)
	mch := &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-mch",
			Namespace: "open-cluster-management",
		},
		Spec: v1.MultiClusterHubSpec{
			ImagePullSecret: "mch-pull-secret",
		},
	}

	resources, errs := RenderChart(
		utils.DynamicScoringFrameworkChartLocation,
		mch,
		map[string]string{
			"dynamic_scoring_addon":      addonImage,
			"dynamic_scoring_controller": controllerImage,
		},
		map[string]string{},
		false,
		"v0",
	)
	if len(errs) != 0 {
		t.Fatalf("render dynamic-scoring-framework chart: %v", errs)
	}

	var addon *unstructured.Unstructured
	var addonDeployment *unstructured.Unstructured
	for _, resource := range resources {
		switch resource.GetKind() {
		case "AddOnDeploymentConfig":
			t.Errorf("chart must not create AddOnDeploymentConfig %q", resource.GetName())
		case "ClusterManagementAddOn":
			if resource.GetName() == "dynamic-scoring-framework" {
				addon = resource
			}
		case "Deployment":
			switch resource.GetName() {
			case "dynamic-scoring-addon-controller":
				addonDeployment = resource
			}
		}
	}
	if addon == nil {
		t.Fatal("rendered ClusterManagementAddOn/dynamic-scoring-framework not found")
	}
	if got, want := addon.GetAPIVersion(), "addon.open-cluster-management.io/v1beta1"; got != want {
		t.Errorf("CMA apiVersion = %q, want %q", got, want)
	}
	if _, found, err := unstructured.NestedFieldNoCopy(addon.Object, "spec", "supportedConfigs"); err != nil {
		t.Fatalf("read CMA supportedConfigs: %v", err)
	} else if found {
		t.Error("CMA must not render the removed supportedConfigs field")
	}

	placements, found, err := unstructured.NestedSlice(addon.Object, "spec", "installStrategy", "placements")
	if err != nil || !found || len(placements) != 1 {
		t.Fatalf("CMA placements = %v, found %t, error %v; want one global placement", placements, found, err)
	}
	placement, ok := placements[0].(map[string]interface{})
	if !ok {
		t.Fatalf("CMA placement has unexpected type %T", placements[0])
	}
	configs, found, err := unstructured.NestedSlice(placement, "configs")
	if err != nil || !found || len(configs) != 1 {
		t.Fatalf("CMA config references = %v, found %t, error %v; want one user-config reference", configs, found, err)
	}
	config, ok := configs[0].(map[string]interface{})
	if !ok {
		t.Fatalf("CMA config reference has unexpected type %T", configs[0])
	}
	if got, want := config["name"], "dynamic-scoring-addon-config"; got != want {
		t.Errorf("CMA config name = %v, want %q", got, want)
	}
	if got, want := config["resource"], "addondeploymentconfigs"; got != want {
		t.Errorf("CMA config resource = %v, want %q", got, want)
	}
	if got, want := config["namespace"], mch.Namespace; got != want {
		t.Errorf("CMA config namespace = %v, want %q", got, want)
	}

	if addonDeployment == nil {
		t.Fatal("rendered Deployment/dynamic-scoring-addon-controller not found")
	}
	containers, found, err := unstructured.NestedSlice(addonDeployment.Object, "spec", "template", "spec", "containers")
	if err != nil || !found || len(containers) != 1 {
		t.Fatalf("addon controller containers = %v, found %t, error %v; want one container", containers, found, err)
	}
	container, ok := containers[0].(map[string]interface{})
	if !ok {
		t.Fatalf("addon controller container has unexpected type %T", containers[0])
	}
	env, found, err := unstructured.NestedSlice(container, "env")
	if err != nil || !found {
		t.Fatalf("addon controller env = %v, found %t, error %v", env, found, err)
	}
	wantEnv := map[string]string{
		"DYNAMIC_SCORING_ADDON_IMAGE":              addonImage,
		"DYNAMIC_SCORING_ADDON_IMAGE_PULL_SECRETS": `["open-cluster-management-image-pull-credentials","hub-registry-secret"]`,
		"DYNAMIC_SCORING_ADDON_NAMESPACE":          "dynamic-scoring",
	}
	for _, entry := range env {
		variable, ok := entry.(map[string]interface{})
		if !ok {
			continue
		}
		name, _ := variable["name"].(string)
		if want, expected := wantEnv[name]; expected {
			if got := variable["value"]; got != want {
				t.Errorf("%s = %v, want %q", name, got, want)
			}
			delete(wantEnv, name)
		}
	}
	for name := range wantEnv {
		t.Errorf("addon controller is missing environment variable %s", name)
	}
}
