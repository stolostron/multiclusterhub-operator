package mcm

import (
	"testing"

	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestControllerDeployment(t *testing.T) {
	replicas := int(1)
	empty := &operatorsv1beta1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1beta1.MultiClusterHubSpec{
			ImagePullPolicy: "",
			ImagePullSecret: "",
			Mongo:           operatorsv1beta1.Mongo{},
			ReplicaCount:    &replicas,
		},
	}

	cs := utils.CacheSpec{
		IngressDomain:  "testIngress",
		ImageOverrides: map[string]string{},
	}

	t.Run("MCH with empty fields", func(t *testing.T) {
		_ = ControllerDeployment(empty, cs)
	})

	essentialsOnly := &operatorsv1beta1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1beta1.MultiClusterHubSpec{
			ImagePullPolicy: "test",
			ReplicaCount:    &replicas,
		},
	}
	t.Run("MCH with only required values", func(t *testing.T) {
		_ = ControllerDeployment(essentialsOnly, cs)
	})
}
