// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import (
	"context"
	"fmt"

	subrelv1 "github.com/open-cluster-management/multicloud-operators-subscription-release/pkg/apis/apps/v1"
	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/foundation"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/helmrepo"
	"github.com/open-cluster-management/multicloudhub-operator/version"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var deployments = []types.NamespacedName{
	{Name: helmrepo.HelmRepoName, Namespace: "open-cluster-management"},
	{Name: foundation.OCMControllerName, Namespace: "open-cluster-management"},
	{Name: foundation.OCMProxyServerName, Namespace: "open-cluster-management"},
	{Name: foundation.WebhookName, Namespace: "open-cluster-management"},
}

var appsubs = []types.NamespacedName{
	{Name: "application-chart-sub", Namespace: "open-cluster-management"},
	{Name: "cert-manager-sub", Namespace: "open-cluster-management"},
	{Name: "cert-manager-webhook-sub", Namespace: "open-cluster-management"},
	{Name: "configmap-watcher-sub", Namespace: "open-cluster-management"},
	{Name: "console-chart-sub", Namespace: "open-cluster-management"},
	{Name: "grc-sub", Namespace: "open-cluster-management"},
	{Name: "kui-web-terminal-sub", Namespace: "open-cluster-management"},
	{Name: "management-ingress-sub", Namespace: "open-cluster-management"},
	{Name: "rcm-sub", Namespace: "open-cluster-management"},
	{Name: "search-prod-sub", Namespace: "open-cluster-management"},
	{Name: "topology-sub", Namespace: "open-cluster-management"},
}

var unknownStatus = operatorsv1.StatusCondition{
	Type:               "Unknown",
	Status:             metav1.ConditionUnknown,
	LastUpdateTime:     metav1.Now(),
	LastTransitionTime: metav1.Now(),
	Reason:             "No conditions available",
	Message:            "No conditions available",
}

// UpdateStatus updates status
func (r *ReconcileMultiClusterHub) UpdateStatus(m *operatorsv1.MultiClusterHub) (reconcile.Result, error) {
	// oldStatus := m.Status
	newStatus := m.Status.DeepCopy()
	newStatus.DesiredVersion = version.Version

	newStatus.Components = make(map[string]operatorsv1.StatusCondition)

	deployment := &appsv1.Deployment{}
	for i, d := range deployments {
		r.client.Get(context.TODO(), deployments[i], deployment)
		newStatus.Components[d.Name] = mapDeployment(deployment)
	}

	for _, d := range appsubs {
		newStatus.Components[d.Name] = unknownStatus
	}

	hrList := &subrelv1.HelmReleaseList{}
	r.client.List(context.TODO(), hrList)
	for _, hr := range hrList.Items {
		if _, ok := newStatus.Components[hr.OwnerReferences[0].Name]; ok {
			newStatus.Components[hr.OwnerReferences[0].Name] = mapHelmRelease(&hr)
		}
	}

	newStatus.Phase = aggregateStatus(newStatus)

	// appsub := &appsubv1.Subscription{}
	// for i, _ := range appsubs {
	// 	r.client.Get(context.TODO(), appsubs[i], appsub)
	// 	newStatus.Components = append(newStatus.Components, mapAppsub(appsub))
	// }

	m.Status = *newStatus

	// ready, deployments, err := deploying.ListDeployments(r.client, multiClusterHub.Namespace)

	/* Update the CR status
	multiClusterHub.Status.Phase = "Pending"
	multiClusterHub.Status.DesiredVersion = version.Version
	ready, _, err := deploying.ListDeployments(r.client, multiClusterHub.Namespace)
	if err != nil {
		reqLogger.Error(err, "Failed to list deployments")
		return reconcile.Result{}, err
	}
	if ready {
		multiClusterHub.Status.Phase = "Running"
		multiClusterHub.Status.CurrentVersion = version.Version
	}

	result, err = r.UpdateStatus(multiClusterHub) */

	return r.applyStatus(m)
}

func (r *ReconcileMultiClusterHub) applyStatus(m *operatorsv1.MultiClusterHub) (reconcile.Result, error) {
	err := r.client.Status().Update(context.TODO(), m)
	if err != nil {
		if errors.IsConflict(err) {
			// Error from object being modified is normal behavior and should not be treated like an error
			log.Info("Failed to update status", "Reason", "Object has been modified")
			return reconcile.Result{RequeueAfter: resyncPeriod}, nil
		}

		log.Error(err, fmt.Sprintf("Failed to update %s/%s status ", m.Namespace, m.Name))
		return reconcile.Result{}, err
	}
	return reconcile.Result{}, nil
}

func successfulDeploy(ds *appsv1.Deployment) bool {
	if ds.Status.ReadyReplicas != ds.Status.Replicas {
		return false
	}
	return true
}

func mapDeployment(ds *appsv1.Deployment) operatorsv1.StatusCondition {
	if len(ds.Status.Conditions) < 1 {
		return unknownStatus
	}

	// TODO: Get lastest condition by LastUpdateTime
	dcs := ds.Status.Conditions[0]
	ret := operatorsv1.StatusCondition{
		Type:               string(dcs.Type),
		Status:             metav1.ConditionStatus(string(dcs.Status)),
		LastUpdateTime:     dcs.LastUpdateTime,
		LastTransitionTime: dcs.LastTransitionTime,
		Reason:             dcs.Reason,
		Message:            dcs.Message,
	}
	if successfulDeploy(ds) {
		ret.Message = ""
	}

	return ret
}

