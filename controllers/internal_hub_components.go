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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

var clusterScopedKinds = map[string]bool{
	"ClusterRole":                       true,
	"ClusterRoleBinding":                true,
	"CustomResourceDefinition":          true,
	"APIService":                        true,
	"ValidatingWebhookConfiguration":    true,
	"MutatingWebhookConfiguration":      true,
	"ConsolePlugin":                     true,
	"ConsoleCLIDownload":                true,
	"ClusterManagementAddOn":            true,
	"ClusterExtension":                  true,
}

func (r *MultiClusterHubReconciler) ensureInternalHubComponent(ctx context.Context, m *operatorv1.MultiClusterHub,
	component string) (ctrl.Result, error) {

	ihc := &operatorv1.InternalHubComponent{
		TypeMeta: metav1.TypeMeta{
			APIVersion: operatorv1.GroupVersion.String(),
			Kind:       "InternalHubComponent",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      component,
			Namespace: m.GetNamespace(),
		},
	}

	if err := r.Client.Get(
		ctx, types.NamespacedName{Name: ihc.GetName(), Namespace: ihc.GetNamespace()}, ihc); err != nil {

		if errors.IsNotFound(err) {
			if err := r.Client.Create(ctx, ihc); err != nil {
				return ctrl.Result{}, fmt.Errorf("failed to create InternalHubComponent CR: %s/%s: %v",
					ihc.GetNamespace(), ihc.GetName(), err)
			}
		} else {
			return ctrl.Result{}, fmt.Errorf("failed to get InternalHubComponent CR: %s/%s: %v",
				ihc.GetNamespace(), ihc.GetName(), err)
		}
	}

	return ctrl.Result{}, nil
}

func (r *MultiClusterHubReconciler) ensureNoInternalHubComponent(ctx context.Context, m *operatorv1.MultiClusterHub,
	component string) (ctrl.Result, error) {

	ihc := &operatorv1.InternalHubComponent{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: component, Namespace: m.GetNamespace()}, ihc); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get InternalHubComponent: %s/%s: %v",
			m.GetNamespace(), component, err)
	}

	// Check if it has a deletion timestamp (indicating it's in the process of being deleted)
	if ihc.GetDeletionTimestamp() != nil {
		log.Info("InternalHubComponent deletion in progress", "Name", ihc.GetName(), "Namespace", ihc.GetNamespace(),
			"DeletionTimestamp", ihc.GetDeletionTimestamp())

		return ctrl.Result{RequeueAfter: resyncPeriod}, nil
	}

	log.Info("Deleting InternalHubComponent", "Name", ihc.GetName(), "Namespace", ihc.GetNamespace())
	if err := r.Client.Delete(ctx, ihc); err != nil {
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, fmt.Errorf("failed to delete InternalHubComponent CR: %s/%s: %v",
				ihc.GetNamespace(), ihc.GetName(), err)
		}
	}

	// Ensure that the resource is fully deleted by attempting to refetch it
	if err := r.Client.Get(ctx,
		types.NamespacedName{Name: ihc.GetName(), Namespace: ihc.GetNamespace()}, ihc); err != nil {
		if errors.IsNotFound(err) {
			logf.Log.Info("InternalHubComponent successfully deleted", "Name", ihc.GetName(), "Namespace", ihc.GetNamespace())
			return ctrl.Result{}, nil
		}

		return ctrl.Result{}, fmt.Errorf("failed to get InternalHubComponent %s/%s: %v",
			ihc.GetNamespace(), ihc.GetName(), err)
	}

	// Requeue to check again after a short delay
	return ctrl.Result{RequeueAfter: resyncPeriod}, nil
}

