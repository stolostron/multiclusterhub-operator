// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-logr/logr"
	subv1alpha1 "github.com/operator-framework/api/pkg/operators/v1alpha1"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	mceutils "github.com/stolostron/multiclusterhub-operator/pkg/multiclusterengine"
	admissionregistrationv1 "k8s.io/api/admissionregistration/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	teardownGateFinalizer = "operator.open-cluster-management.io/teardown-gate"
	acmSubscriptionLabel  = "operators.coreos.com/advanced-cluster-management"

	defaultFinalizerTimeout = 5 * time.Minute
)

// agentInstallWebhookNames lists admission webhook configurations created by
// the assisted-installer whose backing service may be gone after
// AgentServiceConfig is deleted in phaseRemoveBlockingCRs.
var agentInstallWebhookNames = []string{
	"infraenvvalidators.admission.agentinstall.openshift.io",
	"agentvalidators.admission.agentinstall.openshift.io",
	"agentclusterinstallvalidators.admission.agentinstall.openshift.io",
	"agentclusterinstallmutators.admission.agentinstall.openshift.io",
}

// deleteAgentInstallWebhooks unconditionally deletes agent-install admission
// webhooks. Called after AgentServiceConfig is confirmed deleted — the backing
// service is gone, so these webhooks (failurePolicy: Fail) would reject PATCH
// calls in subsequent phases.
func (r *HubTeardownReconciler) deleteAgentInstallWebhooks(
	ctx context.Context,
	log logr.Logger,
	td *operatorv1.HubTeardown,
) error {
	for _, name := range agentInstallWebhookNames {
		key := types.NamespacedName{Name: name}

		vwc := &admissionregistrationv1.ValidatingWebhookConfiguration{}
		if err := r.Client.Get(ctx, key, vwc); err == nil {
			log.Info("Deleting agent-install ValidatingWebhookConfiguration", "name", name)
			if err := r.Client.Delete(ctx, vwc); err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("deleting VWC %s: %w", name, err)
			}
			r.Recorder.Eventf(td, corev1.EventTypeNormal, "AgentWebhookDeleted",
				"Deleted ValidatingWebhookConfiguration %s (agent-install service gone)", name)
			continue
		}

		mwc := &admissionregistrationv1.MutatingWebhookConfiguration{}
		if err := r.Client.Get(ctx, key, mwc); err == nil {
			log.Info("Deleting agent-install MutatingWebhookConfiguration", "name", name)
			if err := r.Client.Delete(ctx, mwc); err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("deleting MWC %s: %w", name, err)
			}
			r.Recorder.Eventf(td, corev1.EventTypeNormal, "AgentWebhookDeleted",
				"Deleted MutatingWebhookConfiguration %s (agent-install service gone)", name)
		}
	}
	return nil
}

// phaseGateOLMSubscription adds a finalizer to the ACM OLM Subscription to prevent
// the operator from being removed before teardown completes. The gate protects
// against operator removal only; it does not prevent OLM catalog-driven CSV
// updates. Set installPlanApproval: Manual on the Subscription if that is a concern.
func (r *HubTeardownReconciler) phaseGateOLMSubscription(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) (bool, error) {
	log.Info("Phase: GateOLMSubscription")

	sub, err := r.findACMSubscription(ctx)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("No ACM Subscription found, skipping OLM gate")
			return true, nil
		}
		return false, fmt.Errorf("finding ACM subscription: %w", err)
	}

	finalizers := sub.GetFinalizers()
	for _, f := range finalizers {
		if f == teardownGateFinalizer {
			log.Info("Teardown gate finalizer already present on Subscription")
			return true, nil
		}
	}

	patch := client.MergeFrom(sub.DeepCopy())
	sub.SetFinalizers(append(finalizers, teardownGateFinalizer))
	if err := r.Client.Patch(ctx, sub, patch); err != nil {
		return false, fmt.Errorf("adding gate finalizer to Subscription: %w", err)
	}

	r.Recorder.Event(td, corev1.EventTypeNormal, "OLMGateAdded",
		"Added teardown gate finalizer to ACM Subscription to prevent premature operator removal")
	log.Info("Added teardown gate finalizer to ACM Subscription", "subscription", sub.GetName())
	return true, nil
}

// phaseRemoveBlockingCRs deletes resources that block MCH webhook validation:
// MultiClusterObservability, DiscoveryConfig, AgentServiceConfig.
func (r *HubTeardownReconciler) phaseRemoveBlockingCRs(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) (bool, error) {
	log.Info("Phase: RemoveBlockingCRs")

	blockingTypes := []struct {
		group   string
		version string
		kind    string
	}{
		{"observability.open-cluster-management.io", "v1beta2", "MultiClusterObservability"},
		{"discovery.open-cluster-management.io", "v1", "DiscoveryConfig"},
		{"agent-install.openshift.io", "v1beta1", "AgentServiceConfig"},
	}

	allGone := true
	for _, bt := range blockingTypes {
		listGVK := schema.GroupVersionKind{
			Group:   bt.group,
			Version: bt.version,
			Kind:    bt.kind + "List",
		}
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(listGVK)

		if err := r.Client.List(ctx, list); err != nil {
			if errors.IsNotFound(err) || isNoMatchError(err) {
				continue
			}
			return false, fmt.Errorf("listing %s: %w", bt.kind, err)
		}

		for i := range list.Items {
			item := &list.Items[i]
			if item.GetDeletionTimestamp() != nil {
				log.Info("Blocking CR already deleting, waiting", "kind", bt.kind, "name", item.GetName())
				allGone = false
				continue
			}
			log.Info("Deleting blocking CR", "kind", bt.kind, "name", item.GetName())
			if err := r.Client.Delete(ctx, item); err != nil && !errors.IsNotFound(err) {
				return false, fmt.Errorf("deleting %s/%s: %w", bt.kind, item.GetName(), err)
			}
			r.Recorder.Eventf(td, corev1.EventTypeNormal, "BlockingCRDeleted",
				"Deleted %s/%s to unblock MCH webhook", bt.kind, item.GetName())
			allGone = false
		}
	}

	if allGone {
		if err := r.deleteAgentInstallWebhooks(ctx, log, td); err != nil {
			log.Error(err, "Error deleting agent-install webhooks after blocking CRs removed")
			return false, nil
		}
	}
	return allGone, nil
}

// phaseDisableAddons deletes ManagedClusterAddOns for non-local ManagedClusters.
// It first disables addon-manager placements to prevent recreation, then deletes
// MCAs and resolves stuck addon finalizers on terminating MCAs.
func (r *HubTeardownReconciler) phaseDisableAddons(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) (bool, error) {
	log.Info("Phase: DisableAddons")

	// Step 1: Disable ClusterManagementAddOn placements to break CMAO recreation loop.
	if err := r.disableAddonPlacements(ctx, log); err != nil {
		log.Error(err, "Failed to disable addon placements, continuing with MCA deletion")
	}

	// Step 2: Scale down addon controllers that recreate MCAs independently of placements.
	// Without this, klusterlet-addon-controller-v2 recreates MCAs as long as ManagedClusters exist.
	r.scaleDownAddonControllers(ctx, log, td)

	// Step 3: Identify unreachable ManagedClusters (skip waiting for their addons)
	unreachableMCs := r.getUnreachableManagedClusters(ctx, log)

	addonGVK := schema.GroupVersionKind{
		Group:   "addon.open-cluster-management.io",
		Version: "v1alpha1",
		Kind:    "ManagedClusterAddOnList",
	}
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(addonGVK)

	if err := r.Client.List(ctx, list); err != nil {
		if isNoMatchError(err) {
			log.Info("ManagedClusterAddOn API not available, skipping")
			return true, nil
		}
		return false, fmt.Errorf("listing ManagedClusterAddOns: %w", err)
	}

	remaining := 0
	for i := range list.Items {
		addon := &list.Items[i]
		ns := addon.GetNamespace()
		if ns == "local-cluster" {
			continue
		}

		// Addons on unreachable clusters will never terminate gracefully
		// because the klusterlet has no connectivity to process finalizers.
		// Still require approvedDestructiveActions + forceFinalizerTimeout
		// to avoid stripping finalizers on clusters with transient network issues.
		if unreachableMCs[ns] {
			if addon.GetDeletionTimestamp() == nil {
				log.Info("Deleting addon on unreachable cluster", "namespace", ns, "name", addon.GetName())
				if err := r.Client.Delete(ctx, addon); err != nil && !errors.IsNotFound(err) {
					return false, fmt.Errorf("deleting addon %s/%s: %w", ns, addon.GetName(), err)
				}
			}
			if addon.GetDeletionTimestamp() != nil && len(addon.GetFinalizers()) > 0 {
				if td.Spec.ApprovedDestructiveActions && r.shouldResolveStuckFinalizers(td, addon) {
					r.resolveAddonFinalizers(ctx, log, td, addon)
				} else {
					remaining++
				}
			}
			continue
		}

		if addon.GetDeletionTimestamp() != nil {
			// Resolve stuck addon finalizers if past timeout and approvedDestructiveActions
			if r.shouldResolveStuckFinalizers(td, addon) {
				r.resolveAddonFinalizers(ctx, log, td, addon)
			} else {
				remaining++
			}
			continue
		}
		log.Info("Deleting addon", "namespace", ns, "name", addon.GetName())
		if err := r.Client.Delete(ctx, addon); err != nil && !errors.IsNotFound(err) {
			return false, fmt.Errorf("deleting addon %s/%s: %w", ns, addon.GetName(), err)
		}
		remaining++
	}

	if remaining > 0 {
		msg := fmt.Sprintf("Waiting for %d addons to terminate.", remaining)
		needsApproval := false
		for i := range list.Items {
			addon := &list.Items[i]
			if addon.GetNamespace() == "local-cluster" || addon.GetDeletionTimestamp() == nil {
				continue
			}
			for _, f := range addon.GetFinalizers() {
				if !IsAllowlistedFinalizer(f) {
					continue
				}
				if IsCloudProtectingFinalizer(f) && !td.Spec.AcknowledgeCloudResourceRisk {
					msg += " Set spec.acknowledgeCloudResourceRisk=true to patch Tier 1 (cloud-protecting) finalizers."
					needsApproval = true
				} else if !IsCloudProtectingFinalizer(f) && !td.Spec.ApprovedDestructiveActions {
					msg += " Set spec.approvedDestructiveActions=true to patch Tier 2 (non-cloud) finalizers."
					needsApproval = true
				}
				if needsApproval {
					break
				}
			}
			if needsApproval {
				break
			}
		}
		r.setPhaseStatus(td, operatorv1.TeardownPhaseDisableAddons, operatorv1.PhaseStateInProgress, msg)
		log.Info(msg, "remaining", remaining)
		return false, nil
	}
	return true, nil
}

