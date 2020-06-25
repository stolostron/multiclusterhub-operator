// Copyright (c) 2020 Red Hat, Inc.

package subscription

import (
	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// MongoDB overrides the multicluster-mongodb chart
func MongoDB(m *operatorsv1.MultiClusterHub, overrides map[string]string) *unstructured.Unstructured {
	sub := &Subscription{
		Name:      "multicluster-mongodb",
		Namespace: m.Namespace,
		Overrides: map[string]interface{}{
			"imagePullSecrets": []string{
				m.Spec.ImagePullSecret,
			},
			"network_ip_version": networkVersion(m),
			"auth": map[string]interface{}{
				"enabled":             true,
				"existingAdminSecret": "mongodb-admin",
			},
			"persistentVolume": map[string]interface{}{
				"accessModes": []string{
					"ReadWriteOnce",
				},
				"enabled":      true,
				"size":         m.Spec.Mongo.Storage,
				"storageClass": m.Spec.Mongo.StorageClass,
			},
			"replicas": utils.DefaultReplicaCount(m),
			"tls": map[string]interface{}{
				"casecret": "multicloud-ca-cert",
				"issuer":   "multicloud-ca-issuer",
				"enabled":  true,
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
	CheckCustomCA(m, sub)

	return newSubscription(m, sub)
}
