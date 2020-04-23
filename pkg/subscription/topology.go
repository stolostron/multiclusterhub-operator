package subscription

import (
	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Topology overrides the grc chart
func Topology(m *operatorsv1beta1.MultiClusterHub, cache utils.CacheSpec) *unstructured.Unstructured {
	sub := &Subscription{
		Name:      "topology",
		Namespace: m.Namespace,
		Overrides: map[string]interface{}{
			"pullSecret": m.Spec.ImagePullSecret,
			"ui": map[string]interface{}{
				"image": map[string]interface{}{
					"pullPolicy": m.Spec.ImagePullPolicy,
				},
			},
			"api": map[string]interface{}{
				"image": map[string]interface{}{
					"pullPolicy": m.Spec.ImagePullPolicy,
				},
			},
			"hubconfig": map[string]interface{}{
				"replicaCount": m.Spec.ReplicaCount,
				"nodeSelector": m.Spec.NodeSelector,
			},
			"global": map[string]interface{}{
				"imageOverrides": cache.ImageOverrides,
			},
		},
	}

	return newSubscription(m, sub)
}
