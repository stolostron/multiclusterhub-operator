// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import (
	"context"
	"fmt"
	"reflect"

	corev1 "k8s.io/api/core/v1"

	subrelv1 "github.com/open-cluster-management/multicloud-operators-subscription-release/pkg/apis/apps/v1"
	operatorsv1 "github.com/open-cluster-management/multicloudhub-operator/pkg/apis/operator/v1"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/foundation"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/helmrepo"
	"github.com/open-cluster-management/multicloudhub-operator/pkg/utils"
	"github.com/open-cluster-management/multicloudhub-operator/version"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// ComponentsAvailableReason is added in a hub when all desired components are
	// installed successfully
	ComponentsAvailableReason = "ComponentsAvailable"
	// ComponentsUnavailableReason is added in a hub when one or more components are
	// in an unready state
	ComponentsUnavailableReason = "ComponentsUnavailable"
	// NewComponentReason is added when the hub creates a new install resource successfully
	NewComponentReason = "NewResourceCreated"
	// CertManagerReason is added when the hub is waiting for cert manager CRDs to come up
	CertManagerReason = "CertManagerInitializing"
	// DeleteTimestampReason is added when the multiclusterhub has been targeted for delete
	DeleteTimestampReason = "DeletionTimestampPresent"
	// PausedReason is added when the multiclusterhub is paused
	PausedReason = "MCHPaused"
	// ResumedReason is added when the multiclusterhub is resumed
	ResumedReason = "MCHResumed"
	// ReconcileReason is added when the multiclusterhub is actively reconciling
	ReconcileReason = "MCHReconciling"
	// HelmReleaseTerminatingReason is added when the multiclusterhub is waiting for the removal
	// of helm releases
	HelmReleaseTerminatingReason = "HelmReleaseTerminating"
	// ManagedClusterTerminatingReason is added when a managed cluster has been deleted and
	// is waiting to be finalized
	ManagedClusterTerminatingReason = "ManagedClusterTerminating"
	// NamespaceTerminatingReason is added when a managed cluster's namespace has been deleted and
	// is waiting to be finalized
	NamespaceTerminatingReason = "ManagedClusterNamespaceTerminating"
)

func getDeployments(m *operatorsv1.MultiClusterHub) []types.NamespacedName {
	return []types.NamespacedName{
		{Name: helmrepo.HelmRepoName, Namespace: m.Namespace},
		{Name: foundation.OCMControllerName, Namespace: m.Namespace},
		{Name: foundation.OCMProxyServerName, Namespace: m.Namespace},
		{Name: foundation.WebhookName, Namespace: m.Namespace},
	}
}

func getAppsubs(m *operatorsv1.MultiClusterHub) []types.NamespacedName {
	return []types.NamespacedName{
		{Name: "application-chart-sub", Namespace: m.Namespace},
		{Name: "cert-manager-sub", Namespace: utils.CertManagerNS(m)},
		{Name: "cert-manager-webhook-sub", Namespace: utils.CertManagerNS(m)},
		{Name: "configmap-watcher-sub", Namespace: utils.CertManagerNS(m)},
		{Name: "console-chart-sub", Namespace: m.Namespace},
		{Name: "grc-sub", Namespace: m.Namespace},
		{Name: "kui-web-terminal-sub", Namespace: m.Namespace},
		{Name: "management-ingress-sub", Namespace: m.Namespace},
		{Name: "rcm-sub", Namespace: m.Namespace},
		{Name: "search-prod-sub", Namespace: m.Namespace},
		{Name: "topology-sub", Namespace: m.Namespace},
	}
}

func newComponentList(m *operatorsv1.MultiClusterHub) map[string]operatorsv1.StatusCondition {
	components := make(map[string]operatorsv1.StatusCondition)
	for _, d := range getDeployments(m) {
		components[d.Name] = unknownStatus
	}
	for _, s := range getAppsubs(m) {
		components[s.Name] = unknownStatus
	}
	return components
}

