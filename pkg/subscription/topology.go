package subscription

import (
	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Topology overrides the grc chart
func Topology(m *operatorsv1alpha1.MultiClusterHub, cache utils.CacheSpec) *unstructured.Unstructured {
	sub := &Subscription{
		Name:      "topology",
		Namespace: m.Namespace,
		Overrides: map[string]interface{}{
			"imageTagPostfix": imageSuffix(m),
			"pullSecret":      m.Spec.ImagePullSecret,
			"ui": map[string]interface{}{
				"image": map[string]interface{}{
					"repository": m.Spec.ImageRepository,
					"pullPolicy": m.Spec.ImagePullPolicy,
				},
			},
			"api": map[string]interface{}{
				"image": map[string]interface{}{
					"repository": m.Spec.ImageRepository,
					"pullPolicy": m.Spec.ImagePullPolicy,
				},
			},
			"hubconfig": map[string]interface{}{
				"replicaCount": m.Spec.ReplicaCount,
			},
			"imageShaDigests": cache.ImageShaDigests,
		},
	}
	return newSubscription(m, sub)
}
