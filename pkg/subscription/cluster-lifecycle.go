// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project


package subscription

import (
	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// cluster-lifecycle overrides the cluster-lifecycle chart
func cluster-lifecycle(m *operatorsv1.MultiClusterHub, overrides map[string]string) *unstructured.Unstructured {
	sub := &Subscription{
		Name:      "cluster-lifecycle",
		Namespace: m.Namespace,
		Overrides: map[string]interface{}{
			"hubconfig": map[string]interface{}{
				"replicaCount": utils.DefaultReplicaCount(m),
				"nodeSelector": m.Spec.NodeSelector,
			},
			"global": map[string]interface{}{
				"pullPolicy":      utils.GetImagePullPolicy(m),
				"imagePullSecret": m.Spec.ImagePullSecret,
				"imageRepository": utils.GetImageRepository(m),
				"imageTagPostfix": imageSuffix(m),
				"imageOverrides":  overrides,
			},
		},
	}
	setCustomCA(m, sub)

	return newSubscription(m, sub)
}
