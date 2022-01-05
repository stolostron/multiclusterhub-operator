// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package multiclusterhub

import (
	"context"
	"fmt"
	"strings"

	foundation "github.com/stolostron/multiclusterhub-operator/pkg/foundation"

	"github.com/go-logr/logr"
	operatorsv1 "github.com/stolostron/multiclusterhub-operator/pkg/apis/operator/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/channel"
	"github.com/stolostron/multiclusterhub-operator/pkg/helmrepo"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	apiregistrationv1 "k8s.io/kube-aggregator/pkg/apis/apiregistration/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func (r *ReconcileMultiClusterHub) cleanupHiveConfigs(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {

	listOptions := client.MatchingLabels{
		"installer.name":      m.GetName(),
		"installer.namespace": m.GetNamespace(),
	}

	found := &unstructured.Unstructured{}
	found.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "hive.openshift.io",
		Kind:    "HiveConfig",
		Version: "v1",
	})
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name: "hive",
	}, found)
	if err != nil && errors.IsNotFound(err) {
		// No HiveConfigs. Return nil
		return nil
	}

	// Delete HiveConfig if it exists
	reqLogger.Info("Deleting hiveconfig", "Resource.Name", found.GetName())
	err = r.client.DeleteAllOf(context.TODO(), found, listOptions)
	if err != nil {
		reqLogger.Error(err, "Error while deleting hiveconfig instances")
		return err
	}

	reqLogger.Info("Hiveconfigs finalized")
	return nil
}