var unknownStatus = operatorsv1.StatusCondition{
	Type:               "Unknown",
	Status:             metav1.ConditionUnknown,
	LastUpdateTime:     metav1.Now(),
	LastTransitionTime: metav1.Now(),
	Reason:             "No conditions available",
	Message:            "No conditions available",
	Available:          false,
}

var wrongVersionStatus = operatorsv1.StatusCondition{
	Type:               "Available",
	Status:             metav1.ConditionFalse,
	LastUpdateTime:     metav1.Now(),
	LastTransitionTime: metav1.Now(),
	Reason:             "WrongVersion",
	Available:          false,
}

// ComponentsAreRunning ...
func (r *ReconcileMultiClusterHub) ComponentsAreRunning(m *operatorsv1.MultiClusterHub) bool {
	deployList, _ := r.listDeployments()
	hrList, _ := r.listHelmReleases()
	componentStatuses := getComponentStatuses(m, hrList, deployList, nil)
	delete(componentStatuses, ManagedClusterName)
	return aggregateStatus(componentStatuses)
}

// syncHubStatus checks if the status is up-to-date and sync it if necessary
func (r *ReconcileMultiClusterHub) syncHubStatus(m *operatorsv1.MultiClusterHub, original *operatorsv1.MultiClusterHubStatus, allDeps []*appsv1.Deployment, allHRs []*subrelv1.HelmRelease) (reconcile.Result, error) {
	localCluster, err := r.ensureManagedClusterIsRunning(m)
	newStatus := calculateStatus(m, allDeps, allHRs, localCluster)
	if reflect.DeepEqual(m.Status, original) {
		log.Info("Status hasn't changed")
		return reconcile.Result{}, nil
	}

	newHub := m
	newHub.Status = newStatus
	err = r.client.Status().Update(context.TODO(), newHub)
	if err != nil {
		if errors.IsConflict(err) {
			// Error from object being modified is normal behavior and should not be treated like an error
			log.Info("Failed to update status", "Reason", "Object has been modified")
			return reconcile.Result{RequeueAfter: resyncPeriod}, nil
		}

		log.Error(err, fmt.Sprintf("Failed to update %s/%s status ", m.Namespace, m.Name))
		return reconcile.Result{}, err
	}

	if m.Status.Phase != operatorsv1.HubRunning {
		return reconcile.Result{RequeueAfter: resyncPeriod}, nil
	} else {
		return reconcile.Result{}, nil
	}
}

func calculateStatus(hub *operatorsv1.MultiClusterHub, allDeps []*appsv1.Deployment, allHRs []*subrelv1.HelmRelease, importClusterStatus []interface{}) operatorsv1.MultiClusterHubStatus {
	components := getComponentStatuses(hub, allHRs, allDeps, importClusterStatus)
	status := operatorsv1.MultiClusterHubStatus{
		CurrentVersion: hub.Status.CurrentVersion,
		DesiredVersion: version.Version,
		Components:     components,
	}

	// Set overall phase

	successful := aggregateStatus(components)
	isHubMarkedToBeDeleted := hub.GetDeletionTimestamp() != nil
	if isHubMarkedToBeDeleted {
		// Hub cleaning up
		status.Phase = operatorsv1.HubUninstalling
	} else if !successful {
		if status.CurrentVersion == "" {
			// Hub has not reached success for first time
			status.Phase = operatorsv1.HubInstalling
		} else if status.CurrentVersion != version.Version {
			// Hub has not completed upgrade to newest version
			status.Phase = operatorsv1.HubUpdating
		} else {
			// Hub has reached desired version, but degraded
			status.Phase = operatorsv1.HubPending
		}
	} else {
		status.Phase = operatorsv1.HubRunning
		status.CurrentVersion = version.Version
	}

	// Copy conditions one by one so we won't mutate the original object.
	conditions := hub.Status.HubConditions
	for i := range conditions {
		status.HubConditions = append(status.HubConditions, conditions[i])
	}

	// Update hub conditions
	if successful {
		available := NewHubCondition(operatorsv1.Complete, v1.ConditionTrue, ComponentsAvailableReason, "All hub components ready.")
		SetHubCondition(&status, *available)
	} else {
		// hub is progressing unless otherwise specified
		if !HubConditionPresent(status, operatorsv1.Progressing) {
			progressing := NewHubCondition(operatorsv1.Progressing, v1.ConditionTrue, ReconcileReason, "Hub is reconciling.")
			SetHubCondition(&status, *progressing)
		}
		// only add unavailable status if complete status already present
		if HubConditionPresent(status, operatorsv1.Complete) {
			unavailable := NewHubCondition(operatorsv1.Complete, v1.ConditionFalse, ComponentsUnavailableReason, "Not all hub components ready.")
			SetHubCondition(&status, *unavailable)
		}
	}

	return status
}

