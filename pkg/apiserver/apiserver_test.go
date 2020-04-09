package apiserver

import (
	"fmt"
	"testing"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestApiServer(t *testing.T) {
	mch := &operatorsv1alpha1.MultiClusterHub{
		TypeMeta:   metav1.TypeMeta{Kind: "MultiClusterHub"},
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1alpha1.MultiClusterHubSpec{
			Version:         "latest",
			ImageRepository: "quay.io/open-cluster-management",
			ImagePullPolicy: "Always",
			ImagePullSecret: "test",
			NodeSelector: &operatorsv1alpha1.NodeSelector{
				OS:                  "test",
				CustomLabelSelector: "test",
				CustomLabelValue:    "test",
			},
			Mongo: operatorsv1alpha1.Mongo{},
		},
	}

	dep := Deployment(mch)
	fmt.Println(dep.String())
}
