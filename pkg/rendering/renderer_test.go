// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package renderer

import (

	// "reflect"

	"os"
	"reflect"
	"testing"

	v1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

const (
	chartsDir = "/charts/toggle"
	crdsDir   = "/crds"
)

var chartPaths = []string{
	utils.InsightsChartLocation,
	utils.SearchV2ChartLocation,
	utils.CLCChartLocation,
	utils.GRCChartLocation,
	utils.ConsoleChartLocation,
	utils.VolsyncChartLocation,
}

func TestRender(t *testing.T) {

	proxyList := []string{"insights-client"}
	mchNodeSelector := map[string]string{
		"select":  "test",
		"select2": "test2",
	}
	mchImagePullSecret := "test"
	mchNamespace := "default"
	mchTolerations := []corev1.Toleration{
		{
			Key:      "dedicated",
			Operator: "Exists",
			Effect:   "NoSchedule",
			Value:    "test",
		},
		{
			Key:      "node.ocs.openshift.io/storage",
			Operator: "Equal",
			Value:    "true",
			Effect:   "NoSchedule",
		},
		{
			Key:      "false",
			Operator: "false",
			Value:    "true",
			Effect:   "true",
		},
		{
			Key:      "22",
			Operator: "23",
			Value:    "24",
			Effect:   "25",
		},
		{
			Key:      "22.0",
			Operator: "23.1",
			Value:    "24.2",
			Effect:   "25.3",
		},
	}
	testMCH := &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testmch",
			Namespace: mchNamespace,
		},
		Spec: v1.MultiClusterHubSpec{
			NodeSelector:    mchNodeSelector,
			ImagePullSecret: mchImagePullSecret,
			Tolerations:     mchTolerations,
		},
	}
	containsHTTP := false
	containsHTTPS := false
	containsNO := false
	os.Setenv("POD_NAMESPACE", "default")
	os.Setenv("HTTP_PROXY", "test1")
	os.Setenv("HTTPS_PROXY", "test2")
	os.Setenv("NO_PROXY", "test3")
	os.Setenv("DIRECTORY_OVERRIDE", "../templates")
	os.Setenv("ACM_HUB_OCP_VERSION", "4.10.0")

	testImages := map[string]string{}
	for _, v := range utils.GetTestImages() {
		testImages[v] = "quay.io/test/test:Test"
	}
	templateOverrides := map[string]string{}

	// multiple charts
	chartsDir := chartsDir
	templates, errs := RenderCharts(chartsDir, testMCH, testImages, templateOverrides, false, "v0")
	if len(errs) > 0 {
		for _, err := range errs {
			t.Log(err.Error())
		}
		t.Fatalf("failed to retrieve templates")
		if len(templates) == 0 {
			t.Fatalf("Unable to render templates")
		}
	}

	for _, template := range templates {
		if template.GetKind() == "Deployment" {
			deployment := &appsv1.Deployment{}
			err := runtime.DefaultUnstructuredConverter.FromUnstructured(template.Object, deployment)
			if err != nil {
				t.Fatal(err.Error())
			}

			selectorEquality := reflect.DeepEqual(deployment.Spec.Template.Spec.NodeSelector, mchNodeSelector)
			if !selectorEquality {
				t.Fatalf("Node Selector did not propagate to the deployments use")
			}
			secretEquality := reflect.DeepEqual(deployment.Spec.Template.Spec.ImagePullSecrets[0].Name, mchImagePullSecret)
			if !secretEquality {
				t.Fatalf("Image Pull Secret did not propagate to the deployments use")
			}
			tolerationEquality := reflect.DeepEqual(deployment.Spec.Template.Spec.Tolerations, mchTolerations)
			if !tolerationEquality {
				t.Fatalf("Toleration did not propagate to the deployments use")
			}
			if deployment.ObjectMeta.Namespace != mchNamespace && deployment.ObjectMeta.Name != "cluster-backup-chart-clusterbackup" {
				t.Fatalf("Namespace did not propagate to the deployments use")
			}
			if utils.Contains(proxyList, deployment.ObjectMeta.Name) {
				for _, proxyVar := range deployment.Spec.Template.Spec.Containers[0].Env {
					switch proxyVar.Name {
					case "HTTP_PROXY":
						containsHTTP = true
						if proxyVar.Value != "test1" {
							t.Fatalf("HTTP_PROXY not propagated")
						}
					case "HTTPS_PROXY":
						containsHTTPS = true
						if proxyVar.Value != "test2" {
							t.Fatalf("HTTPS_PROXY not propagated")
						}
					case "NO_PROXY":
						containsNO = true
						if proxyVar.Value != "test3" {
							t.Fatalf("NO_PROXY not propagated")
						}
					}

				}

				if !containsHTTP || !containsHTTPS || !containsNO {
					t.Fatalf("proxy variables not set in %s", deployment.ObjectMeta.Name)
				}
			}
			containsHTTP = false
			containsHTTPS = false
			containsNO = false
		}

	}

	// single chart
	singleChartTestImages := map[string]string{}
	for _, v := range utils.GetTestImages() {
		singleChartTestImages[v] = "quay.io/test/test:Test"
	}

	for _, chartsPath := range chartPaths {
		chartsPath := chartsPath
		singleChartTemplates, errs := RenderChart(chartsPath, testMCH, singleChartTestImages, templateOverrides, false, "v0")
		if len(errs) > 0 {
			for _, err := range errs {
				t.Log(err.Error())
			}
			t.Fatalf("failed to retrieve templates")
			if len(singleChartTemplates) == 0 {
				t.Fatalf("Unable to render templates")
			}
		}
		for _, template := range singleChartTemplates {
			if template.GetKind() == "Deployment" {
				deployment := &appsv1.Deployment{}
				err := runtime.DefaultUnstructuredConverter.FromUnstructured(template.Object, deployment)
				if err != nil {
					t.Fatal(err.Error())
				}

				selectorEquality := reflect.DeepEqual(deployment.Spec.Template.Spec.NodeSelector, mchNodeSelector)
				if !selectorEquality {
					t.Fatalf("Node Selector did not propagate to the deployments use")
				}
				secretEquality := reflect.DeepEqual(deployment.Spec.Template.Spec.ImagePullSecrets[0].Name, mchImagePullSecret)
				if !secretEquality {
					t.Fatalf("Image Pull Secret did not propagate to the deployments use")
				}
				tolerationEquality := reflect.DeepEqual(deployment.Spec.Template.Spec.Tolerations, mchTolerations)
				if !tolerationEquality {
					t.Fatalf("Toleration did not propagate to the deployments use")
				}
				if deployment.ObjectMeta.Namespace != mchNamespace && deployment.ObjectMeta.Name != "cluster-backup-chart-clusterbackup" {
					t.Fatalf("Namespace did not propagate to the deployments use")
				}

				if utils.Contains(proxyList, deployment.ObjectMeta.Name) {
					for _, proxyVar := range deployment.Spec.Template.Spec.Containers[0].Env {
						switch proxyVar.Name {
						case "HTTP_PROXY":
							containsHTTP = true
							if proxyVar.Value != "test1" {
								t.Fatalf("HTTP_PROXY not propagated")
							}
						case "HTTPS_PROXY":
							containsHTTPS = true
							if proxyVar.Value != "test2" {
								t.Fatalf("HTTPS_PROXY not propagated")
							}
						case "NO_PROXY":
							containsNO = true
							if proxyVar.Value != "test3" {
								t.Fatalf("NO_PROXY not propagated")
							}
						}
					}

					if !containsHTTP || !containsHTTPS || !containsNO {
						t.Fatalf("proxy variables not set")
					}
				}
				containsHTTP = false
				containsHTTPS = false
				containsNO = false
			}

		}
	}

	os.Unsetenv("HTTP_PROXY")
	os.Unsetenv("HTTPS_PROXY")
	os.Unsetenv("NO_PROXY")
	os.Unsetenv("POD_NAMESPACE")
	os.Unsetenv("DIRECTORY_OVERRIDE")
	os.Unsetenv("ACM_HUB_OCP_VERSION")

}

