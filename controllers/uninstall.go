// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"
	"strings"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	MonitoringAPIGroup = "monitoring.coreos.com"
)

var (
	// The uninstallList is the list of all resources from previous installs to remove. Items can be removed
	// from this list in future releases if they are sure to not exist prior to the current installer version
	uninstallList = func(m *operatorsv1.MultiClusterHub) []*unstructured.Unstructured {
		removals := []*unstructured.Unstructured{
			// AI is migrated to MCE in 2.5.0
			newUnstructured(
				types.NamespacedName{Name: "assisted-service-sub", Namespace: m.Namespace},
				schema.GroupVersionKind{Group: "apps.open-cluster-management.io", Kind: "Subscription", Version: "v1"},
			),
			// application-ui is migrated to console starting in 2.5.0
			newUnstructured(
				types.NamespacedName{Name: "application-chart-sub", Namespace: m.Namespace},
				schema.GroupVersionKind{Group: "apps.open-cluster-management.io", Kind: "Subscription", Version: "v1"},
			),
			// charts channel is removed starting in 2.7.0
			newUnstructured(
				types.NamespacedName{Name: "charts-v1", Namespace: m.Namespace},
				schema.GroupVersionKind{Group: "apps.open-cluster-management.io", Kind: "Channel", Version: "v1"},
			),
			// multiclusterhub-repo is removed starting in 2.7.0
			newUnstructured(
				types.NamespacedName{Name: "multiclusterhub-repo", Namespace: m.Namespace},
				schema.GroupVersionKind{Group: "apps", Kind: "Deployment", Version: "v1"},
			),
			newUnstructured(
				types.NamespacedName{Name: "multiclusterhub-repo", Namespace: m.Namespace},
				schema.GroupVersionKind{Group: "", Kind: "Service", Version: "v1"},
			),
			newUnstructured(
				types.NamespacedName{Name: "searchcustomizations.search.open-cluster-management.io"},
				schema.GroupVersionKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition", Version: "v1"},
			),
			newUnstructured(
				types.NamespacedName{Name: "searchoperators.search.open-cluster-management.io"},
				schema.GroupVersionKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition", Version: "v1"},
			),
		}

		if m.Spec.SeparateCertificateManagement && m.Spec.ImagePullSecret != "" {
			removals = append(removals, newUnstructured(
				types.NamespacedName{Name: m.Spec.ImagePullSecret, Namespace: utils.CertManagerNamespace},
				schema.GroupVersionKind{Group: "", Kind: "Secret", Version: "v1"},
			))
		}
		return removals
	}

	appsubCleanupList = func(m *operatorsv1.MultiClusterHub) []*unstructured.Unstructured {
		removals := []*unstructured.Unstructured{
			newUnstructured(
				types.NamespacedName{Name: "policyreport-sub", Namespace: m.Namespace},
				schema.GroupVersionKind{Group: "apps.open-cluster-management.io", Kind: "Subscription", Version: "v1"},
			),
			newUnstructured(
				types.NamespacedName{Name: "cluster-proxy-addon-sub", Namespace: m.Namespace},
				schema.GroupVersionKind{Group: "apps.open-cluster-management.io", Kind: "Subscription", Version: "v1"},
			),
			newUnstructured(
				types.NamespacedName{Name: "search-prod-sub", Namespace: m.Namespace},
				schema.GroupVersionKind{Group: "apps.open-cluster-management.io", Kind: "Subscription", Version: "v1"},
			),
			newUnstructured(
				types.NamespacedName{Name: "cluster-lifecycle-sub", Namespace: m.Namespace},
				schema.GroupVersionKind{Group: "apps.open-cluster-management.io", Kind: "Subscription", Version: "v1"},
			),
			newUnstructured(
				types.NamespacedName{Name: "cluster-backup-chart-sub", Namespace: utils.ClusterSubscriptionNamespace},
				schema.GroupVersionKind{Group: "apps.open-cluster-management.io", Kind: "Subscription", Version: "v1"},
			),
			newUnstructured(
				types.NamespacedName{Name: "grc-sub", Namespace: m.Namespace},
				schema.GroupVersionKind{Group: "apps.open-cluster-management.io", Kind: "Subscription", Version: "v1"},
			),
			newUnstructured(
				types.NamespacedName{Name: "console-chart-sub", Namespace: m.Namespace},
				schema.GroupVersionKind{Group: "apps.open-cluster-management.io", Kind: "Subscription", Version: "v1"},
			),
			newUnstructured(
				types.NamespacedName{Name: "volsync-addon-controller-sub", Namespace: m.Namespace},
				schema.GroupVersionKind{Group: "apps.open-cluster-management.io", Kind: "Subscription", Version: "v1"},
			),
			newUnstructured(
				types.NamespacedName{Name: "management-ingress-sub", Namespace: m.Namespace},
				schema.GroupVersionKind{Group: "apps.open-cluster-management.io", Kind: "Subscription", Version: "v1"},
			),
		}
		return removals
	}
)