// filterDuplicateHRs removes multiple helmreleases owned by the same appsub, keeping the newest
func filterDuplicateHRs(allHRs []*subrelv1.HelmRelease) []*subrelv1.HelmRelease {
	keys := make(map[string]int)

	for i := range allHRs {
		entry := allHRs[i].DeepCopy()

		ownerApps := entry.GetOwnerReferences()
		if len(ownerApps) == 0 {
			log.Info("Helmrelease has no owner reference!", "Name", entry.GetName())
			continue
		}
		ownerApp := ownerApps[0].Name

		index, ok := keys[ownerApp]
		if !ok {
			keys[ownerApp] = i
		} else {
			log.Info("Competing helmreleases found for same appsub", "New", entry.GetName(), "Old", allHRs[index].GetName())
			existingTime := allHRs[index].GetCreationTimestamp().Time
			thisTime := entry.GetCreationTimestamp().Time
			if thisTime.After(existingTime) {
				keys[ownerApp] = i
			}
		}
	}

	list := []*subrelv1.HelmRelease{}
	for _, index := range keys {
		list = append(list, allHRs[index])
	}
	return list
}

// getComponentStatuses populates a complete list of the hub component statuses
func getComponentStatuses(hub *operatorsv1.MultiClusterHub, allHRs []*subrelv1.HelmRelease, allDeps []*appsv1.Deployment, importClusterStatus []interface{}) map[string]operatorsv1.StatusCondition {
	components := newComponentList(hub)

	filteredHRs := filterDuplicateHRs(allHRs)

	for _, hr := range filteredHRs {
		owners := hr.GetOwnerReferences()
		if len(owners) == 0 {
			log.Info("Helmrelease has no owner reference!", "Name", hr.GetName())
			continue
		}
		appsub := owners[0].Name

		if _, ok := components[appsub]; ok {
			components[appsub] = mapHelmRelease(hr)

			// If helmrelease is labeled successful, check its deployments for readiness
			if successfulHelmRelease(hr) {
				hrDeployments := filterDeploymentsByRelease(allDeps, hr.Name)
				for _, d := range hrDeployments {
					// Set status reported to first unready deployment
					if !successfulDeploy(d) {
						components[appsub] = mapDeployment(d)
						break
					}
				}
			}
		}
	}

	for _, d := range allDeps {
		if _, ok := components[d.Name]; ok {
			components[d.Name] = mapDeployment(d)
		}
	}

	if !hub.Spec.DisableHubSelfManagement {
		components["local-cluster"] = mapManagedClusterConditions(importClusterStatus)
	}
	return components
}

func successfulDeploy(d *appsv1.Deployment) bool {
	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionFalse {
			return false
		}
	}

	if d.Status.UnavailableReplicas > 0 {
		return false
	}

	return true
	// latest := latestDeployCondition(d.Status.Conditions)
}