func (r *ReconcileMultiClusterHub) cleanupAPIServices(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
	err := r.client.DeleteAllOf(
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

func (r *ReconcileMultiClusterHub) cleanupClusterRoles(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
	err := r.client.DeleteAllOf(context.TODO(), &rbacv1.ClusterRole{}, client.MatchingLabels{
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

func (r *ReconcileMultiClusterHub) cleanupClusterRoleBindings(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
	err := r.client.DeleteAllOf(context.TODO(), &rbacv1.ClusterRoleBinding{}, client.MatchingLabels{
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

func (r *ReconcileMultiClusterHub) cleanupMutatingWebhooks(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
	err := r.client.DeleteAllOf(
		context.TODO(),
		&admissionregistrationv1.MutatingWebhookConfiguration{},
		client.MatchingLabels{
			"installer.name":      m.GetName(),
			"installer.namespace": m.GetNamespace(),
		})

	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("No matching MutatingWebhookConfigurations to finalize. Continuing.")
			return nil
		}
		reqLogger.Error(err, "Error while deleting MutatingWebhookConfigurations")
		return err
	}

	reqLogger.Info("MutatingWebhookConfigurations finalized")
	return nil
}

func (r *ReconcileMultiClusterHub) cleanupValidatingWebhooks(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
	err := r.client.DeleteAllOf(
		context.TODO(),
		&admissionregistrationv1.ValidatingWebhookConfiguration{},
		client.MatchingLabels{
			"installer.name":      m.GetName(),
			"installer.namespace": m.GetNamespace(),
		})

	if err != nil {
		if errors.IsNotFound(err) {
			reqLogger.Info("No matching ValidatingWebhookConfigurations to finalize. Continuing.")
			return nil
		}
		reqLogger.Error(err, "Error while deleting ValidatingWebhookConfigurations")
		return err
	}

	reqLogger.Info("ValidatingWebhookConfiguration finalized")
	return nil
}

func (r *ReconcileMultiClusterHub) cleanupPullSecret(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
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

	err := r.client.Delete(context.TODO(), secret)
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

func (r *ReconcileMultiClusterHub) cleanupCRDs(log logr.Logger, m *operatorsv1.MultiClusterHub) error {
	err := r.client.DeleteAllOf(
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

func (r *ReconcileMultiClusterHub) cleanupClusterManagers(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {

	listOptions := client.MatchingLabels{
		"installer.name":      m.GetName(),
		"installer.namespace": m.GetNamespace(),
	}

	found := &unstructured.Unstructured{}
	found.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operator.open-cluster-management.io",
		Kind:    "ClusterManager",
		Version: "v1",
	})
	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name: "cluster-manager",
	}, found)
	if err != nil {
		if errors.IsNotFound(err) {
			// ClusterManager successfully removed
			reqLogger.Info("ClusterManagers finalized")
			return nil
		}
		reqLogger.Error(err, "Error while deleting ClusterManagers")
		return err
	}

	// Delete ClusterManager if it exists
	reqLogger.Info("Deleting clustermanager", "Resource.Name", found.GetName())
	err = r.client.DeleteAllOf(context.TODO(), found, listOptions)
	if err != nil {
		reqLogger.Error(err, "Error while deleting clustermanager instances")
		return err
	}
	// Return error, since deletion does not confirm the object was removed
	return fmt.Errorf("Attempted deletion of ClusterManager. Unable to confirm if ClusterManager was removed")
}

func (r *ReconcileMultiClusterHub) cleanupAppSubscriptions(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
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

	err := r.client.List(context.TODO(), appSubList, installerLabels)
	if err != nil && !errors.IsNotFound(err) {
		reqLogger.Error(err, "Error while listing appsubs")
		return err
	}

	err = r.client.List(context.TODO(), helmReleaseList, installerLabels)
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

			err = r.client.Get(context.TODO(), types.NamespacedName{
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
			err = r.client.Update(context.TODO(), helmRelease)
			if err != nil {
				reqLogger.Error(err, fmt.Sprintf("Error updating helmrelease: %s", helmReleaseName))
				return err
			}
		}
	}

	if len(appSubList.Items) > 0 {
		reqLogger.Info("Terminating App Subscriptions")
		for i, appsub := range appSubList.Items {
			err = r.client.Delete(context.TODO(), &appSubList.Items[i])
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

func (r *ReconcileMultiClusterHub) cleanupFoundation(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {

	var emptyOverrides map[string]string

	reqLogger.Info("Deleting OCM controller deployment")
	err := r.client.Delete(context.TODO(), foundation.OCMControllerDeployment(m, emptyOverrides))
	if err != nil && !errors.IsNotFound(err) {
		reqLogger.Error(err, "Error deleting OCM controller deployment")
		return err
	}

	reqLogger.Info("Deleting OCM proxy apiService")
	err = r.client.Delete(context.TODO(), foundation.OCMProxyAPIService(m))
	if err != nil && !errors.IsNotFound(err) {
		reqLogger.Error(err, "Error deleting OCM proxy  apiService")
		return err
	}

	reqLogger.Info("Deleting OCM clusterView v1 apiService")
	err = r.client.Delete(context.TODO(), foundation.OCMClusterViewV1APIService(m))
	if err != nil && !errors.IsNotFound(err) {
		reqLogger.Error(err, "Error deleting OCM clusterView v1 apiService")
		return err
	}

	reqLogger.Info("Deleting OCM  clusterView v1alpha1 apiService")
	err = r.client.Delete(context.TODO(), foundation.OCMClusterViewV1alpha1APIService(m))
	if err != nil && !errors.IsNotFound(err) {
		reqLogger.Error(err, "Error deleting OCM clusterView v1alpha1 apiService")
		return err
	}

	reqLogger.Info("Deleting OCM proxy server service")
	err = r.client.Delete(context.TODO(), foundation.OCMProxyServerService(m))
	if err != nil && !errors.IsNotFound(err) {
		reqLogger.Error(err, "Error deleting OCM proxy server service")
		return err
	}

	reqLogger.Info("Deleting OCM proxy server deployment")
	err = r.client.Delete(context.TODO(), foundation.OCMProxyServerDeployment(m, emptyOverrides))
	if err != nil && !errors.IsNotFound(err) {
		reqLogger.Error(err, "Error deleting OCM proxy server deployment")
		return err
	}

	reqLogger.Info("Deleting OCM webhook service")
	err = r.client.Delete(context.TODO(), foundation.WebhookService(m))
	if err != nil && !errors.IsNotFound(err) {
		reqLogger.Error(err, "Error deleting OCM webhook service")
		return err
	}

	reqLogger.Info("Deleting OCM webhook deployment")
	err = r.client.Delete(context.TODO(), foundation.WebhookDeployment(m, emptyOverrides))
	if err != nil && !errors.IsNotFound(err) {
		reqLogger.Error(err, "Error deleting OCM webhook deployment")
		return err
	}

	reqLogger.Info("Deleting MultiClusterHub repo deployment")
	err = r.client.Delete(context.TODO(), helmrepo.Deployment(m, emptyOverrides))
	if err != nil && !errors.IsNotFound(err) {
		reqLogger.Error(err, "Error deleting MultiClusterHub repo deployment")
		return err
	}

	reqLogger.Info("Deleting MultiClusterHub repo service")
	err = r.client.Delete(context.TODO(), helmrepo.Service(m))
	if err != nil && !errors.IsNotFound(err) {
		reqLogger.Error(err, "Error deleting MultiClusterHub repo service")
		return err
	}

	reqLogger.Info("Deleting MultiClusterHub channel")
	err = r.client.Delete(context.TODO(), channel.Channel(m))
	if err != nil && !errors.IsNotFound(err) {
		reqLogger.Error(err, "Error deleting MultiClusterHub channel")
		return err
	}

	reqLogger.Info("All foundation artefacts have been terminated")

	return nil
}
