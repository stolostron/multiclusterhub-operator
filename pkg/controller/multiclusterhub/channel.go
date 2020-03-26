package multiclusterhub

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
)

// channelName is the name of the open-cluster-management.io channel
var channelName = "charts-v1"

// build Helm pathname from repo name and port
func channelURL(m *operatorsv1alpha1.MultiClusterHub) string {
	return fmt.Sprintf("http://%s.%s:%d/charts", repoName, m.Namespace, repoPort)
}

func (r *ReconcileMultiClusterHub) ensureChannel(m *operatorsv1alpha1.MultiClusterHub, ch *unstructured.Unstructured) (*reconcile.Result, error) {
	chLog := log.WithValues("Channel.Namespace", m.Namespace, "Channel.Name", channelName)
	schema := schema.GroupVersionResource{Group: "apps.open-cluster-management.io", Version: "v1", Resource: "channels"}

	dc, err := createDynamicClient()
	if err != nil {
		chLog.Error(err, "Failed to create dynamic client")
		return &reconcile.Result{}, nil
	}

	// Try to get API group instance
	_, err = dc.Resource(schema).Namespace(m.Namespace).Get(channelName, metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {

		// Create the resource
		_, err = dc.Resource(schema).Namespace(m.Namespace).Create(ch, metav1.CreateOptions{})
		if err != nil {
			// Creation failed
			chLog.Error(err, "Failed to create new Channel")
			return &reconcile.Result{}, err
		}

		// Creation was successful
		chLog.Info("Created a new Channel")
		return nil, nil

	} else if err != nil {
		// Error that isn't due to the resource not existing
		chLog.Error(err, "Failed to get resource", "resource", schema.GroupResource().String())
		return &reconcile.Result{}, err
	}

	return nil, nil
}

func (r *ReconcileMultiClusterHub) helmChannel(m *operatorsv1alpha1.MultiClusterHub) *unstructured.Unstructured {
	ch := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps.open-cluster-management.io/v1",
			"kind":       "Channel",
			"metadata": map[string]interface{}{
				"name": channelName,
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