func TestRenderCRDs(t *testing.T) {
	// realCRDsDir is relative to this test file (pkg/rendering/), pointing at
	// the shipped CRD templates directory.
	realCRDsDir := "../templates/crds"

	mchNamespace := "default"
	testMCH := &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testmch",
			Namespace: mchNamespace,
		},
		Spec: v1.MultiClusterHubSpec{},
	}

	t.Run("renders CRDs from real directory with mch", func(t *testing.T) {
		got, errs := RenderCRDs(realCRDsDir, testMCH)
		if len(errs) > 0 {
			t.Errorf("RenderCRDs() unexpected errors: %v", errs)
		}
		if len(got) == 0 {
			t.Error("RenderCRDs() returned no CRDs, expected at least one")
		}
		for _, u := range got {
			if u.GetKind() != "CustomResourceDefinition" {
				t.Errorf("RenderCRDs() got Kind = %v, want CustomResourceDefinition", u.GetKind())
			}
			if u.GetAPIVersion() != "apiextensions.k8s.io/v1" {
				t.Errorf("RenderCRDs() got APIVersion = %v, want apiextensions.k8s.io/v1", u.GetAPIVersion())
			}
		}
	})

	t.Run("renders CRDs from real directory with nil mch", func(t *testing.T) {
		got, errs := RenderCRDs(realCRDsDir, nil)
		if len(errs) > 0 {
			t.Errorf("RenderCRDs() unexpected errors: %v", errs)
		}
		if len(got) == 0 {
			t.Error("RenderCRDs() returned no CRDs, expected at least one")
		}
	})

	t.Run("returns error for non-existent directory", func(t *testing.T) {
		_, errs := RenderCRDs("/does/not/exist", testMCH)
		if len(errs) == 0 {
			t.Error("RenderCRDs() expected error for non-existent directory, got none")
		}
	})

	t.Run("renders CRDs from temp directory with invalid yaml", func(t *testing.T) {
		dir := t.TempDir()
		// Write a non-YAML file — should be skipped (not .yaml extension)
		if err := os.WriteFile(dir+"/not-yaml.txt", []byte("not yaml"), 0600); err != nil {
			t.Fatal(err)
		}
		// Write an invalid YAML file — should produce an unmarshal error
		if err := os.WriteFile(dir+"/bad.yaml", []byte("{{invalid yaml{{"), 0600); err != nil {
			t.Fatal(err)
		}
		_, errs := RenderCRDs(dir, testMCH)
		if len(errs) == 0 {
			t.Error("RenderCRDs() expected unmarshal error for invalid yaml, got none")
		}
	})

	t.Run("renders CRDs from empty directory", func(t *testing.T) {
		dir := t.TempDir()
		got, errs := RenderCRDs(dir, testMCH)
		if len(errs) > 0 {
			t.Errorf("RenderCRDs() unexpected errors: %v", errs)
		}
		if len(got) != 0 {
			t.Errorf("RenderCRDs() expected 0 CRDs from empty dir, got %d", len(got))
		}
	})
}

