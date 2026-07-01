// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"
	"fmt"

	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	renderer "github.com/stolostron/multiclusterhub-operator/pkg/rendering"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ensureNetworkPolicies relies on existing installer labels to track NetworkPolicy ownership:
//
// Labels (applied by utils.AddInstallerLabel):
//   - installer.name = "multiclusterhub"
//   - installer.namespace = "open-cluster-management"
//
// Annotations (applied by rendering):
//   - installer.open-cluster-management.io/release-version = "2.15.0"
//
// These are sufficient to identify MCH-created NetworkPolicies for deletion when disabled.

// ensureNetworkPolicies implements the create-once NetworkPolicy pattern:
// - component enabled + networkPolicies enabled → CREATE (if missing), SKIP (if exists)
// - component disabled OR networkPolicies disabled → DELETE (if MCH-created)
func (r *MultiClusterHubReconciler) ensureNetworkPolicies(ctx context.Context, mch *operatorv1.MultiClusterHub,
	cacheSpec CacheSpec, isSTSEnabled bool) (ctrl.Result, error) {
	log := r.Log.WithValues("MultiClusterHub", mch.Name, "Namespace", mch.Namespace)

	// Default enabled to true if not explicitly set
	networkPoliciesEnabled := true
	if mch.Spec.NetworkPolicies != nil {
		networkPoliciesEnabled = mch.Spec.NetworkPolicies.Enabled
	}

	// If globally disabled, delete all MCH-created NetworkPolicies
	if !networkPoliciesEnabled {
		npList := &networkingv1.NetworkPolicyList{}
		if err := r.Client.List(ctx, npList, client.InNamespace(mch.Namespace), client.MatchingLabels{
			"installer.name":      mch.Name,
			"installer.namespace": mch.Namespace,
		}); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to list NetworkPolicies: %w", err)
		}

		for _, np := range npList.Items {
			if err := r.Client.Delete(ctx, &np); err != nil && !errors.IsNotFound(err) {
				return ctrl.Result{}, fmt.Errorf("failed to delete NetworkPolicy %s/%s: %w", np.Namespace, np.Name, err)
			}
			log.Info("Deleted NetworkPolicy", "name", np.Name, "namespace", np.Namespace)
		}
		return ctrl.Result{}, nil
	}

	// NetworkPolicies enabled - check each component
	for _, component := range operatorv1.MCHComponents {
		// Skip MCH and MCE components - they don't have NetworkPolicy templates
		if component == operatorv1.MCH || component == operatorv1.MultiClusterEngine {
			continue
		}

		componentEnabled := mch.Enabled(component)
		if !componentEnabled {
			// Component disabled - nothing to create or delete (rely on global deletion above if needed)
			continue
		}

		// Render NetworkPolicy from Helm template
		chartLocation := r.fetchChartLocation(component)
		templates, errs := renderer.RenderChart(chartLocation, mch, cacheSpec.ImageOverrides, cacheSpec.TemplateOverrides,
			isSTSEnabled, r.OLMVersion)

		if len(errs) > 0 {
			// Rendering errors are non-fatal - component may not have NetworkPolicy template yet
			log.V(2).Info("Chart rendering had errors", "component", component, "errors", len(errs))
			continue
		}

		// Filter for NetworkPolicy resources only
		var networkPolicies []*unstructured.Unstructured
		for _, template := range templates {
			if template.GetKind() == "NetworkPolicy" {
				networkPolicies = append(networkPolicies, template)
			}
		}

		// No NetworkPolicy template for this component - skip
		if len(networkPolicies) == 0 {
			continue
		}

		// Handle each NetworkPolicy (usually just one per component)
		for _, npTemplate := range networkPolicies {
			np := &networkingv1.NetworkPolicy{}
			err := r.Client.Get(ctx, types.NamespacedName{
				Name:      npTemplate.GetName(),
				Namespace: npTemplate.GetNamespace(),
			}, np)

			if errors.IsNotFound(err) {
				// Create NetworkPolicy - create-once pattern
				if err := r.Client.Create(ctx, npTemplate); err != nil {
					return ctrl.Result{}, fmt.Errorf("failed to create NetworkPolicy %s/%s: %w", npTemplate.GetNamespace(), npTemplate.GetName(), err)
				}
				log.Info("Created NetworkPolicy", "name", npTemplate.GetName(), "namespace", npTemplate.GetNamespace(), "component", component)
			} else if err == nil {
				// NetworkPolicy exists - SKIP (no reconcile, operand owns it now)
				log.V(2).Info("NetworkPolicy exists, skipping", "name", np.Name, "namespace", np.Namespace, "component", component)
			} else {
				return ctrl.Result{}, fmt.Errorf("failed to get NetworkPolicy %s: %w", npTemplate.GetName(), err)
			}
		}
	}

	return ctrl.Result{}, nil
}
