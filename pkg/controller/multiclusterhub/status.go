// Copyright (c) 2020 Red Hat, Inc.

package multiclusterhub

import (
	"context"
	"fmt"
	"sort"
	"time"

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
	oldStatus := m.Status
	newStatus := m.Status.DeepCopy()
	newStatus.DesiredVersion = version.Version
	newStatus.HubConditions = m.Status.HubConditions

	components := make(map[string]operatorsv1.StatusCondition)

	deployment := &appsv1.Deployment{}
	deployments := getDeployments(m)
	for i, d := range deployments {
		err := r.client.Get(context.TODO(), deployments[i], deployment)
		if err != nil && !errors.IsNotFound(err) {
			return reconcile.Result{}, err
		}
		components[d.Name] = mapDeployment(deployment)
	}

	appsubs := getAppsubs(m)
	for _, d := range appsubs {
		components[d.Name] = unknownStatus
	}

	hrList := &subrelv1.HelmReleaseList{}
	err := r.client.List(context.TODO(), hrList)
	if err != nil && !errors.IsNotFound(err) {
		return reconcile.Result{}, err
	}
	for _, hr := range hrList.Items {
		owner := hr.OwnerReferences[0].Name
		helmrelease := hr
		if _, ok := components[owner]; ok {
			components[owner] = mapHelmRelease(&helmrelease)
		}
	}

	newStatus.Phase = aggregateStatus(components)

	newStatus.CurrentVersion = oldStatus.CurrentVersion
	if newStatus.Phase == operatorsv1.HubRunning {
		AddCondition(m, operatorsv1.StatusCondition{
			Type:               "Success",
			Status:             v1.ConditionTrue,
			LastTransitionTime: v1.Now(),
			LastUpdateTime:     v1.Now(),
			Reason:             "AllComponentsInstalled",
			Message:            "mch is installed",
		})
		newStatus.CurrentVersion = version.Version
	}

	m.Status = *newStatus
	AddCondition(m, operatorsv1.StatusCondition{
		Type:               "New",
		Status:             v1.ConditionTrue,
		LastTransitionTime: v1.Now(),
		LastUpdateTime:     v1.Now(),
		Reason:             "NewConditionAdded",
		Message:            "Hi I'm new",
	})

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

	if m.Status.Phase != operatorsv1.HubRunning {
		return reconcile.Result{RequeueAfter: resyncPeriod}, nil
	} else {
		return reconcile.Result{}, nil
	}
}

func successfulDeploy(d *appsv1.Deployment) bool {
	latest := latestDeployCondition(d.Status.Conditions)
	return latest.Type == appsv1.DeploymentAvailable && latest.Status == corev1.ConditionTrue
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

func mapDeployment(ds *appsv1.Deployment) operatorsv1.StatusCondition {
	if len(ds.Status.Conditions) < 1 {
		return unknownStatus
	}

	dcs := latestDeployCondition(ds.Status.Conditions)
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
		ret.LastTransitionTime = v1.Time{}
	}

	return ret
}

