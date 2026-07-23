// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine"
	v0 "github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine/olm/v0"
	v1 "github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine/olm/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengineutils"
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *MultiClusterHubReconciler) cleanupClusterRoles(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
	RecordFinalizeProgress(m, FinalizePhaseClusterRolesReason, "Deleting ClusterRoles labeled by this MultiClusterHub install.")
	err := r.Client.DeleteAllOf(context.TODO(), &rbacv1.ClusterRole{}, client.MatchingLabels{
		"installer.name":      m.GetName(),
		"installer.namespace": m.GetNamespace(),
	})

	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("No matching clusterroles to finalize. Continuing.")
			return nil
		}
		reqLogger.Error(err, "Error while deleting clusterroles")
		return err
	}

	reqLogger.Info("Clusterroles finalized")
	return nil
}

func (r *MultiClusterHubReconciler) cleanupClusterRoleBindings(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
	RecordFinalizeProgress(m, FinalizePhaseClusterRoleBindingsReason, "Deleting ClusterRoleBindings labeled by this MultiClusterHub install.")
	err := r.Client.DeleteAllOf(context.TODO(), &rbacv1.ClusterRoleBinding{}, client.MatchingLabels{
		"installer.name":      m.GetName(),
		"installer.namespace": m.GetNamespace(),
	})
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("No matching clusterrolebindings to finalize. Continuing.")
			return nil
		}
		reqLogger.Error(err, "Error while deleting clusterrolebindings")
		return err
	}

	reqLogger.Info("Clusterrolebindings finalized")
	return nil
}