// disableAddonPlacements deletes Placement references from ClusterManagementAddOns
// to prevent the addon-manager from recreating MCAs after deletion.
func (r *HubTeardownReconciler) disableAddonPlacements(ctx context.Context, log logr.Logger) error {
	cmaGVK := schema.GroupVersionKind{
		Group:   "addon.open-cluster-management.io",
		Version: "v1alpha1",
		Kind:    "ClusterManagementAddOnList",
	}
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(cmaGVK)

	if err := r.Client.List(ctx, list); err != nil {
		if isNoMatchError(err) {
			return nil
		}
		return fmt.Errorf("listing ClusterManagementAddOns: %w", err)
	}

	for i := range list.Items {
		cma := &list.Items[i]
		spec, ok, _ := unstructured.NestedMap(cma.Object, "spec")
		if !ok || spec == nil {
			continue
		}
		_, hasInstallStrategy, _ := unstructured.NestedSlice(cma.Object, "spec", "installStrategy", "placements")
		if !hasInstallStrategy {
			continue
		}
		patch := client.MergeFrom(cma.DeepCopy())
		unstructured.RemoveNestedField(cma.Object, "spec", "installStrategy", "placements")
		if err := r.Client.Patch(ctx, cma, patch); err != nil {
			if !errors.IsNotFound(err) {
				log.Error(err, "Failed to remove placements from ClusterManagementAddOn", "name", cma.GetName())
			}
			continue
		}
		log.Info("Disabled addon placements", "clusterManagementAddOn", cma.GetName())
	}
	return nil
}

// knownAddonControllerDeployments is a fallback list of deployments that
// recreate MCAs independently of ClusterManagementAddOn placements.
// Used only if label-based discovery finds nothing.
var knownAddonControllerDeployments = []string{
	"klusterlet-addon-controller-v2",
	"grc-policy-addon-controller",
	"volsync-addon-controller",
	"submariner-addon",
}

// scaleDownAddonControllers sets replicas=0 on addon controller deployments
// to break the MCA recreation loop during DisableAddons.
//
// Discovery strategy: first try label-based discovery to find addon controllers
// dynamically (survives new addon additions across releases), then fall back
// to the known-names list for controllers that lack standard labels.
func (r *HubTeardownReconciler) scaleDownAddonControllers(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) {
	ns := td.Namespace
	scaled := make(map[string]bool)

	// Label-based discovery: find deployments that manage addons.
	deployList := &appsv1.DeploymentList{}
	if err := r.Client.List(ctx, deployList, client.InNamespace(ns)); err != nil {
		log.Error(err, "Failed to list deployments for addon controller discovery")
	} else {
		for i := range deployList.Items {
			deploy := &deployList.Items[i]
			if !isAddonControllerDeployment(deploy) {
				continue
			}
			r.scaleDeploymentToZero(ctx, log, deploy)
			scaled[deploy.Name] = true
		}
	}

	// Fallback: scale known names that label discovery may have missed.
	for _, name := range knownAddonControllerDeployments {
		if scaled[name] {
			continue
		}
		deploy := &appsv1.Deployment{}
		key := types.NamespacedName{Name: name, Namespace: ns}
		if err := r.Client.Get(ctx, key, deploy); err != nil {
			continue
		}
		r.scaleDeploymentToZero(ctx, log, deploy)
	}

	r.resolveCMAFinalizersForScaledDownControllers(ctx, log, td)
}

func (r *HubTeardownReconciler) scaleDeploymentToZero(ctx context.Context, log logr.Logger, deploy *appsv1.Deployment) {
	if deploy.Spec.Replicas != nil && *deploy.Spec.Replicas == 0 {
		return
	}
	zero := int32(0)
	patch := client.MergeFrom(deploy.DeepCopy())
	deploy.Spec.Replicas = &zero
	if err := r.Client.Patch(ctx, deploy, patch); err != nil {
		log.Error(err, "Failed to scale down addon controller", "deployment", deploy.Name)
		return
	}
	log.Info("Scaled down addon controller to break recreation loop", "deployment", deploy.Name)
}

// deleteResidualAddonDeployments removes addon controller deployments that were
// scaled to 0 by scaleDownAddonControllers but not deleted during earlier phases.
// Only deployments with Spec.Replicas == 0 that match addon labels or known names
// are deleted.
func (r *HubTeardownReconciler) deleteResidualAddonDeployments(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) {
	ns := td.Namespace
	deployList := &appsv1.DeploymentList{}
	if err := r.Client.List(ctx, deployList, client.InNamespace(ns)); err != nil {
		log.Error(err, "Failed to list deployments for residual addon cleanup")
		return
	}

	knownNames := make(map[string]bool, len(knownAddonControllerDeployments))
	for _, name := range knownAddonControllerDeployments {
		knownNames[name] = true
	}

	for i := range deployList.Items {
		deploy := &deployList.Items[i]
		if deploy.Spec.Replicas == nil || *deploy.Spec.Replicas != 0 {
			continue
		}

		isAddon := knownNames[deploy.Name] || isAddonControllerDeployment(deploy)
		if !isAddon {
			continue
		}

		if err := r.Client.Delete(ctx, deploy); err != nil && !errors.IsNotFound(err) {
			log.Error(err, "Failed to delete residual addon deployment", "deployment", deploy.Name)
			continue
		}
		log.Info("Deleted residual addon deployment", "deployment", deploy.Name)
	}
}

// resolveCMAFinalizersForScaledDownControllers resolves finalizers on
// terminating ClusterManagementAddOns whose owning controller has been scaled down.
func (r *HubTeardownReconciler) resolveCMAFinalizersForScaledDownControllers(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) {
	cmaGVK := schema.GroupVersionKind{
		Group:   "addon.open-cluster-management.io",
		Version: "v1alpha1",
		Kind:    "ClusterManagementAddOnList",
	}
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(cmaGVK)

	if err := r.Client.List(ctx, list); err != nil {
		return
	}

	for i := range list.Items {
		cma := &list.Items[i]
		if cma.GetDeletionTimestamp() == nil || len(cma.GetFinalizers()) == 0 {
			continue
		}
		dr := DiscoveredResource{
			Ref: operatorv1.ResourceRef{
				Group: "addon.open-cluster-management.io",
				Kind:  "ClusterManagementAddOn",
				Name:  cma.GetName(),
			},
			Version:    "v1alpha1",
			Finalizers: cma.GetFinalizers(),
		}
		if _, err := r.resolveStuckFinalizers(ctx, log, td, []DiscoveredResource{dr}); err != nil {
			log.Error(err, "Failed to resolve CMA finalizer", "name", cma.GetName())
		}
	}
}

// getUnreachableManagedClusters returns a set of MC names where the cluster
// has never been imported or has lost connectivity (Available=Unknown/False).
func (r *HubTeardownReconciler) getUnreachableManagedClusters(ctx context.Context, log logr.Logger) map[string]bool {
	result := make(map[string]bool)
	mcGVK := schema.GroupVersionKind{
		Group:   "cluster.open-cluster-management.io",
		Version: "v1",
		Kind:    "ManagedClusterList",
	}
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(mcGVK)

	if err := r.Client.List(ctx, list); err != nil {
		return result
	}

	for i := range list.Items {
		mc := &list.Items[i]
		name := mc.GetName()
		if name == "local-cluster" {
			continue
		}
		conditions, _, _ := unstructured.NestedSlice(mc.Object, "status", "conditions")
		for _, c := range conditions {
			cond, ok := c.(map[string]interface{})
			if !ok {
				continue
			}
			condType, _ := cond["type"].(string)
			condStatus, _ := cond["status"].(string)
			if condType == "ManagedClusterConditionAvailable" && condStatus != "True" {
				log.Info("ManagedCluster unreachable, will force addon cleanup", "cluster", name, "status", condStatus)
				result[name] = true
				break
			}
		}
	}
	return result
}

// resolveAddonFinalizers resolves stuck addon finalizers on a single MCA.
func (r *HubTeardownReconciler) resolveAddonFinalizers(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown, addon *unstructured.Unstructured) {
	finalizers := addon.GetFinalizers()
	if len(finalizers) == 0 {
		return
	}
	dr := DiscoveredResource{
		Ref: operatorv1.ResourceRef{
			Group:     "addon.open-cluster-management.io",
			Kind:      "ManagedClusterAddOn",
			Namespace: addon.GetNamespace(),
			Name:      addon.GetName(),
		},
		Version:    "v1alpha1",
		Finalizers: finalizers,
	}
	if _, err := r.resolveStuckFinalizers(ctx, log, td, []DiscoveredResource{dr}); err != nil {
		log.Error(err, "Error resolving addon finalizers", "addon", addon.GetNamespace()+"/"+addon.GetName())
	}
}

