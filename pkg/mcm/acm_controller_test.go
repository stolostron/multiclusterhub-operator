// Copyright (c) 2020 Red Hat, Inc.

package mcm

import (
	"testing"

	operatorsv11 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAcmControllerDeployment(t *testing.T) {
	empty := &operatorsv11.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec: operatorsv11.MultiClusterHubSpec{
			ImagePullSecret: "",
			Mongo:           operatorsv11.Mongo{},
		},
	}

	ovr := map[string]string{}

	t.Run("MCH with empty fields", func(t *testing.T) {
		_ = ACMControllerDeployment(empty, ovr)
	})

	essentialsOnly := &operatorsv11.MultiClusterHub{
		ObjectMeta: metav1.ObjectMeta{Namespace: "test"},
		Spec:       operatorsv11.MultiClusterHubSpec{},
	}
	t.Run("MCH with only required values", func(t *testing.T) {
		_ = ACMControllerDeployment(essentialsOnly, ovr)
	})
}