func TestOADPAnnotation(t *testing.T) {
	oadp := `{"channel": "stable-1.0", "installPlanApproval": "Manual", "name": "redhat-oadp-operator2", "source": "redhat-operators2", "sourceNamespace": "openshift-marketplace2", "startingCSV": "test-csv"}`
	mch := &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Annotations: map[string]string{
				"installer.open-cluster-management.io/oadp-subscription-spec": oadp,
			},
		},
	}

	test1, test2, test3, test4, test5, test6 := GetOADPConfig(mch)

	if test1 != "redhat-oadp-operator2" {
		t.Error("Cluster Backup missing OADP overrides for name")
	}

	if test2 != "stable-1.0" {
		t.Error("Cluster Backup missing OADP overrides for channel")
	}

	if test3 != "Manual" {
		t.Error("Cluster Backup missing OADP overrides for install plan")
	}

	if test4 != "redhat-operators2" {
		t.Error("Cluster Backup missing OADP overrides for source")
	}

	if test5 != "openshift-marketplace2" {
		t.Error("Cluster Backup missing OADP overrides for source namespace")
	}

	if test6 != "test-csv" {
		t.Error("Cluster Backup missing startingCSV overrides for source")
	}

	mch = &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
		},
	}

	// These should all be the defaults (no overrides)
	// no ACM_HUB_OCP_VERSION set, should return stable channel
	test1, test2, test3, test4, test5, test6 = GetOADPConfig(mch)
	if test2 != defaultOADPStableChannel {
		t.Error("Cluster Backup missing OADP overrides for 1.4 channel on unknown version of ocp")
	}

	// fake the ocp version to 4.18.0, it should result in stable-1.4 channel
	os.Setenv("ACM_HUB_OCP_VERSION", "4.18.0")
	test1, test2, test3, test4, test5, test6 = GetOADPConfig(mch)

	if test1 != defaultOADPName {
		t.Error("Cluster Backup missing OADP overrides for name")
	}

	if test2 != defaultOADPChannel {
		t.Error("Cluster Backup missing OADP overrides for 1.4 channel on ocp 4.18")
	}

	if test3 != defaultOADPInstallPlan {
		t.Error("Cluster Backup missing OADP overrides for install plan")
	}

	if test4 != defaultOADPCatalogSource {
		t.Error("Cluster Backup missing OADP overrides for source")
	}

	if test5 != defaultOADPCatalogSourceNamespace {
		t.Error("Cluster Backup missing OADP overrides for source namespace")
	}

	if test6 != "" {
		t.Error("Cluster Backup Defaulted to something other than \"\"")
	}

	// fake the ocp version to 4.30.0, it should result in stable channel
	os.Setenv("ACM_HUB_OCP_VERSION", "4.30.0")
	test1, test2, test3, test4, test5, test6 = GetOADPConfig(mch)
	if test2 != defaultOADPStableChannel {
		t.Error("Cluster Backup missing OADP overrides for stable channel on ocp 4.30")
	}

	// fake the ocp version to something starting with anything other than 1.4, it should result in stable channel
	os.Setenv("ACM_HUB_OCP_VERSION", "5.1.0")
	test1, test2, test3, test4, test5, test6 = GetOADPConfig(mch)
	if test2 != defaultOADPStableChannel {
		t.Error("Cluster Backup missing OADP overrides for stable channel on ocp 5.1.0")
	}

	// Test OADP ClusterExtension overrides (OLM v1)
	oadpV1 := `{"channels": ["stable-1.5"], "version": ">=1.5.0", "source": "custom-catalog"}`
	mchV1 := &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Annotations: map[string]string{
				"installer.open-cluster-management.io/oadp-clusterextension-spec": oadpV1,
			},
		},
	}

	overrides := parseOADPClusterExtensionAnnotation(mchV1)
	if overrides == nil {
		t.Error("Expected OADP ClusterExtension overrides, got nil")
	} else {
		if len(overrides.Channels) != 1 || overrides.Channels[0] != "stable-1.5" {
			t.Errorf("Expected channels [stable-1.5], got %v", overrides.Channels)
		}
		if overrides.Version != ">=1.5.0" {
			t.Errorf("Expected version >=1.5.0, got %s", overrides.Version)
		}
		if overrides.Source != "custom-catalog" {
			t.Errorf("Expected source custom-catalog, got %s", overrides.Source)
		}
	}

	// Test with no annotation
	mchNoAnnotation := &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
		},
	}
	overridesNil := parseOADPClusterExtensionAnnotation(mchNoAnnotation)
	if overridesNil != nil {
		t.Error("Expected nil for no annotation, got overrides")
	}

	// Test that v1 annotation does NOT apply to v0 rendering
	os.Setenv("DIRECTORY_OVERRIDE", "../templates")
	os.Setenv("ACM_HUB_OCP_VERSION", "4.18.0")
	defer os.Unsetenv("DIRECTORY_OVERRIDE")
	defer os.Unsetenv("ACM_HUB_OCP_VERSION")

	mchBothAnnotations := &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testmch",
			Namespace: "default",
			Annotations: map[string]string{
				"installer.open-cluster-management.io/oadp-subscription-spec":     oadp,
				"installer.open-cluster-management.io/oadp-clusterextension-spec": oadpV1,
			},
		},
	}

	// Render with v0 - should use v0 annotation (stable-1.0), NOT v1 annotation (stable-1.5)
	templatesV0, errsV0 := RenderChart(utils.ClusterBackupChartLocation, mchBothAnnotations,
		map[string]string{"cluster_backup_controller": "quay.io/test:test"},
		map[string]string{}, false, "v0")
	if len(errsV0) > 0 {
		t.Fatalf("Failed to render v0: %v", errsV0)
	}

	// Find Subscription and verify channel is from v0 annotation
	foundV0Subscription := false
	for _, tmpl := range templatesV0 {
		if tmpl.GetKind() == "Subscription" && tmpl.GetAPIVersion() == "operators.coreos.com/v1alpha1" {
			spec, found, _ := unstructured.NestedMap(tmpl.Object, "spec")
			if !found {
				continue
			}
			channel, ok := spec["channel"].(string)
			if !ok {
				continue
			}
			if channel != "stable-1.0" {
				t.Errorf("v0 render should use v0 annotation channel (stable-1.0), got %s", channel)
			}
			foundV0Subscription = true
			break
		}
	}
	if !foundV0Subscription {
		t.Error("v0 render did not produce Subscription")
	}

}