// phaseDeleteInfrastructureCRs deletes all infrastructure provisioning CRs (HostedClusters,
// ClusterDeployments, ClusterPools, NodePools, InfraEnvs) while their controllers are still
// alive to handle graceful deprovisioning. Force-resolves finalizers after timeout.
//
// The acknowledgeCloudResourceRisk gate is only applied when infrastructure CRs
// actually exist. Hubs with no cloud-provisioned clusters skip this gate entirely.
func (r *HubTeardownReconciler) phaseDeleteInfrastructureCRs(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) (bool, error) {
	log.Info("Phase: DeleteInfrastructureCRs")

	infraTypes := []struct {
		group   string
		version string
		kind    string
	}{
		{"hypershift.openshift.io", "v1beta1", "HostedCluster"},
		{"hypershift.openshift.io", "v1beta1", "NodePool"},
		{"hive.openshift.io", "v1", "ClusterDeployment"},
		{"hive.openshift.io", "v1", "ClusterPool"},
		{"agent-install.openshift.io", "v1beta1", "InfraEnv"},
		{"siteconfig.open-cluster-management.io", "v1alpha1", "ClusterInstance"},
	}

	// Collect all existing infrastructure CR instances first.
	type infraItem struct {
		obj     *unstructured.Unstructured
		group   string
		version string
		kind    string
	}
	var allItems []infraItem

	for _, it := range infraTypes {
		listGVK := schema.GroupVersionKind{
			Group:   it.group,
			Version: it.version,
			Kind:    it.kind + "List",
		}
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(listGVK)

		if err := r.Client.List(ctx, list); err != nil {
			if isNoMatchError(err) {
				continue
			}
			return false, fmt.Errorf("listing %s: %w", it.kind, err)
		}

		for i := range list.Items {
			allItems = append(allItems, infraItem{
				obj:     &list.Items[i],
				group:   it.group,
				version: it.version,
				kind:    it.kind,
			})
		}
	}

	// No infrastructure CRs on this hub — skip the cloud risk gate entirely.
	if len(allItems) == 0 {
		log.Info("No infrastructure CRs found, skipping phase")
		return true, nil
	}

	// Gate: require explicit acknowledgment only when infra CRs actually exist.
	if !td.Spec.AcknowledgeCloudResourceRisk {
		log.Info("Waiting for spec.acknowledgeCloudResourceRisk=true before deleting infrastructure CRs",
			"infraCRCount", len(allItems))
		r.Recorder.Eventf(td, corev1.EventTypeWarning, "CloudRiskGate",
			"%d infrastructure CRs (HostedClusters, ClusterDeployments, etc.) require spec.acknowledgeCloudResourceRisk=true to proceed",
			len(allItems))
		return false, nil
	}

	remaining := 0
	for _, ii := range allItems {
		item := ii.obj

		if item.GetDeletionTimestamp() != nil {
			if r.shouldResolveStuckFinalizers(td, item) && td.Spec.AcknowledgeCloudResourceRisk {
				dr := DiscoveredResource{
					Ref: operatorv1.ResourceRef{
						Group:     ii.group,
						Kind:      ii.kind,
						Namespace: item.GetNamespace(),
						Name:      item.GetName(),
					},
					Version:    ii.version,
					Finalizers: item.GetFinalizers(),
				}
				resolved, err := r.resolveStuckFinalizers(ctx, log, td, []DiscoveredResource{dr})
				if err != nil {
					log.Error(err, "Error resolving finalizers on infrastructure CR",
						"kind", ii.kind, "name", item.GetName(), "namespace", item.GetNamespace())
				}
				if !resolved {
					remaining++
				}
			} else {
				remaining++
			}
			continue
		}

		log.Info("Deleting infrastructure CR",
			"kind", ii.kind, "name", item.GetName(), "namespace", item.GetNamespace())
		if err := r.Client.Delete(ctx, item); err != nil && !errors.IsNotFound(err) {
			return false, fmt.Errorf("deleting %s %s/%s: %w", ii.kind, item.GetNamespace(), item.GetName(), err)
		}
		r.Recorder.Eventf(td, corev1.EventTypeNormal, "InfrastructureCRDeleted",
			"Deleted %s %s/%s", ii.kind, item.GetNamespace(), item.GetName())
		remaining++
	}

	if remaining > 0 {
		msg := fmt.Sprintf("Waiting for %d infrastructure CRs to terminate.", remaining)
		needsApproval := false
		for _, ii := range allItems {
			item := ii.obj
			if item.GetDeletionTimestamp() == nil {
				continue
			}
			for _, f := range item.GetFinalizers() {
				if !IsAllowlistedFinalizer(f) {
					continue
				}
				if IsCloudProtectingFinalizer(f) && !td.Spec.AcknowledgeCloudResourceRisk {
					msg += " Set spec.acknowledgeCloudResourceRisk=true to patch Tier 1 (cloud-protecting) finalizers."
					needsApproval = true
				} else if !IsCloudProtectingFinalizer(f) && !td.Spec.ApprovedDestructiveActions {
					msg += " Set spec.approvedDestructiveActions=true to patch Tier 2 (non-cloud) finalizers."
					needsApproval = true
				}
				if needsApproval {
					break
				}
			}
			if needsApproval {
				break
			}
		}
		r.setPhaseStatus(td, operatorv1.TeardownPhaseDeleteInfrastructureCRs, operatorv1.PhaseStateInProgress, msg)
		log.Info(msg, "remaining", remaining)
		return false, nil
	}

	log.Info("All infrastructure CRs deleted")
	return true, nil
}

// phaseDetachManagedClusters deletes non-local ManagedClusters so the MCH webhook passes.
func (r *HubTeardownReconciler) phaseDetachManagedClusters(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) (bool, error) {
	log.Info("Phase: DetachManagedClusters")

	mcGVK := schema.GroupVersionKind{
		Group:   "cluster.open-cluster-management.io",
		Version: "v1",
		Kind:    "ManagedClusterList",
	}
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(mcGVK)

	if err := r.Client.List(ctx, list); err != nil {
		if isNoMatchError(err) {
			return true, nil
		}
		return false, fmt.Errorf("listing ManagedClusters: %w", err)
	}

	remaining := 0
	for i := range list.Items {
		mc := &list.Items[i]
		if mc.GetName() == "local-cluster" {
			continue
		}

		if mc.GetDeletionTimestamp() != nil {
			remaining++

			if r.shouldResolveStuckFinalizers(td, mc) {
				dr := DiscoveredResource{
					Ref: operatorv1.ResourceRef{
						Group: "cluster.open-cluster-management.io",
						Kind:  "ManagedCluster",
						Name:  mc.GetName(),
					},
					Version:    "v1",
					Finalizers: mc.GetFinalizers(),
				}
				resolved, err := r.resolveStuckFinalizers(ctx, log, td, []DiscoveredResource{dr})
				if err != nil {
					log.Error(err, "Error resolving stuck finalizers on ManagedCluster", "name", mc.GetName())
				}
				if resolved {
					remaining--
				} else if td.Spec.ApprovedDestructiveActions {
					// Gap 12 safety net: if allowlisted resolution did not clear all finalizers
					// (e.g. agent-side finalizers from an offline/destroyed cluster), force-strip
					// ALL remaining finalizers after the timeout. This prevents indefinite hang.
					// Require acknowledgeCloudResourceRisk if any cloud-protecting finalizers remain.
					hasCloudFinalizer := false
					for _, f := range mc.GetFinalizers() {
						if IsCloudProtectingFinalizer(f) {
							hasCloudFinalizer = true
							break
						}
					}
					if hasCloudFinalizer && !td.Spec.AcknowledgeCloudResourceRisk {
						log.Info("Skipping force-strip: ManagedCluster has cloud-protecting finalizers but acknowledgeCloudResourceRisk is not set",
							"name", mc.GetName())
					} else {
						ref := operatorv1.ResourceRef{
							Group: "cluster.open-cluster-management.io",
							Kind:  "ManagedCluster",
							Name:  mc.GetName(),
						}
						gvk := schema.GroupVersionKind{
							Group:   "cluster.open-cluster-management.io",
							Version: "v1",
							Kind:    "ManagedCluster",
						}
						if err := r.forceStripAllFinalizers(ctx, log, td, ref, gvk); err != nil {
							log.Error(err, "Failed to force-strip finalizers on ManagedCluster", "name", mc.GetName())
						} else {
							remaining--
						}
					}
				}
			}
			continue
		}

		log.Info("Deleting ManagedCluster", "name", mc.GetName())
		if err := r.Client.Delete(ctx, mc); err != nil && !errors.IsNotFound(err) {
			return false, fmt.Errorf("deleting ManagedCluster %s: %w", mc.GetName(), err)
		}
		r.Recorder.Eventf(td, corev1.EventTypeNormal, "ManagedClusterDetached",
			"Deleted ManagedCluster %s", mc.GetName())
		remaining++
	}

	if remaining > 0 {
		msg := fmt.Sprintf("Waiting for %d ManagedClusters to terminate.", remaining)
		needsApproval := false
		for i := range list.Items {
			mc := &list.Items[i]
			if mc.GetName() == "local-cluster" || mc.GetDeletionTimestamp() == nil {
				continue
			}
			for _, f := range mc.GetFinalizers() {
				if !IsAllowlistedFinalizer(f) {
					continue
				}
				if IsCloudProtectingFinalizer(f) && !td.Spec.AcknowledgeCloudResourceRisk {
					msg += " Set spec.acknowledgeCloudResourceRisk=true to patch Tier 1 (cloud-protecting) finalizers."
					needsApproval = true
				} else if !IsCloudProtectingFinalizer(f) && !td.Spec.ApprovedDestructiveActions {
					msg += " Set spec.approvedDestructiveActions=true to patch Tier 2 (non-cloud) finalizers."
					needsApproval = true
				}
				if needsApproval {
					break
				}
			}
			if needsApproval {
				break
			}
		}
		r.setPhaseStatus(td, operatorv1.TeardownPhaseDetachManagedClusters, operatorv1.PhaseStateInProgress, msg)
		log.Info(msg, "remaining", remaining)
		return false, nil
	}

	r.sweepMCSAndCMA(ctx, log, td)
	return true, nil
}

// phaseDeleteMCH deletes the MultiClusterHub CR.
//
// CONCURRENT RECONCILER NOTE: Deleting the MCH triggers the MCH reconciler's
// finalizeHub in the same operator process. finalizeHub and HubTeardown phases
// 7-9 (MonitorMCEChain, CleanOrphans, DeleteACMCRDs) overlap in scope — both
// delete operands, remove CRDs, and clean up resources. This is by design:
// HubTeardown orchestrates the preconditions so finalizeHub can run cleanly,
// then monitors and mops up what finalizeHub leaves behind. The overlap is
// safe because all delete operations are idempotent (NotFound is ignored),
// but duplicate events and log entries are expected during this window.
func (r *HubTeardownReconciler) phaseDeleteMCH(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) (bool, error) {
	log.Info("Phase: DeleteMCH")

	mchGVK := schema.GroupVersionKind{
		Group:   "operator.open-cluster-management.io",
		Version: "v1",
		Kind:    "MultiClusterHubList",
	}
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(mchGVK)

	if err := r.Client.List(ctx, list); err != nil {
		return false, fmt.Errorf("listing MultiClusterHub: %w", err)
	}

	if len(list.Items) == 0 {
		log.Info("No MultiClusterHub found, phase complete")
		return true, nil
	}

	for i := range list.Items {
		mch := &list.Items[i]
		if mch.GetDeletionTimestamp() != nil {
			if r.shouldResolveStuckFinalizers(td, mch) {
				log.Info("MCH stuck deleting past timeout, resolving blocking finalizers", "name", mch.GetName())
				if err := r.resolveMCHBlockingFinalizers(ctx, log, td); err != nil {
					log.Error(err, "Error resolving MCH-blocking finalizers")
				}
				dr := DiscoveredResource{
					Ref: operatorv1.ResourceRef{
						Group:     "operator.open-cluster-management.io",
						Kind:      "MultiClusterHub",
						Namespace: mch.GetNamespace(),
						Name:      mch.GetName(),
					},
					Version:    "v1",
					Finalizers: mch.GetFinalizers(),
				}
				if _, err := r.resolveStuckFinalizers(ctx, log, td, []DiscoveredResource{dr}); err != nil {
					log.Error(err, "Error resolving MCH finalizers")
				}
			} else {
				log.Info("MCH already deleting, waiting for finalizer to complete", "name", mch.GetName())
			}
			return false, nil
		}
		log.Info("Deleting MultiClusterHub", "name", mch.GetName(), "namespace", mch.GetNamespace())
		if err := r.Client.Delete(ctx, mch); err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return false, fmt.Errorf("deleting MCH %s: %w", mch.GetName(), err)
		}
		r.Recorder.Eventf(td, corev1.EventTypeNormal, "MCHDeleted",
			"Deleted MultiClusterHub %s/%s", mch.GetNamespace(), mch.GetName())
	}

	return false, nil
}

