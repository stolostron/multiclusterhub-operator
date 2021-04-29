// Copyright (c) 2020 Red Hat, Inc.

package foundation

import (
	operatorsv1 "github.com/open-cluster-management/multiclusterhub-operator/pkg/apis/operator/v1"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func ClusterManager(m *operatorsv1.MultiClusterHub, overrides map[string]string) *unstructured.Unstructured {

	cm := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "operator.open-cluster-management.io/v1",
			"kind":       "ClusterManager",
			"metadata": map[string]interface{}{
				"name": "cluster-manager",
			},
			"spec": map[string]interface{}{
				"registrationImagePullSpec": RegistrationImage(overrides),
			},
		},
	}

	utils.AddInstallerLabel(cm, m.GetName(), m.GetNamespace())

	return cm
}