func TestMTVAnnotation(t *testing.T) {
	mtv := `{"channel": "release-v2.11", "name": "mtv-operator2", "source": "redhat-operators", "sourceNamespace": "openshift-marketplace", "startingCSV": "mtv-operator.v0.0.1", "installPlanApproval": "Manual"}`
	mch := &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Annotations: map[string]string{
				"installer.open-cluster-management.io/mtv-subscription-spec": mtv,
			},
		},
	}

	config := GetMTVConfig(mch)
	if config == nil {
		t.Fatal("GetMTVConfig returned nil for a valid annotation, expected non-nil override")
	}

	if config.Subscription.Channel != "release-v2.11" {
		t.Errorf("MTV AddOnTemplate missing override for channel, got %s", config.Subscription.Channel)
	}

	if config.Subscription.Name != "mtv-operator2" {
		t.Errorf("MTV AddOnTemplate missing override for name, got %s", config.Subscription.Name)
	}

	if config.Subscription.Source != "redhat-operators" {
		t.Errorf("MTV AddOnTemplate missing override for source, got %s", config.Subscription.Source)
	}

	if config.Subscription.SourceNamespace != "openshift-marketplace" {
		t.Errorf("MTV AddOnTemplate missing override for sourceNamespace, got %s", config.Subscription.SourceNamespace)
	}

	if config.Subscription.StartingCSV != "mtv-operator.v0.0.1" {
		t.Errorf("MTV AddOnTemplate missing override for startingCSV, got %s", config.Subscription.StartingCSV)
	}

	// subscription.namespace is where the Subscription is created, not a
	// catalog field, and has no counterpart in the OLM v0 SubscriptionSpec, so
	// it is not annotation-driven.
	if config.Subscription.Namespace != defaultMTVSubscriptionNamespace {
		t.Errorf("MTV subscription namespace should stay %s, got %s", defaultMTVSubscriptionNamespace, config.Subscription.Namespace)
	}

	if config.UpgradeApproval != "Manual" {
		t.Errorf("MTV AddOnTemplate missing override for upgrade approval, got %s", config.UpgradeApproval)
	}

	// A partial override must fall back to the Go safety-net defaults for unset fields.
	mch = &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Annotations: map[string]string{
				"installer.open-cluster-management.io/mtv-subscription-spec": `{"channel": "release-v2.10"}`,
			},
		},
	}

	config = GetMTVConfig(mch)
	if config == nil {
		t.Fatal("GetMTVConfig returned nil for a partial annotation, expected non-nil override")
	}

	if config.Subscription.Channel != "release-v2.10" {
		t.Errorf("MTV AddOnTemplate partial override lost channel, got %s", config.Subscription.Channel)
	}
	if config.Subscription.Name != defaultMTVPackageName {
		t.Errorf("MTV AddOnTemplate partial override should keep default name, got %s", config.Subscription.Name)
	}
	if config.UpgradeApproval != defaultMTVUpgradeApproval {
		t.Errorf("MTV AddOnTemplate partial override should keep default upgrade approval, got %s", config.UpgradeApproval)
	}

	// An unset catalog field stays empty so the policy controller inherits the
	// default Subscription value instead of being pinned to nothing.
	if config.Subscription.Source != "" || config.Subscription.SourceNamespace != "" || config.Subscription.StartingCSV != "" {
		t.Errorf("MTV unset catalog fields should stay empty, got source=%q sourceNamespace=%q startingCSV=%q",
			config.Subscription.Source, config.Subscription.SourceNamespace, config.Subscription.StartingCSV)
	}

	// No annotation at all must return nil — values.yaml defaults apply via CoalesceValues.
	mch = &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
		},
	}

	if config := GetMTVConfig(mch); config != nil {
		t.Errorf("GetMTVConfig with no annotation should return nil so values.yaml defaults apply, got %+v", config)
	}

	// Malformed JSON must not panic and must fall back to the Go safety-net defaults.
	mch = &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Annotations: map[string]string{
				"installer.open-cluster-management.io/mtv-subscription-spec": "not-json",
			},
		},
	}

	config = GetMTVConfig(mch)
	if config == nil {
		t.Fatal("GetMTVConfig returned nil for a malformed annotation, expected non-nil with defaults")
	}
	if config.Subscription.Channel != defaultMTVChannel {
		t.Errorf("MTV AddOnTemplate malformed annotation should keep default channel, got %s", config.Subscription.Channel)
	}
}

