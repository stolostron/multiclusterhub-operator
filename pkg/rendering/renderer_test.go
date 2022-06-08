// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package renderer

import (
	"os"
	// "reflect"
	"testing"

	v1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	// appsv1 "k8s.io/api/apps/v1"
	// corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	// "k8s.io/apimachinery/pkg/runtime"
)

const (
	chartsDir  = "pkg/templates/charts/toggle"
	chartsPath = "pkg/templates/charts/toggle/insights"
	// crdsDir    = "pkg/templates/crds"
)

func TestRender(t *testing.T) {

	os.Setenv("DIRECTORY_OVERRIDE", "../../")
	defer os.Unsetenv("DIRECTORY_OVERRIDE")

	// availabilityList := []string{"clusterclaims-controller", "cluster-curator-controller", "managedcluster-import-controller-v2", "ocm-controller", "ocm-proxyserver", "ocm-webhook"}
	// backplaneNodeSelector := map[string]string{"select": "test"}
	backplaneImagePullSecret := "test"
	// backplaneNamespace := "default"
	// backplaneAvailability := backplane.HAHigh
	// backplaneTolerations := []corev1.Toleration{
	// 	{
	// 		Key:      "dedicated",
	// 		Operator: "Exists",
	// 		Effect:   "NoSchedule",
	// 	},
	// }
	testMCH := &v1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name: "testBackplane",
		},
		Spec: v1.MultiClusterHubSpec{
			// AvailabilityConfig: backplaneAvailability,
			// NodeSelector:       backplaneNodeSelector,
			ImagePullSecret: backplaneImagePullSecret,
			// Tolerations:        backplaneTolerations,
			// TargetNamespace:    backplaneNamespace,
		},
		// Status: backplane.MultiClusterEngineStatus{
		// 	Phase: "",
		// },
	}
	// containsHTTP := false
	// containsHTTPS := false
	// containsNO := false
	// os.Setenv("POD_NAMESPACE", "default")
	// os.Setenv("HTTP_PROXY", "test1")
	// os.Setenv("HTTPS_PROXY", "test2")
	// os.Setenv("NO_PROXY", "test3")

	testImages := map[string]string{}
	for _, v := range utils.GetTestImages() {
		testImages[v] = "quay.io/test/test:Test"
	}
	// multiple charts
	chartsDir := chartsPath
	templates, errs := RenderChart(chartsDir, testMCH, testImages)
	if len(errs) > 0 {
		for _, err := range errs {
			t.Logf(err.Error())
		}
		t.Fatalf("failed to retrieve templates")
		if len(templates) == 0 {
			t.Fatalf("Unable to render templates")
		}
	}
}
