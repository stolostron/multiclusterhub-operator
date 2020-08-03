// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	admissionregistrationv1beta1 "k8s.io/api/admissionregistration/v1beta1"
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
		&admissionregistrationv1beta1.MutatingWebhookConfiguration{},
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
		&admissionregistrationv1beta1.ValidatingWebhookConfiguration{},
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

	reqLogger.Info("ValidatingWebhookConfigurations finalized")
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
			reqLogger.Info("No matching ClusterManagers to finalize. Continuing.")
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

	reqLogger.Info("ClusterManagers finalized")
	return nil
}

func (r *ReconcileMultiClusterHub) cleanupAppSubscriptions(reqLogger logr.Logger, m *operatorsv1.MultiClusterHub) error {
	installerLabels := client.MatchingLabels{
		"installer.name":      m.GetName(),
		"installer.namespace": m.GetNamespace(),
	}

	appSubList := &unstructured.UnstructuredList{}
	appSubList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps.open-cluster-management.io",
		Kind:    "Subscription",
		Version: "v1",
	})

	matchingAppSubList := appSubList.DeepCopy()

	helmReleaseList := &unstructured.UnstructuredList{}
	helmReleaseList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps.open-cluster-management.io",
		Kind:    "HelmRelease",
		Version: "v1",
	})

	err := r.client.List(context.TODO(), appSubList)
	if err != nil {
		if err != nil && !errors.IsNotFound(err) {
			reqLogger.Error(err, "Error while listing appsubs")
			return err
		}
	}
	err = r.client.List(context.TODO(), matchingAppSubList, installerLabels)
	if err != nil {
		if !errors.IsNotFound(err) {
			reqLogger.Error(err, "Error while listing appsubs")
			return err
		}
	}

	if len(matchingAppSubList.Items) > 0 {
		reqLogger.Info("Terminating App Subscriptions")
		for _, appsub := range matchingAppSubList.Items {
			err = r.client.Delete(context.TODO(), &appsub)
			if err != nil {
				reqLogger.Error(err, fmt.Sprintf("Error terminating sub: %s", appsub.GetName()))
				return err
			}
		}
	}

	err = r.client.List(context.TODO(), helmReleaseList)
	if err != nil {
		if !errors.IsNotFound(err) {
			reqLogger.Error(err, "Error while listing helmreleases")
			return err
		}
	}

	// Checks to ensure that expected number of helmreleases exist by
	// checking if number of helmreleases equals the total number of appsubs
	// with the operator label subtracted from total appsubs
	if (len(appSubList.Items) - len(matchingAppSubList.Items)) != len(helmReleaseList.Items) {
		reqLogger.Info("Waiting for helmreleases to be terminated")
		return fmt.Errorf("Waiting for helmreleases to be terminated")
	}

	reqLogger.Info("All helmreleases have been terminated")
	return nil
}
