// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/types"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"
	"github.com/stolostron/multiclusterhub-operator/pkg/version"

	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

func (r *MultiClusterHubReconciler) HostedReconcile(ctx context.Context, mch *operatorv1.MultiClusterHub) (retRes ctrl.Result, retErr error) {
	log := log.FromContext(ctx)

	defer func() {
		statusQueue, statusError := r.updateHostedHubStatus(mch) //1
		if statusError != nil {
			r.Log.Error(retErr, "Error updating status")
		}
		if empty := (reconcile.Result{}); retRes == empty {
			retRes = statusQueue
		}
		if retErr == nil {
			retErr = statusError
		}
	}()

	// If deletion detected, finalize backplane config
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
	updatePausedCondition(mch)
	if utils.IsPaused(mch) {
		r.Log.Info("MultiClusterHub reconciliation is paused. Nothing more to do.")
		return ctrl.Result{}, nil
	}

	result, err = r.ensureHostedMultiClusterEngine(ctx, mch)
	if result != (ctrl.Result{}) {
		return result, err
	}

	return ctrl.Result{}, nil
}

// setHostedDefaults configures the MCH with default values and updates
func (r *MultiClusterHubReconciler) setHostedDefaults(ctx context.Context, m *operatorv1.MultiClusterHub) (ctrl.Result, error) {
	log := log.FromContext(ctx)

	updateNecessary := false
	if !utils.AvailabilityConfigIsValid(m.Spec.AvailabilityConfig) {
		m.Spec.AvailabilityConfig = operatorv1.HAHigh
		updateNecessary = true
	}

	if utils.SetHostedDefaultComponents(m) {
		updateNecessary = true
	}

	if utils.DeduplicateComponents(m) {
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

func (r *MultiClusterHubReconciler) finalizeHostedMCH(ctx context.Context, m *operatorv1.MultiClusterHub) error {
	if err := r.ensureNoHostedMultiClusterEngineCR(ctx, m); err != nil {
		return err
	}
	return nil
}

func (r *MultiClusterHubReconciler) ensureHostedMultiClusterEngine(ctx context.Context, m *operatorv1.MultiClusterHub) (ctrl.Result, error) {

	mceTargetNamespace := multiclusterengine.HostedMCENamespace(m)
	// ensure targetnamespace
	result, err := r.ensureNamespace(m, mceTargetNamespace)
	if result != (ctrl.Result{}) {
		return result, err
	}
	// ensure imagepullsecret
	if m.Spec.ImagePullSecret != "" {
		result, err = r.ensurePullSecret(m, mceTargetNamespace.Name)
		if result != (ctrl.Result{}) {
			return result, err
		}
	}
	// ensure hosted kubeconfig secret
	result, err = r.ensureHostedKubeconfigSecret(m, mceTargetNamespace.Name)
	if result != (ctrl.Result{}) {
		return result, err
	}

	result, err = r.ensureHostedMultiClusterEngineCR(ctx, m)
	if result != (ctrl.Result{}) {
		return result, err
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureHostedMultiClusterEngineCR(ctx context.Context, m *operatorv1.MultiClusterHub) (ctrl.Result, error) {
	mce, err := multiclusterengine.GetHostedMCE(ctx, r.Client, m)
	if err != nil {
		return ctrl.Result{}, err
	}

	if mce == nil {
		mce = multiclusterengine.NewHostedMultiClusterEngine(m)
		err = r.Client.Create(ctx, mce)
		if err != nil {
			return ctrl.Result{Requeue: true}, fmt.Errorf("Error creating new MCE: %w", err)
		}
		return ctrl.Result{}, nil
	}

	// secret should be delivered to targetNamespace
	if mce.Spec.TargetNamespace == "" {
		return ctrl.Result{Requeue: true}, fmt.Errorf("MCE %s does not have a target namespace to apply pullsecret", mce.Name)
	}

	calcMCE := multiclusterengine.RenderHostedMultiClusterEngine(mce, m)
	err = r.Client.Update(ctx, calcMCE)
	if err != nil {
		return ctrl.Result{Requeue: true}, fmt.Errorf("Error updating MCE %s: %w", mce.Name, err)
	}
	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureNoHostedMultiClusterEngineCR(ctx context.Context, m *operatorv1.MultiClusterHub) error {
	// Delete MCE
	hostedMCE, err := multiclusterengine.GetHostedMCE(ctx, r.Client, m)
	if err != nil && !apimeta.IsNoMatchError(err) {
		return err
	}
	if hostedMCE != nil {
		r.Log.Info("Deleting MultiClusterEngine resource")
		err = r.Client.Delete(ctx, hostedMCE)
		if err != nil && (!errors.IsNotFound(err) || !errors.IsGone(err)) {
			return err
		}
		return fmt.Errorf("MCE has not yet been terminated")
	}

	// Delete MCE targetNamespace
	// this will clean up any propogated secrets as well
	mceNamespace := &corev1.Namespace{}
	nsName := multiclusterengine.HostedMCENamespace(m).Name
	err = r.Client.Get(ctx, types.NamespacedName{Name: nsName}, mceNamespace)
	if err != nil && !apierrors.IsNotFound(err) {
		return err
	}
	if err == nil {
		err = r.Client.Delete(ctx, mceNamespace)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		return fmt.Errorf("namespace has not yet been terminated")
	}
	return nil
}

// copies the hosted kubeconfig secret from mch to the newNS namespace
func (r *MultiClusterHubReconciler) ensureHostedKubeconfigSecret(m *operatorv1.MultiClusterHub, newNS string) (ctrl.Result, error) {
	if m.Annotations == nil || m.Annotations[utils.AnnotationKubeconfig] == "" {
		return ctrl.Result{}, fmt.Errorf("Kubeconfig annotation missing from hosted MCH")
	}
	secretName := m.Annotations[utils.AnnotationKubeconfig]

	kubeSecret := &corev1.Secret{}
	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      secretName,
		Namespace: m.Namespace,
	}, kubeSecret)
	if err != nil {
		return ctrl.Result{Requeue: true}, err
	}

	mceSecret := &corev1.Secret{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Secret",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      kubeSecret.Name,
			Namespace: newNS,
			Labels:    kubeSecret.Labels,
		},
		Data: kubeSecret.Data,
	}
	mceSecret.SetName(secretName)
	mceSecret.SetNamespace(newNS)
	mceSecret.SetLabels(kubeSecret.Labels)
	addInstallerLabelSecret(mceSecret, m.Name, m.Namespace)

	force := true
	err = r.Client.Patch(context.TODO(), mceSecret, client.Apply, &client.PatchOptions{Force: &force, FieldManager: "multiclusterhub-operator"})
	if err != nil {
		r.Log.Info(fmt.Sprintf("Error applying hosted kubeconfig secret to mce namespace: %s", err.Error()))
		return ctrl.Result{Requeue: true}, err
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) updateHostedHubStatus(m *operatorv1.MultiClusterHub) (reconcile.Result, error) {
	newStatus := r.calculateHostedStatus(m)

	newHub := m
	newHub.Status = newStatus
	err := r.Client.Status().Update(context.TODO(), newHub)
	if err != nil {
		if errors.IsConflict(err) {
			// Error from object being modified is normal behavior and should not be treated like an error
			return reconcile.Result{RequeueAfter: resyncPeriod}, nil
		}

		r.Log.Error(err, fmt.Sprintf("Failed to update %s/%s status ", m.Namespace, m.Name))
		return reconcile.Result{}, err
	}

	if m.Status.Phase != operatorv1.HubRunning {
		return reconcile.Result{RequeueAfter: resyncPeriod}, nil
	} else {
		return reconcile.Result{}, nil
	}
}

func (r *MultiClusterHubReconciler) calculateHostedStatus(m *operatorv1.MultiClusterHub) operatorv1.MultiClusterHubStatus {
	var mce *unstructured.Unstructured
	gotMCE, err := multiclusterengine.GetHostedMCE(context.Background(), r.Client, m)
	if err != nil || gotMCE == nil {
		mce = nil
	} else {
		unstructuredMCE, err := runtime.DefaultUnstructuredConverter.ToUnstructured(gotMCE)
		if err != nil {
			r.Log.Error(err, "Failed to unmarshal MCE")
		}
		mce = &unstructured.Unstructured{Object: unstructuredMCE}
	}

	components := map[string]operatorv1.StatusCondition{
		"multicluster-engine": mapMultiClusterEngine(mce),
	}

	status := operatorv1.MultiClusterHubStatus{
		CurrentVersion: m.Status.CurrentVersion,
		DesiredVersion: version.Version,
		Components:     components,
	}

	// Set current version
	successful := allComponentsSuccessful(components)
	if successful {
		status.CurrentVersion = version.Version
	}

	// Copy conditions one by one to not affect original object
	conditions := m.Status.HubConditions
	for i := range conditions {
		status.HubConditions = append(status.HubConditions, conditions[i])
	}

	// Update hub conditions
	if successful {
		// don't label as complete until component pruning succeeds
		if !hubPruning(status) {
			available := NewHubCondition(operatorv1.Complete, v1.ConditionTrue, ComponentsAvailableReason, "All hub components ready.")
			SetHubCondition(&status, *available)
		} else {
			// only add unavailable status if complete status already present
			if HubConditionPresent(status, operatorv1.Complete) {
				unavailable := NewHubCondition(operatorv1.Complete, v1.ConditionFalse, OldComponentNotRemovedReason, "Not all components successfully pruned.")
				SetHubCondition(&status, *unavailable)
			}
		}
	} else {
		// hub is progressing unless otherwise specified
		if !HubConditionPresent(status, operatorv1.Progressing) {
			progressing := NewHubCondition(operatorv1.Progressing, v1.ConditionTrue, ReconcileReason, "Hub is reconciling.")
			SetHubCondition(&status, *progressing)
		}
		// only add unavailable status if complete status already present
		if HubConditionPresent(status, operatorv1.Complete) {
			unavailable := NewHubCondition(operatorv1.Complete, v1.ConditionFalse, ComponentsUnavailableReason, "Not all hub components ready.")
			SetHubCondition(&status, *unavailable)
		}
	}

	// Set overall phase
	isHubMarkedToBeDeleted := m.GetDeletionTimestamp() != nil
	if isHubMarkedToBeDeleted {
		// Hub cleaning up
		status.Phase = operatorv1.HubUninstalling
	} else {
		status.Phase = aggregatePhase(status)
	}

	return status
}