func latestDeployCondition(conditions []appsv1.DeploymentCondition) appsv1.DeploymentCondition {
	if len(conditions) < 1 {
		return appsv1.DeploymentCondition{}
	}
	latest := conditions[0]
	for i := range conditions {
		if conditions[i].LastTransitionTime.Time.After(latest.LastTransitionTime.Time) {
			latest = conditions[i]
		}
	}
	return latest
}

func progressingDeployCondition(conditions []appsv1.DeploymentCondition) appsv1.DeploymentCondition {
	progressing := appsv1.DeploymentCondition{}
	for i := range conditions {
		if conditions[i].Type == appsv1.DeploymentProgressing {
			progressing = conditions[i]
		}
	}
	return progressing
}

func mapDeployment(ds *appsv1.Deployment) operatorsv1.StatusCondition {
	if len(ds.Status.Conditions) < 1 {
		return unknownStatus
	}

	dcs := latestDeployCondition(ds.Status.Conditions)
	ret := operatorsv1.StatusCondition{
		Kind:               "Deployment",
		Type:               string(dcs.Type),
		Status:             metav1.ConditionStatus(string(dcs.Status)),
		LastUpdateTime:     dcs.LastUpdateTime,
		LastTransitionTime: dcs.LastTransitionTime,
		Reason:             dcs.Reason,
		Message:            dcs.Message,
	}
	if successfulDeploy(ds) {
		ret.Available = true
		ret.Message = ""
	}

	// Because our definition of success is different than the deployment's it is possible we indicate failure
	// despite an available deployment present. To avoid confusion we should show a different status.
	if dcs.Type == appsv1.DeploymentAvailable && dcs.Status == corev1.ConditionTrue && ret.Available == false {
		sub := progressingDeployCondition(ds.Status.Conditions)
		ret = operatorsv1.StatusCondition{
			Kind:               "Deployment",
			Type:               string(sub.Type),
			Status:             metav1.ConditionStatus(string(sub.Status)),
			LastUpdateTime:     sub.LastUpdateTime,
			LastTransitionTime: sub.LastTransitionTime,
			Reason:             sub.Reason,
			Message:            sub.Message,
			Available:          false,
		}
	}

	return ret
}

func mapManagedClusterConditions(conditions []interface{}) operatorsv1.StatusCondition {
	if len(conditions) < 1 {
		return unknownStatus
	}
	accepted, joined, available := false, false, false
	latestCondition := make(map[string]interface{})
	for _, condition := range conditions {
		statusCondition := condition.(map[string]interface{})
		latestCondition = statusCondition
		switch statusCondition["type"] {
		case "HubAcceptedManagedCluster":
			accepted = true
		case "ManagedClusterJoined":
			joined = true
		case "ManagedClusterConditionAvailable":
			available = true
		}
	}

	if !accepted || !joined || !available {
		log.Info("Waiting for managedcluster to be available")
		return operatorsv1.StatusCondition{
			Kind:               "ManagedCluster",
			Type:               latestCondition["type"].(string),
			Status:             metav1.ConditionStatus(latestCondition["status"].(string)),
			LastUpdateTime:     metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             latestCondition["reason"].(string),
			Message:            latestCondition["message"].(string),
			Available:          false,
		}
	}

	return operatorsv1.StatusCondition{
		Kind:               "ManagedCluster",
		Type:               "ManagedClusterImportSuccess",
		Status:             metav1.ConditionTrue,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             "ManagedClusterImported",
		Message:            "ManagedCluster is accepted, joined, and available",
		Available:          true,
	}
}

func successfulHelmRelease(hr *subrelv1.HelmRelease) bool {
	latest := latestHelmReleaseCondition(hr.Status.Conditions)

	// Sometimes helmrelease not properly labeled as deployed
	if latest.Type == subrelv1.ConditionInitialized && hr.Status.DeployedRelease != nil {
		return true
	}
	return latest.Type == subrelv1.ConditionDeployed && latest.Status == subrelv1.StatusTrue
}

