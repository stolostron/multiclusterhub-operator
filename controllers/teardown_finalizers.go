// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// FinalizerTier classifies the risk level of a finalizer.
type FinalizerTier int

const (
	FinalizerTierUnknown   FinalizerTier = iota
	FinalizerTier1Cloud                  // Protects cloud/infrastructure resources
	FinalizerTier2NonCloud               // Protects Kubernetes-level state only
)

// finalizerEntry describes a known finalizer and its risk classification.
type finalizerEntry struct {
	Finalizer   string
	Tier        FinalizerTier
	Description string
}

// cloudProtectingFinalizers is the Tier 1 allowlist: finalizers that protect cloud infrastructure.
var cloudProtectingFinalizers = []finalizerEntry{
	{
		Finalizer:   "hypershift.openshift.io/finalizer",
		Tier:        FinalizerTier1Cloud,
		Description: "Hypershift cluster/nodepool/HCP lifecycle: VMs, LBs, security groups, DNS, control-plane infra",
	},
	{
		Finalizer:   "hypershift.io/aws-oidc-discovery",
		Tier:        FinalizerTier1Cloud,
		Description: "AWS S3 OIDC discovery documents",
	},
	{
		Finalizer:   "hypershift.openshift.io/control-plane-operator-finalizer",
		Tier:        FinalizerTier1Cloud,
		Description: "AWS PrivateLink endpoint services / GCP Private Service Connect",
	},
	{
		Finalizer:   "hive.openshift.io/deprovision",
		Tier:        FinalizerTier1Cloud,
		Description: "Full cluster infrastructure deprovisioning: VMs, networking, storage, DNS",
	},
	{
		Finalizer:   "agentserviceconfig.agent-install.openshift.io/ai-deprovision",
		Tier:        FinalizerTier1Cloud,
		Description: "Assisted-installer provisioned infrastructure resources",
	},
	{
		Finalizer:   "clusterdeployments.agent-install.openshift.io/ai-deprovision",
		Tier:        FinalizerTier1Cloud,
		Description: "Assisted-installer ClusterDeployment infrastructure deprovisioning",
	},
	{
		Finalizer:   "infraenv.agent-install.openshift.io/ai-deprovision",
		Tier:        FinalizerTier1Cloud,
		Description: "Assisted-installer InfraEnv infrastructure cleanup",
	},
	{
		Finalizer:   "agentclusterinstall.agent-install.openshift.io/ai-deprovision",
		Tier:        FinalizerTier1Cloud,
		Description: "Assisted-installer AgentClusterInstall infrastructure cleanup",
	},
	{
		Finalizer:   "agent.agent-install.openshift.io/ai-deprovision",
		Tier:        FinalizerTier1Cloud,
		Description: "Assisted-installer Agent (per-host BMC) infrastructure cleanup",
	},
}

