// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-logr/logr"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	apixv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// acmAPIGroupSuffixes are the API group suffixes that belong to the ACM/MCE ecosystem.
var acmAPIGroupSuffixes = []string{
	"open-cluster-management.io",
	"hive.openshift.io",
	"agent-install.openshift.io",
	"hypershift.openshift.io",
	"multicluster.openshift.io",
	"metal3.io",
}

// DependencyGraph holds the scanned cluster state for teardown planning.
type DependencyGraph struct {
	// All discovered CRD instances that have finalizers or are blocking teardown.
	DiscoveredResources []DiscoveredResource

	// Resources that block MCH webhook validation.
	BlockingResources []operatorv1.BlockingResource

	// Resources with Tier 1 (cloud-protecting) finalizers.
	CloudRiskResources []DiscoveredResource

	// Resources with Tier 2 (non-cloud) finalizers.
	NonCloudFinalizerResources []DiscoveredResource

	// ManagedClusters that are not the local-cluster.
	NonLocalManagedClusters []DiscoveredResource

	// ManagedClusterAddOns across all non-local clusters.
	Addons []DiscoveredResource

	// MCH instance (if found).
	MCH *DiscoveredResource

	// MCE instance (if found).
	MCE *DiscoveredResource
}

// DiscoveredResource represents a single resource instance found during scanning.
type DiscoveredResource struct {
	Ref        operatorv1.ResourceRef
	Version    string
	Finalizers []string
	IsDeleting bool
	Phase      string
	Platform   string
}

// buildDependencyGraph scans the cluster for all ACM/MCE resources and classifies them.
func (r *HubTeardownReconciler) buildDependencyGraph(ctx context.Context, log logr.Logger) (*DependencyGraph, error) {
	graph := &DependencyGraph{}

	crdList := &apixv1.CustomResourceDefinitionList{}
	if err := r.Client.List(ctx, crdList); err != nil {
		return nil, fmt.Errorf("listing CRDs: %w", err)
	}

	for i := range crdList.Items {
		crd := &crdList.Items[i]
		if !isACMAPIGroup(crd.Spec.Group) {
			continue
		}
		if err := r.scanCRDInstances(ctx, log, crd, graph); err != nil {
			log.Error(err, "Failed to scan CRD instances", "crd", crd.Name)
		}
	}

	r.classifyBlockingResources(graph)
	r.classifyManagedClusters(graph)
	r.classifyAddons(graph)
	r.findMCHAndMCE(graph)

	return graph, nil
}

// isACMAPIGroup checks if an API group belongs to the ACM/MCE ecosystem.
func isACMAPIGroup(group string) bool {
	for _, suffix := range acmAPIGroupSuffixes {
		if strings.HasSuffix(group, suffix) || group == suffix {
			return true
		}
	}
	return false
}

// scanCRDInstances lists all instances of a CRD and adds them to the graph.
func (r *HubTeardownReconciler) scanCRDInstances(ctx context.Context, log logr.Logger, crd *apixv1.CustomResourceDefinition, graph *DependencyGraph) error {
	version := preferredVersion(crd)
	if version == "" {
		return nil
	}

	gvk := schema.GroupVersionKind{
		Group:   crd.Spec.Group,
		Version: version,
		Kind:    crd.Spec.Names.Kind + "List",
	}

	list := &unstructured.UnstructuredList{}
	list.SetGroupVersionKind(gvk)

	if err := r.Client.List(ctx, list); err != nil {
		return fmt.Errorf("listing %s: %w", gvk.String(), err)
	}

	for i := range list.Items {
		item := &list.Items[i]
		finalizers := item.GetFinalizers()

		dr := DiscoveredResource{
			Ref: operatorv1.ResourceRef{
				Group:     crd.Spec.Group,
				Kind:      crd.Spec.Names.Kind,
				Namespace: item.GetNamespace(),
				Name:      item.GetName(),
			},
			Version:    version,
			Finalizers: finalizers,
			IsDeleting: item.GetDeletionTimestamp() != nil,
		}

		// Extract phase/status if available
		if phase, ok, _ := unstructured.NestedString(item.Object, "status", "phase"); ok {
			dr.Phase = phase
		}

		// Extract platform for Hypershift HostedClusters and Hive ClusterDeployments
		dr.Platform = detectPlatform(crd.Spec.Names.Kind, item)

		graph.DiscoveredResources = append(graph.DiscoveredResources, dr)

		for _, f := range finalizers {
			if IsCloudProtectingFinalizer(f) {
				graph.CloudRiskResources = append(graph.CloudRiskResources, dr)
				break
			}
		}

		hasNonCloudFinalizer := false
		for _, f := range finalizers {
			if !IsCloudProtectingFinalizer(f) && IsAllowlistedFinalizer(f) {
				hasNonCloudFinalizer = true
				break
			}
		}
		if hasNonCloudFinalizer {
			graph.NonCloudFinalizerResources = append(graph.NonCloudFinalizerResources, dr)
		}
	}

	return nil
}

