package multiclusterhub

import (
	"context"
	"fmt"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/subscription"

	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
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
	sublog := log.WithValues("Subscription.Namespace", sub.GetNamespace(), "Subscription.Name", sub.GetName())

	dc, err := createDynamicClient()
	if err != nil {
		sublog.Error(err, "Failed to create dynamic client")
		return &reconcile.Result{}, err
	}

	_, err = dc.Resource(schema).Namespace(sub.GetNamespace()).Get(sub.GetName(), metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {

		// Create the resource
		_, err = dc.Resource(schema).Namespace(sub.GetNamespace()).Create(sub, metav1.CreateOptions{})
		if err != nil {
			// Creation failed
			sublog.Error(err, "Failed to create new Subscription")
			return &reconcile.Result{}, err
		}
		// Creation was successful
		sublog.Info("Created a new Subscription")
		return nil, nil

	} else if err != nil {
		// Error that isn't due to the resource not existing
		sublog.Error(err, "Failed to get resource", "Resource", schema.GroupResource().String())
		return &reconcile.Result{}, err
	}

	return nil, nil
}

func (r *ReconcileMultiClusterHub) copyPullSecret(originNS, pullSecretName, newNS string) (*reconcile.Result, error) {
	sublog := log.WithValues("Copying Secret to cert-manager namespace", pullSecretName, "Namespace.Name", utils.CertManagerNamespace)

	pullSecret := &v1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      pullSecretName,
		Namespace: originNS,
	}, pullSecret)
	if err != nil {
		sublog.Error(err, "Failed to get secret")
	}

	pullSecret.SetNamespace(newNS)
	pullSecret.SetSelfLink("")
	pullSecret.SetResourceVersion("")
	pullSecret.SetUID("")

	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name:      pullSecretName,
		Namespace: newNS,
	}, pullSecret)

	if err != nil && errors.IsNotFound(err) {
		sublog.Info(fmt.Sprintf("Creating secret %s in namespace %s", pullSecretName, utils.CertManagerNamespace))
		err = r.client.Create(context.TODO(), pullSecret)
		if err != nil {
			sublog.Error(err, "Failed to create secret")
		}
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
