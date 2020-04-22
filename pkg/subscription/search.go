package subscription

import (
	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Search overrides the search-prod chart
func Search(m *operatorsv1beta1.MultiClusterHub, cache utils.CacheSpec) *unstructured.Unstructured {
	sub := &Subscription{
		Name:      "search-prod",
		Namespace: m.Namespace,
		Overrides: map[string]interface{}{
			"imageTagPostfix": imageSuffix(m),
			"global": map[string]interface{}{
				"pullSecret": m.Spec.ImagePullSecret,
			},
			"search": map[string]interface{}{
				"aggregator": map[string]interface{}{
					"image": map[string]interface{}{
						"repository": m.Spec.Overrides.ImageRepository,
						"pullPolicy": m.Spec.ImagePullPolicy,
					},
				},
				"collector": map[string]interface{}{
					"image": map[string]interface{}{
						"repository": m.Spec.Overrides.ImageRepository,
						"pullPolicy": m.Spec.ImagePullPolicy,
					},
				},
				"searchapi": map[string]interface{}{
					"image": map[string]interface{}{
						"repository": m.Spec.Overrides.ImageRepository,
						"pullPolicy": m.Spec.ImagePullPolicy,
					},
				},
				"redisgraph": map[string]interface{}{
					"image": map[string]interface{}{
						"repository": m.Spec.Overrides.ImageRepository,
						"pullPolicy": m.Spec.ImagePullPolicy,
					},
				},
				"operator": map[string]interface{}{
					"image": map[string]interface{}{
						"repository": m.Spec.Overrides.ImageRepository,
						"pullPolicy": m.Spec.ImagePullPolicy,
					},
				},
			},
			"hubconfig": map[string]interface{}{
				"replicaCount": m.Spec.ReplicaCount,
				"nodeSelector": m.Spec.NodeSelector,
			},
		},
	}

	if cache.ImageShaDigests != nil {
		sub.Overrides["imageShaDigests"] = cache.ImageShaDigests
	}

	return newSubscription(m, sub)
}