// preferredVersion returns the served+storage version for a CRD, or the first served version.
func preferredVersion(crd *apixv1.CustomResourceDefinition) string {
	for _, v := range crd.Spec.Versions {
		if v.Served && v.Storage {
			return v.Name
		}
	}
	for _, v := range crd.Spec.Versions {
		if v.Served {
			return v.Name
		}
	}
	return ""
}

// detectPlatform extracts the cloud platform from a HostedCluster or ClusterDeployment.
func detectPlatform(kind string, item *unstructured.Unstructured) string {
	switch kind {
	case "HostedCluster":
		if p, ok, _ := unstructured.NestedString(item.Object, "spec", "platform", "type"); ok {
			return strings.ToUpper(p)
		}
	case "ClusterDeployment":
		spec, ok, _ := unstructured.NestedMap(item.Object, "spec", "platform")
		if ok {
			for platform := range spec {
				return strings.ToUpper(platform)
			}
		}
	case "HostedControlPlane", "NodePool":
		if p, ok, _ := unstructured.NestedString(item.Object, "spec", "platform", "type"); ok {
			return strings.ToUpper(p)
		}
	}
	return ""
}

// classifyBlockingResources identifies resources that block MCH webhook ValidateDelete.
func (r *HubTeardownReconciler) classifyBlockingResources(graph *DependencyGraph) {
	blockingKinds := map[string]string{
		"MultiClusterObservability": "observability.open-cluster-management.io",
		"DiscoveryConfig":           "discovery.open-cluster-management.io",
		"AgentServiceConfig":        "agent-install.openshift.io",
	}

	for _, dr := range graph.DiscoveredResources {
		expectedGroup, isBlocking := blockingKinds[dr.Ref.Kind]
		if isBlocking && dr.Ref.Group == expectedGroup {
			graph.BlockingResources = append(graph.BlockingResources, operatorv1.BlockingResource{
				Group:     dr.Ref.Group,
				Kind:      dr.Ref.Kind,
				Name:      dr.Ref.Name,
				Namespace: dr.Ref.Namespace,
				Reason:    fmt.Sprintf("%s is in MCH webhook blockDeletionResources list", dr.Ref.Kind),
			})
		}
	}
}

// classifyManagedClusters finds non-local ManagedClusters that block MCH deletion.
func (r *HubTeardownReconciler) classifyManagedClusters(graph *DependencyGraph) {
	for _, dr := range graph.DiscoveredResources {
		if dr.Ref.Kind == "ManagedCluster" && dr.Ref.Group == "cluster.open-cluster-management.io" {
			if dr.Ref.Name == "local-cluster" {
				continue
			}
			graph.NonLocalManagedClusters = append(graph.NonLocalManagedClusters, dr)
			graph.BlockingResources = append(graph.BlockingResources, operatorv1.BlockingResource{
				Group:  dr.Ref.Group,
				Kind:   dr.Ref.Kind,
				Name:   dr.Ref.Name,
				Reason: "Non-local ManagedCluster blocks MCH webhook (only local-cluster is exempt)",
			})
		}
	}
}

// classifyAddons collects ManagedClusterAddOns for non-local clusters.
func (r *HubTeardownReconciler) classifyAddons(graph *DependencyGraph) {
	for _, dr := range graph.DiscoveredResources {
		if dr.Ref.Kind == "ManagedClusterAddOn" && dr.Ref.Group == "addon.open-cluster-management.io" {
			if dr.Ref.Namespace == "local-cluster" {
				continue
			}
			graph.Addons = append(graph.Addons, dr)
		}
	}
}

// findMCHAndMCE locates the MultiClusterHub and MultiClusterEngine resources.
func (r *HubTeardownReconciler) findMCHAndMCE(graph *DependencyGraph) {
	for i, dr := range graph.DiscoveredResources {
		if dr.Ref.Kind == "MultiClusterHub" && dr.Ref.Group == "operator.open-cluster-management.io" {
			graph.MCH = &graph.DiscoveredResources[i]
		}
		if dr.Ref.Kind == "MultiClusterEngine" && dr.Ref.Group == "multicluster.openshift.io" {
			graph.MCE = &graph.DiscoveredResources[i]
		}
	}
}

