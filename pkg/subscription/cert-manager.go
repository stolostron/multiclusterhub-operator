// Copyright (c) 2020 Red Hat, Inc.

package subscription

import (
	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// CertManager overrides the cert-manager chart
func CertManager(m *operatorsv1.MultiClusterHub, overrides map[string]string) *unstructured.Unstructured {
	sub := &Subscription{
		Name:      "cert-manager",
		Namespace: utils.CertManagerNS(m),
		Overrides: map[string]interface{}{
			"imagePullSecret": m.Spec.ImagePullSecret,
			"global": map[string]interface{}{
				"isOpenshift":    true,
				"imageOverrides": overrides,
				"pullPolicy":     utils.GetImagePullPolicy(m),
			},
			"serviceAccount": map[string]interface{}{
				"create": true,
				"name":   "cert-manager",
			},
			"extraEnv": []map[string]interface{}{
				{
					"name":  "OWNED_NAMESPACE",
					"value": utils.CertManagerNS(m),
				},
			},
			"hubconfig": map[string]interface{}{
				"replicaCount": utils.DefaultReplicaCount(m),
				"nodeSelector": m.Spec.NodeSelector,
			},
		},
	}
	CheckCustomCA(m, sub)

	return newSubscription(m, sub)
}

// CertWebhook overrides the cert-manager-webhook chart
func CertWebhook(m *operatorsv1.MultiClusterHub, overrides map[string]string) *unstructured.Unstructured {
	sub := &Subscription{
		Name:      "cert-manager-webhook",
		Namespace: utils.CertManagerNS(m),
		Overrides: map[string]interface{}{
			"pkiNamespace": m.Namespace,
			"global": map[string]interface{}{
				"pullSecret":     m.Spec.ImagePullSecret,
				"imageOverrides": overrides,
			},
			"serviceAccount": map[string]interface{}{
				"create": true,
				"name":   "cert-manager-webhook",
			},
			"hubconfig": map[string]interface{}{
				"replicaCount": utils.DefaultReplicaCount(m),
				"nodeSelector": m.Spec.NodeSelector,
			},
		},
	}

	cainjector := map[string]interface{}{
		"serviceAccount": map[string]interface{}{
			"create": false,
			"name":   "default",
		},
		"hubconfig": map[string]interface{}{
			"replicaCount": utils.DefaultReplicaCount(m),
			"nodeSelector": m.Spec.NodeSelector,
		},
	}

	sub.Overrides["cainjector"] = cainjector

	return newSubscription(m, sub)
}

// ConfigWatcher overrides the configmap-watcher chart
func ConfigWatcher(m *operatorsv1.MultiClusterHub, overrides map[string]string) *unstructured.Unstructured {
	sub := &Subscription{
		Name:      "configmap-watcher",
		Namespace: utils.CertManagerNS(m),
		Overrides: map[string]interface{}{
			"global": map[string]interface{}{
				"pullSecret":     m.Spec.ImagePullSecret,
				"imageOverrides": overrides,
				"pullPolicy":     utils.GetImagePullPolicy(m),
			},
			"serviceAccount": map[string]interface{}{
				"create": true,
				"name":   "cert-manager-config",
			},
			"hubconfig": map[string]interface{}{
				"replicaCount": utils.DefaultReplicaCount(m),
				"nodeSelector": m.Spec.NodeSelector,
			},
		},
	}

	return newSubscription(m, sub)
}