func newUnstructured(nn types.NamespacedName, gvk schema.GroupVersionKind) *unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetName(nn.Name)
	u.SetNamespace((nn.Namespace))
	return &u
}

// ensureAppsubsGone validates successful removal of everything in the uninstallList. Return on first error encounter.
func (r *MultiClusterHubReconciler) ensureAppsubsGone(m *operatorsv1.MultiClusterHub) (ctrl.Result, error) {
	removals := appsubCleanupList(m)
	allResourcesDeleted := true
	for i := range removals {
		gone, err := r.uninstall(m, removals[i])
		if err != nil {
			return ctrl.Result{}, err
		}
		if !gone {
			allResourcesDeleted = false
		}
	}

	if !allResourcesDeleted {
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	// Check if oadp-operator CSV exists in open-cluster-management-backup from the stable-1.0 channel
	// If so delete it because it isn't removed with the subscription
	csvList := &unstructured.UnstructuredList{}
	csvList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operators.coreos.com",
		Kind:    "ClusterServiceVersionList",
		Version: "v1alpha1",
	})
	err := r.Client.List(context.Background(), csvList, client.InNamespace(utils.ClusterSubscriptionNamespace))
	if err != nil {
		return ctrl.Result{}, err
	}
	for _, csv := range csvList.Items {
		csv := csv
		if strings.HasPrefix(csv.GetName(), "oadp-operator.v1.0") {
			r.Log.Info(fmt.Sprintf("Deleting OADP v1.0 CSV found in namespace %s", utils.ClusterSubscriptionNamespace))
			_, err := r.uninstall(m, &csv)
			if err != nil {
				return ctrl.Result{}, err
			}
		}
	}

	// Verify all helmreleases have been cleaned up
	hrList := &unstructured.UnstructuredList{}
	hrList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps.open-cluster-management.io",
		Kind:    "HelmReleaseList",
		Version: "v1",
	})
	err = r.Client.List(context.Background(), hrList, client.MatchingLabels{
		"installer.name":      m.GetName(),
		"installer.namespace": m.GetNamespace(),
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	if len(hrList.Items) > 0 {
		names := []string{}
		for i := range hrList.Items {
			names = append(names, hrList.Items[i].GetName())
		}
		message := fmt.Sprintf("Waiting for helmreleases to be removed: %s", strings.Join(names, ","))
		r.Log.Info(message)
		condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, OldComponentRemovedReason, message)
		SetHubCondition(&m.Status, *condition)

		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	// Emit hubcondition once pruning complete if other pruning condition present
	progressingCondition := GetHubCondition(m.Status, operatorsv1.Progressing)
	if progressingCondition != nil {
		if progressingCondition.Reason == OldComponentRemovedReason || progressingCondition.Reason == OldComponentNotRemovedReason {
			condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, AllOldComponentsRemovedReason, "All resources prunedfor new installation method")
			SetHubCondition(&m.Status, *condition)
		}
	}

	return ctrl.Result{}, nil
}

