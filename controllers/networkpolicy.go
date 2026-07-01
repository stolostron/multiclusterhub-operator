// Copyright Contributors to the Open Cluster Management project

package controllers

import (
	"context"

	"github.com/go-logr/logr"
	operatorv1 "github.com/stolostron/multiclusterhub-operator/api/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/yaml"
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
// - enabled=true + missing → CREATE with delegation annotations
// - enabled=true + exists → SKIP (no reconcile)
// - enabled=false + exists → DELETE if MCH-created
func (r *MultiClusterHubReconciler) ensureNetworkPolicies(ctx context.Context, mch *operatorv1.MultiClusterHub, cacheSpec CacheSpec) (ctrl.Result, error) {
	log := r.Log.WithValues("MultiClusterHub", mch.Name, "Namespace", mch.Namespace)

	// Default enabled to true if not explicitly set
	networkPoliciesEnabled := true
	if mch.Spec.NetworkPolicies != nil {
		networkPoliciesEnabled = mch.Spec.NetworkPolicies.Enabled
	}

	// Get all components that should have NetworkPolicies
	components := r.getNetworkPolicyComponents(mch)

	for _, component := range components {
		if err := r.reconcileComponentNetworkPolicy(ctx, mch, component, networkPoliciesEnabled, cacheSpec, log); err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// reconcileComponentNetworkPolicy handles NetworkPolicy for a single component
func (r *MultiClusterHubReconciler) reconcileComponentNetworkPolicy(
	ctx context.Context,
	mch *operatorv1.MultiClusterHub,
	component string,
	enabled bool,
	cacheSpec CacheSpec,
	log logr.Logger,
) error {
	// Render NetworkPolicy from Helm templates for this component
	// Template rendering will be implemented in subsequent PRs
	// This stub allows build to succeed and establishes the reconciliation pattern
	log.V(1).Info("NetworkPolicy reconciliation", "component", component, "enabled", enabled)

	// TODO: Implement full reconciliation logic:
	// 1. Render NetworkPolicy from Helm template using cacheSpec
	// 2. Check if NetworkPolicy exists
	// 3. If !enabled && exists && MCH-created: DELETE
	// 4. If enabled && !exists: CREATE with delegation annotations
	// 5. If enabled && exists: SKIP (no reconcile)

	return nil
}

// getNetworkPolicyComponents returns list of components that should have NetworkPolicies
func (r *MultiClusterHubReconciler) getNetworkPolicyComponents(mch *operatorv1.MultiClusterHub) []string {
	var components []string

	// Include all enabled components
	for _, c := range operatorv1.MCHComponents {
		if mch.Enabled(c) {
			components = append(components, c)
		}
	}

	return components
}

// convertUnstructuredToNetworkPolicy converts unstructured resource to NetworkPolicy
func convertUnstructuredToNetworkPolicy(u *unstructured.Unstructured) (*networkingv1.NetworkPolicy, error) {
	data, err := yaml.Marshal(u)
	if err != nil {
		return nil, err
	}

	np := &networkingv1.NetworkPolicy{}
	err = yaml.Unmarshal(data, np)
	if err != nil {
		return nil, err
	}

	return np, nil
}
