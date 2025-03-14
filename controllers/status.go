// Copyright (c) 2021 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"
	"reflect"
	"strings"

	corev1 "k8s.io/api/core/v1"

	mcev1 "github.com/stolostron/backplane-operator/api/v1"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/version"

	utils "github.com/stolostron/multiclusterhub-operator/pkg/utils"

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

const (
	// AwaitingCRDCreationReason is added in a hub when a desired CRD has not been installed yet
	AwaitingCRDCreationReason = "AwaitingCRDCreation"
	// ComponentsAvailableReason is added in a hub when all desired components are
	// installed successfully
	ComponentsAvailableReason = "ComponentsAvailable"
	// ComponentsUnavailableReason is added in a hub when one or more components are
	// in an unready state
	ComponentsUnavailableReason = "ComponentsUnavailable"
	// ComponentUpdatingReason is added when the hub is actively updating a component resource
	ComponentsUpdatingReason = "UpdatingComponentResource"
	// NewComponentReason is added when the hub creates a new install resource successfully
	NewComponentReason = "NewResourceCreated"
	// DeployFailedReason is added when the hub fails to deploy a resource
	DeployFailedReason = "FailedDeployingComponent"
	//ResourceBlockReason is added when there is an existing resource that prevents an upgrade from progressing
	ResourceBlockReason = "BlockingUpgrade"
	// OldComponentRemovedReason is added when the hub calls delete on an old resource
	OldComponentRemovedReason = "OldResourceDeleted"
	// OldComponentNotRemovedReason is added when a component the hub is trying to delete has not been removed successfully
	OldComponentNotRemovedReason = "OldResourceDeleteFailed"
	// AllOldComponentsRemovedReason is added when the hub successfully prunes all old resources
	AllOldComponentsRemovedReason = "AllOldResourcesDeleted"
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
	// ResourceRenderReason is added when an error occurs while rendering a deployable resource
	ResourceRenderReason = "FailedRenderingResource"
	// CRDRenderReason is added when an error occurs while rendering a CRD
	CRDRenderReason = "FailedRenderingCRD"
	// RequirementsNotMetReason is when there is something missing or misconfigured
	// that is preventing progress
	RequirementsNotMetReason = "RequirementsNotMet"

	FailedApplyingComponent = "FailedApplyingComponent"
)

var (
	prevAvailability = make(map[string]bool)
)

func newComponentList(m *operatorsv1.MultiClusterHub, ocpConsole, isSTSEnabled bool) map[string]operatorsv1.StatusCondition {
	components := make(map[string]operatorsv1.StatusCondition)
	for _, d := range utils.GetDeploymentsForStatus(m, ocpConsole, isSTSEnabled) {
		components[d.Name] = unknownStatus(d.Name, "Deployment")
	}

	for _, cr := range utils.GetCustomResourcesForStatus(m) {
		components[cr.Name] = unknownStatus(cr.Name, "Component")
	}
	return components
}

var consoleUnavailableStatus = operatorsv1.StatusCondition{
	Type:               "Available",
	Status:             metav1.ConditionFalse,
	LastUpdateTime:     metav1.Now(),
	LastTransitionTime: metav1.Now(),
	Reason:             "OCP Console missing",
	Message:            "The OCP Console must be enabled before using ACM Console",
	Available:          true,
}

// ComponentsAreRunning ...
func (r *MultiClusterHubReconciler) ComponentsAreRunning(m *operatorsv1.MultiClusterHub, ocpConsole, isSTSEnabled bool) bool {
	trackedNamespaces := utils.TrackedNamespaces(m)

	deployList, _ := r.listDeployments(trackedNamespaces)
	crList, _ := r.listCustomResources(m)
	componentStatuses := getComponentStatuses(m, deployList, crList, ocpConsole, isSTSEnabled)

	delete(componentStatuses, ManagedClusterName)
	return allComponentsSuccessful(componentStatuses)
}

