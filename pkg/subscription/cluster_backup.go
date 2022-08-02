// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package subscription

import (
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Discovery overrides the discovery chart
func ClusterBackup(m *operatorsv1.MultiClusterHub, overrides map[string]string) *unstructured.Unstructured {
	sub := &Subscription{
		Name:      "cluster-backup-chart",
		Namespace: utils.ClusterSubscriptionNamespace,
		Overrides: map[string]interface{}{
			"hubconfig": map[string]interface{}{
				"replicaCount": utils.DefaultReplicaCount(m),
				"nodeSelector": m.Spec.NodeSelector,
				"name":         m.Name,
				"namespace":    m.Namespace,
				"tolerations":  utils.GetTolerations(m),
			},
			"global": map[string]interface{}{
				"imageOverrides": overrides,
				"pullPolicy":     utils.GetImagePullPolicy(m),
				"pullSecret":     m.Spec.ImagePullSecret,
			},
		},
	}
	setCustomCA(m, sub)
	setCustomOADPConfig(m, sub)

	return newSubscription(m, sub)
}

func OldClusterBackup(m *operatorsv1.MultiClusterHub) *unstructured.Unstructured {
	sub := &Subscription{
		Name:      "cluster-backup-chart",
		Namespace: m.Namespace,
	}

	return newSubscription(m, sub)
}

func BackupNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: utils.ClusterSubscriptionNamespace,
		},
	}
}

func BackupNamespaceUnstructured() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Kind: "Namespace", Version: "v1"})
	u.SetName(utils.ClusterSubscriptionNamespace)
	return u
}