// nonCloudFinalizers is the Tier 2 allowlist: finalizers that protect Kubernetes-level state only.
var nonCloudFinalizers = []finalizerEntry{
	{
		Finalizer:   "managedcluster-import-controller.open-cluster-management.io/cleanup",
		Tier:        FinalizerTier2NonCloud,
		Description: "Import controller state cleanup",
	},
	{
		Finalizer:   "open-cluster-management/managedclusterrole",
		Tier:        FinalizerTier2NonCloud,
		Description: "ManagedCluster role bindings",
	},
	{
		Finalizer:   "cluster.open-cluster-management.io/api-resource-cleanup",
		Tier:        FinalizerTier2NonCloud,
		Description: "ManagedCluster API resource cleanup",
	},
	{
		Finalizer:   "addon.open-cluster-management.io/addon-pre-delete",
		Tier:        FinalizerTier2NonCloud,
		Description: "Addon pre-delete hook execution",
	},
	{
		Finalizer:   "work.open-cluster-management.io/manifest-work-cleanup",
		Tier:        FinalizerTier2NonCloud,
		Description: "ManifestWork cleanup",
	},
	{
		Finalizer:   "cluster.open-cluster-management.io/manifest-work-cleanup",
		Tier:        FinalizerTier2NonCloud,
		Description: "ManagedCluster manifest-work cleanup (agent-side)",
	},
	{
		Finalizer:   "agent.open-cluster-management.io/klusterletadmissioncontroller",
		Tier:        FinalizerTier2NonCloud,
		Description: "Klusterlet admission controller cleanup (agent-side, requires running agent)",
	},
	{
		Finalizer:   "cluster.open-cluster-management.io/managedclusterrole-cleanup",
		Tier:        FinalizerTier2NonCloud,
		Description: "ManagedCluster role binding cleanup (registration controller)",
	},
	{
		Finalizer:   "registration.open-cluster-management.io/managedcluster-cleanup",
		Tier:        FinalizerTier2NonCloud,
		Description: "Registration hub controller ManagedCluster cleanup",
	},
	{
		Finalizer:   "operator.open-cluster-management.io/klusterlet-hosted-cleanup",
		Tier:        FinalizerTier2NonCloud,
		Description: "Hosted klusterlet cleanup on hosting cluster",
	},
	{
		Finalizer:   "observability.open-cluster-management.io/res-cleanup",
		Tier:        FinalizerTier2NonCloud,
		Description: "Observability custom resource cleanup",
	},
	{
		Finalizer:   "observability.open-cluster-management.io/addon-cleanup",
		Tier:        FinalizerTier2NonCloud,
		Description: "Observability addon cleanup",
	},
	{
		Finalizer:   "search.open-cluster-management.io/finalizer",
		Tier:        FinalizerTier2NonCloud,
		Description: "Search index and operator cleanup",
	},
	{
		Finalizer:   "submarineraddon.open-cluster-management.io/submariner-addon-cleanup",
		Tier:        FinalizerTier2NonCloud,
		Description: "Submariner addon agent cleanup",
	},
	{
		Finalizer:   "cluster.open-cluster-management.io/submariner-cleanup",
		Tier:        FinalizerTier2NonCloud,
		Description: "Submariner cluster-level cleanup",
	},
	{
		Finalizer:   "uninstall-helm-release",
		Tier:        FinalizerTier2NonCloud,
		Description: "HelmRelease state cleanup",
	},
	{
		Finalizer:   "clusterinstance.siteconfig.open-cluster-management.io/finalizer",
		Tier:        FinalizerTier2NonCloud,
		Description: "SiteConfig ClusterInstance cleanup",
	},
	{
		Finalizer:   "restores.cluster.open-cluster-management.io/finalizer",
		Tier:        FinalizerTier2NonCloud,
		Description: "Backup restore state cleanup",
	},
	{
		Finalizer:   "operator.open-cluster-management.io/cluster-manager-cleanup",
		Tier:        FinalizerTier2NonCloud,
		Description: "ClusterManager CRD and resource cleanup",
	},
	{
		Finalizer:   "hypershift.openshift.io/component-finalizer",
		Tier:        FinalizerTier2NonCloud,
		Description: "CAPI provider deployments (no cloud infrastructure)",
	},
	{
		Finalizer:   "finalizer.operator.open-cluster-management.io",
		Tier:        FinalizerTier2NonCloud,
		Description: "MCH operator hub finalizer",
	},
	{
		Finalizer:   "finalizer.multicluster.openshift.io",
		Tier:        FinalizerTier2NonCloud,
		Description: "MCE operator backplane finalizer",
	},
}

// IsCloudProtectingFinalizer returns true if the finalizer is Tier 1 (cloud-protecting).
func IsCloudProtectingFinalizer(finalizer string) bool {
	for _, entry := range cloudProtectingFinalizers {
		if entry.Finalizer == finalizer {
			return true
		}
	}
	return false
}

// IsAllowlistedFinalizer returns true if the finalizer is in either Tier 1 or Tier 2.
func IsAllowlistedFinalizer(finalizer string) bool {
	if IsCloudProtectingFinalizer(finalizer) {
		return true
	}
	for _, entry := range nonCloudFinalizers {
		if entry.Finalizer == finalizer {
			return true
		}
	}
	return false
}

// ClassifyFinalizer returns the tier and description for a known finalizer.
func ClassifyFinalizer(finalizer string) (FinalizerTier, string) {
	for _, entry := range cloudProtectingFinalizers {
		if entry.Finalizer == finalizer {
			return entry.Tier, entry.Description
		}
	}
	for _, entry := range nonCloudFinalizers {
		if entry.Finalizer == finalizer {
			return entry.Tier, entry.Description
		}
	}
	return FinalizerTierUnknown, "Unknown finalizer (not in allowlist)"
}