func TestCNVAnnotation(t *testing.T) {
	cnv := `{"channel": "stable-1.16", "name": "kubevirt-hyperconverged2", "source": "redhat-operators", "sourceNamespace": "openshift-marketplace", "startingCSV": "kubevirt-hyperconverged.v1.16.0", "installPlanApproval": "Manual"}`
	mch := &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Annotations: map[string]string{
				"installer.open-cluster-management.io/cnv-subscription-spec": cnv,
			},
		},
	}

	config := GetCNVConfig(mch)
	if config == nil {
		t.Fatal("GetCNVConfig returned nil for a valid annotation, expected non-nil override")
	}

	if config.Subscription.Channel != "stable-1.16" {
		t.Errorf("CNV AddOnTemplate missing override for channel, got %s", config.Subscription.Channel)
	}

	if config.Subscription.Name != "kubevirt-hyperconverged2" {
		t.Errorf("CNV AddOnTemplate missing override for name, got %s", config.Subscription.Name)
	}

	if config.Subscription.Source != "redhat-operators" {
		t.Errorf("CNV AddOnTemplate missing override for source, got %s", config.Subscription.Source)
	}

	if config.Subscription.SourceNamespace != "openshift-marketplace" {
		t.Errorf("CNV AddOnTemplate missing override for sourceNamespace, got %s", config.Subscription.SourceNamespace)
	}

	if config.Subscription.StartingCSV != "kubevirt-hyperconverged.v1.16.0" {
		t.Errorf("CNV AddOnTemplate missing override for startingCSV, got %s", config.Subscription.StartingCSV)
	}

	// subscription.namespace is where the Subscription is created, not a
	// catalog field, and has no counterpart in the OLM v0 SubscriptionSpec, so
	// it is not annotation-driven.
	if config.Subscription.Namespace != defaultCNVSubscriptionNamespace {
		t.Errorf("CNV subscription namespace should stay %s, got %s", defaultCNVSubscriptionNamespace, config.Subscription.Namespace)
	}

	if config.UpgradeApproval != "Manual" {
		t.Errorf("CNV AddOnTemplate missing override for upgrade approval, got %s", config.UpgradeApproval)
	}

	// A partial override must fall back to the Go safety-net defaults for unset fields.
	mch = &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Annotations: map[string]string{
				"installer.open-cluster-management.io/cnv-subscription-spec": `{"channel": "stable-1.15"}`,
			},
		},
	}

	config = GetCNVConfig(mch)
	if config == nil {
		t.Fatal("GetCNVConfig returned nil for a partial annotation, expected non-nil override")
	}

	if config.Subscription.Channel != "stable-1.15" {
		t.Errorf("CNV AddOnTemplate partial override lost channel, got %s", config.Subscription.Channel)
	}
	if config.Subscription.Name != defaultCNVPackageName {
		t.Errorf("CNV AddOnTemplate partial override should keep default name, got %s", config.Subscription.Name)
	}
	if config.UpgradeApproval != defaultCNVUpgradeApproval {
		t.Errorf("CNV AddOnTemplate partial override should keep default upgrade approval, got %s", config.UpgradeApproval)
	}

	// An unset catalog field stays empty so the policy controller inherits the
	// default Subscription value instead of being pinned to nothing.
	if config.Subscription.Source != "" || config.Subscription.SourceNamespace != "" || config.Subscription.StartingCSV != "" {
		t.Errorf("CNV unset catalog fields should stay empty, got source=%q sourceNamespace=%q startingCSV=%q",
			config.Subscription.Source, config.Subscription.SourceNamespace, config.Subscription.StartingCSV)
	}

	// No annotation at all must return nil — values.yaml defaults apply via CoalesceValues.
	mch = &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
		},
	}

	if config := GetCNVConfig(mch); config != nil {
		t.Errorf("GetCNVConfig with no annotation should return nil so values.yaml defaults apply, got %+v", config)
	}

	// The two annotations must stay independent of each other.
	mch = &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Annotations: map[string]string{
				"installer.open-cluster-management.io/cnv-subscription-spec": `{"channel": "stable-1.15"}`,
				"installer.open-cluster-management.io/mtv-subscription-spec": `{"channel": "release-v2.10"}`,
			},
		},
	}

	if config := GetCNVConfig(mch); config == nil || config.Subscription.Channel != "stable-1.15" {
		ch := ""
		if config != nil {
			ch = config.Subscription.Channel
		}
		t.Errorf("CNV config was affected by the MTV annotation, got channel %q", ch)
	}
	if config := GetMTVConfig(mch); config == nil || config.Subscription.Channel != "release-v2.10" {
		ch := ""
		if config != nil {
			ch = config.Subscription.Channel
		}
		t.Errorf("MTV config was affected by the CNV annotation, got channel %q", ch)
	}

	// Malformed JSON must not panic and must fall back to the Go safety-net defaults.
	mch = &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Annotations: map[string]string{
				"installer.open-cluster-management.io/cnv-subscription-spec": "not-json",
			},
		},
	}

	config = GetCNVConfig(mch)
	if config == nil {
		t.Fatal("GetCNVConfig returned nil for a malformed annotation, expected non-nil with defaults")
	}
	if config.Subscription.Channel != defaultCNVChannel {
		t.Errorf("CNV AddOnTemplate malformed annotation should keep default channel, got %s", config.Subscription.Channel)
	}
}