// syncHubStatus checks if the status is up-to-date and sync it if necessary
func (r *MultiClusterHubReconciler) syncHubStatus(m *operatorsv1.MultiClusterHub,
	original *operatorsv1.MultiClusterHubStatus, allDeps []*appsv1.Deployment,
	allCRs map[string]*unstructured.Unstructured, ocpConsole, isSTSEnabled bool) (reconcile.Result, error) {

	newStatus := calculateStatus(m, allDeps, allCRs, ocpConsole, isSTSEnabled)
	if reflect.DeepEqual(m.Status, original) {
		r.Log.Info("Status hasn't changed")
		return reconcile.Result{}, nil
	}

	newHub := m
	newHub.Status = newStatus
	err := r.Client.Status().Update(context.TODO(), newHub)
	if err != nil {
		if errors.IsConflict(err) {
			// Error from object being modified is normal behavior and should not be treated like an error
			r.Log.Info("Failed to update status", "Reason", "Object has been modified")
			return reconcile.Result{RequeueAfter: resyncPeriod}, nil
		}

		r.Log.Error(err, fmt.Sprintf("Failed to update %s/%s status ", m.Namespace, m.Name))
		return reconcile.Result{}, err
	}

	if m.Status.Phase != operatorsv1.HubRunning {
		return reconcile.Result{RequeueAfter: resyncPeriod}, nil
	} else {
		return reconcile.Result{}, nil
	}
}

func calculateStatus(hub *operatorsv1.MultiClusterHub, allDeps []*appsv1.Deployment,
	allCRs map[string]*unstructured.Unstructured, ocpConsole, isSTSEnabled bool) operatorsv1.MultiClusterHubStatus {

	components := map[string]operatorsv1.StatusCondition{}
	if paused := utils.IsPaused(hub); !paused {
		components = getComponentStatuses(hub, allDeps, allCRs, ocpConsole, isSTSEnabled)
	}

	status := operatorsv1.MultiClusterHubStatus{
		CurrentVersion: hub.Status.CurrentVersion,
		DesiredVersion: version.Version,
		Components:     components,
	}

	// Set current version
	successful := allComponentsSuccessful(components)
	if successful {
		status.CurrentVersion = version.Version
	}

	// Copy conditions one by one to not affect original object
	conditions := hub.Status.HubConditions
	status.HubConditions = append(status.HubConditions, conditions...)

	// Update hub conditions
	if successful {
		// don't label as complete until component pruning succeeds
		if !hubPruning(status) && !utils.IsPaused(hub) {
			available := NewHubCondition(operatorsv1.Complete, metav1.ConditionTrue, ComponentsAvailableReason, "All hub components ready.")
			SetHubCondition(&status, *available)
		} else {
			// only add unavailable status if complete status already present
			if HubConditionPresent(status, operatorsv1.Complete) {
				unavailable := NewHubCondition(operatorsv1.Complete, metav1.ConditionFalse, OldComponentNotRemovedReason, "Not all components successfully pruned.")
				SetHubCondition(&status, *unavailable)
			}
		}
	} else {
		// hub is progressing unless otherwise specified
		if !HubConditionPresent(status, operatorsv1.Progressing) {
			progressing := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, ReconcileReason, "Hub is reconciling.")
			SetHubCondition(&status, *progressing)
		}

		// only add unavailable status if complete status already present
		if HubConditionPresent(status, operatorsv1.Complete) {
			unavailable := NewHubCondition(operatorsv1.Complete, metav1.ConditionFalse, ComponentsUnavailableReason, "Not all hub components ready.")
			SetHubCondition(&status, *unavailable)
		}
	}

	// Set overall phase
	isHubMarkedToBeDeleted := hub.GetDeletionTimestamp() != nil
	hasComponentFailure := HubConditionPresentWithSubstring(status, string(operatorsv1.ComponentFailure))
	if isHubMarkedToBeDeleted {
		// Hub cleaning up
		status.Phase = operatorsv1.HubUninstalling
	} else if hasComponentFailure {
		status.Phase = operatorsv1.HubError
	} else {
		status.Phase = aggregatePhase(status)
	}

	return status
}

