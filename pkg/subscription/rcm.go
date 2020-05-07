// Copyright (c) 2020 Red Hat, Inc.

package subscription

import (
	subalpha1 "github.com/open-cluster-management/multicloud-operators-subscription/pkg/apis/apps/v1"
	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
)

// RCM overrides the rcm chart
func RCM(m *operatorsv1beta1.MultiClusterHub, cache utils.CacheSpec) *subalpha1.Subscription {
	sub := &Subscription{
		Name:      "rcm",
		Namespace: m.Namespace,
		Overrides: map[string]interface{}{
			"hubconfig": map[string]interface{}{
				"replicaCount": utils.DefaultReplicaCount(m),
				"nodeSelector": m.Spec.NodeSelector,
			},
			"global": map[string]interface{}{
				"pullPolicy":      utils.GetImagePullPolicy(m),
				"imagePullSecret": m.Spec.ImagePullSecret,
				"imageRepository": m.Spec.Overrides.ImageRepository,
				"imageTagPostfix": imageSuffix(m),
				"imageOverrides":  cache.ImageOverrides,
			},
		},
	}

	return newSubscription(m, sub)
}
