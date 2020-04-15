package mcm

import (
	"testing"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAPIServerDeployment(t *testing.T) {
	replicas := int(1)
	empty := &operatorsv1alpha1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1alpha1.MultiClusterHubSpec{
			Version:         "",
			ImageRepository: "",
			ImagePullPolicy: "",
			ImagePullSecret: "",
			ImageTagSuffix:  "",
			Mongo:           operatorsv1alpha1.Mongo{},
			ReplicaCount:    &replicas, // Adding replicas here to avoid nil pointer dereference
		},
	}
	t.Run("MCH with empty fields", func(t *testing.T) {
		_ = APIServerDeployment(empty)
	})

	essentialsOnly := &operatorsv1alpha1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1alpha1.MultiClusterHubSpec{
			Version:         "test",
			ImageRepository: "test",
			ImagePullPolicy: "test",
			ImageTagSuffix:  "test",
			ReplicaCount:    &replicas, // Adding replicas here to avoid nil pointer dereference
		},
	}
	t.Run("MCH with only required values", func(t *testing.T) {
		_ = APIServerDeployment(essentialsOnly)
	})
}

func TestAPIServerService(t *testing.T) {
	mch := &operatorsv1alpha1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "testName",
			Namespace: "testNS",
		},
	}

	t.Run("Create service", func(t *testing.T) {
		s := APIServerService(mch)
		if ns := s.Namespace; ns != "testNS" {
			t.Errorf("expected namespace %s, got %s", "testNS", ns)
		}
		if ref := s.GetOwnerReferences(); ref[0].Name != "testName" {
			t.Errorf("expected ownerReference %s, got %s", "testName", ref[0].Name)
		}
	})
}
