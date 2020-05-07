// Copyright (c) 2020 Red Hat, Inc.

package subscription

import (
	subalpha1 "github.com/open-cluster-management/multicloud-operators-subscription/pkg/apis/apps/v1"
	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
)

// KUIWebTerminal overrides the kui-web-terminal chart
func KUIWebTerminal(m *operatorsv1beta1.MultiClusterHub, cache utils.CacheSpec) *subalpha1.Subscription {
	sub := &Subscription{
		Name:      "kui-web-terminal",
		Namespace: m.Namespace,
		Overrides: map[string]interface{}{
			"pullSecret": m.Spec.ImagePullSecret,
			"proxy": map[string]interface{}{
				"clusterIP": "icp-management-ingress",
			},
			"hubconfig": map[string]interface{}{
				"replicaCount": utils.DefaultReplicaCount(m),
				"nodeSelector": m.Spec.NodeSelector,
			},
			"global": map[string]interface{}{
				"imageOverrides": cache.ImageOverrides,
				"pullPolicy":     utils.GetImagePullPolicy(m),
			},
		},
	}

	return newSubscription(m, sub)
}