// patchFinalizerOffResource removes a specific finalizer from a resource, respecting tier gates.
// Returns true if the finalizer was removed, false if it was skipped due to missing approval.
func (r *HubTeardownReconciler) patchFinalizerOffResource(
	ctx context.Context,
	log logr.Logger,
	td *operatorv1.HubTeardown,
	ref operatorv1.ResourceRef,
	finalizer string,
	gvk schema.GroupVersionKind,
) (bool, error) {
	tier, description := ClassifyFinalizer(finalizer)

	switch tier {
	case FinalizerTier1Cloud:
		if !td.Spec.AcknowledgeCloudResourceRisk {
			log.Info("Skipping Tier 1 finalizer (cloud-protecting): acknowledgeCloudResourceRisk not set",
				"resource", fmt.Sprintf("%s/%s/%s", ref.Kind, ref.Namespace, ref.Name),
				"finalizer", finalizer)
			r.setWaitingForApproval(td, "Tier1CloudFinalizersBlocked",
				fmt.Sprintf("Cloud-protecting finalizer %q on %s/%s blocked. Set spec.acknowledgeCloudResourceRisk=true after verifying cloud state.",
					finalizer, ref.Kind, ref.Name))
			return false, nil
		}
		r.Recorder.Eventf(td, corev1.EventTypeWarning, "FinalizerRemovalWarning",
			"About to remove cloud-protecting finalizer %q from %s/%s/%s. Cloud resources (%s) will be ORPHANED and require manual cleanup.",
			finalizer, ref.Kind, ref.Namespace, ref.Name, description)

	case FinalizerTier2NonCloud:
		if !td.Spec.ApprovedDestructiveActions {
			log.Info("Skipping Tier 2 finalizer: approvedDestructiveActions not set",
				"resource", fmt.Sprintf("%s/%s/%s", ref.Kind, ref.Namespace, ref.Name),
				"finalizer", finalizer)
			r.setWaitingForApproval(td, "Tier2FinalizersBlocked",
				fmt.Sprintf("Tier 2 finalizer %q on %s/%s blocked. Set spec.approvedDestructiveActions=true to proceed.",
					finalizer, ref.Kind, ref.Name))
			return false, nil
		}

	case FinalizerTierUnknown:
		log.Info("Skipping unknown finalizer (not in allowlist)",
			"resource", fmt.Sprintf("%s/%s/%s", ref.Kind, ref.Namespace, ref.Name),
			"finalizer", finalizer)
		return false, nil
	}

	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	key := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
	if err := r.Client.Get(ctx, key, obj); err != nil {
		return false, fmt.Errorf("getting resource %s/%s: %w", ref.Kind, ref.Name, err)
	}

	existing := obj.GetFinalizers()
	updated := make([]string, 0, len(existing))
	found := false
	for _, f := range existing {
		if f == finalizer {
			found = true
			continue
		}
		updated = append(updated, f)
	}
	if !found {
		return false, nil
	}

	patch := client.MergeFrom(obj.DeepCopy())
	obj.SetFinalizers(updated)
	if err := r.Client.Patch(ctx, obj, patch); err != nil {
		return false, fmt.Errorf("patching finalizer off %s/%s: %w", ref.Kind, ref.Name, err)
	}

	eventType := corev1.EventTypeNormal
	eventReason := "FinalizerRemoved"
	if tier == FinalizerTier1Cloud {
		eventType = corev1.EventTypeWarning
		eventReason = "CloudResourceOrphaned"
	}
	r.Recorder.Eventf(td, eventType, eventReason,
		"Removed finalizer %q from %s/%s/%s (%s)",
		finalizer, ref.Kind, ref.Namespace, ref.Name, description)

	log.Info("Removed finalizer",
		"resource", fmt.Sprintf("%s/%s/%s", ref.Kind, ref.Namespace, ref.Name),
		"finalizer", finalizer, "tier", tier)

	return true, nil
}