func (r *MultiClusterHubReconciler) cleanupMultiClusterEngine(log logr.Logger, m *operatorsv1.MultiClusterHub) error {
	ctx := context.Background()

	RecordFinalizeProgress(m, FinalizePhaseMultiClusterEngineReason,
		"Removing MultiClusterEngine and related Subscription, CSV, OperatorGroup (when owned by this hub).")

	mce, err := multiclusterengineutils.GetManagedMCE(ctx, r.Client)
	if err != nil && !apimeta.IsNoMatchError(err) {
		RecordFinalizeProgress(m, FinalizePhaseMultiClusterEngineReason,
			fmt.Sprintf("Failed to list/get MultiClusterEngine: %v", err))
		return err
	}
	if mce != nil && !multiclusterengine.MCECreatedByMCH(mce, m) {
		r.Log.Info("Preexisting MCE exists, skipping MCE finalization")
		return nil
	}

	if mce != nil {
		r.Log.Info("Deleting MultiClusterEngine resource")
		err = r.Client.Delete(ctx, mce)
		if err != nil && (!errors.IsNotFound(err) || !errors.IsGone(err)) {
			RecordFinalizeProgress(m, FinalizePhaseMultiClusterEngineReason,
				fmt.Sprintf("Deleting MultiClusterEngine failed: %v", err))
			return err
		}
		RecordFinalizeProgress(m, FinalizePhaseMultiClusterEngineReason,
			"MultiClusterEngine delete requested; waiting for the resource and its dependents to finalize.")
		return fmt.Errorf("MCE has not yet been terminated")
	}

	if utils.IsUnitTest() {
		return nil
	}

	// Clean up OLM resources based on detected OLM version
	operandNs := multiclusterengine.OperandNamespace()
	if r.OLMVersion == "v1" {
		// OLM v1 cleanup path (ClusterExtension + ServiceAccount)
		mceCE, err := v1.GetManagedMCEClusterExtension(ctx, r.Client)
		if err != nil {
			return err
		}

		if mceCE != nil && !v1.CreatedByMCH(mceCE, m) {
			r.Log.Info("Preexisting MCE ClusterExtension exists, skipping finalization")
			return nil
		}

		if mceCE != nil {
			r.Log.Info("Deleting MCE ClusterExtension")
			err = r.Client.Delete(ctx, mceCE)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
			// Check if still exists
			err = r.Client.Get(ctx, types.NamespacedName{Name: mceCE.Name}, mceCE)
			if err == nil {
				return fmt.Errorf("ClusterExtension has not yet been terminated")
			}
		}

		// Delete ServiceAccount
		sa := v1.ServiceAccount(operandNs)
		err = r.Client.Delete(ctx, sa)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}

	} else if r.OLMVersion == "v0" {
		// OLM v0 cleanup path (Subscription + CSV + OperatorGroup)
		mceSub, err := v0.GetManagedMCESubscription(ctx, r.Client)
		if err != nil {
			return err
		}

		if mceSub != nil && !v0.CreatedByMCH(mceSub, m) {
			r.Log.Info("Preexisting MCE subscription exists, skipping MCE subscription finalization")
			return nil
		}

		if mceSub != nil {
			csv, err := r.GetCSVFromSubscription(mceSub)
			namespace := multiclusterengine.OperandNamespace()
			if err == nil { // CSV Exists
				err = r.Client.Delete(ctx, csv)
				if err != nil && !errors.IsNotFound(err) {
					return err
				}
				err = r.Client.Get(ctx,
					types.NamespacedName{Name: csv.GetName(), Namespace: namespace},
					csv)
				if err == nil {
					return fmt.Errorf("CSV has not yet been terminated")
				}
			}

			err = r.Client.Get(ctx,
				types.NamespacedName{Name: mceSub.Name, Namespace: namespace},
				&subv1alpha1.Subscription{})
			if err == nil {

				err = r.Client.Delete(ctx, mceSub)
				if err != nil && !errors.IsNotFound(err) {
					return err
				}
				return fmt.Errorf("subscription has not yet been terminated")
			}
		}

		// Delete OperatorGroup (v0 only)
		err = r.Client.Delete(ctx, v0.OperatorGroup(operandNs))
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}
	// If OLMVersion is "", skip OLM resource cleanup (MCE managed externally)

	mceNamespace := &corev1.Namespace{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: multiclusterengine.Namespace().Name}, mceNamespace)
	if m.Namespace != multiclusterengine.Namespace().Name {
		if err == nil {
			err = r.Client.Delete(ctx, multiclusterengine.Namespace())
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
			RecordFinalizeProgress(m, FinalizePhaseMultiClusterEngineReason,
				"Waiting for multicluster-engine namespace to terminate.")
			return fmt.Errorf("namespace has not yet been terminated")
		}
	} else {
		r.Log.Info("MCE shares namespace with MCH; skipping namespace termination")
	}

	log.Info("MultiClusterEngine finalized")
	return nil
}
func (r *MultiClusterHubReconciler) cleanupNamespaces(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
	ctx := context.Background()
	RecordFinalizeProgress(m, FinalizePhaseClusterBackupNamespaceReason,
		"Cleaning up hub-managed namespaces (cluster backup subscription namespace when present).")
	clusterBackupNamespace := &corev1.Namespace{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: utils.ClusterSubscriptionNamespace}, clusterBackupNamespace)
	if err == nil {
		err = r.Client.Delete(ctx, clusterBackupNamespace)
		if err != nil && !errors.IsNotFound(err) {
			RecordFinalizeProgress(m, FinalizePhaseClusterBackupNamespaceReason,
				fmt.Sprintf("Deleting cluster backup namespace failed: %v", err))
			return err
		}
		RecordFinalizeProgress(m, FinalizePhaseClusterBackupNamespaceReason,
			"Waiting for cluster backup subscription namespace to terminate.")
		return fmt.Errorf("namespace has not yet been terminated")
	}

	return nil
}
func (r *MultiClusterHubReconciler) cleanupAppSubscriptions(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
	RecordFinalizeProgress(m, FinalizePhaseApplicationSubscriptionsReason,
		"Listing and removing openshift-multicluster Application Subscription and HelmRelease resources for this hub.")
	installerLabels := client.MatchingLabels{
		"installer.name":      m.GetName(),
		"installer.namespace": m.GetNamespace(),
	}

	appSubList := &unstructured.UnstructuredList{}
	appSubList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps.open-cluster-management.io",
		Kind:    "SubscriptionList",
		Version: "v1",
	})

	helmReleaseList := &unstructured.UnstructuredList{}
	helmReleaseList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps.open-cluster-management.io",
		Kind:    "HelmReleaseList",
		Version: "v1",
	})

	err := r.Client.List(context.TODO(), appSubList, installerLabels)
	if err != nil && !errors.IsNotFound(err) {
		reqLogger.Error(err, "Error while listing appsubs")
		RecordFinalizeProgress(m, FinalizePhaseApplicationSubscriptionsReason,
			fmt.Sprintf("Failed to list Application Subscriptions for uninstall: %v", err))
		return err
	}

	err = r.Client.List(context.TODO(), helmReleaseList, installerLabels)
	if err != nil && !errors.IsNotFound(err) {
		reqLogger.Error(err, "Error while listing helmreleases")
		RecordFinalizeProgress(m, FinalizePhaseApplicationSubscriptionsReason,
			fmt.Sprintf("Failed to list HelmRelease resources for uninstall: %v", err))
		return err
	}

	// If there are more appsubs with our installer label than helmreleases, update helmreleases
	if len(appSubList.Items) > len(helmReleaseList.Items) {
		for _, appsub := range appSubList.Items {
			helmReleaseName := fmt.Sprintf("%s-%s", strings.Replace(appsub.GetName(), "-sub", "", 1), appsub.GetUID()[0:5])

			helmRelease := &unstructured.Unstructured{}
			helmRelease.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "apps.open-cluster-management.io",
				Kind:    "HelmRelease",
				Version: "v1",
			})

			err = r.Client.Get(context.TODO(), types.NamespacedName{
				Name:      helmReleaseName,
				Namespace: appsub.GetNamespace(),
			}, helmRelease)
			if err != nil {
				if errors.IsNotFound(err) {
					reqLogger.Info(fmt.Sprintf("Unable to locate helmrelease: %s", helmReleaseName))
					continue
				}
				reqLogger.Error(err, fmt.Sprintf("Error getting helmrelease: %s", helmReleaseName))
				RecordFinalizeProgress(m, FinalizePhaseApplicationSubscriptionsReason,
					fmt.Sprintf("Preparing HelmRelease %s failed: %v", helmReleaseName, err))
				return err
			}

			utils.AddInstallerLabel(helmRelease, m.GetName(), m.GetNamespace())
			err = r.Client.Update(context.TODO(), helmRelease)
			if err != nil {
				reqLogger.Error(err, fmt.Sprintf("Error updating helmrelease: %s", helmReleaseName))
				RecordFinalizeProgress(m, FinalizePhaseApplicationSubscriptionsReason,
					fmt.Sprintf("Labeling HelmRelease %s failed: %v", helmReleaseName, err))
				return err
			}
		}
	}

	if len(appSubList.Items) > 0 {
		reqLogger.Info("Terminating App Subscriptions")
		for i, appsub := range appSubList.Items {
			err = r.Client.Delete(context.TODO(), &appSubList.Items[i])
			if err != nil {
				reqLogger.Error(err, fmt.Sprintf("Error terminating sub: %s", appsub.GetName()))
				RecordFinalizeProgress(m, FinalizePhaseApplicationSubscriptionsReason,
					fmt.Sprintf("Deleting Application Subscription %s failed: %v", appsub.GetName(), err))
				return err
			}
		}
	}

	if len(appSubList.Items) != 0 || len(helmReleaseList.Items) != 0 {
		reqLogger.Info("Waiting for helmreleases to be terminated")
		waitDetail := fmt.Sprintf("Waiting for %d Application Subscription(s) and %d HelmRelease resource(s) to finish terminating.",
			len(appSubList.Items), len(helmReleaseList.Items))
		RecordFinalizeProgress(m, FinalizePhaseApplicationSubscriptionsReason, waitDetail)
		waiting := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, HelmReleaseTerminatingReason, "Waiting for helmreleases to terminate.")
		SetHubCondition(&m.Status, *waiting)
		return fmt.Errorf("waiting for helmreleases to be terminated")
	}

	reqLogger.Info("All helmreleases have been terminated")
	return nil
}