// resolveMCHBlockingFinalizers resolves stuck finalizers on resources that
// block the MCH's own finalizer (finalizeHub). Submariner's cleanup finalizers
// on ClusterManagementAddOns and ManagedClusterSets are the known blockers.
func (r *HubTeardownReconciler) resolveMCHBlockingFinalizers(
	ctx context.Context,
	log logr.Logger,
	td *operatorv1.HubTeardown,
) error {
	blockerTypes := []struct {
		group   string
		version string
		kind    string
	}{
		{"addon.open-cluster-management.io", "v1alpha1", "ClusterManagementAddOn"},
		{"cluster.open-cluster-management.io", "v1beta2", "ManagedClusterSet"},
	}

	for _, bt := range blockerTypes {
		listGVK := schema.GroupVersionKind{
			Group:   bt.group,
			Version: bt.version,
			Kind:    bt.kind + "List",
		}
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(listGVK)

		if err := r.Client.List(ctx, list); err != nil {
			if isNoMatchError(err) || errors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("listing %s: %w", bt.kind, err)
		}

		for i := range list.Items {
			item := &list.Items[i]
			if item.GetDeletionTimestamp() == nil || len(item.GetFinalizers()) == 0 {
				continue
			}

			log.Info("Resolving stuck finalizers on MCH-blocking resource",
				"kind", bt.kind, "name", item.GetName())
			dr := DiscoveredResource{
				Ref: operatorv1.ResourceRef{
					Group:     bt.group,
					Kind:      bt.kind,
					Namespace: item.GetNamespace(),
					Name:      item.GetName(),
				},
				Version:    bt.version,
				Finalizers: item.GetFinalizers(),
			}
			if _, err := r.resolveStuckFinalizers(ctx, log, td, []DiscoveredResource{dr}); err != nil {
				log.Error(err, "Failed to resolve finalizers on MCH-blocking resource",
					"kind", bt.kind, "name", item.GetName())
			}
		}
	}
	return nil
}

// sweepMCSAndCMA proactively deletes ManagedClusterSet and ClusterManagementAddOn
// resources and strips their finalizers. Called at the end of phaseDetachManagedClusters
// to clear Submariner/addon finalizers whose processing controllers have been
// scaled down or removed, so finalizeHub runs unobstructed when MCH is deleted.
func (r *HubTeardownReconciler) sweepMCSAndCMA(
	ctx context.Context,
	log logr.Logger,
	td *operatorv1.HubTeardown,
) {
	sweepTypes := []struct {
		group   string
		version string
		kind    string
	}{
		{"addon.open-cluster-management.io", "v1alpha1", "ClusterManagementAddOn"},
		{"cluster.open-cluster-management.io", "v1beta2", "ManagedClusterSet"},
	}

	for _, st := range sweepTypes {
		listGVK := schema.GroupVersionKind{
			Group:   st.group,
			Version: st.version,
			Kind:    st.kind + "List",
		}
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(listGVK)

		if err := r.Client.List(ctx, list); err != nil {
			if isNoMatchError(err) || errors.IsNotFound(err) {
				continue
			}
			log.Error(err, "Failed to list resources for MCS/CMA sweep", "kind", st.kind)
			continue
		}

		for i := range list.Items {
			item := &list.Items[i]

			if item.GetDeletionTimestamp() == nil {
				log.Info("Sweeping pre-MCH-delete resource", "kind", st.kind, "name", item.GetName())
				if err := r.Client.Delete(ctx, item); err != nil && !errors.IsNotFound(err) {
					log.Error(err, "Failed to delete in MCS/CMA sweep", "kind", st.kind, "name", item.GetName())
					continue
				}
				r.Recorder.Eventf(td, corev1.EventTypeNormal, "MCSCMASweep",
					"Deleted %s %s before MCH delete (controller scaled down)", st.kind, item.GetName())
			}

			if len(item.GetFinalizers()) > 0 && r.shouldResolveStuckFinalizers(td, item) {
				dr := drFromUnstructured(item, st.group, st.version, st.kind)
				if _, err := r.resolveStuckFinalizers(ctx, log, td, []DiscoveredResource{dr}); err != nil {
					log.Error(err, "Failed to resolve finalizers in MCS/CMA sweep",
						"kind", st.kind, "name", item.GetName())
				}
			}
		}
	}
}

// phaseMonitorMCEChain drives the MCE -> ClusterManager -> Hypershift cleanup chain.
// When the MCH finalizer cascade ran normally, MCE is already deleting and this
// phase monitors its removal. When MCH was already gone (cascade did not fire),
// this phase actively initiates MCE deletion, cleans up local-cluster, and
// resolves orphaned addon/submariner finalizers.
func (r *HubTeardownReconciler) phaseMonitorMCEChain(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) (bool, error) {
	log.Info("Phase: MonitorMCEChain")

	mchGone, err := r.isResourceGone(ctx, "operator.open-cluster-management.io", "v1", "MultiClusterHub")
	if err != nil {
		return false, err
	}
	if !mchGone {
		log.Info("Waiting for MultiClusterHub to be fully removed")
		return false, nil
	}

	// Resolve stuck submariner/addon finalizers on ClusterManagementAddOns and
	// ManagedClusterSets. When MCH was present, resolveMCHBlockingFinalizers ran
	// inside phaseDeleteMCH. When MCH was already gone, the call never fired.
	if err := r.resolveMCHBlockingFinalizers(ctx, log, td); err != nil {
		log.Error(err, "Error resolving MCH-blocking finalizers in MonitorMCEChain")
	}

	// Clean up local-cluster addons and ManagedCluster. Earlier phases skip
	// local-cluster because the MCH webhook requires it for MCH deletion. Now
	// that MCH is gone, local-cluster can be safely removed.
	if err := r.cleanupLocalCluster(ctx, log, td); err != nil {
		log.Error(err, "Error cleaning up local-cluster")
	}

	// Ensure MCE is deleted. When the MCH finalizer cascade ran, MCE is already
	// deleting. When MCH was already gone, nobody issued the delete.
	if err := r.ensureMCEDeleted(ctx, log, td); err != nil {
		return false, err
	}

	mceGone, err := r.isResourceGone(ctx, "multicluster.openshift.io", "v1", "MultiClusterEngine")
	if err != nil {
		return false, err
	}
	if !mceGone {
		log.Info("Waiting for MultiClusterEngine to be fully removed")
		return false, nil
	}

	// Resolve orphaned ClusterManager CRs whose operator deployment was removed
	// during MCE uninstall, leaving an unprocessable finalizer.
	if err := r.resolveOrphanedClusterManager(ctx, log, td); err != nil {
		return false, err
	}

	log.Info("MCH and MCE chain cleanup complete")
	r.Recorder.Event(td, corev1.EventTypeNormal, "MCEChainComplete",
		"MultiClusterHub and MultiClusterEngine have been fully removed")
	return true, nil
}

// ensureMCEDeleted drives MCE deletion with liveness-aware escalation.
// Instead of force-stripping MCE finalizers after a wall-clock timeout, it
// checks if the MCE operator is alive. If alive, the natural chain (MCE
// operator -> ClusterManager -> OCM hub cleanup) is working — just wait.
// If dead, escalate in chain order: resolve ClusterManager first, then MCE.
func (r *HubTeardownReconciler) ensureMCEDeleted(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) error {
	mceGVK := schema.GroupVersionKind{
		Group:   "multicluster.openshift.io",
		Version: "v1",
		Kind:    "MultiClusterEngineList",
	}
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(mceGVK)

	if err := r.Client.List(ctx, list); err != nil {
		if isNoMatchError(err) {
			return nil
		}
		return fmt.Errorf("listing MultiClusterEngine: %w", err)
	}

	for i := range list.Items {
		mce := &list.Items[i]
		if mce.GetDeletionTimestamp() != nil {
			if r.isMCEOperatorAlive(ctx, log) {
				log.Info("MCE is deleting and MCE operator is alive — waiting for natural chain",
					"name", mce.GetName())
				return nil
			}

			log.Info("MCE operator is not alive — escalating cleanup in chain order", "name", mce.GetName())

			cmHandled, err := r.handleOrphanedClusterManagerChain(ctx, log, td)
			if err != nil {
				return fmt.Errorf("handling ClusterManager chain for dead MCE operator: %w", err)
			}
			if !cmHandled {
				log.Info("ClusterManager still being processed, waiting before stripping MCE finalizers")
				return nil
			}

			log.Info("ClusterManager gone — stripping MCE finalizers", "name", mce.GetName())
			dr := drFromUnstructured(mce, "multicluster.openshift.io", "v1", "MultiClusterEngine")
			if _, err := r.resolveStuckFinalizers(ctx, log, td, []DiscoveredResource{dr}); err != nil {
				log.Error(err, "Error resolving MCE finalizers after chain escalation")
			}
			r.Recorder.Eventf(td, corev1.EventTypeNormal, "MCEFinalizersStripped",
				"Stripped MCE finalizers on %s (MCE operator dead, ClusterManager chain resolved)", mce.GetName())
			continue
		}

		log.Info("Deleting MultiClusterEngine (MCH cascade did not fire)", "name", mce.GetName())
		if err := r.Client.Delete(ctx, mce); err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			return fmt.Errorf("deleting MCE %s: %w", mce.GetName(), err)
		}
		r.Recorder.Eventf(td, corev1.EventTypeNormal, "MCEDeleted",
			"Deleted MultiClusterEngine %s", mce.GetName())
	}
	return nil
}

