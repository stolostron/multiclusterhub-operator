package subscription

import (
	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ManagementIngress overrides the management-ingress chart
func ManagementIngress(m *operatorsv1alpha1.MultiClusterHub, cache utils.CacheSpec) *unstructured.Unstructured {
	sub := &Subscription{
		Name:      "management-ingress",
		Namespace: m.Namespace,
		Overrides: map[string]interface{}{
			"imageTagPostfix": imageSuffix(m),
			"pullSecret":      m.Spec.ImagePullSecret,
			"image": map[string]interface{}{
				"repository": m.Spec.ImageRepository,
				"pullPolicy": m.Spec.ImagePullPolicy,
			},
			"cluster_basedomain": cache.IngressDomain,
			"hubconfig": map[string]interface{}{
				"replicaCount": m.Spec.ReplicaCount,
				"nodeSelector": utils.GenerateNodeSelectorNotation(m),
			},
		},
	}
	return newSubscription(m, sub)
}