// TestMTVValuesYamlDefaults verifies that when no mtv-subscription-spec or
// cnv-subscription-spec annotation is set, the rendered mtv-integrations chart
// uses the channel and other subscription fields declared in the chart's
// values.yaml — not any hardcoded Go constant. This is the core behaviour
// guaranteed by the CoalesceValues integration: values.yaml is the single
// source of truth for shipped defaults.
func TestMTVValuesYamlDefaults(t *testing.T) {
	os.Setenv("DIRECTORY_OVERRIDE", "../templates")
	defer os.Unsetenv("DIRECTORY_OVERRIDE")
	os.Setenv("ACM_HUB_OCP_VERSION", "4.18.0")
	defer os.Unsetenv("ACM_HUB_OCP_VERSION")

	mch := &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testmch",
			Namespace: "default",
			// No mtv-subscription-spec or cnv-subscription-spec annotation —
			// values.yaml must supply all subscription defaults.
		},
	}

	testImages := map[string]string{}
	for _, v := range utils.GetTestImages() {
		testImages[v] = "quay.io/test/test:Test"
	}

	templates, errs := RenderChart(utils.MTVIntegrationsChartLocation, mch,
		testImages, map[string]string{}, false, "v0")
	if len(errs) > 0 {
		t.Fatalf("Failed to render mtv-integrations with no annotation: %v", errs)
	}

	cases := []struct {
		addonTemplate string
		policy        string
		wantChannel   string
		wantName      string
		wantNamespace string
		wantUpgrade   string
	}{
		{
			// values.yaml: global.mtvOperator.subscription.channel = release-v5.0
			addonTemplate: "mtv-operator",
			policy:        "mtv-operator",
			wantChannel:   "release-v5.0",
			wantName:      "mtv-operator",
			wantNamespace: "openshift-mtv",
			wantUpgrade:   "Automatic",
		},
		{
			// values.yaml: global.kubevirtHyperconvergedOperator.subscription.channel = stable
			addonTemplate: "kubevirt-hyperconverged",
			policy:        "kubevirt-hyperconverged-operator",
			wantChannel:   "stable",
			wantName:      "kubevirt-hyperconverged",
			wantNamespace: "openshift-cnv",
			wantUpgrade:   "Automatic",
		},
	}

	for _, tc := range cases {
		t.Run(tc.policy, func(t *testing.T) {
			policy := findWorkloadManifest(t, templates, "AddOnTemplate", tc.addonTemplate, "OperatorPolicy", tc.policy)
			if policy == nil {
				t.Errorf("no %s OperatorPolicy in the %s AddOnTemplate workload", tc.policy, tc.addonTemplate)
				return
			}

			sub, ok, err := unstructured.NestedMap(policy.Object, "spec", "subscription")
			if err != nil || !ok {
				t.Fatalf("no spec.subscription in %s OperatorPolicy: %v", tc.policy, err)
			}

			got := map[string]string{
				"channel":   stringOf(sub["channel"]),
				"name":      stringOf(sub["name"]),
				"namespace": stringOf(sub["namespace"]),
			}
			want := map[string]string{
				"channel":   tc.wantChannel,
				"name":      tc.wantName,
				"namespace": tc.wantNamespace,
			}
			for k, w := range want {
				if got[k] != w {
					t.Errorf("%s subscription.%s = %q, want %q (from values.yaml)", tc.policy, k, got[k], w)
				}
			}

			if u := stringOf(mustNested(t, policy.Object, "spec", "upgradeApproval")); u != tc.wantUpgrade {
				t.Errorf("%s upgradeApproval = %q, want %q (from values.yaml)", tc.policy, u, tc.wantUpgrade)
			}
		})
	}
}

func TestRenderChartOLMv1(t *testing.T) {
	os.Setenv("DIRECTORY_OVERRIDE", "../templates")
	os.Setenv("ACM_HUB_OCP_VERSION", "5.0.0")
	defer os.Unsetenv("DIRECTORY_OVERRIDE")
	defer os.Unsetenv("ACM_HUB_OCP_VERSION")

	testMCH := &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testmch",
			Namespace: "default",
		},
		Spec: v1.MultiClusterHubSpec{},
	}
	testImages := map[string]string{
		"cluster_backup_controller": "quay.io/test/cluster-backup:test",
	}
	templateOverrides := map[string]string{}

	// Render cluster-backup chart with OLM v1 — OADP should still use v0 Subscription
	// because OADP bundles don't yet support OLM v1
	chartPath := utils.ClusterBackupChartLocation
	templates, errs := RenderChart(chartPath, testMCH, testImages, templateOverrides, false, "v1")
	if len(errs) > 0 {
		for _, err := range errs {
			t.Log(err.Error())
		}
		t.Fatalf("failed to render cluster-backup with OLM v1")
	}

	// OADP forced to v0: expect Subscription and OperatorGroup, not ClusterExtension
	foundClusterExtension := false
	foundSubscription := false
	foundOperatorGroup := false

	for _, template := range templates {
		kind := template.GetKind()
		apiVersion := template.GetAPIVersion()

		if kind == "ClusterExtension" && apiVersion == "olm.operatorframework.io/v1" {
			foundClusterExtension = true
		}
		if kind == "Subscription" && apiVersion == "operators.coreos.com/v1alpha1" {
			foundSubscription = true
		}
		if kind == "OperatorGroup" {
			foundOperatorGroup = true
		}
	}

	// v0 resources should be present (OADP forced to v0)
	if !foundSubscription {
		t.Error("Expected Subscription for OADP (forced v0), not found")
	}
	if !foundOperatorGroup {
		t.Error("Expected OperatorGroup for OADP (forced v0), not found")
	}

	// v1 resources should be absent
	if foundClusterExtension {
		t.Error("Found ClusterExtension in render, OADP should use v0 Subscription")
	}
}

