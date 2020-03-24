package multiclusterhub

import (
	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/subscription"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// newSubscription creates a new instance of an unstructured open-cluster-management.io Subscription object
func newSubscription(m *operatorsv1alpha1.MultiClusterHub, s *subscription.Subscription) *unstructured.Unstructured {
	sub := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps.open-cluster-management.io/v1",
			"kind":       "Subscription",
			"metadata": map[string]interface{}{
				"name":      s.Name + "-sub",
				"namespace": s.Namespace,
			},
			"spec": map[string]interface{}{
				"channel": m.Namespace + "/" + channelName,
				"name":    s.Name,
				"placement": map[string]interface{}{
					"local": true,
				},
				"packageOverrides": []map[string]interface{}{
					{
						"packageName": s.Name,
						"packageOverrides": []map[string]interface{}{
							{
								"path":  "spec",
								"value": s.Overrides,
							},
						},
					},
				},
			},
		},
	}
	sub.SetOwnerReferences([]metav1.OwnerReference{
		*metav1.NewControllerRef(m, m.GetObjectKind().GroupVersionKind()),
	})
	return sub
}

func (r *ReconcileMultiClusterHub) ensureSubscription(m *operatorsv1alpha1.MultiClusterHub, s *subscription.Subscription) (*reconcile.Result, error) {
	schema := schema.GroupVersionResource{Group: "apps.open-cluster-management.io", Version: "v1", Resource: "subscriptions"}
	sub := newSubscription(m, s)

	dc, err := createDynamicClient()
	if err != nil {
		log.Error(err, "Failed to create dynamic client")
		return &reconcile.Result{}, err
	}

	_, err = dc.Resource(schema).Namespace(sub.GetNamespace()).Get(sub.GetName(), metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {

		// Create the resource
		_, err = dc.Resource(schema).Namespace(sub.GetNamespace()).Create(sub, metav1.CreateOptions{})
		if err != nil {
			// Creation failed
			log.Error(err, "Failed to create new Subscription", "Subscription.Namespace", sub.GetNamespace(), "Subscription.Name", sub.GetName())
			return &reconcile.Result{}, err
		}
		// Creation was successful
		log.Info("Created a new Subscription", "Subscription.Namespace", sub.GetNamespace(), "Subscription.Name", sub.GetName())
		return nil, nil

	} else if err != nil {
		// Error that isn't due to the resource not existing
		log.Error(err, "Failed to get resource", "resource", schema.GroupResource().String())
		return &reconcile.Result{}, err
	}

	return nil, nil
}

func imageSuffix(m *operatorsv1alpha1.MultiClusterHub) (s string) {
	s = m.Spec.ImageTagSuffix
	if s != "" {
		s = "-" + s
	}
	return
}
