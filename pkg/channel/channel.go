// Copyright (c) 2020 Red Hat, Inc.

package channel

import (
	"fmt"

	chnv1alpha1 "github.com/open-cluster-management/multicloud-operators-channel/pkg/apis/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	operatorsv1beta1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1beta1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/helmrepo"
)

// ChannelName is the name of the open-cluster-management.io channel
var ChannelName = "charts-v1"

// Schema is the GVK for an application subscription channel
var Schema = schema.GroupVersionResource{Group: "apps.open-cluster-management.io", Version: "v1", Resource: "channels"}

// build Helm pathname from repo name and por
func channelURL(m *operatorsv1beta1.MultiClusterHub) string {
	return fmt.Sprintf("http://%s.%s:%d/charts", helmrepo.HelmRepoName, m.Namespace, helmrepo.Port)
}

// Channel returns an unstructured Channel object to watch the helm repository
func Channel(m *operatorsv1beta1.MultiClusterHub) *chnv1alpha1.Channel {
	ch := &chnv1alpha1.Channel{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Channel",
			APIVersion: "apps.open-cluster-management.io/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      ChannelName,
			Namespace: m.Namespace,
		},
		Spec: chnv1alpha1.ChannelSpec{
			Type:     "HelmRepo",
			Pathname: channelURL(m),
		},
	}
	// Skip this step on testing, as there is no controller ref to receive
	if m.UID != "" {
		ch.SetOwnerReferences([]metav1.OwnerReference{
			*metav1.NewControllerRef(m, m.GetObjectKind().GroupVersionKind()),
		})
	}

	return ch
}