// ensureRemovalsGone validates successful removal of everything in the uninstallList. Return on first error encounter.
func (r *MultiClusterHubReconciler) ensureRemovalsGone(m *operatorsv1.MultiClusterHub) (ctrl.Result, error) {
	removals := uninstallList(m)
	allResourcesDeleted := true
	for i := range removals {
		gone, err := r.uninstall(m, removals[i])
		if err != nil {
			return ctrl.Result{}, err
		}
		if !gone {
			allResourcesDeleted = false
		}
	}

	if !allResourcesDeleted {
		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	// Emit hubcondition once pruning complete if other pruning condition present
	progressingCondition := GetHubCondition(m.Status, operatorsv1.Progressing)
	if progressingCondition != nil {
		if progressingCondition.Reason == OldComponentRemovedReason || progressingCondition.Reason == OldComponentNotRemovedReason {
			condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, AllOldComponentsRemovedReason, "All old resources pruned")
			SetHubCondition(&m.Status, *condition)
		}
	}

	return ctrl.Result{}, nil
}

// uninstall return true if resource does not exist and returns an error if a GET or DELETE errors unexpectedly. A false response without error
// means the resource is in the process of deleting.
func (r *MultiClusterHubReconciler) uninstall(m *operatorsv1.MultiClusterHub, u *unstructured.Unstructured) (bool, error) {
	obLog := r.Log.WithValues("Namespace", u.GetNamespace(), "Name", u.GetName(), "Kind", u.GetKind())

	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      u.GetName(),
		Namespace: u.GetNamespace(),
	}, u)

	if errors.IsNotFound(err) {
		return true, nil
	}

	// Get resource. Successful if it doesn't exist.
	if err != nil {
		// Error that isn't due to the resource not existing
		obLog.Error(err, "Error getting resource")
		return false, err
	}

	// If resource has deletionTimestamp then re-reconcile and don't try deleting
	if u.GetDeletionTimestamp() != nil {
		condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionFalse, OldComponentNotRemovedReason, fmt.Sprintf("Resource %s/%s finalizing", u.GetKind(), u.GetName()))
		SetHubCondition(&m.Status, *condition)
		obLog.Info("Waiting for resource to finalize")
		return false, nil
	}

	// Attempt deleting resource. No error does not necessarily mean the resource is gone.
	err = r.Client.Delete(context.TODO(), u)
	if err != nil {
		condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionFalse, OldComponentNotRemovedReason, fmt.Sprintf("Failed to remove resource %s/%s", u.GetKind(), u.GetName()))
		SetHubCondition(&m.Status, *condition)
		obLog.Error(err, "Failed to delete resource")
		return false, err
	}
	condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, OldComponentRemovedReason, "Removed old resource")
	SetHubCondition(&m.Status, *condition)
	obLog.Info("Deleted instance")
	return false, nil
}

/*
removeLegacyConfigurations will remove the specified kind of configuration
(PrometheusRule, ServiceMonitor, or Service) in the target namespace. This configuration should be in the controller
namespace instead.
*/
func (r *MultiClusterHubReconciler) removeLegacyConfigurations(ctx context.Context, targetNamespace string,
	kind string) error {
	obj := &unstructured.Unstructured{}
	apiGroup := ""

	if kind == "PrometheusRule" || kind == "ServiceMonitor" {
		apiGroup = MonitoringAPIGroup
	}

	obj.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   apiGroup,
		Kind:    kind,
		Version: "v1",
	})

	var configType string
	var getObjectName func() (string, error)

	for _, c := range operatorsv1.MCHComponents {
		switch kind {
		case "PrometheusRule":
			configType = "PrometheusRule"
			getObjectName = func() (string, error) {
				return operatorsv1.GetLegacyPrometheusRulesName(c)
			}

		case "ServiceMonitor":
			configType = "ServiceMonitor"
			getObjectName = func() (string, error) {
				return operatorsv1.GetLegacyServiceMonitorName(c)
			}

		case "Service":
			configType = "Service"
			getObjectName = func() (string, error) {
				return operatorsv1.GetLegacyServiceName(c)
			}

		default:
			return fmt.Errorf("unsupported kind detected when trying to remove legacy configuration: %s", kind)
		}

		res, err := getObjectName()
		if err != nil {
			continue
		}

		obj.SetName(res)
		obj.SetNamespace(targetNamespace)

		err = r.Client.Delete(ctx, obj)
		if err != nil {
			if !errors.IsNotFound(err) && !apimeta.IsNoMatchError(err) {
				r.Log.Error(
					err,
					fmt.Sprintf("Error while deleting the legacy %s configuration", configType),
					"kind", kind,
					"name", obj.GetName(),
				)
				return err
			}
		} else {
			r.Log.Info(fmt.Sprintf("Deleted the legacy %s configuration: %s", configType, obj.GetName()))
		}
	}
	return nil
}

