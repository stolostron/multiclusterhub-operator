package multiclusterhub

import (
	"context"
	"time"

	operatorsv1alpha1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operators/v1alpha1"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *ReconcileMultiClusterHub) ensureDeployment(m *operatorsv1alpha1.MultiClusterHub, dep *appsv1.Deployment) (*reconcile.Result, error) {
	// See if deployment already exists and create if it doesn't
	found := &appsv1.Deployment{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      dep.Name,
		Namespace: m.Namespace,
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

func (r *ReconcileMultiClusterHub) ensureService(m *operatorsv1alpha1.MultiClusterHub, s *corev1.Service) (*reconcile.Result, error) {
	found := &corev1.Service{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      s.Name,
		Namespace: m.Namespace,
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

func (r *ReconcileMultiClusterHub) ensureSecret(m *operatorsv1alpha1.MultiClusterHub, s *corev1.Secret) (*reconcile.Result, error) {
	found := &corev1.Secret{}
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      s.Name,
		Namespace: m.Namespace,
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

func createDynamicClient() (dynamic.Interface, error) {
	config, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	dynClient, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return dynClient, err
}

func (r *ReconcileMultiClusterHub) apiReady(gv schema.GroupVersion) (*reconcile.Result, error) {
	cfg, err := config.GetConfig()
	if err != nil {
		log.Error(err, "Failed to create rest config")
		return &reconcile.Result{}, err
	}

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
