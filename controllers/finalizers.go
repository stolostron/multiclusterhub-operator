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
	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/types"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

func (r *MultiClusterHubReconciler) cleanupAPIServices(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
	err := r.Client.DeleteAllOf(
		context.TODO(),
		&apiregistrationv1.APIService{},
		client.MatchingLabels{
			"installer.name":      m.GetName(),
			"installer.namespace": m.GetNamespace(),
		},
	)

	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("No matching API services to finalize. Continuing.")
			return nil
		}
		reqLogger.Error(err, "Error while deleting API services")
		return err
	}

	reqLogger.Info("API services finalized")
	return nil
}

func (r *MultiClusterHubReconciler) cleanupClusterRoles(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
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

func (r *MultiClusterHubReconciler) cleanupPullSecret(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
	// TODO: Handle scenario where ImagePullSecret is changed after install
	if m.Spec.ImagePullSecret == "" {
		reqLogger.Info("No ImagePullSecret to cleanup. Continuing.")
		return nil
	}

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: utils.CertManagerNamespace,
			Name:      m.Spec.ImagePullSecret,
		},
	}

	err := r.Client.Delete(context.TODO(), secret)
	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("No matching secret to finalize. Continuing.")
			return nil
		}
		reqLogger.Error(err, "Error while deleting secret")
		return err
	}

	reqLogger.Info(fmt.Sprintf("%s secret finalized", utils.CertManagerNS(m)))
	return nil
}

func (r *MultiClusterHubReconciler) cleanupCRDs(log logr.Logger, m *operatorsv1.MultiClusterHub) error {
	err := r.Client.DeleteAllOf(
		context.TODO(),
		&apixv1.CustomResourceDefinition{},
		client.MatchingLabels{
			"installer.name":      m.GetName(),
			"installer.namespace": m.GetNamespace(),
		},
	)

	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("No matching CRDs to finalize. Continuing.")
			return nil
		}
		log.Error(err, "Error while deleting CRDs")
		return err
	}

	log.Info("CRDs finalized")
	return nil
}

func (r *MultiClusterHubReconciler) cleanupMultiClusterEngine(log logr.Logger, m *operatorsv1.MultiClusterHub) error {
	ctx := context.Background()

	mce, err := multiclusterengine.GetManagedMCE(ctx, r.Client)
	if err != nil {
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
		return fmt.Errorf("MCE has not yet been terminated")
	}

	if utils.IsUnitTest() {
		return nil
	}

	mceSub, err := multiclusterengine.GetManagedMCESubscription(ctx, r.Client)
	if err != nil {
		return err
	}
	if mceSub != nil && !multiclusterengine.CreatedByMCH(mceSub, m) {
		r.Log.Info("Preexisting MCE subscription exists, skipping MCE subscription finalization")
		return nil
	}

	if mceSub != nil {
		csv, err := r.GetCSVFromSubscription(mceSub)
		if err == nil { // CSV Exists
			err = r.Client.Delete(ctx, csv)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
			err = r.Client.Get(ctx,
				types.NamespacedName{Name: csv.GetName(), Namespace: utils.MCESubscriptionNamespace},
				csv)
			if err == nil {
				return fmt.Errorf("CSV has not yet been terminated")
			}
		}

		err = r.Client.Get(ctx,
			types.NamespacedName{Name: mceSub.Name, Namespace: mceSub.Namespace},
			&subv1alpha1.Subscription{})
		if err == nil {

			err = r.Client.Delete(ctx, mceSub)
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
			return fmt.Errorf("subscription has not yet been terminated")
		}
	}

	err = r.Client.Delete(ctx, multiclusterengine.OperatorGroup())
	if err != nil && !errors.IsNotFound(err) {
		return err
	}

	mceNamespace := &corev1.Namespace{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: multiclusterengine.Namespace().Name}, mceNamespace)
	if m.Namespace != multiclusterengine.Namespace().Name {
		if err == nil {
			err = r.Client.Delete(ctx, multiclusterengine.Namespace())
			if err != nil && !errors.IsNotFound(err) {
				return err
			}
			return fmt.Errorf("namespace has not yet been terminated")
		}
	} else {
		r.Log.Info("MCE shares namespace with MCH; skipping namespace termination")
	}

	return nil
}
func (r *MultiClusterHubReconciler) cleanupNamespaces(reqLogger logr.Logger) error {
	ctx := context.Background()
	clusterBackupNamespace := &corev1.Namespace{}
	err := r.Client.Get(ctx, types.NamespacedName{Name: utils.ClusterSubscriptionNamespace}, clusterBackupNamespace)
	if err == nil {
		err = r.Client.Delete(ctx, clusterBackupNamespace)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		return fmt.Errorf("namespace has not yet been terminated")
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
		reqLogger.Error(err, "Error while listing appsubs")
		return err
	}

	err = r.Client.List(context.TODO(), helmReleaseList, installerLabels)
	if err != nil && !errors.IsNotFound(err) {
		reqLogger.Error(err, "Error while listing helmreleases")
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
				return err
			}

			utils.AddInstallerLabel(helmRelease, m.GetName(), m.GetNamespace())
			err = r.Client.Update(context.TODO(), helmRelease)
			if err != nil {
				reqLogger.Error(err, fmt.Sprintf("Error updating helmrelease: %s", helmReleaseName))
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
				return err
			}
		}
	}

	if len(appSubList.Items) != 0 || len(helmReleaseList.Items) != 0 {
		reqLogger.Info("Waiting for helmreleases to be terminated")
		waiting := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, HelmReleaseTerminatingReason, "Waiting for helmreleases to terminate.")
		SetHubCondition(&m.Status, *waiting)
		return fmt.Errorf("Waiting for helmreleases to be terminated")
	}

	reqLogger.Info("All helmreleases have been terminated")
	return nil
}

func (r *MultiClusterHubReconciler) cleanupFoundation(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {

	reqLogger.Info("All foundation artefacts have been terminated")

	return nil
}

func (r *MultiClusterHubReconciler) orphanOwnedMultiClusterEngine(m *operatorsv1.MultiClusterHub) error {
	ctx := context.Background()

	mce, err := multiclusterengine.GetManagedMCE(ctx, r.Client)
	if err != nil {
		return err
	}
	if mce == nil {
		// MCE does not exist
		return nil
	}

	r.Log.Info("Preexisting MCE exists, orphaning resource")
	controllerutil.RemoveFinalizer(mce, hubFinalizer)
	labels := mce.GetLabels()
	delete(labels, utils.MCEManagedByLabel)
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