// getComponentStatuses populates a complete list of the hub component statuses
func getComponentStatuses(hub *operatorsv1.MultiClusterHub, allDeps []*appsv1.Deployment,
	allCRs map[string]*unstructured.Unstructured, ocpConsole, isSTSEnabled bool) map[string]operatorsv1.StatusCondition {
	components := newComponentList(hub, ocpConsole, isSTSEnabled)

	for _, d := range allDeps {
		if _, ok := components[d.Name]; ok {
			components[d.Name] = mapDeployment(d)
		}
	}

	for key, cr := range allCRs {
		if cr == nil {
			continue
		}
		switch key {
		case "mce-sub":
			components["multicluster-engine-sub"] = mapSubscription(cr)
		case "mce-csv":
			components["multicluster-engine-csv"] = mapCSV(cr)
		case "mce":
			components["multicluster-engine"] = mapMultiClusterEngine(cr)
		}
	}

	if !ocpConsole {
		components["console-chart-console-v2"] = consoleUnavailableStatus
	}

	return components
}

func successfulDeploy(d *appsv1.Deployment) bool {
	for _, c := range d.Status.Conditions {
		if c.Type == appsv1.DeploymentAvailable && c.Status == corev1.ConditionFalse {
			return false
		}
	}

	return d.Status.UnavailableReplicas <= 0
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
		return unknownStatus(ds.Name, ds.Kind)
	}

	dcs := latestDeployCondition(ds.Status.Conditions)
	ret := operatorsv1.StatusCondition{
		Name:               ds.Name,
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
	if dcs.Type == appsv1.DeploymentAvailable && dcs.Status == corev1.ConditionTrue && !ret.Available {
		sub := progressingDeployCondition(ds.Status.Conditions)
		ret = operatorsv1.StatusCondition{
			Name:               ds.Name,
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

func mapSubscription(sub *unstructured.Unstructured) operatorsv1.StatusCondition {
	if sub == nil {
		return unknownStatus(sub.GetName(), "Subscription")
	}

	spec, ok := sub.Object["spec"].(map[string]interface{})
	if !ok {
		return unknownStatus(sub.GetName(), "Subscription")
	}

	status, ok := sub.Object["status"].(map[string]interface{})
	if !ok {
		return unknownStatus(sub.GetName(), "Subscription")
	}

	installPlanRef, ok := status["installPlanRef"].(map[string]interface{})
	if !ok {
		return unknownStatus(sub.GetName(), "Subscription")
	}

	componentStatus := "True"
	reason, _ := status["state"].(string)
	installPlanApproval, _ := spec["installPlanApproval"].(string)
	installPlanNamespace, _ := installPlanRef["namespace"].(string)
	installPlanName, _ := installPlanRef["name"].(string)

	message := fmt.Sprintf("installPlanApproval: %s. installPlan: %s/%s",
		installPlanApproval, installPlanNamespace, installPlanName)

	/*
		Based on the descriptions in the OLM Subscription CRD, the following holds true:
		• InstalledCSV refers to the CSV that is currently installed by the Subscription.
		• CurrentCSV refers to the CSV that the Subscription is in the process of upgrading to.
	*/
	if reason == "UpgradePending" {
		currentCSV, _ := status["currentCSV"].(string)
		installedCSV, _ := status["installedCSV"].(string)
		message = fmt.Sprintf("Upgrade pending. Installed CSV: %s. Pending CSV: %s", installedCSV, currentCSV)
	}

	return operatorsv1.StatusCondition{
		Name:               sub.GetName(),
		Kind:               "Subscription",
		Status:             metav1.ConditionStatus(componentStatus),
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             reason,
		Message:            message,
		Type:               "Available",
		Available:          true,
	}
}

func mapMultiClusterEngine(mce *unstructured.Unstructured) operatorsv1.StatusCondition {
	if mce == nil {
		return unknownStatus(mce.GetName(), "MultiClusterEngine")
	}

	status, ok := mce.Object["status"].(map[string]interface{})
	if !ok {
		return unknownStatus(mce.GetName(), "MultiClusterEngine")
	}
	conditions, ok := status["conditions"].([]interface{})
	if !ok {
		return unknownStatus(mce.GetName(), "MultiClusterEngine")
	}

	componentCondition := operatorsv1.StatusCondition{}

	for _, condition := range conditions {
		statusCondition, ok := condition.(map[string]interface{})
		if !ok {
			return unknownStatus(mce.GetName(), "MultiClusterEngine")
		}

		status, _ := statusCondition["status"].(string)
		message, _ := statusCondition["message"].(string)
		reason, _ := statusCondition["reason"].(string)
		conditionType, _ := statusCondition["type"].(string)

		componentCondition = operatorsv1.StatusCondition{
			Name:               mce.GetName(),
			Kind:               "MultiClusterEngine",
			Status:             metav1.ConditionStatus(status),
			LastUpdateTime:     metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             reason,
			Message:            message,
			Type:               conditionType,
			Available:          false,
		}

		// Return condition with Applied = true
		if conditionType == string(mcev1.MultiClusterEngineAvailable) && status == "True" {
			componentCondition.Available = true
			return componentCondition
		}
	}

	// If no condition with applied true, then return last condition in list
	return componentCondition
}

func mapCSV(csv *unstructured.Unstructured) operatorsv1.StatusCondition {
	if csv == nil {
		return unknownStatus(csv.GetName(), "ClusterServiceVersion")
	}

	status, ok := csv.Object["status"].(map[string]interface{})
	if !ok {
		return unknownStatus(csv.GetName(), "ClusterServiceVersion")
	}
	conditions, ok := status["conditions"].([]interface{})
	if !ok {
		return unknownStatus(csv.GetName(), "ClusterServiceVersion")
	}

	componentCondition := operatorsv1.StatusCondition{}

	for _, condition := range conditions {
		statusCondition, ok := condition.(map[string]interface{})
		if !ok {
			return unknownStatus(csv.GetName(), "ClusterServiceVersion")
		}

		phase, _ := statusCondition["phase"].(string)
		message, _ := statusCondition["message"].(string)
		reason, _ := statusCondition["reason"].(string)
		status := "False"

		if phase == "Succeeded" {
			status = "True"
		}

		componentCondition = operatorsv1.StatusCondition{
			Name:               csv.GetName(),
			Kind:               "ClusterServiceVersion",
			Status:             metav1.ConditionStatus(status),
			LastUpdateTime:     metav1.Now(),
			LastTransitionTime: metav1.Now(),
			Reason:             reason,
			Message:            message,
			Type:               "Unavailable",
			Available:          false,
		}

		// Return condition with Applied = true
		if phase == "Succeeded" && reason == "InstallSucceeded" {
			componentCondition.Type = "Available"
			componentCondition.Available = true
			return componentCondition
		}
	}

	// If no condition with applied true, then return last condition in list
	return componentCondition
}

// allComponentsSuccessful returns true if all components are successful, otherwise false
func allComponentsSuccessful(components map[string]operatorsv1.StatusCondition) bool {
	if len(components) == 0 {
		return false
	}

	// Track availability status.
	allAvailable := true

	for _, val := range components {
		if !val.Available {
			// Check if the component's availability status has changed since the last reconciliation.
			if prevStatus, exists := prevAvailability[val.Name]; !exists || prevStatus {
				// Log the information about the newly unavailable component
				log.Info("The component is not yet available.", "Kind", val.Kind, "Name", val.Name, "Reason", val.Reason)
			}

			// Update the previous availability status for this component
			prevAvailability[val.Name] = false
			allAvailable = false
		} else {
			// Check if the component's availability status has changed since the last reconciliation
			if prevStatus, exists := prevAvailability[val.Name]; !exists || !prevStatus {
				// Log the information about the newly available component
				log.Info("The component is now available.", "Kind", val.Kind, "Name", val.Name)
			}

			// Update the previous availability status for this component
			prevAvailability[val.Name] = true
		}
	}

	// Return the overall availability status
	return allAvailable
}

// aggregatePhase calculates overall HubPhaseType based on hub status. This does NOT account for
// a hub in the process of deletion.
func aggregatePhase(status operatorsv1.MultiClusterHubStatus) operatorsv1.HubPhaseType {
	if utils.IsUnitTest() {
		return operatorsv1.HubRunning
	}

	for _, condition := range status.HubConditions {
		if condition.Reason == PausedReason {
			return operatorsv1.HubPaused
		}
	}

	if successful := allComponentsSuccessful(status.Components); successful {
		if hubPruning(status) {
			// hub is in pruning phase
			return operatorsv1.HubPending
		}

		// Hub running
		return operatorsv1.HubRunning
	}

	switch cv := status.CurrentVersion; {
	case cv == "":
		// Hub has not reached success for first time
		return operatorsv1.HubInstalling

	case cv != version.Version:
		if HubConditionPresent(status, operatorsv1.Blocked) {
			return operatorsv1.HubUpdatingBlocked
		} else {
			// Hub has not completed upgrade to newest version
			return operatorsv1.HubUpdating
		}

	default:
		// Hub has reached desired version, but degraded
		return operatorsv1.HubPending
	}
}

// NewHubCondition creates a new hub condition.
func NewHubCondition(condType operatorsv1.HubConditionType, status metav1.ConditionStatus, reason, message string) *operatorsv1.HubCondition {
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

// GetHubCondition returns the condition you're looking for or nil.
func GetHubCondition(status operatorsv1.MultiClusterHubStatus, condType operatorsv1.HubConditionType) *operatorsv1.HubCondition {
	for i := range status.HubConditions {
		c := status.HubConditions[i]
		if c.Type == condType {
			return &c
		}
	}
	return nil
}

// hubPruning returns true when the status reports hub is in the process of pruning
func hubPruning(status operatorsv1.MultiClusterHubStatus) bool {
	progressingCondition := GetHubCondition(status, operatorsv1.Progressing)
	if progressingCondition != nil {
		if progressingCondition.Reason == OldComponentRemovedReason || progressingCondition.Reason == OldComponentNotRemovedReason {
			return true
		}
	}
	return false
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

func filterOutConditionWithSubstring(conditions []operatorsv1.HubCondition, substring string) []operatorsv1.HubCondition {
	var newConditions []operatorsv1.HubCondition
	for _, c := range conditions {
		if strings.Contains(string(c.Type), substring) {
			continue
		}
		newConditions = append(newConditions, c)
	}
	return newConditions
}

// HubConditionPresent indicates if the condition is present and equal to the given status.
func HubConditionPresent(status operatorsv1.MultiClusterHubStatus, conditionType operatorsv1.HubConditionType) bool {
	for _, condition := range status.HubConditions {
		if condition.Type == conditionType {
			return true
		}
	}
	return false
}

// Variant of `HubConditionPresent()` that checks for substring instead of exact match
func HubConditionPresentWithSubstring(status operatorsv1.MultiClusterHubStatus, substring string) bool {
	for _, condition := range status.HubConditions {
		if strings.Contains(string(condition.Type), substring) {
			return true
		}
	}
	return false
}

func unknownStatus(name, kind string) operatorsv1.StatusCondition {
	return operatorsv1.StatusCondition{
		Name:               name,
		Kind:               kind,
		Type:               "Unknown",
		Status:             metav1.ConditionUnknown,
		LastUpdateTime:     metav1.Now(),
		LastTransitionTime: metav1.Now(),
		Reason:             "No conditions available",
		Message:            "No conditions available",
		Available:          false,
	}
}