func successfulHelmRelease(hr *subrelv1.HelmRelease) bool {
	c := hr.Status.Conditions[len(hr.Status.Conditions)-1]
	if c.Reason == subrelv1.ReasonInstallError ||
		c.Reason == subrelv1.ReasonUpdateError ||
		c.Reason == subrelv1.ReasonReconcileError ||
		c.Reason == subrelv1.ReasonUninstallError {
		return false
	}
	return true
}

func mapHelmRelease(hr *subrelv1.HelmRelease) operatorsv1.StatusCondition {
	if len(hr.Status.Conditions) < 1 {
		return unknownStatus
	}

	// TODO: Get lastest condition by LastUpdateTime
	condition := hr.Status.Conditions[len(hr.Status.Conditions)-1]
	ret := operatorsv1.StatusCondition{
		Type:               string(condition.Type),
		Status:             metav1.ConditionStatus(condition.Status),
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: condition.LastTransitionTime,
		Reason:             string(condition.Reason),
		Message:            condition.Message,
	}
	// Ignore success messages
	if !isErrorType(ret.Type) {
		ret.Message = ""
	}
	return ret
}

// func mapAppsub(as *appsubv1.Subscription) operatorsv1.ComponentCondition {
// 	var component operatorsv1.ComponentCondition

// 	if len(as.Status.Statuses) < 1 {
// 		component = operatorsv1.ComponentCondition{
// 			Name:         as.Name,
// 			ResourceType: operatorsv1.ComponentSubscription,
// 			Condition: operatorsv1.StatusCondition{
// 				Type:               "Unknown",
// 				Status:             metav1.ConditionUnknown,
// 				LastUpdateTime:     metav1.Now(),
// 				LastTransitionTime: metav1.Now(),
// 				Reason:             "No conditions available",
// 				Message:            "No conditions available",
// 			},
// 		}
// 	}

// 	name, unit := getUnitStatus(as)
// 	if unit == nil {
// 		log.Info("Unit status empty")
// 		return component
// 	} else {
// 		component.Name = name
// 		component.Condition = marshalAppsub(unit.ResourceStatus)
// 	}
// 	return component
// }

// // Assumes single packagename and clustername
// func getUnitStatus(sub *appsubv1.Subscription) (string, *appsubv1.SubscriptionUnitStatus) {
// 	subStatus := sub.Status

// 	if _, ok := subStatus.Statuses["/"]; ok != true {
// 		return "", nil
// 	}

// 	sps := subStatus.Statuses["/"].SubscriptionPackageStatus // "packages"
// 	for pkgName, unitStatus := range sps {
// 		// SubscriptionClusterStatusMap defines per cluster status
// 		// For endpoint, it is the status of subscription, key is packagename
// 		return pkgName, unitStatus
// 	}
// 	return "", nil
// }

// func getReleaseStatus(sub *appsubv1.Subscription) (subrelv1.HelmAppStatus, error) {
// 	_, unit := getUnitStatus(sub)
// 	if unit == nil {
// 		log.Info("Unit status empty")
// 	}

// 	resourceStatus := unit.ResourceStatus
// 	if resourceStatus == nil {
// 		log.Info("ResourceStatus nil")
// 	}

// 	// Marshal resourceStatus into a HelmRelease status
// 	var helmStatus subrelv1.HelmAppStatus
// 	if err := json.Unmarshal(resourceStatus.Raw, &helmStatus); err != nil {
// 		log.Error(err, "Could not unmarshall to helmstatus")
// 		return helmStatus, err
// 	}
// }

// // marshalAppsub marshals a runtime.RawExtension into a HelmAppStatus and then converts that further into
// // a ComponentCondition. If the RawExtension is nil or cannot be marshalled it will return a default
// // componentCondition with unknown status
// func marshalAppsub(resourceStatus *runtime.RawExtension) operatorsv1.StatusCondition {
// 	c := operatorsv1.StatusCondition{
// 		Type:               "Unknown",
// 		Status:             metav1.ConditionUnknown,
// 		LastUpdateTime:     metav1.Now(),
// 		LastTransitionTime: metav1.Now(),
// 		Reason:             "No conditions available",
// 		Message:            "No conditions available",
// 	}

// 	// If resourceStatus is nil, set defaults
// 	if resourceStatus == nil {
// 		log.Info("ResourceStatus nil")
// 		return c
// 	}

// 	// Marshal resourceStatus into a HelmRelease status
// 	var helmStatus subrelv1.HelmAppStatus
// 	if err := json.Unmarshal(resourceStatus.Raw, &helmStatus); err != nil {
// 		log.Error(err, "Could not unmarshall to helmstatus")
// 		return c
// 	}

// 	condition, err := lastCondition(helmStatus.Conditions)
// 	if err != nil {
// 		log.Error(err, "Could not get most recent condition")
// 		return c
// 	}

// 	return operatorsv1.StatusCondition{
// 		Type:               string(condition.Type),
// 		Status:             metav1.ConditionStatus(condition.Status),
// 		LastUpdateTime:     metav1.Now(),
// 		LastTransitionTime: condition.LastTransitionTime,
// 		Reason:             string(condition.Reason),
// 		Message:            condition.Message,
// 	}

// }

func aggregateStatus(m *operatorsv1.MultiClusterHubStatus) operatorsv1.HubPhaseType {
	for _, val := range m.Components {
		if isErrorType(string(val.Reason)) || val.Status == metav1.ConditionUnknown {
			return operatorsv1.HubPending
		}
	}
	return operatorsv1.HubRunning
}

func isErrorType(cr string) bool {
	return cr == string(subrelv1.ReasonInstallError) ||
		cr == string(subrelv1.ReasonUpdateError) ||
		cr == string(subrelv1.ReasonReconcileError) ||
		cr == string(subrelv1.ReasonUninstallError)
}