func successfulHelmRelease(hr *subrelv1.HelmRelease) bool {
	latest := latestHelmReleaseCondition(hr.Status.Conditions)
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

func successfulComponent(sc operatorsv1.StatusCondition) bool {
	return (sc.Status == metav1.ConditionTrue) && (sc.Type == "Available" || sc.Type == "Deployed")
}

func aggregateStatus(components map[string]operatorsv1.StatusCondition) operatorsv1.HubPhaseType {
	for k, val := range components {
		if !successfulComponent(val) {
			log.Info("Waiting on", "name", k)
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

type byTransitionTime []operatorsv1.StatusCondition

func (a byTransitionTime) Len() int      { return len(a) }
func (a byTransitionTime) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a byTransitionTime) Less(i, j int) bool {
	return a[i].LastTransitionTime.Time.Before(a[j].LastTransitionTime.Time)
}

// Adds the statusCondition to a multiclusterhub
func AddCondition(m *operatorsv1.MultiClusterHub, sc operatorsv1.StatusCondition) {
	log.Info("Adding condition", "Condition", sc.Reason)
	for i, x := range m.Status.HubConditions {
		if x.Reason == sc.Reason && x.Status == sc.Status {
			ltt := x.LastTransitionTime
			m.Status.HubConditions[i] = sc
			m.Status.HubConditions[i].LastTransitionTime = ltt
			return
		}
	}
	m.Status.HubConditions = append(m.Status.HubConditions, sc)

	// Trim conditions
	sort.Sort(sort.Reverse(byTransitionTime(m.Status.HubConditions)))
	if len(m.Status.HubConditions) > 2 {
		m.Status.HubConditions = m.Status.HubConditions[:1]
	}

}

// SetHubCondition sets the status condition. It either overwrites the existing one or creates a new one.
func SetHubCondition(m *operatorsv1.MultiClusterHub, newCondition operatorsv1.StatusCondition) {
	newCondition.LastTransitionTime = metav1.NewTime(time.Now())

	existingCondition := FindHubCondition(m, newCondition.Type)
	if existingCondition == nil {
		m.Status.HubConditions = append(m.Status.HubConditions, newCondition)
		return
	}

	if existingCondition.Status != newCondition.Status || existingCondition.LastTransitionTime.IsZero() {
		existingCondition.LastTransitionTime = newCondition.LastTransitionTime
	}

	existingCondition.Status = newCondition.Status
	existingCondition.Reason = newCondition.Reason
	existingCondition.Message = newCondition.Message
}

// RemoveCRDCondition removes the status condition.
func RemoveHubCondition(m *operatorsv1.MultiClusterHub, conditionType operatorsv1.HubConditionType) {
	newConditions := []operatorsv1.HubCondition{}
	for _, condition := range m.Status.HubConditions {
		if condition.Type != conditionType {
			newConditions = append(newConditions, condition)
		}
	}
	m.Status.HubConditions = newConditions
}

// FindHubCondition returns the condition you're looking for or nil.
func FindHubCondition(m *operatorsv1.MultiClusterHub, conditionType operatorsv1.HubConditionType) *operatorsv1.HubCondition {
	for i := range m.Status.HubConditions {
		if m.Status.HubConditions[i].Type == conditionType {
			return &m.Status.HubConditions[i]
		}
	}

	return nil
}

// IsHubConditionTrue indicates if the condition is present and strictly true.
func IsHubConditionTrue(crd *operatorsv1.MultiClusterHub, conditionType operatorsv1.HubConditionType) bool {
	return IsHubConditionPresentAndEqual(crd, conditionType, v1.ConditionTrue)
}

// IsHubConditionFalse indicates if the condition is present and false.
func IsHubConditionFalse(m *operatorsv1.MultiClusterHub, conditionType operatorsv1.HubConditionType) bool {
	return IsHubConditionPresentAndEqual(m, conditionType, v1.ConditionFalse)
}

// IsHubConditionPresentAndEqual indicates if the condition is present and equal to the given status.
func IsHubConditionPresentAndEqual(m *operatorsv1.MultiClusterHub, conditionType operatorsv1.HubConditionType, status v1.ConditionStatus) bool {
	for _, condition := range m.Status.HubConditions {
		if condition.Type == conditionType {
			return condition.Status == status
		}
	}
	return false
}

// IsCRDConditionEquivalent returns true if the lhs and rhs are equivalent except for times.
func IsCRDConditionEquivalent(lhs, rhs *operatorsv1.HubCondition) bool {
	if lhs == nil && rhs == nil {
		return true
	}
	if lhs == nil || rhs == nil {
		return false
	}

	return lhs.Message == rhs.Message && lhs.Reason == rhs.Reason && lhs.Status == rhs.Status && lhs.Type == rhs.Type
}

// DeploymentTimedOut considers a deployment to have timed out once its condition that reports progress
// is older than progressDeadlineSeconds or a Progressing condition with a TimedOutReason reason already
// exists.
// func DeploymentTimedOut(deployment *apps.Deployment, newStatus *apps.DeploymentStatus) bool {
// 	if !HasProgressDeadline(deployment) {
// 		return false
// 	}

// 	// Look for the Progressing condition. If it doesn't exist, we have no base to estimate progress.
// 	// If it's already set with a TimedOutReason reason, we have already timed out, no need to check
// 	// again.
// 	condition := GetDeploymentCondition(*newStatus, apps.DeploymentProgressing)
// 	if condition == nil {
// 		return false
// 	}
// 	// If the previous condition has been a successful rollout then we shouldn't try to
// 	// estimate any progress. Scenario:
// 	//
// 	// * progressDeadlineSeconds is smaller than the difference between now and the time
// 	//   the last rollout finished in the past.
// 	// * the creation of a new ReplicaSet triggers a resync of the Deployment prior to the
// 	//   cached copy of the Deployment getting updated with the status.condition that indicates
// 	//   the creation of the new ReplicaSet.
// 	//
// 	// The Deployment will be resynced and eventually its Progressing condition will catch
// 	// up with the state of the world.
// 	if condition.Reason == NewRSAvailableReason {
// 		return false
// 	}
// 	if condition.Reason == TimedOutReason {
// 		return true
// 	}

// 	// Look at the difference in seconds between now and the last time we reported any
// 	// progress or tried to create a replica set, or resumed a paused deployment and
// 	// compare against progressDeadlineSeconds.
// 	from := condition.LastUpdateTime
// 	now := nowFn()
// 	delta := time.Duration(*deployment.Spec.ProgressDeadlineSeconds) * time.Second
// 	timedOut := from.Add(delta).Before(now)

// 	klog.V(4).Infof("Deployment %q timed out (%t) [last progress check: %v - now: %v]", deployment.Name, timedOut, from, now)
// 	return timedOut
// }
