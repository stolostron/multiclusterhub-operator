package subscription

import (
	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// KUIWebTerminal overrides the kui-web-terminal chart
func KUIWebTerminal(m *operatorsv1alpha1.MultiClusterHub) *unstructured.Unstructured {
	sub := &Subscription{
		Name:      "kui-web-terminal",
		Namespace: m.Namespace,
		Overrides: map[string]interface{}{
			"imageTagPostfix": imageSuffix(m),
			"pullSecret":      m.Spec.ImagePullSecret,
			"proxy": map[string]interface{}{
				"clusterIP": "icp-management-ingress",
				"image": map[string]interface{}{
					"repository": m.Spec.ImageRepository,
					"pullPolicy": m.Spec.ImagePullPolicy,
				},
			},
			"hubconfig": map[string]interface{}{
				"replicaCount": m.Spec.ReplicaCount,
				"nodeSelector": utils.GenerateNodeSelectorNotation(m),
			},
		},
	}
	return newSubscription(m, sub)
}
