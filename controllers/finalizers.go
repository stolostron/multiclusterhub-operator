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
	err := r.Client.DeleteAllOf(context.TODO(), &rbacv1.ClusterRole{}, client.MatchingLabels{
		"installer.name":      m.GetName(),
		"installer.namespace": m.GetNamespace(),
	})

	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("No matching ClusterRoles to finalize")
			return nil
		}
		reqLogger.Error(err, "Failed to delete ClusterRoles")
		return err
	}

	reqLogger.Info("ClusterRoles finalized")
	return nil
}

func (r *MultiClusterHubReconciler) cleanupClusterRoleBindings(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
	err := r.Client.DeleteAllOf(context.TODO(), &rbacv1.ClusterRoleBinding{}, client.MatchingLabels{
		"installer.name":      m.GetName(),
		"installer.namespace": m.GetNamespace(),
	})
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("No matching ClusterRoleBindings to finalize")
			return nil
		}
		reqLogger.Error(err, "Failed to delete ClusterRoleBindings")
		return err
	}

	reqLogger.Info("ClusterRoleBindings finalized")
	return nil
}

func (r *MultiClusterHubReconciler) cleanupMultiClusterEngine(log logr.Logger, m *operatorsv1.MultiClusterHub) error {
	ctx := context.Background()

	mce, err := multiclusterengineutils.GetManagedMCE(ctx, r.Client)
	if err != nil && !apimeta.IsNoMatchError(err) {
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
			return err
		}
		return fmt.Errorf("MultiClusterEngine %s has not yet been terminated", mce.GetName())
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
				return fmt.Errorf("ClusterExtension %s has not yet been terminated", mceCE.Name)
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
					return fmt.Errorf("CSV %s has not yet been terminated", csv.GetName())
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
				return fmt.Errorf("Subscription %s has not yet been terminated", mceSub.Name)
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
			return fmt.Errorf("namespace %s has not yet been terminated", multiclusterengine.Namespace().Name)
		}
	} else {
		r.Log.Info("MCE shares namespace with MCH; skipping namespace termination")
	}

	log.Info("MultiClusterEngine finalized")
	return nil
}
func (r *MultiClusterHubReconciler) cleanupNamespaces(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
	ctx := context.Background()
	clusterBackupNamespace := &corev1.Namespace{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: utils.ClusterSubscriptionNamespace}, clusterBackupNamespace)
	if err == nil {
		err = r.Client.Delete(ctx, clusterBackupNamespace)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		return fmt.Errorf("namespace %s has not yet been terminated", utils.ClusterSubscriptionNamespace)
	}

	return nil
}
func (r *MultiClusterHubReconciler) cleanupAppSubscriptions(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
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
		reqLogger.Error(err, "Failed to list Subscriptions", "labels", installerLabels)
		return err
	}

	err = r.Client.List(context.TODO(), helmReleaseList, installerLabels)
	if err != nil && !errors.IsNotFound(err) {
		reqLogger.Error(err, "Failed to list HelmReleases", "labels", installerLabels)
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
					reqLogger.Info("Unable to locate HelmRelease", "name", helmReleaseName)
					continue
				}
				reqLogger.Error(err, "Failed to get HelmRelease", "name", helmReleaseName)
				return err
			}

			utils.AddInstallerLabel(helmRelease, m.GetName(), m.GetNamespace())
			err = r.Client.Update(context.TODO(), helmRelease)
			if err != nil {
				reqLogger.Error(err, "Failed to update HelmRelease", "name", helmReleaseName)
				return err
			}
		}
	}

	if len(appSubList.Items) > 0 {
		reqLogger.Info("Terminating App Subscriptions")
		for i, appsub := range appSubList.Items {
			err = r.Client.Delete(context.TODO(), &appSubList.Items[i])
			if err != nil {
				reqLogger.Error(err, "Failed to terminate Subscription", "name", appsub.GetName())
				return err
			}
		}
	}

	if len(appSubList.Items) != 0 || len(helmReleaseList.Items) != 0 {
		reqLogger.Info("Waiting for sub-components to terminate before finalization",
			"subscriptionCount", len(appSubList.Items),
			"helmReleaseCount", len(helmReleaseList.Items))
		msg := fmt.Sprintf("Waiting for %d Subscriptions and %d HelmReleases to terminate",
			len(appSubList.Items), len(helmReleaseList.Items))
		waiting := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, HelmReleaseTerminatingReason, msg)
		SetHubCondition(&m.Status, *waiting)
		return fmt.Errorf("%s", msg)
	}

	reqLogger.Info("All HelmReleases have been terminated")
	return nil
}

func (r *MultiClusterHubReconciler) orphanOwnedMultiClusterEngine(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
	ctx := context.Background()

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
