// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package renderer

import (

	// "reflect"
	"fmt"
	"os"
	"reflect"
	"testing"

	v1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	mchNodeSelector := map[string]string{"select": "test"}
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

	testImages := map[string]string{}
	for _, v := range utils.GetTestImages() {
		testImages[v] = "quay.io/test/test:Test"
	}
	// multiple charts
	chartsDir := chartsDir
	templates, errs := RenderCharts(chartsDir, testMCH, testImages)
	if len(errs) > 0 {
		for _, err := range errs {
			t.Logf(err.Error())
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
				t.Fatalf(err.Error())
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
		singleChartTemplates, errs := RenderChart(chartsPath, testMCH, singleChartTestImages)
		if len(errs) > 0 {
			for _, err := range errs {
				t.Logf(err.Error())
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
					t.Fatalf(err.Error())
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

}

func TestRenderCRDs(t *testing.T) {
	os.Setenv("DIRECTORY_OVERRIDE", "../templates")
	tests := []struct {
		name   string
		crdDir string
		want   []error
	}{
		{
			name:   "Render CRDs directory",
			crdDir: crdsDir,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, errs := RenderCRDs(tt.crdDir)
			if errs != nil && len(errs) > 1 {
				t.Errorf("RenderCRDs() got = %v, want %v", errs, nil)
			}

			for _, u := range got {
				kind := "CustomResourceDefinition"
				apiVersion := "apiextensions.k8s.io/v1"
				if u.GetKind() != kind {
					t.Errorf("RenderCRDs() got Kind = %v, want Kind %v", errs, kind)
				}

				if u.GetAPIVersion() != apiVersion {
					t.Errorf("RenderCRDs() got apiversion = %v, want apiversion %v", errs, apiVersion)
				}
			}
		})
	}

	os.Setenv("CRD_OVERRIDE", "pkg/doesnotexist")
	_, errs := RenderCRDs(crdsDir)
	if errs == nil {
		t.Fatalf("Should have received an error")
	}
	os.Unsetenv("CRD_OVERRIDE")

}

func TestOADPAnnotation(t *testing.T) {
	oadp := `{"channel": "stable-1.0", "installPlanApproval": "Manual", "name": "redhat-oadp-operator2", "source": "redhat-operators2", "sourceNamespace": "openshift-marketplace2"}`
	mch := &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
			Annotations: map[string]string{
				"installer.open-cluster-management.io/oadp-subscription-spec": oadp,
			},
		},
	}

	test1, test2, test3, test4, test5 := GetOADPConfig(mch)

	if test1 != "redhat-oadp-operator2" {
		t.Error(fmt.Sprintf("Cluster Backup missing OADP overrides for name"))
	}

	if test2 != "stable-1.0" {
		t.Error(fmt.Sprintf("Cluster Backup missing OADP overrides for channel"))
	}

	if test3 != "Manual" {
		t.Error(fmt.Sprintf("Cluster Backup missing OADP overrides for install plan"))
	}

	if test4 != "redhat-operators2" {
		t.Error(fmt.Sprintf("Cluster Backup missing OADP overrides for source"))
	}

	if test5 != "openshift-marketplace2" {
		t.Error(fmt.Sprintf("Cluster Backup missing OADP overrides for source namespace"))
	}

	mch = &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "test",
		},
	}

	test1, test2, test3, test4, test5 = GetOADPConfig(mch)

	if test1 != "redhat-oadp-operator" {
		t.Error(fmt.Sprintf("Cluster Backup missing OADP overrides for name"))
	}

	if test2 != "stable-1.3" {
		t.Error(fmt.Sprintf("Cluster Backup missing OADP overrides for channel"))
	}

	if test3 != "Automatic" {
		t.Error(fmt.Sprintf("Cluster Backup missing OADP overrides for install plan"))
	}

	if test4 != "redhat-operators" {
		t.Error(fmt.Sprintf("Cluster Backup missing OADP overrides for source"))
	}

	if test5 != "openshift-marketplace" {
		t.Error(fmt.Sprintf("Cluster Backup missing OADP overrides for source namespace"))
	}
}

// func testFailures(t *testing.T) {
// os.Setenv("CRD_OVERRIDE", "pkg/doesnotexist")
// _, errs := RenderCRDs(crdsDir)
// if errs == nil {
// 	t.Fatalf("Should have received an error")
// }
// os.Unsetenv("CRD_OVERRIDE")
// }
