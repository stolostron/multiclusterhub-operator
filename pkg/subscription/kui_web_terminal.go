// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package subscription

import (
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/pkg/apis/operator/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// KUIWebTerminal overrides the kui-web-terminal chart
func KUIWebTerminal(m *operatorsv1.MultiClusterHub, overrides map[string]string, ingress string) *unstructured.Unstructured {
	sub := &Subscription{
		Name:      "kui-web-terminal",
		Namespace: m.Namespace,
		Overrides: map[string]interface{}{
			"pullSecret": m.Spec.ImagePullSecret,
			"ocpingress": ingress,
			"proxy": map[string]interface{}{
				"clusterIP": "icp-management-ingress",
			},
			"hubconfig": map[string]interface{}{
				"replicaCount": utils.DefaultReplicaCount(m),
				"nodeSelector": m.Spec.NodeSelector,
			},
			"global": map[string]interface{}{
				"imageOverrides": overrides,
				"pullPolicy":     utils.GetImagePullPolicy(m),
			},
		},
	}
	setCustomCA(m, sub)

	return newSubscription(m, sub)
}
