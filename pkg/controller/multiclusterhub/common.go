package multiclusterhub

import (
	"context"
	"time"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"
	subscription "github.com/open-cluster-management/multicloudhub-operator/pkg/deploying/subscription"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *ReconcileMultiClusterHub) ensureDeployment(request reconcile.Request,
	instance *operatorsv1alpha1.MultiClusterHub,
	dep *appsv1.Deployment,
) (*reconcile.Result, error) {

	// See if deployment already exists and create if it doesn't
	found := &appsv1.Deployment{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      dep.Name,
		Namespace: instance.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {

		// Create the deployment
		err = r.client.Create(context.TODO(), dep)
		if err != nil {
			// Deployment failed
			log.Error(err, "Failed to create new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
			return &reconcile.Result{}, err
		}

		// Deployment was successful
		log.Info("Created a new Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		return nil, nil

	} else if err != nil {
		// Error that isn't due to the deployment not existing
		log.Error(err, "Failed to get Deployment", "Deployment.Namespace", dep.Namespace, "Deployment.Name", dep.Name)
		return &reconcile.Result{}, err
	}

	return nil, nil
}

func (r *ReconcileMultiClusterHub) ensureService(request reconcile.Request,
	instance *operatorsv1alpha1.MultiClusterHub,
	s *corev1.Service,
) (*reconcile.Result, error) {
	found := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      s.Name,
		Namespace: instance.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {

		// Create the service
		err = r.client.Create(context.TODO(), s)

		if err != nil {
			// Creation failed
			log.Error(err, "Failed to create new Service", "Service.Namespace", s.Namespace, "Service.Name", s.Name)
			return &reconcile.Result{}, err
		}

		// Creation was successful
		log.Info("Created a new Service", "Service.Namespace", s.Namespace, "Service.Name", s.Name)
		return nil, nil

	} else if err != nil {
		// Error that isn't due to the service not existing
		log.Error(err, "Failed to get Service")
		return &reconcile.Result{}, err
	}

	return nil, nil
}

func (r *ReconcileMultiClusterHub) ensureSecret(request reconcile.Request,
	instance *operatorsv1alpha1.MultiClusterHub,
	s *corev1.Secret,
) (*reconcile.Result, error) {
	found := &corev1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      s.Name,
		Namespace: instance.Namespace,
	}, found)
	if err != nil && errors.IsNotFound(err) {

		// Create the secret
		err = r.client.Create(context.TODO(), s)
		if err != nil {
			// Creation failed
			log.Error(err, "Failed to create new Secret", "Secret.Namespace", s.Namespace, "Secret.Name", s.Name)
			return &reconcile.Result{}, err
		}

		// Creation was successful
		log.Info("Created a new secret", "Secret.Namespace", s.Namespace, "Secret.Name", s.Name)
		return nil, nil

	} else if err != nil {
		// Error that isn't due to the secret not existing
		log.Error(err, "Failed to get Secret", "Secret.Namespace", s.Namespace, "Secret.Name", s.Name)
		return &reconcile.Result{}, err
	}

	return nil, nil
}

func createDynamicClient() (*dynamic.Interface, *rest.Config, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		log.Error(err, "Failed to get cluster config for API host discovery/authentication")
		return nil, nil, err
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		log.Error(err, "Failed to create dynamic client from cluster config")
		return nil, nil, err
	}
	return &dynClient, config, err
}

func (r *ReconcileMultiClusterHub) ensureSubscription(m *operatorsv1alpha1.MultiClusterHub, dc dynamic.Interface, s *subscription.Subscription) (*reconcile.Result, error) {
	schema := schema.GroupVersionResource{Group: "apps.open-cluster-management.io", Version: "v1", Resource: "subscriptions"}
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

	_, err := dc.Resource(schema).Namespace(sub.GetNamespace()).Get(sub.GetName(), metav1.GetOptions{})
	if err != nil && errors.IsNotFound(err) {

		// Create the resource
		_, err := dc.Resource(schema).Namespace(sub.GetNamespace()).Create(sub, metav1.CreateOptions{})
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

func (r *ReconcileMultiClusterHub) apiReady(cfg *rest.Config, gv schema.GroupVersion) (*reconcile.Result, error) {
	c, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		log.Error(err, "Failed to create discovery client")
		return &reconcile.Result{}, err
	}

	err = discovery.ServerSupportsVersion(c, gv)
	if err != nil {
		// Wait a little and try again
		log.Info("Waiting for API group to be available", "API group", gv)
		return &reconcile.Result{RequeueAfter: time.Second * 10}, nil
	}
	return nil, nil
}
