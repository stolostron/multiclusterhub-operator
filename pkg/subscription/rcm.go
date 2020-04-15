package subscription

import (
	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// RCM overrides the rcm chart
func RCM(m *operatorsv1alpha1.MultiClusterHub, cache utils.CacheSpec) *unstructured.Unstructured {
	sub := &Subscription{
		Name:      "rcm",
		Namespace: m.Namespace,
		Overrides: map[string]interface{}{
			"imageTagPostfix": imageSuffix(m),
			"imageRepository": m.Spec.ImageRepository,
			"imagePullSecret": m.Spec.ImagePullSecret,
			"clusterController": map[string]interface{}{
				"image": map[string]interface{}{
					"repository": m.Spec.ImageRepository,
					"pullPolicy": m.Spec.ImagePullPolicy,
					"pullSecret": m.Spec.ImagePullSecret,
				},
			},
			"clusterLifecycle": map[string]interface{}{
				"image": map[string]interface{}{
					"repository": m.Spec.ImageRepository,
					"pullPolicy": m.Spec.ImagePullPolicy,
					"pullSecret": m.Spec.ImagePullSecret,
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
