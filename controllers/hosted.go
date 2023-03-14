// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"errors"
	"fmt"
	"time"

	"k8s.io/client-go/tools/clientcmd"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

var ErrBadFormat = errors.New("bad format")

func (r *MultiClusterHubReconciler) HostedReconcile(ctx context.Context, mch *operatorv1.MultiClusterHub) (retRes ctrl.Result, retErr error) {
	log := log.FromContext(ctx)

	defer func() {
		err := r.Client.Status().Update(ctx, mch)
		if mch.Status.Phase != operatorv1.HubRunning && !utils.IsPaused(mch) {
			retRes = ctrl.Result{RequeueAfter: resyncPeriod}
		}
		if err != nil {
			retErr = err
		}
	}()

	// If deletion detected, finalize mch config
	if mch.GetDeletionTimestamp() != nil {
		if controllerutil.ContainsFinalizer(mch, hubFinalizer) {
			err := r.finalizeHostedMCH(ctx, mch) // returns all errors
			if err != nil {
				log.Info(err.Error())
				return ctrl.Result{RequeueAfter: 5 * time.Second}, nil
			}

			log.Info("all subcomponents have been finalized successfully - removing finalizer")
			controllerutil.RemoveFinalizer(mch, hubFinalizer)
			if err := r.Client.Update(ctx, mch); err != nil {
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil // Object finalized successfully
	}

	// Add finalizer for this CR
	if !controllerutil.ContainsFinalizer(mch, hubFinalizer) {
		controllerutil.AddFinalizer(mch, hubFinalizer)
		if err := r.Client.Update(ctx, mch); err != nil {
			return ctrl.Result{}, err
		}
	}

	var result ctrl.Result
	var err error

	result, err = r.setHostedDefaults(ctx, mch)
	if result != (ctrl.Result{}) {
		return ctrl.Result{}, err
	}
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	// Do not reconcile objects if this instance of mch is labeled "paused"
	if utils.IsPaused(mch) {
		log.Info("MultiClusterHub reconciliation is paused. Nothing more to do.")
		return ctrl.Result{}, nil
	}

	hostedClient, err := r.GetHostedClient(ctx, mch)
	if err != nil {
		return ctrl.Result{RequeueAfter: resyncPeriod}, err
	}

	err = hostedClient.Create(ctx, &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: mch.Namespace},
	})
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return ctrl.Result{RequeueAfter: resyncPeriod}, err
	}

	result, err = r.ensureMultiClusterEngine(ctx, mch)
	if result != (ctrl.Result{}) {
		return result, err
	}

	result, err = r.waitForMCEReady(ctx)
	if result != (ctrl.Result{}) {
		return result, err
	}

	err = r.Client.Status().Update(ctx, mch)
	if err != nil {
		return ctrl.Result{RequeueAfter: resyncPeriod}, err
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) GetHostedClient(ctx context.Context, mch *operatorv1.MultiClusterHub) (client.Client, error) {
	secretNN, err := utils.GetHostedCredentialsSecret(mch)
	if err != nil {
		return nil, err
	}

	// Parse Kube credentials from secret
	kubeConfigSecret := &corev1.Secret{}
	if err := r.Client.Get(context.TODO(), secretNN, kubeConfigSecret); err != nil {
		if apierrors.IsNotFound(err) {
			if err != nil {
				return nil, err
			}
		}
	}
	kubeconfig, err := parseKubeCreds(kubeConfigSecret)
	if err != nil {
		return nil, err
	}

	restconfig, err := clientcmd.RESTConfigFromKubeConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	uncachedClient, err := client.New(restconfig, client.Options{
		Scheme: r.Client.Scheme(),
	})
	if err != nil {
		return nil, err
	}

	return uncachedClient, nil
}

// parseKubeCreds takes a secret cotaining credentials and returns the stored Kubeconfig.
func parseKubeCreds(secret *corev1.Secret) ([]byte, error) {
	kubeconfig, ok := secret.Data["kubeconfig"]
	if !ok {
		return []byte{}, fmt.Errorf("%s: %w", secret.Name, ErrBadFormat)
	}
	return kubeconfig, nil
}

// setHostedDefaults configures the MCH with default values and updates
func (r *MultiClusterHubReconciler) setHostedDefaults(ctx context.Context, m *operatorv1.MultiClusterHub) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	updateNecessary := false
	if !utils.AvailabilityConfigIsValid(m.Spec.AvailabilityConfig) {
		m.Spec.AvailabilityConfig = operatorv1.HAHigh
		updateNecessary = true
	}

	// Apply defaults to server
	if updateNecessary {
		log.Info("Setting hosted defaults")
		err := r.Client.Update(ctx, m)
		if err != nil {
			log.Error(err, "Failed to update MultiClusterHub")
			return ctrl.Result{}, err
		}
		log.Info("MultiClusterHub successfully updated")
		return ctrl.Result{Requeue: true}, nil
	} else {
		return ctrl.Result{}, nil
	}
}

func (r *MultiClusterHubReconciler) finalizeHostedMCH(ctx context.Context, mch *operatorv1.MultiClusterHub) error {

	if utils.IsUnitTest() {
		return nil
	}
	return nil
}
