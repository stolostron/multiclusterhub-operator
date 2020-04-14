package mcm

import (
	"testing"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestControllerDeployment(t *testing.T) {
	empty := &operatorsv1alpha1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1alpha1.MultiClusterHubSpec{
			Version:         "",
			ImageRepository: "",
			ImagePullPolicy: "",
			ImagePullSecret: "",
			ImageTagSuffix:  "",
			NodeSelector: &operatorsv1alpha1.NodeSelector{
				OS:                  "",
				CustomLabelSelector: "",
				CustomLabelValue:    "",
			},
			Mongo: operatorsv1alpha1.Mongo{},
		},
	}
	t.Run("MCH with empty fields", func(t *testing.T) {
		_ = ControllerDeployment(empty)
	})

	essentialsOnly := &operatorsv1alpha1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1alpha1.MultiClusterHubSpec{
			Version:         "test",
			ImageRepository: "test",
			ImagePullPolicy: "test",
			ImageTagSuffix:  "test",
		},
	}
	t.Run("MCH with only required values", func(t *testing.T) {
		_ = ControllerDeployment(essentialsOnly)
	})
}
