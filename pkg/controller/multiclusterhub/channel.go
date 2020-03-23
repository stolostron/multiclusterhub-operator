package multiclusterhub

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
)

// ChannelName is the name of the open-cluster-management.io channel
var ChannelName = "charts-v1"

// build Helm pathname from repo name and port
func channelURL(m *operatorsv1alpha1.MultiClusterHub) string {
	return fmt.Sprintf("http://%s.%s:%d/charts", repoName, m.Namespace, repoPort)
}

func (r *ReconcileMultiClusterHub) ensureChannel(m *operatorsv1alpha1.MultiClusterHub, dc dynamic.Interface) (*reconcile.Result, error) {
	channel := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps.open-cluster-management.io/v1",
			"kind":       "Channel",
			"metadata": map[string]interface{}{
				"name": ChannelName,
			},
			"spec": map[string]interface{}{
				"type":     "HelmRepo",
				"pathname": channelURL(m),
			},
		},
	}

	channel.SetOwnerReferences([]metav1.OwnerReference{
		*metav1.NewControllerRef(m, m.GetObjectKind().GroupVersionKind()),
	})

	schema := schema.GroupVersionResource{Group: "apps.open-cluster-management.io", Version: "v1", Resource: "channels"}

	// Try to get API group instance
	_, err := dc.Resource(schema).Namespace(m.Namespace).Get(ChannelName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {

		// Create the resource
		result, err := dc.Resource(schema).Namespace(m.Namespace).Create(channel, metav1.CreateOptions{})
		if err != nil {
			// Creation failed
			log.Error(err, "Failed to create new Channel", "Channel.Namespace", m.Namespace, "Channel.Name", ChannelName)
			return &reconcile.Result{}, err
		}

		// Creation was successful
		log.Info("Created a new Channel", "Channel.Namespace", result.GetNamespace(), "Channel.Name", result.GetName())
		return nil, nil

	} else if err != nil {
		// Error that isn't due to the resource not existing
		log.Error(err, "Failed to get resource", "resource", schema.GroupResource().String())
		return &reconcile.Result{}, err
	}

	return nil, nil
}
