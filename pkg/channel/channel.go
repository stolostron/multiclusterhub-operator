// Copyright (c) 2020 Red Hat, Inc.

package channel

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/pkg/apis/operator/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/helmrepo"
)

// ChannelName is the name of the open-cluster-management.io channel
var ChannelName = "charts-v1"

// Schema is the GVK for an application subscription channel
var Schema = schema.GroupVersionResource{Group: "apps.open-cluster-management.io", Version: "v1", Resource: "channels"}

// build Helm pathname from repo name and por
func channelURL(m *operatorsv1.MultiClusterHub) string {
	return fmt.Sprintf("http://%s.%s.svc.cluster.local:%d/charts", helmrepo.HelmRepoName, m.Namespace, helmrepo.Port)
}

// Channel returns an unstructured Channel object to watch the helm repository
func Channel(m *operatorsv1.MultiClusterHub) *unstructured.Unstructured {
	ch := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps.open-cluster-management.io/v1",
			"kind":       "Channel",
			"metadata": map[string]interface{}{
				"name":      ChannelName,
				"namespace": m.Namespace,
			},
			"spec": map[string]interface{}{
				"type":     "HelmRepo",
				"pathname": channelURL(m),
			},
		},
	}
	ch.SetOwnerReferences([]metav1.OwnerReference{
		*metav1.NewControllerRef(m, m.GetObjectKind().GroupVersionKind()),
	})
	return ch
}
