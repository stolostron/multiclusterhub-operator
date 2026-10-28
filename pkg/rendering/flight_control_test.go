package renderer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	imagev1 "github.com/openshift/api/image/v1"
	operatorsv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	v1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/overrides"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

func TestFlightControlRedisBundleContract(t *testing.T) {
	readBundleYAML := func(name string, target interface{}) {
		t.Helper()
		data, err := os.ReadFile(filepath.Join("..", "..", "bundle", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := yaml.Unmarshal(data, target); err != nil {
			t.Fatal(err)
		}
	}

	csv := &operatorsv1alpha1.ClusterServiceVersion{}
	readBundleYAML(filepath.Join("manifests", "advanced-cluster-management.clusterserviceversion.yaml"), csv)
	for _, env := range os.Environ() {
		name, _, _ := strings.Cut(env, "=")
		if strings.HasPrefix(name, overrides.OperandImagePrefix) {
			t.Setenv(name, "")
		}
	}
	for _, deployment := range csv.Spec.InstallStrategy.StrategySpec.DeploymentSpecs {
		for _, container := range deployment.Spec.Template.Spec.Containers {
			for _, env := range container.Env {
				if env.Name == "OPERAND_IMAGE_REDIS_7_C9S" {
					t.Fatal("ACM 2.13 CSV must use OPERAND_IMAGE_REDIS, not OPERAND_IMAGE_REDIS_7_C9S")
				}
				if strings.HasPrefix(env.Name, overrides.OperandImagePrefix) {
					t.Setenv(env.Name, env.Value)
				}
			}
		}
	}
	images := overrides.GetOverridesFromEnv(overrides.OperandImagePrefix)
	const redisPlaceholder = "image-registry.openshift-image-registry.svc:5000/open-cluster-management/redis:latest"
	if images["redis"] != redisPlaceholder {
		t.Fatalf("CSV Redis image = %q, want %q", images["redis"], redisPlaceholder)
	}
	if _, exists := images["redis_7_c9s"]; exists {
		t.Fatal("ACM 2.13 must render without a redis_7_c9s alias")
	}

	relatedImageFound := false
	for _, image := range csv.Spec.RelatedImages {
		if image.Name == "redis_7_c9s" {
			t.Fatal("ACM 2.13 related image must be named redis")
		}
		if image.Name == "redis" {
			relatedImageFound = true
			if image.Image != redisPlaceholder {
				t.Fatalf("related Redis image = %q, want %q", image.Image, redisPlaceholder)
			}
		}
	}
	if !relatedImageFound {
		t.Fatal("CSV related image redis not found")
	}

	imageReferences := &imagev1.ImageStream{}
	readBundleYAML("image-references", imageReferences)
	artConfig := struct {
		ExternalImages []struct {
			Name   string `json:"name"`
			Search string `json:"search"`
		} `json:"external-images"`
	}{}
	readBundleYAML("art.yaml", &artConfig)
	references := map[string]map[string]string{"image-references": {}, "art.yaml": {}}
	for _, tag := range imageReferences.Spec.Tags {
		if tag.From != nil {
			references["image-references"][tag.Name] = tag.From.Name
		}
	}
	for _, image := range artConfig.ExternalImages {
		references["art.yaml"][image.Name] = image.Search
	}
	for name, images := range references {
		if _, exists := images["redis_7_c9s"]; exists {
			t.Fatalf("%s must name the Redis image redis, not redis_7_c9s", name)
		}
		if images["redis"] != redisPlaceholder {
			t.Fatalf("%s Redis placeholder = %q, want %q", name, images["redis"], redisPlaceholder)
		}
	}

	t.Setenv("DIRECTORY_OVERRIDE", "../templates")
	t.Setenv("ACM_HUB_OCP_VERSION", "4.18.0")
	mch := &v1.MultiClusterHub{ObjectMeta: metav1.ObjectMeta{Name: "testmch", Namespace: "default"}}
	templates, errs := RenderChart(utils.EdgeManagerChartLocation, mch, images, map[string]string{}, false)
	if len(errs) > 0 {
		t.Fatalf("render Flight Control with ACM 2.13 CSV image overrides: %v", errs)
	}
	for _, template := range templates {
		if template.GetKind() != "StatefulSet" || template.GetName() != "flightctl-kv" {
			continue
		}
		statefulSet := &appsv1.StatefulSet{}
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(template.Object, statefulSet); err != nil {
			t.Fatal(err)
		}
		for _, container := range statefulSet.Spec.Template.Spec.Containers {
			if container.Name == "flightctl-kv" {
				if container.Image != images["redis"] {
					t.Fatalf("Redis container image = %q, want %q", container.Image, images["redis"])
				}
				return
			}
		}
		t.Fatal("flightctl-kv container not found")
	}
	t.Fatal("flightctl-kv StatefulSet not found")
}
