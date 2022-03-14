// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package subscription

import (
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// Volsync overrides the volsync chart
func Volsync(m *operatorsv1.MultiClusterHub, overrides map[string]string) *unstructured.Unstructured {
	sub := &Subscription{
		Name:      "volsync-addon-controller",
		Namespace: m.Namespace,
		Overrides: map[string]interface{}{
			"pullSecret": m.Spec.ImagePullSecret,
			"hubconfig": map[string]interface{}{
				"replicaCount": utils.DefaultReplicaCount(m),
				"nodeSelector": m.Spec.NodeSelector,
				"tolerations":  utils.GetTolerations(m),
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
