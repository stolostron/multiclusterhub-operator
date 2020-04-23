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
			"global": map[string]interface{}{
				"pullSecret":     m.Spec.ImagePullSecret,
				"imageOverrides": cache.ImageOverrides,
			},
			"search": map[string]interface{}{
				"aggregator": map[string]interface{}{
					"image": map[string]interface{}{
						"pullPolicy": m.Spec.ImagePullPolicy,
					},
				},
				"collector": map[string]interface{}{
					"image": map[string]interface{}{
						"pullPolicy": m.Spec.ImagePullPolicy,
					},
				},
				"searchapi": map[string]interface{}{
					"image": map[string]interface{}{
						"pullPolicy": m.Spec.ImagePullPolicy,
					},
				},
				"redisgraph": map[string]interface{}{
					"image": map[string]interface{}{
						"pullPolicy": m.Spec.ImagePullPolicy,
					},
				},
				"operator": map[string]interface{}{
					"image": map[string]interface{}{
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

	return newSubscription(m, sub)
}
