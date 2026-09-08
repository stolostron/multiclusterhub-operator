// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"
	"strings"
	"time"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
)

/*
legacyManagedResources lists resources that were removed from a component's Helm chart before
per-component resource tracking (InternalHubComponentSpec.ManagedResources) existed. Because
InternalHubComponent CRs created by older operator versions have no recorded resource history,
the generic drift-detection in cleanupOrphanedManagedResources cannot infer that these resources
should be deleted on the first reconcile after upgrading to an operator version that includes this
list. Entries here are checked unconditionally (in addition to the generic tracked-resource diff)
so that already-orphaned resources from prior releases are still cleaned up.

This is a temporary bridge: once a component has reconciled at least once with resource tracking
enabled, InternalHubComponent.Spec.ManagedResources will accurately reflect its previously deployed
resources, and future drift will be caught generically. Entries below can be removed once all
supported upgrade paths have passed through a release that populates ManagedResources.

See ACM-40355: the console component's legacy ServiceMonitor ("console-monitor") was removed from
the chart in #3832, but upgrades that keep console enabled never rendered/deleted it, leaving
customers with stale ServiceMonitors and TargetDown alerts.
*/
var legacyManagedResources = map[string][]operatorv1.ManagedResource{
	operatorv1.Console: {
		{
			APIVersion: "monitoring.coreos.com/v1",
			Kind:       "ServiceMonitor",
			Name:       "console-monitor",
			// Namespace is set to the MultiClusterHub namespace at cleanup time, since the
			// legacy resource was always deployed alongside MCH (not the operator's own
			// namespace, in cases where those differ).
		},
	},
}

// managedResourceKey returns a string uniquely identifying a ManagedResource, including its
// APIVersion, for exact-match comparisons (see managedResourcesEqual).
func managedResourceKey(resource operatorv1.ManagedResource) string {
	return fmt.Sprintf("%s/%s/%s/%s", resource.APIVersion, resource.Kind, resource.Namespace, resource.Name)
}

// apiGroupFromAPIVersion extracts the API group from an apiVersion string (e.g. "apps/v1" ->
// "apps", "v1" -> "" for the core group), ignoring the version component.
func apiGroupFromAPIVersion(apiVersion string) string {
	if idx := strings.Index(apiVersion, "/"); idx != -1 {
		return apiVersion[:idx]
	}
	return ""
}

// managedResourceIdentityKey returns a version-independent identity for a ManagedResource (API
// group + kind + namespace + name). Kubernetes objects are identified by group/kind/namespace/name;
// the API version is just an alternate representation of the same underlying object. Using the
// full APIVersion-sensitive key here would cause a pure version bump in a chart (e.g. a CRD moving
// from v1beta1 to v1) to be misdetected as the resource being removed, triggering an unnecessary
// delete-then-recreate cycle in cleanupOrphanedManagedResources instead of an in-place update.
func managedResourceIdentityKey(resource operatorv1.ManagedResource) string {
	return fmt.Sprintf("%s/%s/%s/%s", apiGroupFromAPIVersion(resource.APIVersion), resource.Kind,
		resource.Namespace, resource.Name)
}

// extractManagedResources builds the list of resources represented by the given rendered
// templates. NetworkPolicy resources are intentionally excluded because they are managed
// separately by ensureNetworkPolicies using a create-once pattern.
func extractManagedResources(templates []*unstructured.Unstructured) []operatorv1.ManagedResource {
	resources := make([]operatorv1.ManagedResource, 0, len(templates))
	for _, template := range templates {
		if template.GetKind() == "NetworkPolicy" {
			continue
		}

		resources = append(resources, operatorv1.ManagedResource{
			APIVersion: template.GetAPIVersion(),
			Kind:       template.GetKind(),
			Name:       template.GetName(),
			Namespace:  template.GetNamespace(),
		})
	}
	return resources
}

// managedResourcesEqual reports whether two ManagedResource lists represent the same set of
// resources, regardless of order.
func managedResourcesEqual(a, b []operatorv1.ManagedResource) bool {
	if len(a) != len(b) {
		return false
	}

	seen := make(map[string]struct{}, len(a))
	for _, resource := range a {
		seen[managedResourceKey(resource)] = struct{}{}
	}

	for _, resource := range b {
		if _, ok := seen[managedResourceKey(resource)]; !ok {
			return false
		}
	}
	return true
}

// getManagedResources returns the resources currently recorded on the component's
// InternalHubComponent CR. It returns (nil, nil) if the CR does not exist, since callers treat "no
// tracked resources" as an empty diff baseline rather than a failure. Any other error is returned
// to the caller rather than swallowed, since silently treating a transient read failure as "no
// history" could cause cleanupOrphanedManagedResources to miss real orphans, or worse, lose the
// tracked history permanently if the caller goes on to delete the InternalHubComponent CR.
func (r *MultiClusterHubReconciler) getManagedResources(ctx context.Context, m *operatorv1.MultiClusterHub,
	component string) ([]operatorv1.ManagedResource, error) {

	ihc := &operatorv1.InternalHubComponent{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: component, Namespace: m.GetNamespace()}, ihc); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get InternalHubComponent %s/%s: %v", m.GetNamespace(), component, err)
	}

	return ihc.Spec.ManagedResources, nil
}

