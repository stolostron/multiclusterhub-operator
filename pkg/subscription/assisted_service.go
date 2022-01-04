// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package subscription

import (
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/pkg/apis/operator/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// AssistedService overrides the AssistedService chart
func AssistedService(m *operatorsv1.MultiClusterHub, overrides map[string]string) *unstructured.Unstructured {
	sub := &Subscription{
		Name:      "assisted-service",
		Namespace: m.Namespace,
		Overrides: map[string]interface{}{
			"hubconfig": map[string]interface{}{
				"replicaCount": utils.DefaultReplicaCount(m),
				"nodeSelector": m.Spec.NodeSelector,
				"name":         m.Name,
				"namespace":    m.Namespace,
			},
			"global": map[string]interface{}{
				"imageOverrides": overrides,
				"pullPolicy":     utils.GetImagePullPolicy(m),
				"pullSecret":     m.Spec.ImagePullSecret,
			},
		},
	}
	setCustomCA(m, sub)

	return newSubscription(m, sub)
}