// handleOrphanedClusterManagerChain drives ClusterManager cleanup when the MCE
// operator is dead. Follows chain order: delete ClusterManager, let its operator
// clean up OCM hub resources, or strip finalizers if the operator is also dead.
// Returns (true, nil) when all ClusterManagers are gone.
func (r *HubTeardownReconciler) handleOrphanedClusterManagerChain(
	ctx context.Context,
	log logr.Logger,
	td *operatorv1.HubTeardown,
) (bool, error) {
	cmGVK := schema.GroupVersionKind{
		Group:   "operator.open-cluster-management.io",
		Version: "v1",
		Kind:    "ClusterManagerList",
	}
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(cmGVK)

	if err := r.Client.List(ctx, list); err != nil {
		if isNoMatchError(err) || errors.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("listing ClusterManagers: %w", err)
	}

	if len(list.Items) == 0 {
		return true, nil
	}

	for i := range list.Items {
		cm := &list.Items[i]

		if cm.GetDeletionTimestamp() == nil {
			log.Info("ClusterManager not yet deleting — sending Delete", "name", cm.GetName())
			if err := r.Client.Delete(ctx, cm); err != nil && !errors.IsNotFound(err) {
				return false, fmt.Errorf("deleting ClusterManager %s: %w", cm.GetName(), err)
			}
			r.Recorder.Eventf(td, corev1.EventTypeNormal, "ClusterManagerDeleteSent",
				"Sent Delete for ClusterManager %s (MCE operator dead)", cm.GetName())

			if r.isClusterManagerOperatorAlive(ctx, log) {
				log.Info("cluster-manager operator is alive — waiting for it to process ClusterManager deletion")
				return false, nil
			}

			if len(cm.GetFinalizers()) > 0 {
				log.Info("cluster-manager operator is dead — stripping ClusterManager finalizers", "name", cm.GetName())
				dr := drFromUnstructured(cm, "operator.open-cluster-management.io", "v1", "ClusterManager")
				if _, err := r.resolveStuckFinalizers(ctx, log, td, []DiscoveredResource{dr}); err != nil {
					log.Error(err, "Failed to resolve ClusterManager finalizers", "name", cm.GetName())
				}
				r.Recorder.Eventf(td, corev1.EventTypeWarning, "ClusterManagerFinalizersStripped",
					"Stripped finalizers from ClusterManager %s (cluster-manager operator dead)", cm.GetName())
			}
			continue
		}

		if r.isClusterManagerOperatorAlive(ctx, log) {
			log.Info("ClusterManager is deleting and cluster-manager operator is alive — waiting",
				"name", cm.GetName())
			return false, nil
		}

		if len(cm.GetFinalizers()) > 0 {
			log.Info("ClusterManager deleting but cluster-manager operator dead — stripping finalizers",
				"name", cm.GetName())
			dr := drFromUnstructured(cm, "operator.open-cluster-management.io", "v1", "ClusterManager")
			if _, err := r.resolveStuckFinalizers(ctx, log, td, []DiscoveredResource{dr}); err != nil {
				log.Error(err, "Failed to resolve ClusterManager finalizers", "name", cm.GetName())
			}
			r.Recorder.Eventf(td, corev1.EventTypeWarning, "ClusterManagerFinalizersStripped",
				"Stripped finalizers from ClusterManager %s (cluster-manager operator dead)", cm.GetName())
		}
	}

	return false, nil
}

// cleanupLocalCluster deletes local-cluster ManagedClusterAddOns and the
// local-cluster ManagedCluster. Earlier phases skip local-cluster because the
// MCH webhook requires it for MCH deletion. This function runs after MCH is
// confirmed gone, so the webhook constraint is satisfied.
func (r *HubTeardownReconciler) cleanupLocalCluster(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) error {
	addonGVK := schema.GroupVersionKind{
		Group:   "addon.open-cluster-management.io",
		Version: "v1alpha1",
		Kind:    "ManagedClusterAddOnList",
	}
	addonList := &unstructured.UnstructuredList{}
	addonList.SetGroupVersionKind(addonGVK)

	if err := r.Client.List(ctx, addonList, client.InNamespace("local-cluster")); err != nil {
		if !isNoMatchError(err) {
			return fmt.Errorf("listing local-cluster addons: %w", err)
		}
	} else {
		for i := range addonList.Items {
			addon := &addonList.Items[i]
			if addon.GetDeletionTimestamp() != nil {
				if r.shouldResolveStuckFinalizers(td, addon) {
					r.resolveAddonFinalizers(ctx, log, td, addon)
				}
				continue
			}
			log.Info("Deleting local-cluster addon", "name", addon.GetName())
			if err := r.Client.Delete(ctx, addon); err != nil && !errors.IsNotFound(err) {
				return fmt.Errorf("deleting local-cluster addon %s: %w", addon.GetName(), err)
			}
			r.Recorder.Eventf(td, corev1.EventTypeNormal, "LocalClusterAddonDeleted",
				"Deleted local-cluster ManagedClusterAddOn %s", addon.GetName())
		}
	}

	mcGVK := schema.GroupVersionKind{
		Group:   "cluster.open-cluster-management.io",
		Version: "v1",
		Kind:    "ManagedCluster",
	}
	mc := &unstructured.Unstructured{}
	mc.SetGroupVersionKind(mcGVK)
	mcKey := types.NamespacedName{Name: "local-cluster"}
	if err := r.Client.Get(ctx, mcKey, mc); err != nil {
		if errors.IsNotFound(err) || isNoMatchError(err) {
			return nil
		}
		return fmt.Errorf("getting local-cluster ManagedCluster: %w", err)
	}

	if mc.GetDeletionTimestamp() != nil {
		if r.shouldResolveStuckFinalizers(td, mc) {
			dr := DiscoveredResource{
				Ref: operatorv1.ResourceRef{
					Group: "cluster.open-cluster-management.io",
					Kind:  "ManagedCluster",
					Name:  "local-cluster",
				},
				Version:    "v1",
				Finalizers: mc.GetFinalizers(),
			}
			if _, err := r.resolveStuckFinalizers(ctx, log, td, []DiscoveredResource{dr}); err != nil {
				log.Error(err, "Error resolving local-cluster finalizers")
			}
		}
		return nil
	}

	log.Info("Deleting local-cluster ManagedCluster")
	if err := r.Client.Delete(ctx, mc); err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("deleting local-cluster ManagedCluster: %w", err)
	}
	r.Recorder.Eventf(td, corev1.EventTypeNormal, "LocalClusterDetached",
		"Deleted local-cluster ManagedCluster (post-MCH cleanup)")
	return nil
}

// resolveOrphanedClusterManager detects ClusterManager CRs whose owning
// operator is gone. Handles both deleting CMs (stuck finalizer) and
// non-deleting CMs (MCE operator died before sending Delete).
func (r *HubTeardownReconciler) resolveOrphanedClusterManager(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) error {
	cmGVK := schema.GroupVersionKind{
		Group:   "operator.open-cluster-management.io",
		Version: "v1",
		Kind:    "ClusterManagerList",
	}
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(cmGVK)

	if err := r.Client.List(ctx, list); err != nil {
		if isNoMatchError(err) || errors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("listing ClusterManagers: %w", err)
	}

	for i := range list.Items {
		cm := &list.Items[i]
		if len(cm.GetFinalizers()) == 0 {
			continue
		}

		if cm.GetDeletionTimestamp() == nil {
			if r.isClusterManagerOperatorAlive(ctx, log) {
				continue
			}
			log.Info("ClusterManager not deleting, operator dead — sending Delete and stripping finalizers",
				"name", cm.GetName())
			if err := r.Client.Delete(ctx, cm); err != nil && !errors.IsNotFound(err) {
				log.Error(err, "Failed to send Delete for non-deleting ClusterManager", "name", cm.GetName())
			}
			dr := drFromUnstructured(cm, "operator.open-cluster-management.io", "v1", "ClusterManager")
			if _, err := r.resolveStuckFinalizers(ctx, log, td, []DiscoveredResource{dr}); err != nil {
				log.Error(err, "Failed to resolve ClusterManager finalizers", "name", cm.GetName())
			}
			continue
		}

		if r.isClusterManagerOperatorAlive(ctx, log) {
			log.Info("ClusterManager operator still alive, waiting for native cleanup", "name", cm.GetName())
			continue
		}

		log.Info("ClusterManager orphaned — operator dead, resolving finalizers", "name", cm.GetName())
		dr := drFromUnstructured(cm, "operator.open-cluster-management.io", "v1", "ClusterManager")
		if _, err := r.resolveStuckFinalizers(ctx, log, td, []DiscoveredResource{dr}); err != nil {
			log.Error(err, "Failed to resolve ClusterManager finalizers", "name", cm.GetName())
		}
	}

	return nil
}

// phaseCleanOrphans detects and resolves stuck-terminating resources left after MCE chain completes.
func (r *HubTeardownReconciler) phaseCleanOrphans(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) (bool, error) {
	log.Info("Phase: CleanOrphans")

	graph, err := r.buildDependencyGraph(ctx, log)
	if err != nil {
		return false, fmt.Errorf("re-scanning for orphans: %w", err)
	}

	// Update cloud warnings with current state
	td.Status.CloudResourceWarnings = r.buildCloudWarnings(ctx, log, graph)

	stuckResources := make([]DiscoveredResource, 0)
	for _, dr := range graph.DiscoveredResources {
		if dr.IsDeleting && len(dr.Finalizers) > 0 {
			stuckResources = append(stuckResources, dr)
		}
	}

	if len(stuckResources) == 0 {
		log.Info("No stuck-terminating resources found")
		return r.finishCleanOrphans(ctx, log, td)
	}

	log.Info("Found stuck-terminating resources", "count", len(stuckResources),
		"details", listFinalizerSummary(stuckResources))

	allResolved, err := r.resolveStuckFinalizers(ctx, log, td, stuckResources)
	if err != nil {
		return false, err
	}

	if !allResolved {
		needsCloudApproval := false
		needsDestructiveApproval := false
		for _, dr := range stuckResources {
			for _, f := range dr.Finalizers {
				if IsCloudProtectingFinalizer(f) && !td.Spec.AcknowledgeCloudResourceRisk {
					needsCloudApproval = true
				}
				if !IsCloudProtectingFinalizer(f) && IsAllowlistedFinalizer(f) && !td.Spec.ApprovedDestructiveActions {
					needsDestructiveApproval = true
				}
			}
		}

		msg := "Stuck resources remain. "
		if needsDestructiveApproval {
			msg += "Set spec.approvedDestructiveActions=true to patch Tier 2 finalizers. "
		}
		if needsCloudApproval {
			msg += "Set spec.acknowledgeCloudResourceRisk=true to patch Tier 1 (cloud-protecting) finalizers. "
		}
		r.setPhaseStatus(td, operatorv1.TeardownPhaseCleanOrphans, operatorv1.PhaseStateInProgress, msg)
		return false, nil
	}

	return r.finishCleanOrphans(ctx, log, td)
}

