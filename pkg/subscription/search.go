// Copyright (c) 2020 Red Hat, Inc.

package subscription

import (
	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Search overrides the search-prod chart
func Search(m *operatorsv1.MultiClusterHub, overrides map[string]string) *unstructured.Unstructured {
	sub := &Subscription{
		Name:      "search-prod",
		Namespace: m.Namespace,
		Overrides: map[string]interface{}{
			"global": map[string]interface{}{
				"pullSecret":     m.Spec.ImagePullSecret,
				"imageOverrides": overrides,
				"pullPolicy":     utils.GetImagePullPolicy(m),
			},
			"hubconfig": map[string]interface{}{
				"replicaCount": utils.DefaultReplicaCount(m),
				"nodeSelector": m.Spec.NodeSelector,
			},
		},
	}
	if m.Spec.CustomCAConfigmap != "" {
		sub.Overrides["hubconfig"].(map[string]interface{})["customCAConfigmap"] = m.Spec.CustomCAConfigmap;
	}

	return newSubscription(m, sub)
}