func TestParseProbeConfigFromAnnotations(t *testing.T) {
	t.Run("No annotations returns nil", func(t *testing.T) {
		mch := &v1.MultiClusterHub{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-mch",
				Namespace: "default",
			},
		}

		result := parseProbeConfigFromAnnotations(mch)
		if result != nil {
			t.Error("Expected nil when no annotations present")
		}
	})

	t.Run("All three annotations are parsed correctly", func(t *testing.T) {
		mch := &v1.MultiClusterHub{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-mch",
				Namespace: "default",
				Annotations: map[string]string{
					utils.AnnotationProbeTimeoutSeconds:   "10",
					utils.AnnotationProbeFailureThreshold: "5",
					utils.AnnotationProbeSuccessThreshold: "2",
				},
			},
		}

		result := parseProbeConfigFromAnnotations(mch)
		if result == nil {
			t.Fatal("Expected ProbeConfig, got nil")
		}

		if result.TimeoutSeconds == nil || *result.TimeoutSeconds != 10 {
			t.Errorf("Expected TimeoutSeconds=10, got %v", result.TimeoutSeconds)
		}
		if result.FailureThreshold == nil || *result.FailureThreshold != 5 {
			t.Errorf("Expected FailureThreshold=5, got %v", result.FailureThreshold)
		}
		if result.SuccessThreshold == nil || *result.SuccessThreshold != 2 {
			t.Errorf("Expected SuccessThreshold=2, got %v", result.SuccessThreshold)
		}
	})

	t.Run("Partial annotations are parsed correctly", func(t *testing.T) {
		mch := &v1.MultiClusterHub{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-mch",
				Namespace: "default",
				Annotations: map[string]string{
					utils.AnnotationProbeTimeoutSeconds: "15",
				},
			},
		}

		result := parseProbeConfigFromAnnotations(mch)
		if result == nil {
			t.Fatal("Expected ProbeConfig, got nil")
		}

		if result.TimeoutSeconds == nil || *result.TimeoutSeconds != 15 {
			t.Errorf("Expected TimeoutSeconds=15, got %v", result.TimeoutSeconds)
		}
		if result.FailureThreshold != nil {
			t.Errorf("Expected FailureThreshold=nil, got %v", *result.FailureThreshold)
		}
		if result.SuccessThreshold != nil {
			t.Errorf("Expected SuccessThreshold=nil, got %v", *result.SuccessThreshold)
		}
	})

	t.Run("Invalid values are ignored", func(t *testing.T) {
		mch := &v1.MultiClusterHub{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-mch",
				Namespace: "default",
				Annotations: map[string]string{
					utils.AnnotationProbeTimeoutSeconds:   "not-a-number",
					utils.AnnotationProbeFailureThreshold: "10",
				},
			},
		}

		result := parseProbeConfigFromAnnotations(mch)
		if result == nil {
			t.Fatal("Expected ProbeConfig, got nil")
		}

		if result.TimeoutSeconds != nil {
			t.Errorf("Expected TimeoutSeconds=nil (invalid value), got %v", *result.TimeoutSeconds)
		}
		if result.FailureThreshold == nil || *result.FailureThreshold != 10 {
			t.Errorf("Expected FailureThreshold=10, got %v", result.FailureThreshold)
		}
	})

	t.Run("Zero and negative values are ignored", func(t *testing.T) {
		mch := &v1.MultiClusterHub{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-mch",
				Namespace: "default",
				Annotations: map[string]string{
					utils.AnnotationProbeTimeoutSeconds:   "0",
					utils.AnnotationProbeFailureThreshold: "-5",
					utils.AnnotationProbeSuccessThreshold: "3",
				},
			},
		}

		result := parseProbeConfigFromAnnotations(mch)
		if result == nil {
			t.Fatal("Expected ProbeConfig, got nil")
		}

		if result.TimeoutSeconds != nil {
			t.Errorf("Expected TimeoutSeconds=nil (zero value), got %v", *result.TimeoutSeconds)
		}
		if result.FailureThreshold != nil {
			t.Errorf("Expected FailureThreshold=nil (negative value), got %v", *result.FailureThreshold)
		}
		if result.SuccessThreshold == nil || *result.SuccessThreshold != 3 {
			t.Errorf("Expected SuccessThreshold=3, got %v", result.SuccessThreshold)
		}
	})

	t.Run("Other annotations don't interfere", func(t *testing.T) {
		mch := &v1.MultiClusterHub{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-mch",
				Namespace: "default",
				Annotations: map[string]string{
					"installer.open-cluster-management.io/pause": "true",
					utils.AnnotationProbeTimeoutSeconds:          "20",
					"some-other-annotation":                      "value",
				},
			},
		}

		result := parseProbeConfigFromAnnotations(mch)
		if result == nil {
			t.Fatal("Expected ProbeConfig, got nil")
		}

		if result.TimeoutSeconds == nil || *result.TimeoutSeconds != 20 {
			t.Errorf("Expected TimeoutSeconds=20, got %v", result.TimeoutSeconds)
		}
	})
}