func (r *MultiClusterHubReconciler) orphanOwnedMultiClusterEngine(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
	ctx := context.Background()

	RecordFinalizeProgress(m, FinalizePhaseOrphanMultiClusterEngineReason,
		"If a pre-existing MultiClusterEngine is present, remove MCH ownership finalizer and managed-by labels.")

	mce, err := multiclusterengineutils.GetManagedMCE(ctx, r.Client)
	if mce == nil {
		// MCE does not exist
		return nil
	}
	if err != nil {
		if apimeta.IsNoMatchError(err) {
			// MCE does not exist
			return nil
		}
		return err
	}

	r.Log.Info("Preexisting MCE exists, orphaning resource")
	controllerutil.RemoveFinalizer(mce, hubFinalizer)
	labels := mce.GetLabels()
	delete(labels, multiclusterengineutils.MCEManagedByLabel)
	mce.SetLabels(labels)
	if err = r.Client.Update(ctx, mce); err != nil {
		RecordFinalizeProgress(m, FinalizePhaseOrphanMultiClusterEngineReason,
			fmt.Sprintf("Updating pre-existing MultiClusterEngine to orphan from MCH failed: %v", err))
		return err
	}
	r.Log.Info("MCE orphaned")
	return nil
}

func BackupNamespace() *corev1.Namespace {
	return &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			APIVersion: corev1.SchemeGroupVersion.String(),
			Kind:       "Namespace",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: utils.ClusterSubscriptionNamespace,
		},
	}
}

func BackupNamespaceUnstructured() *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{Group: "", Kind: "Namespace", Version: "v1"})
	u.SetName(utils.ClusterSubscriptionNamespace)
	return u
}