// ensureRemovalsGone validates successful removal of everything in the uninstallList. Return on first error encounter.
func (r *MultiClusterHubReconciler) cleanupGRCAppsub(m *operatorsv1.MultiClusterHub) error {
	grcAppsub := newUnstructured(
		types.NamespacedName{Name: "grc-sub", Namespace: m.Namespace},
		schema.GroupVersionKind{Group: "apps.open-cluster-management.io", Kind: "Subscription", Version: "v1"},
	)

	// Get GRC appsub
	err := r.Client.Get(context.TODO(), types.NamespacedName{
		Name:      grcAppsub.GetName(),
		Namespace: grcAppsub.GetNamespace(),
	}, grcAppsub)
	if errors.IsNotFound(err) || apimeta.IsNoMatchError(err) {
		return nil
	}
	r.Log.Info("GRC appsub exists. Running upgrade cleanup step.")

	// Find GRC helmrelease
	hrList := &unstructured.UnstructuredList{}
	hrList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "apps.open-cluster-management.io",
		Kind:    "HelmReleaseList",
		Version: "v1",
	})
	_ = r.Client.List(context.Background(), hrList, client.InNamespace(m.Namespace))
	if err != nil {
		return err
	}

	var grcHelmrelease unstructured.Unstructured
	for _, hr := range hrList.Items {
		oRefs := hr.GetOwnerReferences()
		if len(oRefs) > 0 && oRefs[0].Name == "grc-sub" {
			r.Log.Info(fmt.Sprintf("Found GRC helmrelease %s", hr.GetName()))
			grcHelmrelease = hr
			break
		}
	}
	if grcHelmrelease.GetName() == "" {
		r.Log.Info("GRC helmrelease has no name")
		return nil
	}

	// Remove finalizer on helmrelease
	grcHelmrelease.SetFinalizers([]string{})
	r.Log.Info(fmt.Sprintf("Removing finalizers from GRC helmrelease %s", grcHelmrelease.GetName()))
	err = r.Client.Update(context.Background(), &grcHelmrelease)
	if err != nil {
		return err
	}

	// Manually delete GRC cluster-scope and cross-namespace resources
	r.Log.Info("Deleting GRC clusterroles")
	err = r.Client.DeleteAllOf(context.TODO(), &rbacv1.ClusterRole{}, client.MatchingLabels{"app": "grc"})
	if err != nil {
		r.Log.Error(err, "Error while deleting clusterroles")
		return err
	}
	r.Log.Info("Deleting GRC clusterrolebindings")
	err = r.Client.DeleteAllOf(context.TODO(), &rbacv1.ClusterRoleBinding{}, client.MatchingLabels{"app": "grc"})
	if err != nil {
		r.Log.Error(err, "Error while deleting clusterroles")
		return err
	}
	r.Log.Info("Deleting GRC PrometheusRule")
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   MonitoringAPIGroup,
		Kind:    "PrometheusRule",
		Version: "v1",
	})
	err = r.Client.DeleteAllOf(context.TODO(), u, client.InNamespace("openshift-monitoring"), client.MatchingLabels{"app": "grc"})
	if err != nil {
		r.Log.Error(err, "Error while deleting PrometheusRule")
		return err
	}

	// Delete appsub
	r.Log.Info("Deleting GRC appsub")
	err = r.Client.Delete(context.Background(), grcAppsub)
	if err != nil {
		return err
	}

	return nil
}
