// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package subscription

import (
	operatorsv1 "github.com/open-cluster-management/multiclusterhub-operator/api/v1"
	"github.com/open-cluster-management/multiclusterhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// cluster-proxy-addon overrides the cluster-proxy-addon chart
func ClusterProxyAddOn(m *operatorsv1.MultiClusterHub, overrides map[string]string, ingress string) *unstructured.Unstructured {
	sub := &Subscription{
		Name:      "cluster-proxy-addon",
		Namespace: m.Namespace,
		Overrides: map[string]interface{}{
			"cluster_basedomain": ingress,
			"hubconfig": map[string]interface{}{
				"replicaCount": utils.DefaultReplicaCount(m),
				"nodeSelector": m.Spec.NodeSelector,
			},
			"global": map[string]interface{}{
				"pullPolicy":     utils.GetImagePullPolicy(m),
				"imageOverrides": overrides,
			},
		},
	}
	setCustomCA(m, sub)

	return newSubscription(m, sub)
}