func buildManagedResources(templates []*unstructured.Unstructured) []operatorv1.ManagedResource {
	resources := make([]operatorv1.ManagedResource, 0, len(templates))
	for _, t := range templates {
		scope := operatorv1.NamespaceScoped
		if clusterScopedKinds[t.GetKind()] {
			scope = operatorv1.ClusterScoped
		}
		gvk := t.GroupVersionKind()
		resources = append(resources, operatorv1.ManagedResource{
			Group:     gvk.Group,
			Version:   gvk.Version,
			Kind:      gvk.Kind,
			Name:      t.GetName(),
			Namespace: t.GetNamespace(),
			Scope:     scope,
		})
	}
	return resources
}

type managedResourceKey struct {
	Group     string
	Kind      string
	Name      string
	Namespace string
}

func keyFromManagedResource(r operatorv1.ManagedResource) managedResourceKey {
	return managedResourceKey{
		Group:     r.Group,
		Kind:      r.Kind,
		Name:      r.Name,
		Namespace: r.Namespace,
	}
}

func findOrphanedResources(old, current []operatorv1.ManagedResource) []operatorv1.ManagedResource {
	currentSet := make(map[managedResourceKey]bool, len(current))
	for _, r := range current {
		currentSet[keyFromManagedResource(r)] = true
	}

	var orphans []operatorv1.ManagedResource
	for _, r := range old {
		if !currentSet[keyFromManagedResource(r)] {
			orphans = append(orphans, r)
		}
	}
	return orphans
}

func (r *MultiClusterHubReconciler) getManagedResources(ctx context.Context, m *operatorv1.MultiClusterHub,
	component string) ([]operatorv1.ManagedResource, error) {

	ihc := &operatorv1.InternalHubComponent{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: component, Namespace: m.GetNamespace()}, ihc); err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get InternalHubComponent: %s/%s: %v",
			m.GetNamespace(), component, err)
	}
	return ihc.Spec.ManagedResources, nil
}

func (r *MultiClusterHubReconciler) updateManagedResources(ctx context.Context, m *operatorv1.MultiClusterHub,
	component string, resources []operatorv1.ManagedResource) error {

	ihc := &operatorv1.InternalHubComponent{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: component, Namespace: m.GetNamespace()}, ihc); err != nil {
		return fmt.Errorf("failed to get InternalHubComponent for update: %s/%s: %v",
			m.GetNamespace(), component, err)
	}

	ihc.Spec.ManagedResources = resources
	if err := r.Client.Update(ctx, ihc); err != nil {
		return fmt.Errorf("failed to update InternalHubComponent managed resources: %s/%s: %v",
			m.GetNamespace(), component, err)
	}
	return nil
}

func (r *MultiClusterHubReconciler) pruneOrphanedResources(ctx context.Context, m *operatorv1.MultiClusterHub,
	orphans []operatorv1.ManagedResource) error {

	for _, orphan := range orphans {
		resource := &unstructured.Unstructured{}
		resource.SetGroupVersionKind(orphan.GroupVersionKind())
		resource.SetName(orphan.Name)
		if orphan.Scope == operatorv1.NamespaceScoped {
			resource.SetNamespace(orphan.Namespace)
		}

		err := r.Client.Get(ctx, types.NamespacedName{Name: orphan.Name, Namespace: orphan.Namespace}, resource)
		if errors.IsNotFound(err) {
			log.Info("Orphaned resource already removed", "Kind", orphan.Kind, "Name", orphan.Name)
			continue
		}
		if err != nil {
			log.Error(err, "Failed to get orphaned resource", "Kind", orphan.Kind, "Name", orphan.Name)
			continue
		}

		if !r.ensureResourceOwnership(resource, resource, m) {
			log.Info("Skipping orphan deletion, resource not managed by this operator",
				"Kind", orphan.Kind, "Name", orphan.Name)
			continue
		}

		if err := r.Client.Delete(ctx, resource); err != nil && !errors.IsNotFound(err) {
			return fmt.Errorf("failed to delete orphaned resource %s/%s: %v", orphan.Kind, orphan.Name, err)
		}
		log.Info("Deleted orphaned resource", "Kind", orphan.Kind, "Name", orphan.Name,
			"Namespace", orphan.Namespace)
	}
	return nil
}
