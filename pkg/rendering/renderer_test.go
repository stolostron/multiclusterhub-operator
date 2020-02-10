package rendering

import (
	"os"
	"path"
	"testing"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/rendering/templates"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestRender(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working dir %v", err)
	}
	templatesPath := path.Join(path.Dir(path.Dir(wd)), "templates")
	os.Setenv(templates.TemplatesPathEnvVar, templatesPath)
	defer os.Unsetenv(templates.TemplatesPathEnvVar)

	var replicas int32 = 1
	mchcr := &operatorsv1alpha1.MultiCloudHub{
		TypeMeta:   metav1.TypeMeta{Kind: "MultiCloudHub"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1alpha1.MultiCloudHubSpec{
			Version:         "latest",
			ImageRepository: "quay.io/rhibmcollab",
			ImagePullPolicy: "Always",
			ImagePullSecret: "test",
			NodeSelector: &operatorsv1alpha1.NodeSelector{
				OS:                  "test",
				CustomLabelSelector: "test",
				CustomLabelValue:    "test",
			},
			Foundation: operatorsv1alpha1.Foundation{
				Apiserver: operatorsv1alpha1.Apiserver{
					Replicas: &replicas,
					Configuration: map[string]string{
						"test": "test",
					},
				},
				Controller: operatorsv1alpha1.Controller{
					Replicas: &replicas,
					Configuration: map[string]string{
						"test": "test",
					},
				},
			},
			Etcd: operatorsv1alpha1.Etcd{Endpoints: "test"},
			Mongo: operatorsv1alpha1.Mongo{
				Endpoints:  "test",
				ReplicaSet: "test",
			},
		},
	}

	renderer := NewRenderer(mchcr)
	_, err = renderer.Render()
	if err != nil {
		t.Fatalf("failed to render multicloudhub %v", err)
	}
}
