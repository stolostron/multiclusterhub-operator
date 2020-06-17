// Copyright (c) 2020 Red Hat, Inc.

package subscription

import (
	operatorsv11 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ManagementIngress overrides the management-ingress chart
func ManagementIngress(m *operatorsv11.MultiClusterHub, overrides map[string]string, ingress string) *unstructured.Unstructured {
	sub := &Subscription{
		Name:      "management-ingress",
		Namespace: m.Namespace,
		Overrides: map[string]interface{}{
			"pullSecret":         m.Spec.ImagePullSecret,
			"cluster_basedomain": ingress,
			"hubconfig": map[string]interface{}{
				"replicaCount": utils.DefaultReplicaCount(m),
				"nodeSelector": m.Spec.NodeSelector,
			},
			"global": map[string]interface{}{
				"imageOverrides": overrides,
				"pullPolicy":     utils.GetImagePullPolicy(m),
			},
			"config": map[string]interface{}{
				"ssl-ciphers": utils.FormatSSLCiphers(m.Spec.Ingress.SSLCiphers),
			},
		},
	}

	return newSubscription(m, sub)
}
