package mcm

import (
	"testing"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestControllerDeployment(t *testing.T) {
	replicas := int(1)
	empty := &operatorsv1alpha1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1alpha1.MultiClusterHubSpec{
			ImageRepository: "",
			ImagePullPolicy: "",
			ImagePullSecret: "",
			ImageTagSuffix:  "",
			Mongo:           operatorsv1alpha1.Mongo{},
			ReplicaCount:    &replicas,
		},
	}

	cs := utils.CacheSpec{
		IngressDomain:   "testIngress",
		ImageShaDigests: map[string]string{},
	}

	t.Run("MCH with empty fields", func(t *testing.T) {
		_ = ControllerDeployment(empty, cs)
	})

	essentialsOnly := &operatorsv1alpha1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1alpha1.MultiClusterHubSpec{
			ImageRepository: "test",
			ImagePullPolicy: "test",
			ImageTagSuffix:  "test",
			ReplicaCount:    &replicas,
		},
	}
	t.Run("MCH with only required values", func(t *testing.T) {
		_ = ControllerDeployment(essentialsOnly, cs)
	})
}