func TestOperatorPolicyAnnotationRendered(t *testing.T) {
	os.Setenv("DIRECTORY_OVERRIDE", "../templates")
	defer os.Unsetenv("DIRECTORY_OVERRIDE")
	os.Setenv("ACM_HUB_OCP_VERSION", "4.18.0")
	defer os.Unsetenv("ACM_HUB_OCP_VERSION")

	// TestRender only proves every referenced value key exists. These cases
	// assert that each annotation field is bound to the manifest field it is
	// named after, which is the class of bug a missing-key render error cannot
	// catch.
	mch := &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testmch",
			Namespace: "default",
			Annotations: map[string]string{
				"installer.open-cluster-management.io/mtv-subscription-spec": `{"channel":"release-v2.11","name":"mtv-operator","source":"redhat-operators","sourceNamespace":"openshift-marketplace","startingCSV":"mtv-operator.v2.11.0","installPlanApproval":"Manual"}`,
				"installer.open-cluster-management.io/cnv-subscription-spec": `{"channel":"stable-1.16","name":"kubevirt-hyperconverged","source":"redhat-operators","sourceNamespace":"openshift-marketplace","startingCSV":"kubevirt-hyperconverged.v1.16.0","installPlanApproval":"Manual"}`,
			},
		},
	}

	testImages := map[string]string{}
	for _, v := range utils.GetTestImages() {
		testImages[v] = "quay.io/test/test:Test"
	}

	templates, errs := RenderChart(utils.MTVIntegrationsChartLocation, mch,
		testImages, map[string]string{}, false, "v0")
	if len(errs) > 0 {
		t.Fatalf("Failed to render mtv-integrations: %v", errs)
	}

	cases := []struct {
		addonTemplate   string
		policy          string
		wantChannel     string
		wantName        string
		wantNamespace   string
		wantSource      string
		wantSourceNS    string
		wantStartingCSV string
		wantUpgrade     string
	}{
		{
			addonTemplate:   "mtv-operator",
			policy:          "mtv-operator",
			wantChannel:     "release-v2.11",
			wantName:        "mtv-operator",
			wantNamespace:   defaultMTVSubscriptionNamespace,
			wantSource:      "redhat-operators",
			wantSourceNS:    "openshift-marketplace",
			wantStartingCSV: "mtv-operator.v2.11.0",
			wantUpgrade:     "Manual",
		},
		{
			addonTemplate:   "kubevirt-hyperconverged",
			policy:          "kubevirt-hyperconverged-operator",
			wantChannel:     "stable-1.16",
			wantName:        "kubevirt-hyperconverged",
			wantNamespace:   defaultCNVSubscriptionNamespace,
			wantSource:      "redhat-operators",
			wantSourceNS:    "openshift-marketplace",
			wantStartingCSV: "kubevirt-hyperconverged.v1.16.0",
			wantUpgrade:     "Manual",
		},
	}

	for _, tc := range cases {
		t.Run(tc.policy, func(t *testing.T) {
			// The OperatorPolicy is carried as a manifest inside the
			// AddOnTemplate workload, not rendered as a top-level object.
			policy := findWorkloadManifest(t, templates, "AddOnTemplate", tc.addonTemplate, "OperatorPolicy", tc.policy)
			if policy == nil {
				t.Errorf("no %s OperatorPolicy in the %s AddOnTemplate workload", tc.policy, tc.addonTemplate)
				return
			}
			{
				tmpl := policy
				sub, ok, err := unstructured.NestedMap(tmpl.Object, "spec", "subscription")
				if err != nil || !ok {
					t.Fatalf("no spec.subscription in %s OperatorPolicy: %v", tc.policy, err)
				}
				got := map[string]string{
					"channel":         stringOf(sub["channel"]),
					"name":            stringOf(sub["name"]),
					"namespace":       stringOf(sub["namespace"]),
					"source":          stringOf(sub["source"]),
					"sourceNamespace": stringOf(sub["sourceNamespace"]),
					"startingCSV":     stringOf(sub["startingCSV"]),
				}
				want := map[string]string{
					"channel":         tc.wantChannel,
					"name":            tc.wantName,
					"namespace":       tc.wantNamespace,
					"source":          tc.wantSource,
					"sourceNamespace": tc.wantSourceNS,
					"startingCSV":     tc.wantStartingCSV,
				}
				for k, w := range want {
					if got[k] != w {
						t.Errorf("%s subscription.%s = %q, want %q", tc.policy, k, got[k], w)
					}
				}

				// The subscription namespace must keep matching the
				// operatorGroup namespace, and must not be reachable from the
				// catalog sourceNamespace field.
				ogNS, _, _ := unstructured.NestedString(tmpl.Object, "spec", "operatorGroup", "namespace")
				if got["namespace"] != ogNS {
					t.Errorf("%s subscription.namespace %q does not match operatorGroup namespace %q",
						tc.policy, got["namespace"], ogNS)
				}

				if u := stringOf(mustNested(t, tmpl.Object, "spec", "upgradeApproval")); u != tc.wantUpgrade {
					t.Errorf("%s upgradeApproval = %q, want %q", tc.policy, u, tc.wantUpgrade)
				}
			}
		})
	}
}

// findWorkloadManifest returns the manifest of kind manifestKind named
// manifestName nested in the workload of the addonTemplateKind named
// addonTemplateName, or nil when it is absent.
func findWorkloadManifest(t *testing.T, templates []*unstructured.Unstructured,
	addonTemplateKind, addonTemplateName, manifestKind, manifestName string) *unstructured.Unstructured {
	t.Helper()

	for _, tmpl := range templates {
		if tmpl.GetKind() != addonTemplateKind || tmpl.GetName() != addonTemplateName {
			continue
		}
		manifests, found, err := unstructured.NestedSlice(tmpl.Object, "spec", "agentSpec", "workload", "manifests")
		if err != nil || !found {
			continue
		}
		for _, m := range manifests {
			manifest, ok := m.(map[string]interface{})
			if !ok {
				continue
			}
			u := &unstructured.Unstructured{Object: manifest}
			if u.GetKind() == manifestKind && u.GetName() == manifestName {
				return u
			}
		}
	}
	return nil
}

func mustNested(t *testing.T, obj map[string]interface{}, fields ...string) interface{} {
	t.Helper()
	v, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if err != nil || !found {
		t.Fatalf("field %v not found: %v", fields, err)
	}
	return v
}

func stringOf(v interface{}) string {
	s, _ := v.(string)
	return s
}