// finishCleanOrphans runs the cleanup steps common to all successful exit paths of
// phaseCleanOrphans: delete residual addon deployments scaled to 0 by earlier phases,
// sweep dangling webhooks/RBAC/namespaces left behind by phase ordering, emit the
// orphan summary event, then release the OLM gate finalizer.
func (r *HubTeardownReconciler) finishCleanOrphans(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) (bool, error) {
	r.deleteResidualAddonDeployments(ctx, log, td)

	// v11 gap fixes: earlier phases pre-clean resources that finalizeHub normally
	// deletes along with their parent namespaces. By the time Phase 6 runs
	// finalizeHub, those namespaces are already empty so finalizeHub skips their
	// deletion. These sweeps clean up what finalizeHub would have caught if the
	// ordering were the same as manual oc delete multiclusterhub.
	r.sweepDanglingWebhooks(ctx, log, td)
	r.sweepOrphanedRBAC(ctx, log, td)
	r.sweepEmptyNamespaces(ctx, log, td)

	r.emitOrphanSummaryEvent(td)
	return r.releaseOLMGate(ctx, log, td)
}

// sweepDanglingWebhooks deletes ValidatingWebhookConfigurations and
// MutatingWebhookConfigurations whose backing Service no longer exists.
// These survive teardown because they are cluster-scoped and not governed
// by any ACM finalizer. A webhook with failurePolicy=Fail pointing at a
// deleted service will reject API calls, so this is a functional fix.
func (r *HubTeardownReconciler) sweepDanglingWebhooks(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) {
	vwhList := &admissionregistrationv1.ValidatingWebhookConfigurationList{}
	if err := r.Client.List(ctx, vwhList); err != nil {
		log.Error(err, "Failed to list ValidatingWebhookConfigurations for sweep")
	} else {
		for i := range vwhList.Items {
			vwh := &vwhList.Items[i]
			if r.isWebhookBackingServiceGone(ctx, vwh.Webhooks) {
				log.Info("Deleting dangling ValidatingWebhookConfiguration", "name", vwh.Name)
				if err := r.Client.Delete(ctx, vwh); err != nil && !errors.IsNotFound(err) {
					log.Error(err, "Failed to delete dangling VWH", "name", vwh.Name)
					continue
				}
				r.Recorder.Eventf(td, corev1.EventTypeNormal, "DanglingWebhookDeleted",
					"Deleted ValidatingWebhookConfiguration %s (backing service gone)", vwh.Name)
			}
		}
	}

	mwhList := &admissionregistrationv1.MutatingWebhookConfigurationList{}
	if err := r.Client.List(ctx, mwhList); err != nil {
		log.Error(err, "Failed to list MutatingWebhookConfigurations for sweep")
	} else {
		for i := range mwhList.Items {
			mwh := &mwhList.Items[i]
			if r.isMutatingWebhookBackingServiceGone(ctx, mwh.Webhooks) {
				log.Info("Deleting dangling MutatingWebhookConfiguration", "name", mwh.Name)
				if err := r.Client.Delete(ctx, mwh); err != nil && !errors.IsNotFound(err) {
					log.Error(err, "Failed to delete dangling MWH", "name", mwh.Name)
					continue
				}
				r.Recorder.Eventf(td, corev1.EventTypeNormal, "DanglingWebhookDeleted",
					"Deleted MutatingWebhookConfiguration %s (backing service gone)", mwh.Name)
			}
		}
	}
}

// isWebhookBackingServiceGone checks if any webhook in the list references a
// Service in an ACM-related namespace that no longer exists.
func (r *HubTeardownReconciler) isWebhookBackingServiceGone(ctx context.Context, webhooks []admissionregistrationv1.ValidatingWebhook) bool {
	for _, wh := range webhooks {
		if wh.ClientConfig.Service == nil {
			continue
		}
		svcRef := wh.ClientConfig.Service
		if !r.isACMNamespace(svcRef.Namespace) {
			continue
		}
		svc := &corev1.Service{}
		key := types.NamespacedName{Name: svcRef.Name, Namespace: svcRef.Namespace}
		if err := r.Client.Get(ctx, key, svc); err != nil {
			if errors.IsNotFound(err) {
				return true
			}
		}
	}
	return false
}

// isMutatingWebhookBackingServiceGone checks if any mutating webhook references
// a Service in an ACM-related namespace that no longer exists.
func (r *HubTeardownReconciler) isMutatingWebhookBackingServiceGone(ctx context.Context, webhooks []admissionregistrationv1.MutatingWebhook) bool {
	for _, wh := range webhooks {
		if wh.ClientConfig.Service == nil {
			continue
		}
		svcRef := wh.ClientConfig.Service
		if !r.isACMNamespace(svcRef.Namespace) {
			continue
		}
		svc := &corev1.Service{}
		key := types.NamespacedName{Name: svcRef.Name, Namespace: svcRef.Namespace}
		if err := r.Client.Get(ctx, key, svc); err != nil {
			if errors.IsNotFound(err) {
				return true
			}
		}
	}
	return false
}

// acmNamespaces are namespaces created or managed by the ACM/MCE install.
// open-cluster-management is excluded here because it is cleaned in
// phaseRemoveOLMOperator after the operator self-destructs.
var acmNamespaces = map[string]bool{
	"multicluster-engine":                   true,
	"hive":                                  true,
	"open-cluster-management-hub":           true,
	"open-cluster-management-agent":         true,
	"open-cluster-management-agent-addon":   true,
	"open-cluster-management-observability": true,
	"open-cluster-management-backup":        true,
	"hypershift":                            true,
}

// isACMNamespace checks if a namespace belongs to the ACM/MCE ecosystem.
func (r *HubTeardownReconciler) isACMNamespace(ns string) bool {
	if acmNamespaces[ns] {
		return true
	}
	if ns == "open-cluster-management" {
		return true
	}
	if strings.HasPrefix(ns, "clusters-") || strings.HasPrefix(ns, "klusterlet-") {
		return true
	}
	return false
}