// updateManagedResources patches the component's InternalHubComponent CR with the current list of
// managed resources, but only if the list has actually changed, to avoid unnecessary writes on
// every reconcile.
func (r *MultiClusterHubReconciler) updateManagedResources(ctx context.Context, m *operatorv1.MultiClusterHub,
	component string, resources []operatorv1.ManagedResource) error {

	ihc := &operatorv1.InternalHubComponent{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: component, Namespace: m.GetNamespace()}, ihc); err != nil {
		if errors.IsNotFound(err) {
			// The InternalHubComponent CR is created by ensureInternalHubComponent; if it's
			// missing here there's nothing to update.
			return nil
		}
		return fmt.Errorf("failed to get InternalHubComponent %s/%s: %v", m.GetNamespace(), component, err)
	}

	if managedResourcesEqual(ihc.Spec.ManagedResources, resources) {
		return nil
	}

	ihc.Spec.ManagedResources = resources
	if err := r.Client.Update(ctx, ihc); err != nil {
		return fmt.Errorf("failed to update InternalHubComponent %s/%s managed resources: %v",
			m.GetNamespace(), component, err)
	}

	return nil
}

// cleanupOrphanedManagedResources deletes resources that are present in oldResources but absent
// from newResources - i.e. resources that were deployed by a previous version of the component's
// templates but are no longer rendered. Deletion is delegated to deleteTemplate, which only
// removes resources that still carry this operator's installer ownership labels, so resources
// that were manually recreated (and therefore lack those labels) are left untouched.
//
// This function is best-effort and does not stop at the first candidate that fails or needs a
// requeue: on the disable path, ensureNoComponent must delete the InternalHubComponent tracking CR
// promptly (other controllers watch for its removal as a signal), so this reconcile is the only
// chance to use the resource history captured before that CR is gone. Stopping early would leave
// every remaining candidate un-attempted, and a future reconcile would have no history left to
// retry them with. Attempting every candidate in one pass instead means only a genuinely
// finalizer-blocked resource is left for the caller to report/requeue on; unrelated candidates are
// still cleaned up.
func (r *MultiClusterHubReconciler) cleanupOrphanedManagedResources(ctx context.Context, m *operatorv1.MultiClusterHub,
	component string, oldResources, newResources []operatorv1.ManagedResource) (ctrl.Result, error) {

	current := make(map[string]struct{}, len(newResources))
	for _, resource := range newResources {
		current[managedResourceIdentityKey(resource)] = struct{}{}
	}

	// Merge in any known legacy resources for this component that predate resource tracking (see
	// legacyManagedResources doc comment). Namespace defaults to the MultiClusterHub namespace
	// when unset, matching how these resources were originally deployed.
	orphanCandidates := append([]operatorv1.ManagedResource{}, oldResources...)
	for _, legacy := range legacyManagedResources[component] {
		if legacy.Namespace == "" {
			legacy.Namespace = m.GetNamespace()
		}
		orphanCandidates = append(orphanCandidates, legacy)
	}

	var (
		firstErr     error
		needsRequeue bool
		requeueAfter time.Duration
	)

	seenCandidates := make(map[string]struct{}, len(orphanCandidates))
	for _, resource := range orphanCandidates {
		key := managedResourceIdentityKey(resource)
		if _, alreadyHandled := seenCandidates[key]; alreadyHandled {
			continue
		}
		seenCandidates[key] = struct{}{}

		if _, stillRendered := current[key]; stillRendered {
			continue
		}

		stub := &unstructured.Unstructured{}
		stub.SetAPIVersion(resource.APIVersion)
		stub.SetKind(resource.Kind)
		stub.SetName(resource.Name)
		stub.SetNamespace(resource.Namespace)

		log.Info("Cleaning up resource no longer present in component templates",
			"Component", component, "APIVersion", resource.APIVersion, "Kind", resource.Kind,
			"Name", resource.Name, "Namespace", resource.Namespace)

		result, err := r.deleteTemplate(ctx, m, stub)
		if err != nil {
			log.Error(err, "failed to clean up orphaned managed resource; continuing with remaining resources",
				"Component", component, "Kind", resource.Kind, "Name", resource.Name, "Namespace", resource.Namespace)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		if result != (ctrl.Result{}) {
			needsRequeue = true
			if result.RequeueAfter > requeueAfter {
				requeueAfter = result.RequeueAfter
			}
		}
	}

	if firstErr != nil {
		return ctrl.Result{}, firstErr
	}
	if needsRequeue {
		return ctrl.Result{RequeueAfter: requeueAfter}, nil
	}
	return ctrl.Result{}, nil
}