// buildDryRunReport generates a structured preview of the teardown plan from the dependency graph.
func (r *HubTeardownReconciler) buildDryRunReport(graph *DependencyGraph) *operatorv1.DryRunReport {
	report := &operatorv1.DryRunReport{
		ScanTime:                metav1.Now(),
		TotalBlockingResources:  len(graph.BlockingResources),
		TotalCloudRiskResources: len(graph.CloudRiskResources),
	}

	var phases []operatorv1.PlannedPhase

	// Phase: GateOLMSubscription
	phases = append(phases, operatorv1.PlannedPhase{
		Phase:             operatorv1.TeardownPhaseGateOLMSubscription,
		Action:            "Add finalizer to ACM OLM Subscription to prevent operator removal during teardown",
		ResourcesAffected: 1,
	})

	// Phase: RemoveBlockingCRs
	blockingDetails := make([]string, 0)
	blockingCount := 0
	for _, br := range graph.BlockingResources {
		if br.Kind != "ManagedCluster" {
			detail := fmt.Sprintf("%s/%s", br.Kind, br.Name)
			if br.Namespace != "" {
				detail = fmt.Sprintf("%s/%s/%s", br.Kind, br.Namespace, br.Name)
			}
			finalizerInfo := r.finalizerInfoForResource(graph, br.Kind, br.Group, br.Name, br.Namespace)
			detail += finalizerInfo
			blockingDetails = append(blockingDetails, detail)
			blockingCount++
		}
	}
	phases = append(phases, operatorv1.PlannedPhase{
		Phase:             operatorv1.TeardownPhaseRemoveBlockingCRs,
		Action:            "Delete webhook-blocking CRs: MultiClusterObservability, DiscoveryConfig, AgentServiceConfig",
		ResourcesAffected: blockingCount,
		Details:           blockingDetails,
	})

	// Phase: DisableAddons
	addonDetails := make([]string, 0)
	for _, addon := range graph.Addons {
		detail := fmt.Sprintf("ManagedClusterAddOn/%s/%s", addon.Ref.Namespace, addon.Ref.Name)
		if len(addon.Finalizers) > 0 {
			detail += fmt.Sprintf(" (finalizers: %s -- Tier 2)", strings.Join(addon.Finalizers, ", "))
		}
		addonDetails = append(addonDetails, detail)
	}
	phases = append(phases, operatorv1.PlannedPhase{
		Phase:             operatorv1.TeardownPhaseDisableAddons,
		Action:            "Delete ManagedClusterAddOns for non-local ManagedClusters",
		ResourcesAffected: len(graph.Addons),
		Details:           addonDetails,
	})

	// Phase: DeleteInfrastructureCRs
	infraDetails := make([]string, 0)
	infraCount := 0
	for _, cr := range graph.CloudRiskResources {
		if cr.Ref.Kind == "HostedCluster" || cr.Ref.Kind == "ClusterDeployment" ||
			cr.Ref.Kind == "InfraEnv" || cr.Ref.Kind == "NodePool" ||
			cr.Ref.Kind == "HostedControlPlane" {
			detail := fmt.Sprintf("%s/%s/%s", cr.Ref.Kind, cr.Ref.Namespace, cr.Ref.Name)
			if cr.Platform != "" {
				detail += fmt.Sprintf(" (platform: %s)", cr.Platform)
			}
			infraDetails = append(infraDetails, detail)
			infraCount++
		}
	}
	phases = append(phases, operatorv1.PlannedPhase{
		Phase:              operatorv1.TeardownPhaseDeleteInfrastructureCRs,
		Action:             "Delete Hive ClusterDeployments, HyperShift HostedClusters, InfraEnvs, NodePools",
		ResourcesAffected:  infraCount,
		CloudRiskResources: infraCount,
		Details:            infraDetails,
	})

	// Phase: DetachManagedClusters
	mcDetails := make([]string, 0)
	for _, mc := range graph.NonLocalManagedClusters {
		detail := fmt.Sprintf("ManagedCluster/%s", mc.Ref.Name)
		if len(mc.Finalizers) > 0 {
			detail += fmt.Sprintf(" (%d finalizers -- Tier 2)", len(mc.Finalizers))
		}
		mcDetails = append(mcDetails, detail)
	}
	phases = append(phases, operatorv1.PlannedPhase{
		Phase:             operatorv1.TeardownPhaseDetachManagedClusters,
		Action:            "Delete ManagedClusters other than local-cluster",
		ResourcesAffected: len(graph.NonLocalManagedClusters),
		Details:           mcDetails,
	})

	// Phase: DeleteMCH
	mchDetail := "MultiClusterHub not found"
	if graph.MCH != nil {
		mchDetail = fmt.Sprintf("MultiClusterHub/%s/%s", graph.MCH.Ref.Namespace, graph.MCH.Ref.Name)
	}
	phases = append(phases, operatorv1.PlannedPhase{
		Phase:             operatorv1.TeardownPhaseDeleteMCH,
		Action:            "Delete MultiClusterHub CR (webhook should pass after prior phases)",
		ResourcesAffected: 1,
		Details:           []string{mchDetail},
	})

	// Phase: MonitorMCEChain
	mceDetails := []string{}
	if graph.MCE != nil {
		mceDetails = append(mceDetails, fmt.Sprintf("MultiClusterEngine/%s (finalizers: %s)",
			graph.MCE.Ref.Name, strings.Join(graph.MCE.Finalizers, ", ")))
	}
	phases = append(phases, operatorv1.PlannedPhase{
		Phase:             operatorv1.TeardownPhaseMonitorMCEChain,
		Action:            "Track MCE -> ClusterManager -> Hypershift operator removal chain",
		ResourcesAffected: len(mceDetails),
		Details:           mceDetails,
	})

	// Phase: CleanOrphans
	orphanDetails := make([]string, 0)
	cloudRiskCount := 0
	for _, cr := range graph.CloudRiskResources {
		detail := fmt.Sprintf("%s/%s/%s POTENTIAL ORPHAN", cr.Ref.Kind, cr.Ref.Namespace, cr.Ref.Name)
		hasCloudFinalizer := false
		for _, f := range cr.Finalizers {
			if IsCloudProtectingFinalizer(f) {
				detail += fmt.Sprintf(" -- finalizer %s (Tier 1 CLOUD RISK", f)
				if cr.Platform != "" {
					detail += fmt.Sprintf(": %s", cr.Platform)
				}
				detail += ")"
				hasCloudFinalizer = true
			}
		}
		if hasCloudFinalizer {
			cloudRiskCount++
		}
		orphanDetails = append(orphanDetails, detail)
	}
	phases = append(phases, operatorv1.PlannedPhase{
		Phase:              operatorv1.TeardownPhaseCleanOrphans,
		Action:             "Detect and resolve stuck-terminating resources after MCE chain completes",
		ResourcesAffected:  len(graph.CloudRiskResources) + len(graph.NonCloudFinalizerResources),
		CloudRiskResources: cloudRiskCount,
		Details:            orphanDetails,
	})

	// Phase: DeleteACMCRDs
	crdCount := 0
	for _, dr := range graph.DiscoveredResources {
		if dr.Ref.Kind != "HubTeardown" {
			crdCount++
		}
	}
	phases = append(phases, operatorv1.PlannedPhase{
		Phase:             operatorv1.TeardownPhaseDeleteACMCRDs,
		Action:            "Remove all ACM/MCE CRDs (HubTeardown CRD is preserved)",
		ResourcesAffected: crdCount,
	})

	// Phase: RemoveOLMOperator
	phases = append(phases, operatorv1.PlannedPhase{
		Phase:             operatorv1.TeardownPhaseRemoveOLMOperator,
		Action:            "Delete ACM and MCE Subscriptions and CSVs, self-destruct",
		ResourcesAffected: 2,
		Details:           []string{"ACM Subscription + CSV", "MCE Subscription + CSV"},
	})

	// Phase: Complete
	phases = append(phases, operatorv1.PlannedPhase{
		Phase:             operatorv1.TeardownPhaseComplete,
		Action:            "Release OLM Subscription gate finalizer, emit teardown summary",
		ResourcesAffected: 1,
	})

	report.PlannedPhases = phases

	totalResources := 0
	for _, p := range phases {
		totalResources += p.ResourcesAffected
	}
	report.Summary = fmt.Sprintf(
		"Found %d blocking resources across the RHACM ecosystem. %d resources have cloud-protecting finalizers (Tier 1). %d resources have non-cloud finalizers (Tier 2). Estimated teardown phases: %d.",
		report.TotalBlockingResources, report.TotalCloudRiskResources,
		len(graph.NonCloudFinalizerResources), len(phases))

	return report
}

// finalizerInfoForResource returns a formatted string of finalizer info for a resource in the graph.
func (r *HubTeardownReconciler) finalizerInfoForResource(graph *DependencyGraph, kind, group, name, namespace string) string {
	for _, dr := range graph.DiscoveredResources {
		if dr.Ref.Kind == kind && dr.Ref.Group == group && dr.Ref.Name == name && dr.Ref.Namespace == namespace {
			if len(dr.Finalizers) > 0 {
				tier := "Tier 2"
				for _, f := range dr.Finalizers {
					if IsCloudProtectingFinalizer(f) {
						tier = "Tier 1 CLOUD RISK"
						break
					}
				}
				return fmt.Sprintf(" (finalizers: %s -- %s)", strings.Join(dr.Finalizers, ", "), tier)
			}
			return " (no finalizers, clean delete)"
		}
	}
	return ""
}
