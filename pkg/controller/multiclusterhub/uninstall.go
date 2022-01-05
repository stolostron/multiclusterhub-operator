// Copyright (c) 2020 Red Hat, Inc.
// Copyright Contributors to the Open Cluster Management project

package multiclusterhub

import (
	"context"
	"fmt"

	operatorsv1 "github.com/stolostron/multiclusterhub-operator/pkg/apis/operator/v1"
	"github.com/stolostron/multiclusterhub-operator/pkg/utils"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	// The uninstallList is the list of all resources from previous installs to remove. Items can be removed
	// from this list in future releases if they are sure to not exist prior to the current installer version
	uninstallList = func(m *operatorsv1.MultiClusterHub) []*unstructured.Unstructured {
		removals := []*unstructured.Unstructured{
			// topology-sub removed in 2.3.0
			newUnstructured(
				types.NamespacedName{Name: "topology-sub", Namespace: m.Namespace},
				schema.GroupVersionKind{Group: "apps.open-cluster-management.io", Kind: "Subscription", Version: "v1"},
			),
			newUnstructured(
				types.NamespacedName{Name: "rcm-sub", Namespace: m.Namespace},
				schema.GroupVersionKind{Group: "apps.open-cluster-management.io", Kind: "Subscription", Version: "v1"},
			),
			// searchservices CRD replaced in 2.2.0
			newUnstructured(
				types.NamespacedName{Name: "searchservices.search.acm.com"},
				schema.GroupVersionKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition", Version: "v1"},
			),
			// mirroredmanagedclusters CRD removed in 2.3.0
			newUnstructured(
				types.NamespacedName{Name: "mirroredmanagedclusters.cluster.open-cluster-management.io"},
				schema.GroupVersionKind{Group: "apiextensions.k8s.io", Kind: "CustomResourceDefinition", Version: "v1"},
			),
			// cert-manager removed in 2.3.0
			newUnstructured(
				types.NamespacedName{Name: "cert-manager-sub", Namespace: utils.CertManagerNS(m)},
				schema.GroupVersionKind{Group: "apps.open-cluster-management.io", Kind: "Subscription", Version: "v1"},
			),
			newUnstructured(
				types.NamespacedName{Name: "cert-manager-webhook-sub", Namespace: utils.CertManagerNS(m)},
				schema.GroupVersionKind{Group: "apps.open-cluster-management.io", Kind: "Subscription", Version: "v1"},
			),
			newUnstructured(
				types.NamespacedName{Name: "configmap-watcher-sub", Namespace: utils.CertManagerNS(m)},
				schema.GroupVersionKind{Group: "apps.open-cluster-management.io", Kind: "Subscription", Version: "v1"},
			),
		}

		if m.Spec.SeparateCertificateManagement && m.Spec.ImagePullSecret != "" {
			removals = append(removals, newUnstructured(
				types.NamespacedName{Name: m.Spec.ImagePullSecret, Namespace: utils.CertManagerNamespace},
				schema.GroupVersionKind{Group: "", Kind: "Secret", Version: "v1"},
			))
		}
		return removals
	}
)

func newUnstructured(nn types.NamespacedName, gvk schema.GroupVersionKind) *unstructured.Unstructured {
	u := unstructured.Unstructured{}
	u.SetGroupVersionKind(gvk)
	u.SetName(nn.Name)
	u.SetNamespace((nn.Namespace))
	return &u
}

// ensureRemovalsGone validates successful removal of everything in the uninstallList. Return on first error encounter.
func (r *ReconcileMultiClusterHub) ensureRemovalsGone(m *operatorsv1.MultiClusterHub) (*reconcile.Result, error) {
	removals := uninstallList(m)
	allResourcesDeleted := true
	for i := range removals {
		gone, err := r.uninstall(m, removals[i])
		if err != nil {
			return &reconcile.Result{}, err
		}
		if !gone {
			allResourcesDeleted = false
		}
	}

	if !allResourcesDeleted {
		return &reconcile.Result{RequeueAfter: resyncPeriod}, nil
	}

	// Emit hubcondition once pruning complete if other pruning condition present
	progressingCondition := GetHubCondition(m.Status, operatorsv1.Progressing)
	if progressingCondition != nil {
		if progressingCondition.Reason == OldComponentRemovedReason || progressingCondition.Reason == OldComponentNotRemovedReason {
			condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, AllOldComponentsRemovedReason, "All old resources pruned")
			SetHubCondition(&m.Status, *condition)
		}
	}

	return nil, nil
}

// uninstall return true if resource does not exist and returns an error if a GET or DELETE errors unexpectedly. A false response without error
// means the resource is in the process of deleting.
func (r *ReconcileMultiClusterHub) uninstall(m *operatorsv1.MultiClusterHub, u *unstructured.Unstructured) (bool, error) {
	obLog := log.WithValues("Namespace", u.GetNamespace(), "Name", u.GetName(), "Kind", u.GetKind())

	err := r.client.Get(context.TODO(), types.NamespacedName{
		Name:      u.GetName(),
		Namespace: u.GetNamespace(),
	}, u)

	if errors.IsNotFound(err) {
		return true, nil
	}

	// Get resource. Successful if it doesn't exist.
	if err != nil {
		// Error that isn't due to the resource not existing
		obLog.Error(err, "Error getting resource")
		return false, err
	}

	// If resource has deletionTimestamp then re-reconcile and don't try deleting
	if u.GetDeletionTimestamp() != nil {
		condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionFalse, OldComponentNotRemovedReason, fmt.Sprintf("Resource %s/%s finalizing", u.GetKind(), u.GetName()))
		SetHubCondition(&m.Status, *condition)
		obLog.Info("Waiting for resource to finalize")
		return false, nil
	}

	// Attempt deleting resource. No error does not necessarily mean the resource is gone.
	err = r.client.Delete(context.TODO(), u)
	if err != nil {
		condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionFalse, OldComponentNotRemovedReason, fmt.Sprintf("Failed to remove resource %s/%s", u.GetKind(), u.GetName()))
		SetHubCondition(&m.Status, *condition)
		obLog.Error(err, "Failed to delete resource")
		return false, err
	}
	condition := NewHubCondition(operatorsv1.Progressing, metav1.ConditionTrue, OldComponentRemovedReason, "Removed old resource")
	SetHubCondition(&m.Status, *condition)
	obLog.Info("Deleted instance")
	return false, nil
}
