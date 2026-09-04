// Copyright Contributors to the Open Cluster Management project

/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"

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

// managedResourceKey returns a string uniquely identifying a ManagedResource for set comparisons.
func managedResourceKey(resource operatorv1.ManagedResource) string {
	return fmt.Sprintf("%s/%s/%s/%s", resource.APIVersion, resource.Kind, resource.Namespace, resource.Name)
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
// InternalHubComponent CR. It returns nil (without error) if the CR does not exist, since callers
// treat "no tracked resources" as an empty diff baseline rather than a failure.
func (r *MultiClusterHubReconciler) getManagedResources(ctx context.Context, m *operatorv1.MultiClusterHub,
	component string) []operatorv1.ManagedResource {

	ihc := &operatorv1.InternalHubComponent{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: component, Namespace: m.GetNamespace()}, ihc); err != nil {
		if !errors.IsNotFound(err) {
			log.Error(err, "failed to get InternalHubComponent while reading managed resources",
				"Component", component, "Namespace", m.GetNamespace())
		}
		return nil
	}

	return ihc.Spec.ManagedResources
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
func (r *MultiClusterHubReconciler) cleanupOrphanedManagedResources(ctx context.Context, m *operatorv1.MultiClusterHub,
	component string, oldResources, newResources []operatorv1.ManagedResource) (ctrl.Result, error) {

	current := make(map[string]struct{}, len(newResources))
	for _, resource := range newResources {
		current[managedResourceKey(resource)] = struct{}{}
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

	seenCandidates := make(map[string]struct{}, len(orphanCandidates))
	for _, resource := range orphanCandidates {
		key := managedResourceKey(resource)
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

		if result, err := r.deleteTemplate(ctx, m, stub); result != (ctrl.Result{}) || err != nil {
			return result, err
		}
	}

	return ctrl.Result{}, nil
}