// resolveStuckFinalizers attempts to patch off stuck finalizers on a list of resources.
// Returns (allResolved, error).
func (r *HubTeardownReconciler) resolveStuckFinalizers(
	ctx context.Context,
	log logr.Logger,
	td *operatorv1.HubTeardown,
	resources []DiscoveredResource,
) (bool, error) {
	allResolved := true
	for _, dr := range resources {
		if len(dr.Finalizers) == 0 {
			continue
		}

		for _, finalizer := range dr.Finalizers {
			if !IsAllowlistedFinalizer(finalizer) {
				log.Info("Finalizer not in allowlist, skipping",
					"resource", fmt.Sprintf("%s/%s/%s", dr.Ref.Kind, dr.Ref.Namespace, dr.Ref.Name),
					"finalizer", finalizer)
				allResolved = false
				continue
			}

			version := dr.Version
			if version == "" {
				version = "v1"
			}
			gvk := schema.GroupVersionKind{
				Group:   dr.Ref.Group,
				Version: version,
				Kind:    dr.Ref.Kind,
			}

			removed, err := r.patchFinalizerOffResource(ctx, log, td, dr.Ref, finalizer, gvk)
			if err != nil {
				return false, err
			}
			if !removed {
				allResolved = false
			}
		}
	}
	return allResolved, nil
}

// listFinalizerSummary returns a human-readable summary of finalizers across resources.
func listFinalizerSummary(resources []DiscoveredResource) string {
	var parts []string
	for _, dr := range resources {
		if len(dr.Finalizers) > 0 {
			parts = append(parts, fmt.Sprintf("%s/%s/%s: [%s]",
				dr.Ref.Kind, dr.Ref.Namespace, dr.Ref.Name,
				strings.Join(dr.Finalizers, ", ")))
		}
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, "; ")
}

// setWaitingForApproval sets or updates the WaitingForApproval condition on the HubTeardown.
// Called immediately when a finalizer is skipped due to missing approval, so the user
// sees the requirement without waiting for the overall stall timeout.
func (r *HubTeardownReconciler) setWaitingForApproval(td *operatorv1.HubTeardown, reason, message string) {
	if r.isConditionTrue(td, conditionTypeWaitingForApproval) {
		return
	}
	r.setCondition(td, conditionTypeWaitingForApproval, metav1.ConditionTrue, reason, message)
	r.Recorder.Event(td, corev1.EventTypeWarning, "WaitingForApproval", message)
}

// clearWaitingForApproval clears the WaitingForApproval condition once approvals are granted.
func (r *HubTeardownReconciler) clearWaitingForApproval(td *operatorv1.HubTeardown) {
	if !r.isConditionTrue(td, conditionTypeWaitingForApproval) {
		return
	}
	r.setCondition(td, conditionTypeWaitingForApproval, metav1.ConditionFalse, "ApprovalsGranted",
		"All required approvals have been set.")
}

// forceStripAllFinalizers force-removes ALL finalizers from a resource regardless of tier/allowlist.
// Used as a safety net when the phase timeout expires and the cluster agent is unreachable.
// Emits a Warning event for each removed finalizer to maintain audit trail.
func (r *HubTeardownReconciler) forceStripAllFinalizers(
	ctx context.Context,
	log logr.Logger,
	td *operatorv1.HubTeardown,
	ref operatorv1.ResourceRef,
	gvk schema.GroupVersionKind,
) error {
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(gvk)
	key := types.NamespacedName{Namespace: ref.Namespace, Name: ref.Name}
	if err := r.Client.Get(ctx, key, obj); err != nil {
		return fmt.Errorf("getting resource %s/%s for force-strip: %w", ref.Kind, ref.Name, err)
	}

	finalizers := obj.GetFinalizers()
	if len(finalizers) == 0 {
		return nil
	}

	for _, f := range finalizers {
		r.Recorder.Eventf(td, corev1.EventTypeWarning, "FinalizerForceStripped",
			"Force-stripped finalizer %q from %s/%s/%s (phase timeout expired, agent unreachable)",
			f, ref.Kind, ref.Namespace, ref.Name)
		log.Info("Force-stripping finalizer (phase timeout)",
			"resource", fmt.Sprintf("%s/%s/%s", ref.Kind, ref.Namespace, ref.Name),
			"finalizer", f)
	}

	patch := client.MergeFrom(obj.DeepCopy())
	obj.SetFinalizers(nil)
	if err := r.Client.Patch(ctx, obj, patch); err != nil {
		return fmt.Errorf("force-stripping finalizers from %s/%s: %w", ref.Kind, ref.Name, err)
	}
	return nil
}
