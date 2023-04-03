// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"errors"
	"fmt"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"
	"github.com/stolostron/multiclusterhub-operator/pkg/version"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var ErrBadFormat = errors.New("bad format")

func (r *MultiClusterHubReconciler) HostedReconcile(ctx context.Context, mch *operatorv1.MultiClusterHub) (retRes ctrl.Result, retErr error) {

	trackedNamespaces := utils.TrackedNamespaces(mch)

	allDeploys, err := r.listDeployments(trackedNamespaces)
	if err != nil {
		return ctrl.Result{}, err
	}

	allCRs, err := r.listHostedCustomResources(mch)
	if err != nil {
		return ctrl.Result{}, err
	}

	ocpConsole, err := r.CheckConsole(ctx)
	if err != nil {
		r.Log.Error(err, "error finding OCP Console")
		return ctrl.Result{}, err
	}

	originalStatus := mch.Status.DeepCopy()
	defer func() {
		statusQueue, statusError := r.syncHostedHubStatus(mch, originalStatus, allDeploys, allCRs, ocpConsole) //1
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

	isHubMarkedToBeDeleted := mch.GetDeletionTimestamp() != nil
	if isHubMarkedToBeDeleted {
		terminating := NewHubCondition(operatorv1.Terminating, metav1.ConditionTrue, DeleteTimestampReason, "Multiclusterhub is being cleaned up.")
		SetHubCondition(&mch.Status, *terminating)

		if controllerutil.ContainsFinalizer(mch, hubFinalizer) {
			// Run finalization logic. If the finalization
			// logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.finalizeHostedMCH(r.Log, mch); err != nil {
				// Logging err and returning nil to ensure 45 second wait
				r.Log.Info(fmt.Sprintf("Finalizing: %s", err.Error()))
				return ctrl.Result{RequeueAfter: resyncPeriod}, nil
			}

			// Remove hubFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			controllerutil.RemoveFinalizer(mch, hubFinalizer)

			err := r.Client.Update(context.TODO(), mch)
			if err != nil {
				return ctrl.Result{}, err
			}
		}

		return ctrl.Result{}, nil
	}

	// Add finalizer for this CR
	if !controllerutil.ContainsFinalizer(mch, hubFinalizer) {
		controllerutil.AddFinalizer(mch, hubFinalizer)
		if err := r.Client.Update(ctx, mch); err != nil {
			return ctrl.Result{}, err
		}
	}

	var result ctrl.Result

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

	if !utils.ShouldIgnoreOCPVersion(mch) {
		currentOCPVersion, err := r.getClusterVersion(ctx)
		if err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to detect clusterversion: %w", err)
		}
		if err := version.ValidOCPVersion(currentOCPVersion); err != nil {
			condition := NewHubCondition(operatorv1.Progressing, metav1.ConditionFalse, RequirementsNotMetReason, fmt.Sprintf("OCP version requirement not met: %s", err.Error()))
			SetHubCondition(&mch.Status, *condition)
			return ctrl.Result{}, err
		}
	}

	result, err = r.ensureHostedMultiClusterEngine(ctx, mch)
	if result != (ctrl.Result{}) {
		return result, err
	}

	result, err = r.waitForHostedMCEReady(ctx, mch)
	if result != (ctrl.Result{}) {
		return result, err
	}

	err = r.Client.Status().Update(ctx, mch)
	if err != nil {
		return ctrl.Result{RequeueAfter: resyncPeriod}, err
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

func (r *MultiClusterHubReconciler) finalizeHostedMCH(reqLogger logr.Logger, m *operatorv1.MultiClusterHub) error {

	if utils.IsUnitTest() {
		return nil
	}

	if err := r.cleanupClusterRoles(reqLogger, m); err != nil {
		return err
	}
	if err := r.cleanupClusterRoleBindings(reqLogger, m); err != nil {
		return err
	}

	if err := r.cleanupHostedMultiClusterEngine(reqLogger, m); err != nil {
		return err
	}

	if err := r.orphanOwnedMultiClusterEngine(m); err != nil {
		return err
	}

	reqLogger.Info("Successfully finalized multiClusterHub")
	return nil
}