// sweepOrphanedRBAC deletes ACM-labeled ClusterRoles and ClusterRoleBindings.
// These are created at install time with installer.name/installer.namespace labels
// but no finalizer governs their removal during teardown.
func (r *HubTeardownReconciler) sweepOrphanedRBAC(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) {
	deleted := 0

	crList := &unstructured.UnstructuredList{}
	crList.SetGroupVersionKind(schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRoleList"})
	if err := r.Client.List(ctx, crList); err != nil {
		log.Error(err, "Failed to list ClusterRoles for RBAC sweep")
	} else {
		for i := range crList.Items {
			cr := &crList.Items[i]
			if !isACMOwnedRBAC(cr) {
				continue
			}
			if err := r.Client.Delete(ctx, cr); err != nil && !errors.IsNotFound(err) {
				log.Error(err, "Failed to delete ACM ClusterRole", "name", cr.GetName())
				continue
			}
			deleted++
		}
	}

	crbList := &unstructured.UnstructuredList{}
	crbList.SetGroupVersionKind(schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRoleBindingList"})
	if err := r.Client.List(ctx, crbList); err != nil {
		log.Error(err, "Failed to list ClusterRoleBindings for RBAC sweep")
	} else {
		for i := range crbList.Items {
			crb := &crbList.Items[i]
			if !isACMOwnedRBAC(crb) {
				continue
			}
			if err := r.Client.Delete(ctx, crb); err != nil && !errors.IsNotFound(err) {
				log.Error(err, "Failed to delete ACM ClusterRoleBinding", "name", crb.GetName())
				continue
			}
			deleted++
		}
	}

	if deleted > 0 {
		log.Info("Swept orphaned ACM RBAC resources", "deleted", deleted)
		r.Recorder.Eventf(td, corev1.EventTypeNormal, "OrphanedRBACDeleted",
			"Deleted %d orphaned ACM ClusterRoles/ClusterRoleBindings", deleted)
	}
}

// isACMOwnedRBAC checks if a cluster-scoped RBAC resource was created by the
// ACM installer using the standard installer.name label, or belongs to known
// ACM/Submariner naming patterns.
func isACMOwnedRBAC(obj *unstructured.Unstructured) bool {
	labels := obj.GetLabels()
	if labels != nil {
		if labels["installer.name"] == "multiclusterhub" {
			return true
		}
		if labels["installer.namespace"] == "open-cluster-management" {
			return true
		}
	}
	name := obj.GetName()
	return strings.HasPrefix(name, "open-cluster-management:") ||
		strings.HasPrefix(name, "multiclusterhub") ||
		strings.Contains(name, "submariner")
}

// sweepEmptyNamespaces deletes ACM-related namespaces that are empty of workloads.
// Earlier phases pre-clean the resources inside these namespaces, so finalizeHub
// skips their deletion. This sweep catches the empty shells left behind.
func (r *HubTeardownReconciler) sweepEmptyNamespaces(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) {
	// Collect candidate namespaces: known ACM namespaces plus any that
	// were used by infrastructure CRs detected during dry-run.
	candidates := make(map[string]bool)
	for ns := range acmNamespaces {
		candidates[ns] = true
	}

	// Add namespaces from cloud resource warnings discovered during dry-run.
	for _, w := range td.Status.CloudResourceWarnings {
		if w.Resource.Namespace != "" {
			candidates[w.Resource.Namespace] = true
		}
	}

	deleted := 0
	for ns := range candidates {
		namespace := &corev1.Namespace{}
		key := types.NamespacedName{Name: ns}
		if err := r.Client.Get(ctx, key, namespace); err != nil {
			continue
		}
		if namespace.GetDeletionTimestamp() != nil {
			continue
		}

		if !r.isNamespaceEmpty(ctx, ns) {
			continue
		}

		log.Info("Deleting empty ACM namespace", "namespace", ns)
		if err := r.Client.Delete(ctx, namespace); err != nil && !errors.IsNotFound(err) {
			log.Error(err, "Failed to delete empty namespace", "namespace", ns)
			continue
		}
		r.Recorder.Eventf(td, corev1.EventTypeNormal, "EmptyNamespaceDeleted",
			"Deleted empty ACM namespace %s", ns)
		deleted++
	}

	if deleted > 0 {
		log.Info("Swept empty ACM namespaces", "deleted", deleted)
	}
}

// isNamespaceEmpty returns true if a namespace has no Pods and no running workloads.
// Secrets and ConfigMaps alone do not count as "non-empty" since they are
// typically leftover TLS certs and service-ca artifacts.
func (r *HubTeardownReconciler) isNamespaceEmpty(ctx context.Context, ns string) bool {
	podList := &corev1.PodList{}
	if err := r.Client.List(ctx, podList, client.InNamespace(ns)); err != nil {
		return false
	}
	if len(podList.Items) > 0 {
		return false
	}

	deployList := &appsv1.DeploymentList{}
	if err := r.Client.List(ctx, deployList, client.InNamespace(ns)); err != nil {
		return false
	}
	if len(deployList.Items) > 0 {
		return false
	}

	return true
}

// releaseOLMGate removes the teardown gate finalizer from the ACM Subscription.
// The teardown Job is NOT cleaned up here — it must survive until
// RemoveOLMOperator completes (or self-clean via TTLSecondsAfterFinished).
func (r *HubTeardownReconciler) releaseOLMGate(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) (bool, error) {
	sub, err := r.findACMSubscription(ctx)
	if err != nil {
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("finding ACM subscription for gate release: %w", err)
	}

	finalizers := sub.GetFinalizers()
	updated := make([]string, 0, len(finalizers))
	found := false
	for _, f := range finalizers {
		if f == teardownGateFinalizer {
			found = true
			continue
		}
		updated = append(updated, f)
	}
	if !found {
		return true, nil
	}

	patch := client.MergeFrom(sub.DeepCopy())
	sub.SetFinalizers(updated)
	if err := r.Client.Patch(ctx, sub, patch); err != nil {
		return false, fmt.Errorf("removing gate finalizer from Subscription: %w", err)
	}

	r.Recorder.Event(td, corev1.EventTypeNormal, "OLMGateReleased",
		"Removed teardown gate finalizer from ACM Subscription")
	log.Info("Released OLM gate finalizer")
	return true, nil
}

// hubteardownCRDName is the fully-qualified CRD name for HubTeardown.
// Matching by CRD name (group + plural) is structurally safe — it cannot
// accidentally match a renamed Kind or a different resource in the same group.
const hubteardownCRDName = "hubteardowns.operator.open-cluster-management.io"

// phaseDeleteACMCRDs removes all RHACM/MCE CRDs from the cluster. Kubernetes
// garbage-collects all CR instances when their CRD is deleted, ensuring zero
// traces of Policies, Apps, Channels, and other feature CRs remain.
//
// The HubTeardown CRD itself is preserved by matching its fully-qualified CRD
// name, not the Kind string — this survives Kind renames and avoids the
// bootstrap paradox where the controller deletes the CRD it's reconciling.
func (r *HubTeardownReconciler) phaseDeleteACMCRDs(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) (bool, error) {
	log.Info("Phase: DeleteACMCRDs")

	crdList := &apixv1.CustomResourceDefinitionList{}
	if err := r.Client.List(ctx, crdList); err != nil {
		return false, fmt.Errorf("listing CRDs: %w", err)
	}

	// Pre-check: verify no stuck-terminating CRs remain before deleting CRDs.
	// Deleting a CRD while CRs with finalizers are still terminating can wedge
	// the CRD in a Terminating state because Kubernetes cannot garbage-collect
	// CRs whose finalizers are still being processed.
	for i := range crdList.Items {
		crd := &crdList.Items[i]
		if !isACMAPIGroup(crd.Spec.Group) || crd.Name == hubteardownCRDName {
			continue
		}
		version := preferredVersion(crd)
		if version == "" {
			continue
		}
		listGVK := schema.GroupVersionKind{
			Group:   crd.Spec.Group,
			Version: version,
			Kind:    crd.Spec.Names.Kind + "List",
		}
		list := &unstructured.UnstructuredList{}
		list.SetGroupVersionKind(listGVK)
		if err := r.Client.List(ctx, list); err != nil {
			continue
		}
		for j := range list.Items {
			item := &list.Items[j]
			if item.GetDeletionTimestamp() != nil && len(item.GetFinalizers()) > 0 {
				log.Info("Stuck-terminating CR blocks CRD deletion — waiting for CleanOrphans to resolve",
					"crd", crd.Name, "cr", item.GetName(), "namespace", item.GetNamespace(),
					"finalizers", item.GetFinalizers())
				return false, nil
			}
		}
	}

	remaining := 0
	for i := range crdList.Items {
		crd := &crdList.Items[i]

		if !isACMAPIGroup(crd.Spec.Group) {
			continue
		}

		// Preserve the HubTeardown CRD by fully-qualified name (group + plural).
		if crd.Name == hubteardownCRDName {
			continue
		}

		if crd.GetDeletionTimestamp() != nil {
			remaining++
			continue
		}

		log.Info("Deleting CRD", "name", crd.Name, "group", crd.Spec.Group, "kind", crd.Spec.Names.Kind)
		if err := r.Client.Delete(ctx, crd); err != nil {
			if errors.IsNotFound(err) {
				continue
			}
			log.Error(err, "Failed to delete CRD", "name", crd.Name)
			remaining++
			continue
		}
		remaining++
	}

	if remaining > 0 {
		log.Info("Waiting for ACM/MCE CRDs to be removed", "remaining", remaining)
		return false, nil
	}

	log.Info("All ACM/MCE CRDs removed")
	r.Recorder.Event(td, corev1.EventTypeNormal, "CRDsDeleted",
		"All RHACM/MCE CRDs and their instances have been garbage-collected")
	return true, nil
}

// phaseRemoveOLMOperator deletes the ACM Subscription and CSV so the operator
// is fully removed from "Installed Operators". This is the final phase — by this
// point the HubTeardown CR is in its terminal state.
//
// Ordering is critical because deleting the CSV terminates the operator pod
// (self-destruct). Every step that must survive must complete before the CSV
// delete — the point of no return.
//
// Order:
//  1. Persist Complete status (status subresource update)
//  2. Read-back via UncachedClient to verify persistence
//  3. Re-fetch CR and remove finalizer (avoids resourceVersion conflict)
//  4. Delete Subscription (stops OLM from recreating CSV)
//  5. Delete CSV (kills operator pod — point of no return)
//  6. Teardown Job self-cleans via TTLSecondsAfterFinished
func (r *HubTeardownReconciler) phaseRemoveOLMOperator(ctx context.Context, log logr.Logger, td *operatorv1.HubTeardown) (bool, error) {
	log.Info("Phase: RemoveOLMOperator")

	sub, err := r.findACMSubscription(ctx)
	if err != nil {
		if errors.IsNotFound(err) {
			log.Info("ACM Subscription already gone, checking for lingering CSV")
			if deleted, delErr := r.deleteLingeringACMCSV(ctx, log, td); delErr != nil {
				return false, fmt.Errorf("deleting lingering ACM CSV: %w", delErr)
			} else if deleted {
				log.Info("Deleted lingering ACM CSV after Subscription was already gone")
			}
			return true, nil
		}
		return false, fmt.Errorf("finding ACM subscription for removal: %w", err)
	}

	// Step 1: Persist "Complete" status BEFORE any destructive action.
	now := metav1.Now()
	r.setPhaseStatusWithTime(td, operatorv1.TeardownPhaseRemoveOLMOperator, operatorv1.PhaseStateComplete, "Phase completed", &now)
	td.Status.Phase = operatorv1.TeardownPhaseComplete
	r.setCondition(td, conditionTypeTeardownProgressing, metav1.ConditionFalse, "TeardownComplete",
		"All teardown phases completed successfully.")
	r.setCondition(td, conditionTypeTeardownComplete, metav1.ConditionTrue, "Complete",
		"RHACM teardown finished. Review events for cloud resource orphan details.")
	// Gap 7: Explicitly clear any stale Stalled=True condition when setting Complete.
	r.setCondition(td, conditionTypeTeardownStalled, metav1.ConditionFalse, "TeardownComplete",
		"Teardown completed successfully.")
	// Gap 8: Clear stale blockingResources now that teardown is complete.
	td.Status.BlockingResources = nil
	// v11-A: Clear stale dry-run fields so printer columns and conditions
	// don't mislead support engineers into thinking teardown is incomplete.
	td.Status.CloudResourceWarnings = nil
	if td.Status.DryRunReport != nil {
		td.Status.DryRunReport.TotalBlockingResources = 0
		td.Status.DryRunReport.TotalCloudRiskResources = 0
	}
	r.setCondition(td, conditionTypeCloudResourcesRisk, metav1.ConditionFalse, "TeardownComplete",
		"Teardown completed — cloud resource warnings cleared.")

	if err := r.Client.Status().Update(ctx, td); err != nil {
		return false, fmt.Errorf("persisting Complete status before OLM removal: %w", err)
	}
	log.Info("Persisted Complete status on HubTeardown CR")

	// Step 2: Verify status was actually persisted via uncached read.
	// In degraded API server scenarios the cached client may report success
	// while the write was lost. Do not proceed to self-destruct without
	// confirming the terminal status is durable.
	verified := &operatorv1.HubTeardown{}
	verifyKey := types.NamespacedName{Name: td.Name, Namespace: td.Namespace}
	if err := r.UncachedClient.Get(ctx, verifyKey, verified); err != nil {
		return false, fmt.Errorf("read-back verification failed: %w", err)
	}
	if verified.Status.Phase != operatorv1.TeardownPhaseComplete {
		log.Info("Read-back shows status not yet Complete, retrying", "readBackPhase", verified.Status.Phase)
		return false, nil
	}
	log.Info("Read-back confirmed Complete status is persisted")

	// Step 3: Re-fetch the CR to get a fresh resourceVersion, then remove
	// the finalizer. This avoids the conflict error that occurs when the
	// status update in Step 1 changes the resourceVersion under us.
	fresh := &operatorv1.HubTeardown{}
	if err := r.Client.Get(ctx, verifyKey, fresh); err != nil {
		return false, fmt.Errorf("re-fetching HubTeardown for finalizer removal: %w", err)
	}
	if controllerutil.ContainsFinalizer(fresh, teardownJobFinalizer) {
		controllerutil.RemoveFinalizer(fresh, teardownJobFinalizer)
		if err := r.Client.Update(ctx, fresh); err != nil {
			return false, fmt.Errorf("removing teardown finalizer: %w", err)
		}
		log.Info("Removed teardown finalizer from HubTeardown CR")
	}

	r.Recorder.Event(td, corev1.EventTypeNormal, "TeardownComplete",
		"RHACM teardown completed. Removing operator via OLM.")

	// Step 4: Delete the Subscription FIRST. Once gone, OLM will not
	// reconcile or recreate the CSV. This prevents a zombie operator
	// deployment from appearing between Subscription and CSV deletion.
	// Flaw E fix: a transient delete failure must block progression to CSV
	// self-destruct — falling through would leave OLM able to recreate the
	// operator from the surviving Subscription (zombie operator risk).
	if err := r.Client.Delete(ctx, sub); err != nil && !errors.IsNotFound(err) {
		return false, fmt.Errorf("deleting ACM Subscription: %w", err)
	}
	log.Info("Deleted ACM Subscription")

	// Gap 9: Clean up the teardown Job now that teardown is Complete.
	// The Job's TTLSecondsAfterFinished only fires after the Job finishes,
	// but by this point the Job's RBAC (ACM ClusterRoles) has been deleted,
	// leaving it running with continuous Unauthorized errors.
	if err := r.cleanupTeardownJob(ctx, log, td); err != nil {
		log.Error(err, "Failed to cleanup teardown Job (non-fatal)")
	}

	// Gap 6: Delete the MCE Subscription and its installed CSV.
	// The MCE operator's own uninstall flow does not clean up its own
	// Subscription/CSV, leaving them orphaned in multicluster-engine namespace.
	mceSub, mceErr := r.findMCESubscription(ctx)
	if mceErr != nil {
		if !errors.IsNotFound(mceErr) {
			log.Error(mceErr, "Failed to find MCE Subscription")
		}
	} else {
		// Delete MCE Subscription FIRST to stop OLM from recreating the CSV.
		// This matches the ACM pattern above (Step 4).
		mceCsvName, _, _ := unstructured.NestedString(mceSub.Object, "status", "installedCSV")
		if delErr := r.Client.Delete(ctx, mceSub); delErr != nil && !errors.IsNotFound(delErr) {
			log.Error(delErr, "Failed to delete MCE Subscription")
		} else {
			log.Info("Deleted MCE Subscription", "name", mceSub.GetName())
			r.Recorder.Eventf(td, corev1.EventTypeNormal, "MCESubscriptionDeleted",
				"Deleted MCE Subscription %s", mceSub.GetName())
		}
		// Now delete the MCE CSV — OLM will not recreate it since the Subscription is gone.
		if mceCsvName != "" {
			mceCsv := &unstructured.Unstructured{}
			mceCsv.SetGroupVersionKind(schema.GroupVersionKind{
				Group:   "operators.coreos.com",
				Version: "v1alpha1",
				Kind:    "ClusterServiceVersion",
			})
			mceCsvKey := types.NamespacedName{Name: mceCsvName, Namespace: mceSub.GetNamespace()}
			if getErr := r.Client.Get(ctx, mceCsvKey, mceCsv); getErr == nil {
				if delErr := r.Client.Delete(ctx, mceCsv); delErr != nil && !errors.IsNotFound(delErr) {
					log.Error(delErr, "Failed to delete MCE CSV", "csv", mceCsvName)
				} else {
					log.Info("Deleted MCE ClusterServiceVersion", "csv", mceCsvName)
					r.Recorder.Eventf(td, corev1.EventTypeNormal, "MCECSVDeleted",
						"Deleted MCE ClusterServiceVersion %s", mceCsvName)
				}
			}
		}
	}

	// Step 5: Delete the CSV — point of no return. This terminates the
	// operator pod. Everything above this line must have completed
	// successfully for teardown to be considered durable.
	csvName, _, _ := unstructured.NestedString(sub.Object, "status", "installedCSV")
	if csvName != "" {
		csv := &unstructured.Unstructured{}
		csv.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   "operators.coreos.com",
			Version: "v1alpha1",
			Kind:    "ClusterServiceVersion",
		})
		csvKey := types.NamespacedName{Name: csvName, Namespace: sub.GetNamespace()}
		if getErr := r.Client.Get(ctx, csvKey, csv); getErr == nil {
			if delErr := r.Client.Delete(ctx, csv); delErr != nil && !errors.IsNotFound(delErr) {
				log.Error(delErr, "Failed to delete CSV", "csv", csvName)
			} else {
				log.Info("Deleted ACM ClusterServiceVersion — operator pod will terminate", "csv", csvName)
			}
		}
	}

	// Step 6: Teardown Job is NOT explicitly deleted here. Its
	// TTLSecondsAfterFinished (1 hour) handles cleanup automatically.
	// Deleting it before the CSV would remove the resilience backstop
	// during the critical self-destruct window.

	return true, nil
}

