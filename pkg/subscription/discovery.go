// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project


package subscription

import (
	operatorsv1 "github.com/open-cluster-management/multiclusterhub-operator/pkg/apis/operator/v1"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Discovery overrides the discovery chart
func Discovery(m *operatorsv1.MultiClusterHub, overrides map[string]string) *unstructured.Unstructured {
	sub := &Subscription{
		Name:      "discovery-operator",
		Namespace: m.Namespace,
		Overrides: map[string]interface{}{
			"pullSecret":   m.Spec.ImagePullSecret,
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
			"clusterImageSets": map[string]interface{}{
				"subscriptionPause": utils.GetDisableClusterImageSets(m),
			},
		},
	}
	setCustomCA(m, sub)

	return newSubscription(m, sub)
}
