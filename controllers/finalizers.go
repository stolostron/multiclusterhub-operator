// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	mcev1 "github.com/stolostron/backplane-operator/api/v1"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/channel"
	"github.com/stolostron/multiclusterhub-operator/pkg/helmrepo"
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

	managedByMCE, err := r.ManagedByMCEExists()
	if err != nil {
		return err
	}

	if err == nil && managedByMCE != nil {
		// Preexisting MCE exists, no need to terminate resources
		r.Log.Info("Preexisting MCE exists, skipping MCE finalization")
		return nil
	}

	// If no preexisting MCE exists, proceed with finalization of installed MCE and its resources
	existingMCE := &mcev1.MultiClusterEngine{}
	err = r.Client.Get(ctx, types.NamespacedName{Name: multiclusterengine.MulticlusterengineName}, existingMCE)
	if err == nil {
		labels := existingMCE.Labels
		if name, ok := labels["installer.name"]; ok && name == m.GetName() {
			if namespace, ok := labels["installer.namespace"]; ok && namespace == m.GetNamespace() {
				// MCE is installed by the MCH, no need to manage. Return
				r.Log.Info("Deleting MultiClusterEngine resources")
				err := r.Client.Delete(ctx, multiclusterengine.MultiClusterEngine(m))
				if err != nil && (!errors.IsNotFound(err) || !errors.IsGone(err)) {
					return err
				}
				return fmt.Errorf("MCE has not yet been terminated")
			}
		} else {
			r.Log.Info("MCE is not managed by this MCH, skipping MCE finalization")
			return nil
		}

	}
	// subConfig = &subv1alpha1.SubscriptionConfig{}
	subConfig, err := r.GetSubConfig()
	if err != nil {
		return err
	}

	if utils.IsUnitTest() {
		return nil
	}

	community, err := operatorsv1.IsCommunity()
	if err != nil {
		return err
	}

	csv, err := r.GetCSVFromSubscription(multiclusterengine.Subscription(m, subConfig, community))
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
		types.NamespacedName{Name: utils.MCESubscriptionName, Namespace: utils.MCESubscriptionNamespace},
		&subv1alpha1.Subscription{})
	if err == nil {

		err = r.Client.Delete(ctx, multiclusterengine.Subscription(m, subConfig, community))
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
		return fmt.Errorf("subscription has not yet been terminated")
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

	var emptyOverrides map[string]string

	reqLogger.Info("Deleting MultiClusterHub repo deployment")
	err := r.Client.Delete(context.TODO(), helmrepo.Deployment(m, emptyOverrides))
	if err != nil && !errors.IsNotFound(err) {
		reqLogger.Error(err, "Error deleting MultiClusterHub repo deployment")
		return err
	}

	reqLogger.Info("Deleting MultiClusterHub repo service")
	err = r.Client.Delete(context.TODO(), helmrepo.Service(m))
	if err != nil && !errors.IsNotFound(err) {
		reqLogger.Error(err, "Error deleting MultiClusterHub repo service")
		return err
	}

	reqLogger.Info("Deleting MultiClusterHub channel")
	err = r.Client.Delete(context.TODO(), channel.Channel(m))
	if err != nil && !errors.IsNotFound(err) {
		reqLogger.Error(err, "Error deleting MultiClusterHub channel")
		return err
	}

	reqLogger.Info("All foundation artefacts have been terminated")

	return nil
}

func (r *MultiClusterHubReconciler) orphanOwnedMultiClusterEngine(m *operatorsv1.MultiClusterHub) error {
	ctx := context.Background()

	managedByMCE, err := r.ManagedByMCEExists()
	if err != nil {
		return err
	}
	if managedByMCE == nil {
		// MCE does not exist
		return nil
	}
	r.Log.Info("Preexisting MCE exists, orphaning resource")
	controllerutil.RemoveFinalizer(managedByMCE, hubFinalizer)
	labels := managedByMCE.GetLabels()
	delete(labels, utils.MCEManagedByLabel)
	managedByMCE.SetLabels(labels)
	if err = r.Client.Update(ctx, managedByMCE); err != nil {
		return err
	}
	r.Log.Info("MCE orphaned")
	return nil
}
