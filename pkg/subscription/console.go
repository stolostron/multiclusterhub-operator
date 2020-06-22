// Copyright (c) 2020 Red Hat, Inc.

package subscription

import (
	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Console overrides the console-chart chart
func Console(m *operatorsv1.MultiClusterHub, overrides map[string]string, ingress string) *unstructured.Unstructured {
	sub := &Subscription{
		Name:      "console-chart",
		Namespace: m.Namespace,
		Overrides: map[string]interface{}{
			"pullSecret":   m.Spec.ImagePullSecret,
			"ocpingress":   ingress,
			"cfcRouterUrl": "https://management-ingress:443",
			"hubconfig": map[string]interface{}{
				"replicaCount": utils.DefaultReplicaCount(m),
				"nodeSelector": m.Spec.NodeSelector,
				"name":         m.Name,
				"namespace":    m.Namespace,
			},
			"global": map[string]interface{}{
				"imageOverrides": overrides,
				"pullPolicy":     utils.GetImagePullPolicy(m),
			},
		},
	}

	return newSubscription(m, sub)
}
