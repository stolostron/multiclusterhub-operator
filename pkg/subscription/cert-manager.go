package subscription

import (
	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
)

// CertManager overrides the cert-manager chart
func CertManager(m *operatorsv1alpha1.MultiClusterHub) *Subscription {
	return &Subscription{
		Name:      "cert-manager",
		Namespace: utils.CertManagerNS(m),
		Overrides: map[string]interface{}{
			"imageTagPostfix": imageSuffix(m),
			"imagePullSecret": m.Spec.ImagePullSecret,
			"global": map[string]interface{}{
				"isOpenshift": true,
			},
			"image": map[string]interface{}{
				"repository": m.Spec.ImageRepository,
				"pullPolicy": m.Spec.ImagePullPolicy,
			},
			"serviceAccount": map[string]interface{}{
				"create": true,
				"name":   "cert-manager",
			},
			"solver": map[string]interface{}{
				"repository": m.Spec.ImageRepository,
			},
			"extraEnv": []map[string]interface{}{
				{
					"name":  "OWNED_NAMESPACE",
					"value": utils.CertManagerNS(m),
				},
			},
		},
	}
}

// CertWebhook overrides the cert-manager-webhook chart
func CertWebhook(m *operatorsv1alpha1.MultiClusterHub) *Subscription {
	return &Subscription{
		Name:      "cert-manager-webhook",
		Namespace: utils.CertManagerNS(m),
		Overrides: map[string]interface{}{
			"imageTagPostfix": imageSuffix(m),
			"pkiNamespace":    m.Namespace,
			"global": map[string]interface{}{
				"pullSecret": m.Spec.ImagePullSecret,
			},
			"cainjector": map[string]interface{}{
				"imageTagPostfix": imageSuffix(m),
				"image": map[string]interface{}{
					"repository": m.Spec.ImageRepository,
				},
				"serviceAccount": map[string]interface{}{
					"create": false,
					"name":   "default",
				},
			},
			"image": map[string]interface{}{
				"repository": m.Spec.ImageRepository,
			},
			"serviceAccount": map[string]interface{}{
				"create": true,
				"name":   "cert-manager-webhook",
			},
		},
	}
}

// ConfigWatcher overrides the configmap-watcher chart
func ConfigWatcher(m *operatorsv1alpha1.MultiClusterHub) *Subscription {
	return &Subscription{
		Name:      "configmap-watcher",
		Namespace: utils.CertManagerNS(m),
		Overrides: map[string]interface{}{
			"imageTagPostfix": imageSuffix(m),
			"global": map[string]interface{}{
				"pullSecret": m.Spec.ImagePullSecret,
			},
			"image": map[string]interface{}{
				"repository": m.Spec.ImageRepository,
				"pullPolicy": m.Spec.ImagePullPolicy,
			},
			"serviceAccount": map[string]interface{}{
				"create": true,
				"name":   "cert-manager-config",
			},
		},
	}
}