func latestHelmReleaseCondition(conditions []subrelv1.HelmAppCondition) subrelv1.HelmAppCondition {
	if len(conditions) < 1 {
		return subrelv1.HelmAppCondition{}
	}
	latest := conditions[0]
	for i := range conditions {
		if conditions[i].LastTransitionTime.Time.After(latest.LastTransitionTime.Time) {
			latest = conditions[i]
		}
	}
	return latest
}

func mapHelmRelease(hr *subrelv1.HelmRelease) operatorsv1.StatusCondition {
	if len(hr.Status.Conditions) < 1 {
		return unknownStatus
	}

	condition := latestHelmReleaseCondition(hr.Status.Conditions)
	ret := operatorsv1.StatusCondition{
		Kind:               "HelmRelease",
		Type:               string(condition.Type),
		Status:             metav1.ConditionStatus(condition.Status),
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: condition.LastTransitionTime,
		Reason:             string(condition.Reason),
		Message:            condition.Message,
	}

	if condition.Type == "Initialized" && hr.Status.DeployedRelease != nil {
		ret.Type = "DeployedRelease"
	}

	if successfulHelmRelease(hr) {
		ret.Available = true
		ret.Message = ""

		// Check if using desired chart version
		if v := hr.Repo.Version; v != version.Version {
			ret = wrongVersionStatus
			ret.Message = fmt.Sprintf("expected version `%s`, current version is `%s`", version.Version, v)
		}
	}

	return ret
}

func successfulComponent(sc operatorsv1.StatusCondition) bool { return sc.Available }

// aggregateStatus returns true if all components are successful, otherwise false
func aggregateStatus(components map[string]operatorsv1.StatusCondition) bool {
	for _, val := range components {
		if !successfulComponent(val) {
			return false
		}
	}
	return true
}

// NewHubCondition creates a new hub condition.
func NewHubCondition(condType operatorsv1.HubConditionType, status v1.ConditionStatus, reason, message string) *operatorsv1.HubCondition {
	return &operatorsv1.HubCondition{
		Type:               condType,
		Status:             status,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
	}
}

// SetHubCondition sets the status condition. It either overwrites the existing one or creates a new one.
func SetHubCondition(status *operatorsv1.MultiClusterHubStatus, condition operatorsv1.HubCondition) {
	currentCond := GetHubCondition(*status, condition.Type)
	if currentCond != nil && currentCond.Status == condition.Status && currentCond.Reason == condition.Reason {
		return
	}
	// Do not update lastTransitionTime if the status of the condition doesn't change.
	if currentCond != nil && currentCond.Status == condition.Status {
		condition.LastTransitionTime = currentCond.LastTransitionTime
	}
	newConditions := filterOutCondition(status.HubConditions, condition.Type)
	status.HubConditions = append(newConditions, condition)
}

// RemoveCRDCondition removes the status condition.
func RemoveHubCondition(status *operatorsv1.MultiClusterHubStatus, condType operatorsv1.HubConditionType) {
	status.HubConditions = filterOutCondition(status.HubConditions, condType)
}

// FindHubCondition returns the condition you're looking for or nil.
func GetHubCondition(status operatorsv1.MultiClusterHubStatus, condType operatorsv1.HubConditionType) *operatorsv1.HubCondition {
	for i := range status.HubConditions {
		c := status.HubConditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

// filterOutCondition returns a new slice of hub conditions without conditions with the provided type.
func filterOutCondition(conditions []operatorsv1.HubCondition, condType operatorsv1.HubConditionType) []operatorsv1.HubCondition {
	var newConditions []operatorsv1.HubCondition
	for _, c := range conditions {
		if c.Type == condType {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}

// IsHubConditionPresentAndEqual indicates if the condition is present and equal to the given status.
func HubConditionPresent(status operatorsv1.MultiClusterHubStatus, conditionType operatorsv1.HubConditionType) bool {
	for _, condition := range status.HubConditions {
		if condition.Type == conditionType {
			return true
		}
	}
	return false
}
