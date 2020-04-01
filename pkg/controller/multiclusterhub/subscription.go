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
	"sigs.k8s.io/yaml"
)

const certManagerNamespaceTemplate = `
apiVersion: v1
kind: Namespace
metadata:
  name: cert-manager
`

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

func (r *ReconcileMultiClusterHub) ensureNamespace() (*reconcile.Result, error) {
	sublog := log.WithValues("Creating cert-manager namespace", utils.CertManagerNamespace, "Namespace.Name", utils.CertManagerNamespace)

	json, err := yaml.YAMLToJSON([]byte(certManagerNamespaceTemplate))
	if err != nil {
		return &reconcile.Result{}, err
	}
	// var u unstructured.Unstructured
	// err = u.UnmarshalJSON(json)
	// if err != nil {
	// 	return &reconcile.Result{}, err
	// }

	// var ns v1.Namespace
	// err = runtime.DefaultUnstructuredConverter.FromUnstructured(u.UnstructuredContent(), &ns)
	// if err != nil {
	// 	sublog.Error(err, "Failed to unmarshal namespace")
	// 	return nil, err
	// }

	var ns v1.Namespace
	err = ns.Unmarshal(json)
	if err != nil {
		return &reconcile.Result{}, err
	}

	log.Info(fmt.Sprintf("Error: %+v", ns))

	found := &v1.Namespace{}
	err = r.client.Get(context.TODO(), types.NamespacedName{
		Name: utils.CertManagerNamespace,
	}, found)

	if err != nil && errors.IsNotFound(err) {
		// Create the namespace
		sublog.Info("Creating a new namespace", "Namespace.Name", ns.Name)
		err = r.client.Create(context.TODO(), &ns)
		if err != nil {
			// Creation failed
			log.Error(err, "Failed to create new Namespace", "Namespace.Name", ns.Name)
			return &reconcile.Result{}, err
		}
		// Creation was successful
		return nil, nil
	} else if err != nil {
		// Error that isn't due to the secret not existing
		sublog.Error(err, "Failed to get Namespace")
		return &reconcile.Result{}, err
	}
	return nil, nil
}

func (r *ReconcileMultiClusterHub) copyPullSecret(originNS, pullSecretName, newNS string) (*reconcile.Result, error) {
	schema := schema.GroupVersionResource{Group: "", Version: "v1", Resource: "secrets"}
	sublog := log.WithValues("Copying Secret to cert-manager namespace", pullSecretName, "Namespace.Name", utils.CertManagerNamespace)

	dc, err := createDynamicClient()
	if err != nil {
		sublog.Error(err, "Failed to create dynamic client")
		return &reconcile.Result{}, err
	}
	pullSecret, err := dc.Resource(schema).Namespace(originNS).Get(pullSecretName, metav1.GetOptions{})
	if err != nil {
		return &reconcile.Result{}, err
	}

	pullSecret.SetNamespace(newNS)
	pullSecret.SetSelfLink("")
	pullSecret.SetResourceVersion("")
	pullSecret.SetUID("")

	_, err = dc.Resource(schema).Namespace(newNS).Create(pullSecret, metav1.CreateOptions{})
	if err != nil && errors.IsNotFound(err) {
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