// findSubscriptionByPackage locates the OLM Subscription whose spec.package matches pkg.
func (r *HubTeardownReconciler) findSubscriptionByPackage(ctx context.Context, pkg string) (*unstructured.Unstructured, error) {
	subList := &unstructured.UnstructuredList{}
	subList.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "operators.coreos.com",
		Version: "v1alpha1",
		Kind:    "SubscriptionList",
	})
	if err := r.Client.List(ctx, subList); err != nil {
		return nil, err
	}

	for i := range subList.Items {
		sub := &subList.Items[i]
		specPkg, _, _ := unstructured.NestedString(sub.Object, "spec", "name")
		if specPkg == pkg {
			return sub, nil
		}
	}

	return nil, errors.NewNotFound(schema.GroupResource{
		Group:    "operators.coreos.com",
		Resource: "subscriptions",
	}, pkg)
}

// findACMSubscription locates the ACM OLM Subscription.
func (r *HubTeardownReconciler) findACMSubscription(ctx context.Context) (*unstructured.Unstructured, error) {
	return r.findSubscriptionByPackage(ctx, "advanced-cluster-management")
}

// findMCESubscription locates the MCE OLM Subscription.
func (r *HubTeardownReconciler) findMCESubscription(ctx context.Context) (*unstructured.Unstructured, error) {
	return r.findSubscriptionByPackage(ctx, "multicluster-engine")
}

// deleteLingeringACMCSV finds and deletes any ACM ClusterServiceVersion that
// still exists after the Subscription has been removed.
func (r *HubTeardownReconciler) deleteLingeringACMCSV(
	ctx context.Context,
	log logr.Logger,
	td *operatorv1.HubTeardown,
) (bool, error) {
	csvList := &subv1alpha1.ClusterServiceVersionList{}
	if err := r.Client.List(ctx, csvList, client.HasLabels{acmSubscriptionLabel}); err != nil {
		if isNoMatchError(err) {
			return false, nil
		}
		return false, fmt.Errorf("listing CSVs with ACM label: %w", err)
	}

	deleted := false
	for i := range csvList.Items {
		csv := &csvList.Items[i]
		log.Info("Deleting lingering ACM CSV", "name", csv.Name, "namespace", csv.Namespace)
		if err := r.Client.Delete(ctx, csv); err != nil && !errors.IsNotFound(err) {
			return false, fmt.Errorf("deleting lingering CSV %s/%s: %w", csv.Namespace, csv.Name, err)
		}
		r.Recorder.Eventf(td, corev1.EventTypeNormal, "LingeringCSVDeleted",
			"Deleted lingering ACM CSV %s/%s (Subscription already gone)", csv.Namespace, csv.Name)
		deleted = true
	}
	return deleted, nil
}

// isResourceGone checks if all instances of a resource type have been deleted.
func (r *HubTeardownReconciler) isResourceGone(ctx context.Context, group, version, kind string) (bool, error) {
	listGVK := schema.GroupVersionKind{
		Group:   group,
		Version: version,
		Kind:    kind + "List",
	}
	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(listGVK)

	if err := r.Client.List(ctx, list); err != nil {
		if isNoMatchError(err) || errors.IsNotFound(err) {
			return true, nil
		}
		return false, fmt.Errorf("listing %s: %w", kind, err)
	}
	return len(list.Items) == 0, nil
}

// shouldResolveStuckFinalizers determines if enough time has passed for a stuck resource.
// Uses spec.forceFinalizerTimeout if set, otherwise falls back to defaultFinalizerTimeout.
func (r *HubTeardownReconciler) shouldResolveStuckFinalizers(td *operatorv1.HubTeardown, obj *unstructured.Unstructured) bool {
	ts := obj.GetDeletionTimestamp()
	if ts == nil {
		return false
	}
	timeout := defaultFinalizerTimeout
	if td.Spec.ForceFinalizerTimeout != nil {
		timeout = td.Spec.ForceFinalizerTimeout.Duration
	}
	return time.Since(ts.Time) > timeout
}

// drFromUnstructured builds a DiscoveredResource from an unstructured item.
func drFromUnstructured(item *unstructured.Unstructured, group, version, kind string) DiscoveredResource {
	return DiscoveredResource{
		Ref: operatorv1.ResourceRef{
			Group:     group,
			Kind:      kind,
			Namespace: item.GetNamespace(),
			Name:      item.GetName(),
		},
		Version:    version,
		Finalizers: item.GetFinalizers(),
	}
}

// isAddonControllerDeployment returns true if the deployment matches addon
// controller labels or known addon controller name patterns.
func isAddonControllerDeployment(deploy *appsv1.Deployment) bool {
	labels := deploy.GetLabels()
	if labels == nil {
		return false
	}
	component := labels["app.kubernetes.io/component"]
	partOf := labels["app.kubernetes.io/part-of"]
	appName := labels["app"]
	return component == "addon-controller" ||
		component == "addon-manager" ||
		strings.Contains(appName, "addon") ||
		strings.Contains(partOf, "addon")
}

// isMCEOperatorAlive checks if the MCE operator deployment has at least one
// ready replica. The MCE operator is deployed by OLM in the MCE operand
// namespace with a name containing the MCE package name.
func (r *HubTeardownReconciler) isMCEOperatorAlive(ctx context.Context, log logr.Logger) bool {
	ns := mceutils.OperandNamespace()
	deployList := &appsv1.DeploymentList{}
	if err := r.Client.List(ctx, deployList, client.InNamespace(ns)); err != nil {
		log.Info("Failed to list deployments in MCE namespace, treating operator as dead", "namespace", ns, "error", err)
		return false
	}

	packageName := mceutils.DesiredPackage()
	for i := range deployList.Items {
		deploy := &deployList.Items[i]
		if strings.Contains(deploy.Name, packageName) || strings.Contains(deploy.Name, "backplane-operator") {
			if deploy.Status.ReadyReplicas > 0 {
				return true
			}
			return false
		}
	}
	return false
}

// isClusterManagerOperatorAlive checks if the cluster-manager operator
// deployment has at least one ready replica. Checks both the MCE operand
// namespace and open-cluster-management (upgrade path).
func (r *HubTeardownReconciler) isClusterManagerOperatorAlive(ctx context.Context, log logr.Logger) bool {
	for _, ns := range []string{mceutils.OperandNamespace(), "open-cluster-management"} {
		deploy := &appsv1.Deployment{}
		key := types.NamespacedName{Name: "cluster-manager", Namespace: ns}
		if err := r.Client.Get(ctx, key, deploy); err != nil {
			continue
		}
		if deploy.Status.ReadyReplicas > 0 {
			return true
		}
		return false
	}
	return false
}

// isNoMatchError checks if an error is a "no match" error from the API discovery.
func isNoMatchError(err error) bool {
	if err == nil {
		return false
	}
	return errors.IsNotFound(err) || apimeta.IsNoMatchError(err)
}
