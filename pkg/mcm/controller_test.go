// Copyright (c) 2020 Red Hat, Inc.

package mcm

import (
	"testing"

	operatorsv11 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestControllerDeployment(t *testing.T) {
	empty := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv1.MultiClusterHubSpec{
			ImagePullSecret: "",
			Mongo:           operatorsv11.Mongo{},
		},
	}
	ovr := map[string]string{}

	t.Run("MCH with empty fields", func(t *testing.T) {
		_ = ControllerDeployment(empty, ovr)
	})

	essentialsOnly := &operatorsv1.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec:       operatorsv1.MultiClusterHubSpec{},
	}
	t.Run("MCH with only required values", func(t *testing.T) {
		_ = ControllerDeployment(essentialsOnly, ovr)
	})
}
